import type { BrowserManager } from '../../browser/manager.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';

function isHttpUrl(url: string): boolean {
  try {
    const u = new URL(url);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

function notImplemented(message: string) {
  return async (_browser: BrowserManager, _input: CollectInput) => {
    throw new Error(`PROVIDER_NOT_IMPLEMENTED:${message}`);
  };
}

/** 拼多多：规划中，仅做链接形态粗校验 */
export const sourcePddProvider: CollectorProvider = {
  sourceId: 'pdd',
  meta: {
    name: '拼多多采集器',
    description: '采集拼多多商品详情（规划中）。',
    status: 'planned',
    batchSupported: false,
    urlPatterns: ['https://mobile.yangkeduo.com/…', 'https://*.yangkeduo.com/…'],
    features: [],
    notes: '',
  },
  canHandle(url: string): boolean {
    if (!isHttpUrl(url)) return false;
    try {
      const h = new URL(url).hostname.toLowerCase();
      return (
        h === 'yangkeduo.com' ||
        h.endsWith('.yangkeduo.com') ||
        h === 'mobile.yangkeduo.com' ||
        h.includes('pinduoduo') ||
        h.includes('pdd')
      );
    } catch {
      return false;
    }
  },
  collect: notImplemented('拼多多采集器暂未实现'),
};

/** 淘宝 / 天猫 */
export const sourceTaobaoProvider: CollectorProvider = {
  sourceId: 'taobao',
  meta: {
    name: '淘宝/天猫采集器',
    description: '采集淘宝、天猫商品详情（规划中）。',
    status: 'planned',
    batchSupported: false,
    urlPatterns: ['https://item.taobao.com/item.htm?id=*', 'https://detail.tmall.com/item.htm?id=*'],
    features: [],
    notes: '',
  },
  canHandle(url: string): boolean {
    if (!isHttpUrl(url)) return false;
    try {
      const h = new URL(url).hostname.toLowerCase();
      return (
        h === 'taobao.com' ||
        h.endsWith('.taobao.com') ||
        h === 'tmall.com' ||
        h.endsWith('.tmall.com') ||
        h.includes('tmall') ||
        h.includes('taobao')
      );
    } catch {
      return false;
    }
  },
  collect: notImplemented('淘宝/天猫采集器暂未实现'),
};

/** 速卖通 */
export const sourceAliExpressProvider: CollectorProvider = {
  sourceId: 'aliexpress',
  meta: {
    name: '速卖通采集器',
    description: '采集 AliExpress 商品详情（规划中）。',
    status: 'planned',
    batchSupported: false,
    urlPatterns: ['https://www.aliexpress.com/item/*.html'],
    features: [],
    notes: '',
  },
  canHandle(url: string): boolean {
    if (!isHttpUrl(url)) return false;
    try {
      const h = new URL(url).hostname.toLowerCase();
      return h === 'aliexpress.com' || h.endsWith('.aliexpress.com');
    } catch {
      return false;
    }
  },
  collect: notImplemented('速卖通采集器暂未实现'),
};

/** SHEIN / Temu 合并入口 */
export const sourceSheinTemuProvider: CollectorProvider = {
  sourceId: 'shein_temu',
  meta: {
    name: 'SHEIN/Temu采集器',
    description: '采集 SHEIN、Temu 等平台商品详情（规划中）。',
    status: 'planned',
    batchSupported: false,
    urlPatterns: ['https://www.shein.com/...', 'https://www.temu.com/...'],
    features: [],
    notes: '',
  },
  canHandle(url: string): boolean {
    if (!isHttpUrl(url)) return false;
    try {
      const h = new URL(url).hostname.toLowerCase();
      return (
        h.includes('shein') ||
        h.includes('temu') ||
        h.endsWith('.shein.com') ||
        h.endsWith('.temu.com')
      );
    } catch {
      return false;
    }
  },
  collect: notImplemented('SHEIN/Temu采集器暂未实现'),
};

/** 自定义链接：占位，后续规则化抽取 */
export const sourceCustomProvider: CollectorProvider = {
  sourceId: 'custom',
  meta: {
    name: '自定义链接采集器',
    description:
      '后续支持通过 CSS 选择器、JSON-LD、OpenGraph 等自动抽取通用商品信息（占位）。当前版本暂未开放采集执行。',
    status: 'planned',
    batchSupported: false,
    urlPatterns: ['https://… 任意公有商品详情页链接'],
    features: [],
    notes: '',
  },
  canHandle(url: string): boolean {
    return isHttpUrl(url) && url.length >= 12 && url.length <= 8192;
  },
  collect: notImplemented('自定义链接采集器暂未实现'),
};
