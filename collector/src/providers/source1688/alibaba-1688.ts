import type { BrowserManager } from '../../browser/manager.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { NormalizedProduct } from '../../types/product.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';

import { assembleParsedProduct, extractBrowserPayload } from './parser.js';

function is1688Host(hostname: string): boolean {
  return hostname === '1688.com' || hostname.endsWith('.1688.com');
}

function isLikelyOfferPath(urlStr: string): boolean {
  try {
    const u = new URL(urlStr);
    if (!is1688Host(u.hostname)) return false;
    /** 兼容 detail / m / sale 等不同子域与路径形态 */
    return (
      /\/offer\/?/i.test(u.pathname) ||
      /offerId=/i.test(u.search) ||
      /offer(?:id)?\.html$/i.test(u.pathname)
    );
  } catch {
    return false;
  }
}

function isHardEmptyCollected(r: ReturnType<typeof assembleParsedProduct>): boolean {
  return (
    r.mainImages.length === 0 &&
    r.descriptionImages.length === 0 &&
    Object.keys(r.attributes).length === 0 &&
    r.skus.length === 0
  );
}

/**
 * 1688 结构化解析：DOM + 内联脚本 JSON（尽量提取主图/详情图/属性/SKU）。
 * 风控或页面变更时降级为不完整数据，避免因解析抛错阻断任务（除非法链接与完全不可用页）。
 */
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
      return u.protocol === 'http:' || u.protocol === 'https:' ? is1688Host(u.hostname) : false;
    } catch {
      return false;
    }
  }

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    if (!this.canHandle(input.url)) {
      throw new Error('INVALID_URL:not_a_1688_product_url');
    }

    return browser.withPage(async (page) => {
      const gotoTimeout = getDefaultNavigationTimeoutMs();
      try {
        await page.goto(input.url, { waitUntil: 'domcontentloaded', timeout: gotoTimeout });
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        throw new Error(`NAVIGATION_FAILED:${err}`);
      }

      await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);

      try {
        await page.waitForSelector('h1, h1.d-title, [class*="title"], body', { timeout: 8000 });
      } catch {
        /** 兜底：不因选择器超时失败 */
      }

      const finalHref = page.url();
      /** 跳转后仍需是 1688 商品语义 URL */
      if (!isLikelyOfferPath(finalHref)) {
        throw new Error('INVALID_URL:not_a_1688_offer_detail_page');
      }

      const payload = await extractBrowserPayload(page);
      const assembled = assembleParsedProduct(input.url, payload);

      if (assembled.blocked && isHardEmptyCollected(assembled)) {
        throw new Error('PAGE_BLOCKED_OR_VERIFY_REQUIRED:verification_challenge_or_offer_unreadable');
      }

      const product: NormalizedProduct = {
        source: this.sourceId,
        sourceUrl: input.url,
        title: assembled.title.trim(),
        currency: 'CNY',
        mainImages: assembled.mainImages,
        descriptionImages: assembled.descriptionImages,
        attributes: assembled.attributes,
        skus: assembled.skus,
        raw: assembled.raw,
      };

      return product;
    });
  }
}

export const alibaba1688Provider = new Alibaba1688Provider();
