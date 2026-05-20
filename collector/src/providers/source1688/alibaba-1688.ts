import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import { with1688BatchGate } from '../../browser/batch-gate.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { NormalizedProduct } from '../../types/product.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';

import { log1688CollectDebug, save1688FailureSnapshot } from './debug-snapshot.js';
import { assembleParsedProduct, extractBrowserPayload } from './parser.js';
import { prepare1688OfferPage } from './page-prep.js';
import type { Parse1688Result } from './types.js';

function is1688Host(hostname: string): boolean {
  return hostname === '1688.com' || hostname.endsWith('.1688.com');
}

function isLikelyOfferPath(urlStr: string): boolean {
  try {
    const u = new URL(urlStr);
    if (!is1688Host(u.hostname)) return false;
    return (
      /\/offer\/?/i.test(u.pathname) ||
      /offerId=/i.test(u.search) ||
      /offer(?:id)?\.html$/i.test(u.pathname)
    );
  } catch {
    return false;
  }
}

function normalizeOfferNavUrl(raw: string): string {
  try {
    const u = new URL(raw.trim());
    const m = u.pathname.match(/\/offer\/(\d+)\.html/i);
    if (m?.[1]) {
      return `https://detail.1688.com/offer/${m[1]}.html`;
    }
  } catch {
    /* keep raw */
  }
  return raw.trim();
}

function isHardEmptyCollected(r: ReturnType<typeof assembleParsedProduct>): boolean {
  return (
    !r.title?.trim() &&
    r.mainImages.length === 0 &&
    r.descriptionImages.length === 0 &&
    Object.keys(r.attributes).length === 0 &&
    r.skus.length === 0
  );
}

function isBlockedPage(assembled: ReturnType<typeof assembleParsedProduct>): boolean {
  if (assembled.blocked) return true;
  const hint = assembled.raw?.blockedHint;
  return hint === true || hint === 1;
}

function isPlaceholderTitle(title: string): boolean {
  const t = title.trim();
  return t === '' || t === '（解析：未命名商品）' || t.length < 4;
}

function fieldMissingSummary(assembled: Parse1688Result): Record<string, boolean> {
  const priceFromSku = assembled.skus.some((s) => typeof s.price === 'number' && s.price > 0);
  const productPrice = assembled.raw?.productPrice;
  const hasProductPrice = typeof productPrice === 'number' && productPrice > 0;
  return {
    title: isPlaceholderTitle(assembled.title ?? ''),
    price: !priceFromSku && !hasProductPrice,
    images: assembled.mainImages.length === 0,
    sku: assembled.skus.length === 0,
  };
}

function isCaptchaRedirectUrl(href: string): boolean {
  try {
    const u = new URL(href);
    const blob = `${u.hostname}${u.pathname}${u.search}`.toLowerCase();
    if (/punish|x5secdata|captcha|_____tmd_____|sec\.1688\.com\/.*(?:verify|captcha)/i.test(blob)) {
      return true;
    }
    if (/passport\.1688\.com|login\.1688\.com/i.test(u.hostname) && !isLikelyOfferPath(href)) {
      return true;
    }
  } catch {
    return false;
  }
  return false;
}

async function isStrictCaptchaSurface(page: Page): Promise<boolean> {
  try {
    return await page.evaluate(() => {
      const href = typeof location?.href === 'string' ? location.href : '';
      const body = document.body?.innerText?.slice(0, 1600) ?? '';
      const hasProductChrome = !!(
        document.querySelector('h1, h1.d-title, [class*="title"]') &&
        (document.querySelector('.detail-gallery, [class*="gallery"], [class*="offer-img"]') ||
          document.querySelector('[class*="sku"], [class*="obj-sku"]'))
      );
      if (hasProductChrome) return false;
      if (/安全验证|请完成验证|滑块验证|人机验证|访问过于频繁|拖动.*验证/.test(body)) return true;
      if (/验证/.test(body) && body.length < 600) return true;
      if (/请登录|账号登录/.test(body) && body.length < 400) return true;
      return /punish|x5secdata|captcha/i.test(href);
    });
  } catch {
    return false;
  }
}

