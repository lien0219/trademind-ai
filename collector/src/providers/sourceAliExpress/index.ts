import type { BrowserManager } from '../../browser/manager.js';
import type { CollectorProvider } from '../collector-provider.js';
import type { CollectInput } from '../collector-provider.js';
import type { NormalizedProduct } from '../../types/product.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import { assembleAeProduct, extractBrowserPayload } from './parser.js';

function isAllowedAliExpressHostname(hostname: string): boolean {
  const h = hostname.toLowerCase();
  return h.includes('aliexpress');
}


/** 语义校验：`http(s)`、`host` 含 aliexpress、`path` 含 `/item/`，并为常见商品详情 `.html`。 */
export function aeCanHandlePublicURL(urlStr: string): boolean {
  try {
    const u = new URL(urlStr);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
    if (!isAllowedAliExpressHostname(u.hostname)) return false;
    if (!u.pathname.includes('/item/')) return false;
    /** AE 单品页通常形如 `/item/<id>.html` */
    if (!/\/[^/]+\.html?\s*$/i.test(u.pathname.replace(/\/$/, '/'))) return false;
    return true;
  } catch {
    return false;
  }
}

/** 供 Provider 注册的实现 */
export const aliExpressCollectorProvider: CollectorProvider = new (class AE implements CollectorProvider {
  readonly sourceId = 'aliexpress';
  readonly meta = {
    name: '速卖通采集器',
    description: '采集 AliExpress 商品详情页，提取标题、图片、属性、SKU 等信息',
    status: 'beta' as const,
    batchSupported: false,
    urlPatterns: [
      'https://www.aliexpress.com/item/*.html',
      'https://*.aliexpress.com/item/*.html',
    ],
    features: ['title', 'mainImages', 'descriptionImages', 'attributes', 'skus'] satisfies CollectFeature[],
    notes: '部分页面受风控影响，SKU/详情图抽取可能不完整；批量采集暂不开放。',
  };

  canHandle(url: string): boolean {
    return aeCanHandlePublicURL(url);
  }

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    if (!this.canHandle(input.url)) {
      throw new Error('INVALID_URL:not_an_aliexpress_item_product_url');
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
        await page.waitForSelector('body', { timeout: 8000 }).catch(() => undefined);
      } catch {
        /** ignore */
      }

      const payload = await extractBrowserPayload(page);

      /** 跳转后仍为合法 AliExpress `/item/` 语义 */
      try {
        const finalU = new URL(page.url());
        if (
          (finalU.protocol !== 'http:' && finalU.protocol !== 'https:') ||
          !isAllowedAliExpressHostname(finalU.hostname)
        ) {
          throw new Error('INVALID_URL:left_aliexpress_host_after_navigation');
        }
        if (!finalU.pathname.toLowerCase().includes('/item/')) {
          throw new Error('INVALID_URL:not_an_aliexpress_item_detail_route');
        }
      } catch (e) {
        if (e instanceof Error && e.message.startsWith('INVALID_URL')) throw e;
        throw new Error('INVALID_URL:malformed_navigation_result');
      }

      const assembled = assembleAeProduct(input.url, payload);
      const title = assembled.title.trim();

      /** 优先级：人机验证且无标题先于「结构化失败」分类 */
      if (!title.length) {
        if (assembled.blocked) throw new Error('PAGE_BLOCKED_OR_VERIFY_REQUIRED:verification_or_login_challenge');
        throw new Error('COLLECT_FAILED:missing_product_title');
      }

      const raw: Record<string, unknown> = {
        ...assembled.rawShell,
        stateDigest: assembled.stateDigest,
      };

      const product: NormalizedProduct = {
        source: this.sourceId,
        sourceUrl: input.url,
        title,
        currency: assembled.currency,
        mainImages: assembled.mainImages,
        descriptionImages: assembled.descriptionImages,
        attributes: assembled.attributes,
        skus: assembled.skus,
        raw,
      };

      return product;
    });
  }
})();
