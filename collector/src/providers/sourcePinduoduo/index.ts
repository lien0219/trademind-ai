import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { NormalizedProduct } from '../../types/product.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import { PAGE_EVALUATE_POLYFILL } from '../../browser/evaluate-in-page.js';
import { detectPinduoduoAccessStatus, throwAccessError } from './access-detect.js';
import { extractAndAssemblePinduoduo } from './parser.js';
import { normalizePinduoduoNavUrl, validatePinduoduoUrl } from './validate-url.js';

const MOBILE_UA =
  process.env.COLLECTOR_PDD_USER_AGENT ??
  'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.38(0x18002633) NetType/WIFI Language/zh_CN';

class PinduoduoCollectorProvider implements CollectorProvider {
  readonly sourceId = 'pinduoduo';
  readonly meta = {
    name: '拼多多采集器',
    description:
      '采集拼多多商品详情页，适合提取商品标题、价格、图片、参数等基础信息。商品规格、库存、动态价格可能受页面结构和风控影响，第一版不保证完整。',
    status: 'beta' as const,
    batchSupported: false,
    urlPatterns: [
      'https://mobile.yangkeduo.com/goods.html?goods_id=*',
      'https://yangkeduo.com/goods.html?goods_id=*',
      'https://mobile.pinduoduo.com/goods.html?goods_id=*',
      'https://pinduoduo.com/goods.html?goods_id=*',
    ],
    features: [
      'title',
      'price',
      'mainImages',
      'descriptionImages',
      'attributes',
      'skus',
    ] satisfies CollectFeature[],
    notes: '拼多多批量采集暂未开放，请先使用单链接采集验证稳定性。',
  };

  canHandle(url: string): boolean {
    return validatePinduoduoUrl(url);
  }

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    if (!this.canHandle(input.url)) {
      throw new Error('INVALID_URL:not_a_pinduoduo_product_url');
    }

    const sourceUrl = input.url.trim();
    const navUrl = normalizePinduoduoNavUrl(sourceUrl);
    const useProfile =
      input.options?.useBrowserProfile === true &&
      typeof input.options?.profileKey === 'string' &&
      input.options.profileKey.trim().length > 0;

    const run = async (page: Page) => {
      const gotoTimeout = getDefaultNavigationTimeoutMs();
      try {
        await page.goto(navUrl, { waitUntil: 'domcontentloaded', timeout: gotoTimeout });
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        if (/timeout/i.test(err)) throw new Error(`TIMEOUT:navigation_${err}`);
        throw new Error(`NAVIGATION_FAILED:${err}`);
      }

      await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);
      await page.waitForTimeout(800);

      const access = await detectPinduoduoAccessStatus(page);
      if (access.status !== 'public') {
        if (access.errorCode) throwAccessError(access);
      }

      const assembled = await extractAndAssemblePinduoduo(page, sourceUrl);
      const title = assembled.title.trim();

      if (!title) {
        if (access.status === 'verify_required' || access.status === 'app_guide') {
          throw new Error('PAGE_BLOCKED_OR_VERIFY_REQUIRED:verification_or_app_guide');
        }
        throw new Error('PARSE_FAILED_TITLE_MISSING:no_product_title');
      }

      if (assembled.mainImages.length === 0) {
        throw new Error('PARSE_FAILED_IMAGE_MISSING:no_main_images');
      }

      const raw: Record<string, unknown> = {
        ...assembled.raw,
        productPrice: assembled.price,
        priceText: assembled.priceText,
        qualityWarnings: assembled.warnings,
        accessStatus: access.status,
        finalUrl: page.url(),
        navUrl,
        collectStatus: assembled.warnings.length ? 'partial_success' : 'success',
      };

      return {
        source: this.sourceId,
        sourceUrl,
        title,
        currency: assembled.currency,
        mainImages: assembled.mainImages,
        descriptionImages: assembled.descriptionImages,
        attributes: assembled.attributes,
        skus: assembled.skus,
        raw,
      } satisfies NormalizedProduct;
    };

    if (useProfile) {
      return browser.withCustomProfilePage(String(input.options!.profileKey), run);
    }

    const browserInstance = await browser.ensureBrowser();
    const context = await browserInstance.newContext({
      userAgent: MOBILE_UA,
      locale: 'zh-CN',
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
    });
    await context.addInitScript(PAGE_EVALUATE_POLYFILL);
    const page = await context.newPage();
    page.setDefaultNavigationTimeout(getDefaultNavigationTimeoutMs());
    page.setDefaultTimeout(getDefaultNavigationTimeoutMs());
    try {
      return await run(page);
    } finally {
      await page.close().catch(() => undefined);
      await context.close().catch(() => undefined);
    }
  }
}

export const pinduoduoCollectorProvider = new PinduoduoCollectorProvider();
