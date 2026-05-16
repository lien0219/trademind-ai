import type { BrowserManager } from '../../browser/manager.js';
import type { CollectorProvider } from '../collector-provider.js';
import type { CollectInput } from '../collector-provider.js';
import type { NormalizedProduct } from '../../types/product.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import type { CustomCollectOptions, CustomRuleDecl } from './types.js';
import { parseCustomProduct } from './parser.js';

function isHttpUrl(url: string): boolean {
  try {
    const u = new URL(url);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

function hostnameOf(urlStr: string): string {
  try {
    return new URL(urlStr).hostname.toLowerCase();
  } catch {
    return '';
  }
}

function domainMatches(host: string, domain: string): boolean {
  const h = host.trim().toLowerCase();
  const d = domain.trim().toLowerCase();
  if (!h || !d) return false;
  return h === d || h.endsWith(`.${d}`);
}

export const sourceCustomCollectorProvider: CollectorProvider = {
  sourceId: 'custom',
  meta: {
    name: '自定义链接采集器',
    description: '通过用户配置的选择器规则采集通用商品页',
    status: 'beta',
    batchSupported: false,
    urlPatterns: ['https://example.com/product/...'],
    features: ['title', 'mainImages', 'descriptionImages', 'attributes'] satisfies CollectFeature[],
    notes: '批量采集暂不开放；规则由后端传入 options。',
  },

  canHandle(urlStr: string): boolean {
    return isHttpUrl(urlStr) && urlStr.length >= 12 && urlStr.length <= 8192;
  },

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    const urlStr = input.url?.trim() ?? '';
    if (!this.canHandle(urlStr)) {
      throw new Error('INVALID_URL:not_http_url');
    }

    const opts = input.options as CustomCollectOptions | undefined;
    const ruleUnknown = opts?.rule as unknown;
    if (!ruleUnknown || typeof ruleUnknown !== 'object') {
      throw new Error('INVALID_REQUEST:missing rule in options');
    }
    const rule = ruleUnknown as CustomRuleDecl;

    const domain = String(opts?.domain ?? '').trim().toLowerCase();
    if (!domain) {
      throw new Error('INVALID_REQUEST:missing domain in options');
    }
    const host = hostnameOf(urlStr);
    if (!domainMatches(host, domain)) {
      throw new Error('INVALID_URL:hostname does not match rule domain');
    }
    const mp = String(opts?.matchPattern ?? '').trim();
    if (mp) {
      try {
        const re = new RegExp(mp);
        if (!re.test(urlStr)) {
          throw new Error('INVALID_URL:url does not match matchPattern');
        }
      } catch {
        throw new Error('INVALID_REQUEST:invalid matchPattern regexp');
      }
    }

    return browser.withPage(async (page) => {
      const gotoTimeout = getDefaultNavigationTimeoutMs();
      try {
        await page.goto(urlStr, { waitUntil: 'domcontentloaded', timeout: gotoTimeout });
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

      const product = await parseCustomProduct(page, urlStr, rule);
      if (!product.title?.trim()) {
        throw new Error('COLLECT_FAILED:empty_title_after_rules_and_fallbacks');
      }
      return product;
    });
  },
};
