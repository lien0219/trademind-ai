export type PinduoduoUrlType =
  | 'goods_detail'
  | 'wholesale_detail'
  | 'wholesale_homepage'
  | 'login_page'
  | 'wechat_auth'
  | 'app_redirect'
  | 'unknown';

function isPifaHost(host: string): boolean {
  return host === 'pifa.pinduoduo.com' || host.endsWith('.pifa.pinduoduo.com');
}

function isPddHost(host: string): boolean {
  if (
    host === 'pinduoduo.com' ||
    host === 'yangkeduo.com' ||
    host === 'mobile.yangkeduo.com' ||
    host === 'mobile.pinduoduo.com'
  ) {
    return true;
  }
  return host.endsWith('.pinduoduo.com') || host.endsWith('.yangkeduo.com') || isPifaHost(host);
}

export function classifyPinduoduoUrl(urlStr: string): PinduoduoUrlType {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return 'unknown';
    const host = u.hostname.toLowerCase();
    const path = u.pathname.toLowerCase();
    if (/weixin\.qq\.com|open\.weixin/i.test(host)) return 'wechat_auth';
    if (!isPddHost(host)) return 'unknown';
    if (/login|passport|auth/i.test(path)) return 'login_page';
    if (isPifaHost(host)) {
      const gid = u.searchParams.get('gid') ?? u.searchParams.get('goods_id');
      const barePath = path.replace(/\/$/, '') || '/';
      if (barePath === '/' || barePath === '/index.html') return 'wholesale_homepage';
      if (path.includes('goods') && gid && /^\d+$/.test(gid)) return 'wholesale_detail';
      if (path.includes('goods/detail')) return 'wholesale_detail';
      if (!path.includes('goods')) return 'wholesale_homepage';
      return 'unknown';
    }
    const goodsId = u.searchParams.get('goods_id') ?? u.searchParams.get('goodsId');
    const isGoodsPath =
      path.includes('goods') || path.includes('goods_detail') || path.includes('comm_goods');
    if (isGoodsPath && goodsId && /^\d+$/.test(goodsId)) return 'goods_detail';
    if (isGoodsPath && /(?:^|[?&])goods_id=\d+/i.test(u.search)) return 'goods_detail';
    if (/app|download|redirect/i.test(path)) return 'app_redirect';
    return 'unknown';
  } catch {
    return 'unknown';
  }
}

export function pinduoduoUrlHint(urlStr: string): string | null {
  const t = classifyPinduoduoUrl(urlStr);
  switch (t) {
    case 'wholesale_detail':
      return '已识别为拼多多批发商品详情页，可开始采集。';
    case 'goods_detail':
      return '已识别为移动端商品页。当前版本优先支持批发详情页（pifa.pinduoduo.com/goods/detail），请换用批发链接。';
    case 'wholesale_homepage':
      return '该链接为拼多多批发首页，请输入带 gid 的商品详情链接。';
    case 'wechat_auth':
      return '该链接为微信授权页，请在采集浏览器中完成扫码后再采集。';
    case 'login_page':
    case 'app_redirect':
    case 'unknown':
      return '请输入拼多多批发商品详情链接（pifa.pinduoduo.com/goods/detail/?gid=）。';
    default:
      return null;
  }
}

export function pinduoduoProfileDomain(): string {
  return 'pinduoduo.com';
}

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

export function resolvePinduoduoLoginTargetUrl(contextUrl?: string): string {
  const raw = contextUrl?.trim() ?? '';
  if (raw) {
    try {
      const u = new URL(raw);
      if (u.protocol === 'http:' || u.protocol === 'https:') {
        const host = u.hostname.toLowerCase();
        const okHost =
          host.endsWith('.pinduoduo.com') ||
          host.endsWith('.yangkeduo.com') ||
          host === 'pinduoduo.com' ||
          host === 'yangkeduo.com';
        if (okHost && !isPinduoduoMobileHomeOnly(raw)) {
          return raw;
        }
      }
    } catch {
      /* fall through */
    }
  }
  return 'https://pifa.pinduoduo.com/';
}

export function hasPinduoduoLoginContext(contextUrl?: string): boolean {
  const raw = contextUrl?.trim() ?? '';
  if (!raw) return false;
  try {
    const u = new URL(raw);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
    const host = u.hostname.toLowerCase();
    const okHost =
      host.endsWith('.pinduoduo.com') ||
      host.endsWith('.yangkeduo.com') ||
      host === 'pinduoduo.com' ||
      host === 'yangkeduo.com';
    return okHost && !isPinduoduoMobileHomeOnly(raw);
  } catch {
    return false;
  }
}
