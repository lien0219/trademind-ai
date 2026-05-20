import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import { with1688BatchGate } from '../../browser/batch-gate.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { NormalizedProduct } from '../../types/product.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';

import { assembleParsedProduct, extractBrowserPayload } from './parser.js';
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

/** 去掉站外追踪参数，用 canonical offer 页打开（失败时回退原始 URL）。 */
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

function fieldMissingSummary(assembled: ReturnType<typeof assembleParsedProduct>): Record<string, boolean> {
  const priceFromSku = assembled.skus.some((s) => typeof s.price === 'number' && s.price > 0);
  return {
    title: !assembled.title?.trim() || assembled.title.trim().length < 4,
    price: !priceFromSku,
    images: assembled.mainImages.length === 0,
    sku: assembled.skus.length === 0,
  };
}

function isPlaceholderTitle(title: string): boolean {
  const t = title.trim();
  return t === '' || t === '（解析：未命名商品）' || t.length < 4;
}

/** 仅识别明确的验证码/惩罚跳转 URL，避免把详情页页眉「登录」误判为风控。 */
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

/**
 * 仅在缺少商品 DOM 结构时，才用正文关键词判断验证码页（详情页页眉的「登录」不会触发）。
 */
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

async function waitForOfferGallery(page: Page, timeoutMs: number): Promise<void> {
  await page
    .waitForSelector(
      '.detail-gallery img, [class*="gallery"] img, [class*="img-list"] img, [class*="offer-img"] img, .tab-pane img',
      { timeout: timeoutMs },
    )
    .catch(() => undefined);
}

async function extractAssembled(page: Page, sourceUrl: string): Promise<Parse1688Result & { blocked?: boolean }> {
  const payload = await extractBrowserPayload(page);
  return assembleParsedProduct(sourceUrl, payload);
}

function assertOfferQuality(assembled: ReturnType<typeof assembleParsedProduct>, missing: Record<string, boolean>): void {
  const title = assembled.title?.trim() ?? '';
  const hasImages = assembled.mainImages.length > 0;
  const hasSkus = assembled.skus.length > 0;
  const blocked = isBlockedPage(assembled);

  if (isPlaceholderTitle(title) && !hasImages && !hasSkus) {
    if (blocked) {
      throwBlocked('verification_challenge_or_offer_unreadable');
    }
    throw new Error(`PARSE_FAILED:missing_core_fields:${JSON.stringify(missing)}`);
  }

  if (!hasImages) {
    if (blocked) {
      throwBlocked('offer_partial_no_images_likely_blocked');
    }
    throw new Error(`PARSE_FAILED:missing_main_images:${JSON.stringify(missing)}`);
  }

  if (isPlaceholderTitle(title)) {
    throw new Error(`PARSE_FAILED:missing_title:${JSON.stringify(missing)}`);
  }
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
      browser.withPage(async (page) => {
        const gotoTimeout = getDefaultNavigationTimeoutMs();
        try {
          await gotoOfferPage(page, navUrl, sourceUrl, gotoTimeout);
        } catch (e) {
          if (e instanceof Error && /^(TIMEOUT|NAVIGATION_FAILED):/.test(e.message)) {
            throw e;
          }
          const err = e instanceof Error ? e.message : String(e);
          if (/timeout/i.test(err)) {
            throw new Error(`TIMEOUT:navigation_${err}`);
          }
          throw new Error(`NAVIGATION_FAILED:${err}`);
        }

        await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);

        try {
          await page.waitForSelector('h1, h1.d-title, [class*="title"], body', { timeout: 8000 });
        } catch {
          /** 兜底 */
        }

        await waitForOfferGallery(page, batchMode ? 12_000 : 10_000);

        try {
          await page
            .waitForSelector(
              '[class*="sku"], [class*="obj-sku"], [class*="sale-prop"], [class*="sku-item"]',
              { timeout: 6000 },
            )
            .catch(() => undefined);
          await page.getByText(/库存\s*\d+/).first().waitFor({ timeout: 5000 }).catch(() => undefined);
        } catch {
          /** SKU 区异步渲染时仍走 DOM/JSON 兜底 */
        }

        const finalHref = page.url();
        if (isCaptchaRedirectUrl(finalHref)) {
          throwBlocked('captcha_redirect_url');
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
          await page.waitForTimeout(batchMode ? 3500 : 2000);
          await waitForOfferGallery(page, 8000);
          assembled = await extractAssembled(page, sourceUrl);
        }

        const missing = fieldMissingSummary(assembled);

        if (!onOfferPath && (isBlockedPage(assembled) || isHardEmptyCollected(assembled))) {
          if ((await isStrictCaptchaSurface(page)) || isCaptchaRedirectUrl(finalHref)) {
            throwBlocked('offer_path_lost_with_no_product_data');
          }
        }

        if (isBlockedPage(assembled) && isHardEmptyCollected(assembled)) {
          throwBlocked('verification_challenge_or_offer_unreadable');
        }

        if (isHardEmptyCollected(assembled)) {
          if ((await isStrictCaptchaSurface(page)) || isCaptchaRedirectUrl(finalHref)) {
            throwBlocked('verification_challenge_or_offer_unreadable');
          }
        }

        assertOfferQuality(assembled, missing);

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
          },
        };
      }),
    );
  }
}

export const alibaba1688Provider = new Alibaba1688Provider();
