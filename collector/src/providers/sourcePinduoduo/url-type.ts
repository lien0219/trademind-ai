import { isPinduoduoHost } from './validate-url.js';

export type PinduoduoUrlType =
  | 'goods_detail'
  | 'wholesale_detail'
  | 'wholesale_homepage'
  | 'login_page'
  | 'wechat_auth'
  | 'app_redirect'
  | 'unknown';

export function isPifaWholesaleHost(hostname: string): boolean {
  const h = hostname.trim().toLowerCase();
  return h === 'pifa.pinduoduo.com' || h.endsWith('.pifa.pinduoduo.com');
}

/** Classify拼多多链接类型（不访问页面，仅 URL 语义）。 */
export function classifyPinduoduoUrl(urlStr: string): PinduoduoUrlType {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return 'unknown';
    const host = u.hostname.toLowerCase();
    const path = u.pathname.toLowerCase();
    const search = u.search.toLowerCase();

    if (/weixin\.qq\.com|open\.weixin/i.test(host)) {
      return 'wechat_auth';
    }

    if (!isPinduoduoHost(host) && !isPifaWholesaleHost(host)) {
      return 'unknown';
    }

    if (/login|passport|auth/i.test(path) || /login|passport/i.test(search)) {
      return 'login_page';
    }

    if (isPifaWholesaleHost(host)) {
      const gid = u.searchParams.get('gid') ?? u.searchParams.get('goods_id');
      const barePath = path.replace(/\/$/, '') || '/';
      if (barePath === '/' || barePath === '/index.html') {
        return 'wholesale_homepage';
      }
      if (path.includes('goods') && gid && /^\d+$/.test(gid)) {
        return 'wholesale_detail';
      }
      if (path.includes('goods/detail')) {
        return 'wholesale_detail';
      }
      if (!path.includes('goods')) {
        return 'wholesale_homepage';
      }
      return 'unknown';
    }

    const goodsId = u.searchParams.get('goods_id') ?? u.searchParams.get('goodsId');
    const isGoodsPath =
      path.includes('goods') || path.includes('goods_detail') || path.includes('comm_goods');
    if (isGoodsPath && goodsId && /^\d+$/.test(goodsId)) {
      return 'goods_detail';
    }
    if (isGoodsPath && /(?:^|[?&])goods_id=\d+/i.test(u.search)) {
      return 'goods_detail';
    }

    if (/app|download|redirect/i.test(path)) {
      return 'app_redirect';
    }

    return 'unknown';
  } catch {
    return 'unknown';
  }
}

export function pinduoduoUrlTypeLabel(urlType: PinduoduoUrlType): string {
  switch (urlType) {
    case 'goods_detail':
      return '普通商品页（移动端）';
    case 'wholesale_detail':
      return '拼多多批发详情页';
    case 'wholesale_homepage':
      return '拼多多批发首页';
    case 'login_page':
      return '登录页';
    case 'wechat_auth':
      return '微信授权页';
    case 'app_redirect':
      return 'App 引导页';
    default:
      return '未识别';
  }
}

export function unsupportedPinduoduoUrlMessage(urlType: PinduoduoUrlType): string {
  switch (urlType) {
    case 'goods_detail':
      return '当前版本优先支持拼多多批发详情页（pifa.pinduoduo.com/goods/detail）。移动端商品页暂未完整支持，请换用批发详情链接。';
    case 'wholesale_homepage':
      return '该链接为拼多多批发首页，不是商品详情页。请输入带 gid 的商品详情链接。';
    case 'login_page':
      return '该链接为登录页，请使用商品详情链接并在采集浏览器中完成登录。';
    case 'wechat_auth':
      return '该链接为微信授权页，请在采集浏览器中完成扫码授权后再采集商品。';
    case 'app_redirect':
      return '该链接为 App 引导页，请换用拼多多批发商品详情链接。';
    default:
      return invalidPinduoduoUrlHint();
  }
}

export function wholesaleLoginSuggestion(): string {
  return '该链接属于拼多多批发页，可能需要登录后才能采集。建议先使用公开商品详情页链接，或打开采集浏览器登录后重试。';
}

export function goodsDetailReadyHint(): string {
  return '已识别为拼多多商品详情页，可开始采集。';
}

export function wholesaleUrlHint(): string {
  return '该链接属于拼多多批发页，可能需要登录后才能采集。建议优先使用普通商品详情页链接；如需采集该链接，请先使用采集浏览器登录拼多多。';
}

export function invalidPinduoduoUrlHint(): string {
  return '请输入拼多多商品详情页链接。';
}
