import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import { PAGE_EVALUATE_POLYFILL } from '../../browser/evaluate-in-page.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { NormalizedProduct } from '../../types/product.js';
import { detectTaobaoAccessStatus, throwAccessError } from './access-detect.js';
import { extractAndAssembleTaobao, validateTaobaoCollectQuality } from './parser.js';
import { TAOBAO_TMALL_PROFILE_KEY, TAOBAO_TMALL_PROVIDER } from './profile.js';
import { taobaoTmallUrlHint, validateTaobaoTmallUrl } from './validate-url.js';
import { waitForProductCore } from './page-extract.js';

function resolveGotoTimeoutMs(options?: Record<string, unknown>): number {
  const raw = options?.gotoTimeoutMs ?? options?.timeoutMs;
  const n = typeof raw === 'number' ? raw : Number(raw);
  if (Number.isFinite(n) && n > 0) return Math.min(n, 300_000);
  return getDefaultNavigationTimeoutMs();
}

function accessCheckEnabled(options?: Record<string, unknown>): boolean {
  if (options?.accessCheckEnabled === false) return false;
  if (options?.accessCheckEnabled === '0' || options?.accessCheckEnabled === 'false') {
    return false;
  }
  return true;
}

class TaobaoTmallCollectorProvider implements CollectorProvider {
  readonly sourceId = TAOBAO_TMALL_PROVIDER;
  readonly meta = {
    name: '淘宝/天猫采集器',
    description:
      '采集淘宝、天猫商品详情，支持标题、价格、主图、详情图、商品参数。部分商品可能需要登录后采集。',
    status: 'beta' as const,
    batchSupported: false,
    urlPatterns: [
      'https://item.taobao.com/item.htm?id=*',
      'https://detail.tmall.com/item.htm?id=*',
      'https://detail.tmall.hk/item.htm?id=*',
      'https://world.taobao.com/item/*.htm',
    ],
    features: [
      'title',
      'price',
      'mainImages',
      'descriptionImages',
      'attributes',
      'skus',
    ] satisfies CollectFeature[],
    notes: '批量采集暂未开放。部分商品需要登录或手动完成安全验证。',
  };

  canHandle(url: string): boolean {
    return validateTaobaoTmallUrl(url);
  }

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    const sourceUrl = input.url.trim();
    if (!this.canHandle(sourceUrl)) {
      throw new Error(`INVALID_URL:${taobaoTmallUrlHint()}`);
    }

    const profileKey = String(input.options?.profileKey ?? '').trim();
    const useDedicatedProfile =
      input.options?.useBrowserProfile === true && profileKey === TAOBAO_TMALL_PROFILE_KEY;
    const gotoTimeout = resolveGotoTimeoutMs(input.options);
    const runAccessCheck = accessCheckEnabled(input.options);

    const run = async (page: Page) => {
      try {
        await page.goto(sourceUrl, { waitUntil: 'domcontentloaded', timeout: gotoTimeout });
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        if (/timeout/i.test(err)) throw new Error(`PAGE_LOAD_TIMEOUT:${err}`);
        throw new Error(`NAVIGATION_FAILED:${err}`);
      }

      await page
        .waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) })
        .catch(() => undefined);
      await page.waitForTimeout(800);

      if (runAccessCheck) {
        const access = await detectTaobaoAccessStatus(page, sourceUrl);
        if (access.status !== 'public') {
          throwAccessError(access);
        }
      }

      await waitForProductCore(page, gotoTimeout);

      const assembled = await extractAndAssembleTaobao(page, sourceUrl);
      const quality = validateTaobaoCollectQuality(assembled);
      if (!quality.ok && quality.error) {
        throw new Error(quality.error);
      }

      const collectStatus = quality.partial || assembled.warnings.length > 0 ? 'partial_success' : 'success';

      return {
        source: this.sourceId,
        sourceUrl,
        title: assembled.title,
        currency: assembled.currency,
        mainDescription: '',
        mainImages: assembled.mainImages,
        descriptionImages: assembled.descriptionImages,
        attributes: assembled.attributes,
        skus: assembled.skus,
        raw: {
          ...assembled.raw,
          collectStatus,
          qualityWarnings: assembled.warnings,
        },
      } satisfies NormalizedProduct;
    };

    if (useDedicatedProfile) {
      return browser.withTaobaoTmallPage(run);
    }

    const browserInstance = await browser.ensureBrowser();
    const context = await browserInstance.newContext({
      userAgent:
        process.env.COLLECTOR_USER_AGENT ??
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      locale: 'zh-CN',
      viewport: { width: 1280, height: 900 },
    });
    await context.addInitScript(PAGE_EVALUATE_POLYFILL);
    const page = await context.newPage();
    page.setDefaultNavigationTimeout(gotoTimeout);
    page.setDefaultTimeout(gotoTimeout);
    try {
      return await run(page);
    } finally {
      await page.close().catch(() => undefined);
      await context.close().catch(() => undefined);
    }
  }
}

export const taobaoTmallCollectorProvider = new TaobaoTmallCollectorProvider();