function throwBlocked(reason: string): never {
  throw new Error(`PAGE_BLOCKED_OR_VERIFY_REQUIRED:${reason}`);
}

async function gotoOfferPage(page: Page, navUrl: string, fallbackUrl: string, timeout: number): Promise<void> {
  try {
    await page.goto(navUrl, { waitUntil: 'domcontentloaded', timeout });
  } catch (firstErr) {
    if (navUrl === fallbackUrl) {
      const err = firstErr instanceof Error ? firstErr.message : String(firstErr);
      if (/timeout/i.test(err)) throw new Error(`TIMEOUT:navigation_${err}`);
      throw new Error(`NAVIGATION_FAILED:${err}`);
    }
    await page.goto(fallbackUrl, { waitUntil: 'domcontentloaded', timeout });
  }
}

type CollectOutcome =
  | { kind: 'success' | 'partial_success'; assembled: Parse1688Result }
  | { kind: 'failed'; code: string; message: string };

function resolveCollectOutcome(
  assembled: Parse1688Result,
  missing: Record<string, boolean>,
  blocked: boolean,
): CollectOutcome {
  if (blocked && isHardEmptyCollected(assembled)) {
    return { kind: 'failed', code: 'blocked', message: 'verification_challenge_or_offer_unreadable' };
  }

  if (isPlaceholderTitle(assembled.title ?? '')) {
    if (blocked) {
      return { kind: 'failed', code: 'blocked', message: 'verification_or_login_page_detected' };
    }
    return { kind: 'failed', code: 'missing_title', message: `missing_title:${JSON.stringify(missing)}` };
  }

  const hasTitle = !missing.title;
  const hasImages = !missing.images;
  const hasSku = !missing.sku;
  const hasPrice = !missing.price;
  const successCount = [hasTitle, hasImages, hasSku, hasPrice].filter(Boolean).length;

  if (hasTitle && !hasImages && (hasSku || assembled.descriptionImages.length > 0)) {
    return { kind: 'partial_success', assembled };
  }

  if (hasTitle && successCount >= 2 && (!hasImages || !hasPrice)) {
    return { kind: 'partial_success', assembled };
  }

  if (!hasImages) {
    if (blocked) {
      return { kind: 'failed', code: 'blocked', message: 'offer_partial_no_images_likely_blocked' };
    }
    if (hasTitle && hasSku) {
      return { kind: 'partial_success', assembled };
    }
    return { kind: 'failed', code: 'missing_main_images', message: `missing_main_images:${JSON.stringify(missing)}` };
  }

  if (!hasPrice && hasTitle && (hasImages || hasSku)) {
    return { kind: 'partial_success', assembled };
  }

  return { kind: 'success', assembled };
}

async function extractAssembled(page: Page, sourceUrl: string): Promise<Parse1688Result & { blocked?: boolean }> {
  const payload = await extractBrowserPayload(page);
  return assembleParsedProduct(sourceUrl, payload);
}

class Alibaba1688Provider implements CollectorProvider {
  readonly sourceId = '1688';
  readonly meta = {
    name: '1688采集器',
    description: '采集 1688 商品详情页，支持标题、主图、详情图、属性、SKU',
    status: 'available' as const,
    batchSupported: true,
    urlPatterns: ['https://detail.1688.com/offer/*.html'],
    features: ['title', 'mainImages', 'descriptionImages', 'attributes', 'skus'] satisfies CollectFeature[],
    notes: '',
  };

  canHandle(url: string): boolean {
    try {
      const u = new URL(url);
      if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
      return is1688Host(u.hostname) && isLikelyOfferPath(url);
    } catch {
      return false;
    }
  }

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    if (!this.canHandle(input.url)) {
      throw new Error('INVALID_URL:not_a_1688_product_url');
    }

    const batchMode = input.options?.batchMode === true;
    const sourceUrl = input.url.trim();
    const navUrl = normalizeOfferNavUrl(sourceUrl);

