import type { BrowserManager } from '../../browser/manager.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { NormalizedProduct } from '../../types/product.js';

function is1688Host(hostname: string): boolean {
  return hostname === '1688.com' || hostname.endsWith('.1688.com');
}

/**
 * 1688 采集占位：能校验域名、打开页面并填入标题等基础字段；
 * SKU/主图/详情的稳定解析在后续迭代完成（raw 保留现场信息）。
 */
class Alibaba1688Provider implements CollectorProvider {
  readonly sourceId = '1688';

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
      try {
        await page.goto(input.url, { waitUntil: 'domcontentloaded' });
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        throw new Error(`NAVIGATION_FAILED:${err}`);
      }

      const title = await page.title().catch(() => '');
      const mainImages: string[] = [];

      const product: NormalizedProduct = {
        source: this.sourceId,
        sourceUrl: input.url,
        title: title?.trim() || '（占位：未解析到标题）',
        currency: 'CNY',
        mainImages,
        descriptionImages: [],
        attributes: {},
        skus: [],
        raw: {
          placeholder: true,
          hint: '1688Provider 占位实现：仅拉取标题与页面元数据，详细 SKU/图片解析待实现',
          titleFromPage: title,
          url: page.url(),
        },
      };

      return product;
    });
  }
}

export const alibaba1688Provider = new Alibaba1688Provider();
