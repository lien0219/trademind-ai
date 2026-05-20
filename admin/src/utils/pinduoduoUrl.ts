export type PinduoduoUrlType =
  | 'goods_detail'
  | 'wholesale_detail'
  | 'login_page'
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
    if (!isPddHost(host)) return 'unknown';

    if (/login|passport|auth/i.test(path)) return 'login_page';

    if (isPifaHost(host)) {
      const gid = u.searchParams.get('gid') ?? u.searchParams.get('goods_id');
      if (path.includes('goods') && gid && /^\d+$/.test(gid)) return 'wholesale_detail';
      if (path.includes('goods')) return 'wholesale_detail';
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
    case 'goods_detail':
      return '已识别为拼多多商品详情页，可开始采集。';
    case 'wholesale_detail':
      return '该链接属于拼多多批发页，可能需要登录后才能采集。建议优先使用普通商品详情页链接；如需采集该链接，请先使用采集浏览器登录拼多多。';
    case 'login_page':
    case 'app_redirect':
    case 'unknown':
      return '请输入拼多多商品详情页链接。';
    default:
      return null;
  }
}

export function pinduoduoProfileDomain(): string {
  return 'pinduoduo.com';
}

/** 移动端首页（无商品参数），不适合作为登录入口。 */
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

/** 打开登录 / 重新检测时优先使用的上下文 URL（排除移动端 App 首页）。 */
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