    return with1688BatchGate(batchMode, () =>
      browser.with1688Page(async (page) => {
        const gotoTimeout = getDefaultNavigationTimeoutMs();
        let loginOrVerifyHit = false;

        try {
          await gotoOfferPage(page, navUrl, sourceUrl, gotoTimeout);
        } catch (e) {
          if (e instanceof Error && /^(TIMEOUT|NAVIGATION_FAILED):/.test(e.message)) {
            throw e;
          }
          const err = e instanceof Error ? e.message : String(e);
          if (/timeout/i.test(err)) throw new Error(`TIMEOUT:navigation_${err}`);
          throw new Error(`NAVIGATION_FAILED:${err}`);
        }

        await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);
        await prepare1688OfferPage(page, batchMode);

        const finalHref = page.url();
        if (isCaptchaRedirectUrl(finalHref)) {
          loginOrVerifyHit = true;
          throwBlocked('verification_or_login_page_detected');
        }

        let hostOk = false;
        try {
          hostOk = is1688Host(new URL(finalHref).hostname);
        } catch {
          hostOk = false;
        }
        if (!hostOk) {
          throwBlocked('redirected_off_1688_host');
        }

        const onOfferPath = isLikelyOfferPath(finalHref);
        let assembled = await extractAssembled(page, sourceUrl);

        if (assembled.mainImages.length === 0) {
          await prepare1688OfferPage(page, batchMode);
          assembled = await extractAssembled(page, sourceUrl);
        }

        const missing = fieldMissingSummary(assembled);
        loginOrVerifyHit =
          loginOrVerifyHit ||
          isBlockedPage(assembled) ||
          (await isStrictCaptchaSurface(page)) ||
          isCaptchaRedirectUrl(finalHref);

        if (!onOfferPath && (isBlockedPage(assembled) || isHardEmptyCollected(assembled))) {
          if (loginOrVerifyHit) {
            throwBlocked('verification_or_login_page_detected');
          }
        }

        if (isBlockedPage(assembled) && isHardEmptyCollected(assembled)) {
          throwBlocked('verification_challenge_or_offer_unreadable');
        }

        if (isHardEmptyCollected(assembled)) {
          if (loginOrVerifyHit) {
            throwBlocked('verification_challenge_or_offer_unreadable');
          }
        }

        const outcome = resolveCollectOutcome(assembled, missing, isBlockedPage(assembled));

        const debugBase = {
          sourceUrl,
          finalUrl: finalHref,
          pageTitle: (assembled.raw?.pageMeta as { docTitle?: string } | undefined)?.docTitle ?? '',
          loginOrVerifyHit,
          titleFound: !missing.title,
          priceFound: !missing.price,
          mainImagesCount: assembled.mainImages.length,
          detailImagesCount: assembled.descriptionImages.length,
          skuCount: assembled.skus.length,
          extractors: (assembled.extractDebug?.extractors as string[] | undefined) ?? [],
          missingFields: assembled.missingFields ?? [],
        };

        if (outcome.kind === 'failed') {
          const snap = await save1688FailureSnapshot(page, outcome.code);
          log1688CollectDebug({
            ...debugBase,
            collectStatus: 'failed',
            error: outcome.message,
            snapshotHtml: snap.htmlPath,
            snapshotPng: snap.screenshotPath,
          });

          if (outcome.code === 'blocked') {
            throwBlocked(outcome.message);
          }
          throw new Error(`PARSE_FAILED:${outcome.message}`);
        }

        const collectStatus = outcome.kind === 'partial_success' ? 'partial_success' : 'success';
        log1688CollectDebug({
          ...debugBase,
          collectStatus,
        });

        return {
          source: this.sourceId,
          sourceUrl,
          title: assembled.title.trim(),
          currency: 'CNY',
          mainImages: assembled.mainImages,
          descriptionImages: assembled.descriptionImages,
          attributes: assembled.attributes,
          skus: assembled.skus,
          raw: {
            ...assembled.raw,
            fieldMissing: missing,
            finalUrl: finalHref,
            navUrl,
            onOfferPath,
            collectStatus,
            completeness: assembled.completeness,
            missingFields: assembled.missingFields,
            warnings: assembled.warnings,
            extractDebug: assembled.extractDebug,
          },
        };
      }),
    );
  }
}

export const alibaba1688Provider = new Alibaba1688Provider();
