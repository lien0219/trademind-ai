import type { BrowserManager } from '../../browser/manager.js';
import type { CollectorProvider } from '../collector-provider.js';
import type { CollectInput } from '../collector-provider.js';
import type { NormalizedProduct } from '../../types/product.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { CustomCollectOptions } from './types.js';
import { normalizeCustomRuleDecl } from './normalize-rule.js';
import { runCustomCollect } from './run-custom.js';

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
    description:
      '适合采集没有专用采集器的网站商品页，可采集商品标题、价格、图片、参数等基础信息。',
    status: 'beta',
    batchSupported: false,
    urlPatterns: ['https://example.com/product/...'],
    features: ['title', 'price', 'mainImages', 'descriptionImages', 'attributes'] satisfies CollectFeature[],
    notes:
      '商品规格、库存、动态价格不保证完整。使用前建议先测试采集规则。已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放。',
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
      throw new Error('CUSTOM_RULE_MISSING:missing rule in options');
    }

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
        throw new Error('CUSTOM_RULE_INVALID:invalid matchPattern regexp');
      }
    }

    const result = await runCustomCollect(
      browser,
      urlStr,
      { ...opts, rule: normalizeCustomRuleDecl(ruleUnknown) },
      'task',
    );

    if (!result.product) {
      throw new Error('COLLECT_FAILED:empty_product');
    }
    return result.product;
  },
};

/** Rule test entry — always returns preview payload via HTTP layer. */
export async function runCustomRuleTest(
  browser: BrowserManager,
  urlStr: string,
  opts: CustomCollectOptions,
): Promise<import('./run-custom.js').CustomRunResult> {
  return runCustomCollect(browser, urlStr, opts, 'rule_test');
}
