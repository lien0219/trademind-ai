import { classifyPinduoduoUrl, isPifaWholesaleHost } from './url-type.js';
import { isPinduoduoHost } from './validate-url.js';

export const PIFA_HOME = 'https://pifa.pinduoduo.com/';

/** 仅移动端首页（无商品参数），会展示 App 扫码引导。 */
export function isPinduoduoMobileHomeOnly(urlStr: string): boolean {
  try {
    const u = new URL(urlStr.trim());
    const host = u.hostname.toLowerCase();
    if (host !== 'mobile.yangkeduo.com' && host !== 'yangkeduo.com') return false;
    const path = u.pathname.replace(/\/$/, '') || '/';
    const hasGoods =
      u.searchParams.has('goods_id') ||
      u.searchParams.has('goodsId') ||
      /goods/.test(path);
    return path === '/' && !hasGoods;
  } catch {
    return false;
  }
}

/** pifa 批发站首页（非 goods/detail 商品页）。 */
export function isPifaHomeOnly(urlStr: string): boolean {
  try {
    const u = new URL(urlStr.trim());
    if (!isPifaWholesaleHost(u.hostname.toLowerCase())) return false;
    const path = u.pathname.replace(/\/$/, '') || '/';
    if (path === '/' || path === '') return true;
    return !/\/goods\/detail/i.test(path);
  } catch {
    return false;
  }
}

/** 拼多多 / 批发商品详情页（用于登录态确认，非首页列表）。 */
export function isPinduoduoProductDetailUrl(urlStr: string): boolean {
  try {
    const u = new URL(urlStr.trim());
    const host = u.hostname.toLowerCase();
    if (isPifaWholesaleHost(host)) {
      return (
        /\/goods\/detail/i.test(u.pathname) &&
        Boolean(u.searchParams.get('gid') || u.searchParams.get('goods_id'))
      );
    }
    if (!isPinduoduoHost(host)) return false;
    const path = u.pathname.toLowerCase();
    const goodsId = u.searchParams.get('goods_id') ?? u.searchParams.get('goodsId');
    return (
      (/goods/.test(path) || path.includes('goods_detail')) &&
      Boolean(goodsId && /^\d+$/.test(goodsId))
    );
  } catch {
    return false;
  }
}

export function isPinduoduoLoginContextUrl(urlStr: string): boolean {
  const raw = urlStr.trim();
  if (!raw) return false;
  try {
    const u = new URL(raw);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
    const host = u.hostname.toLowerCase();
    if (!isPinduoduoHost(host) && !isPifaWholesaleHost(host)) return false;
    if (isPinduoduoMobileHomeOnly(raw)) return false;
    return true;
  } catch {
    return false;
  }
}

export type PinduoduoAuthUrlType =
  | 'wholesale_detail'
  | 'goods_detail'
  | 'homepage'
  | 'app_redirect'
  | 'unknown';

export function authUrlTypeLabel(checkUrl: string, finalUrl: string): PinduoduoAuthUrlType {
  const target = (finalUrl || checkUrl).trim();
  if (!target) return 'unknown';
  if (isPinduoduoMobileHomeOnly(target) || isPifaHomeOnly(target)) return 'homepage';
  const cls = classifyPinduoduoUrl(target);
  if (cls === 'wholesale_detail') return 'wholesale_detail';
  if (cls === 'goods_detail') return 'goods_detail';
  if (cls === 'app_redirect') return 'app_redirect';
  return 'unknown';
}

export function resolvePinduoduoOpenLoginUrl(contextUrl?: string): {
  url: string;
  hasContext: boolean;
} {
  const raw = contextUrl?.trim() ?? '';
  if (raw && isPinduoduoLoginContextUrl(raw)) {
    return { url: raw, hasContext: true };
  }
  return { url: PIFA_HOME, hasContext: false };
}

export type PinduoduoAuthCheckPlan = {
  checkUrl: string;
  mode: 'product_detail' | 'homepage_fallback';
  hint?: string;
};

/** 登录态检测 URL 解析：优先商品详情，否则仅首页（homepage_fallback）。 */
export function resolvePinduoduoAuthCheckPlan(
  contextUrl?: string,
  settingsTestUrl?: string,
): PinduoduoAuthCheckPlan {
  const custom = process.env.COLLECTOR_PDD_AUTH_CHECK_URL?.trim();
  if (custom && isPinduoduoLoginContextUrl(custom)) {
    return { checkUrl: custom, mode: 'product_detail' };
  }

  const candidates = [
    contextUrl?.trim(),
    settingsTestUrl?.trim(),
    process.env.COLLECTOR_PDD_AUTH_GOODS_TEST_URL?.trim(),
  ].filter(Boolean) as string[];

  for (const raw of candidates) {
    if (isPinduoduoProductDetailUrl(raw) || isPinduoduoLoginContextUrl(raw)) {
      return { checkUrl: raw, mode: 'product_detail' };
    }
  }

  return {
    checkUrl: PIFA_HOME,
    mode: 'homepage_fallback',
    hint: '未提供商品详情链接，本次仅检测拼多多批发首页是否可访问，不能准确判断是否已登录',
  };
}
