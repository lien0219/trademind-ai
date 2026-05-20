import type { Page } from 'playwright';
import { PINDUODUO_PROFILE_KEY } from './profile.js';
import {
  authUrlTypeLabel,
  isPifaHomeOnly,
  isPinduoduoLoginContextUrl,
  isPinduoduoMobileHomeOnly,
  isPinduoduoProductDetailUrl,
  type PinduoduoAuthUrlType,
} from './login-url.js';

export type PinduoduoAuthCheckStatus =
  | 'ok'
  | 'not_logged_in'
  | 'wechat_auth_required'
  | 'verification_required'
  | 'app_redirect'
  | 'homepage_only'
  | 'unknown';

export type PinduoduoAuthPageSignals = {
  loggedInHit: boolean;
  notLoggedInHit: boolean;
  verificationHit: boolean;
  wechatAuthHit: boolean;
  appRedirectHit: boolean;
  pageAnomaly: boolean;
  pageTitle: string;
  pageHref: string;
  bodySnippet: string;
};

export type PinduoduoProductEvidence = {
  hasProductTitle: boolean;
  hasPrice: boolean;
  hasMainImage: boolean;
  hasLoginText: boolean;
  hasWechatAuth: boolean;
  hasAppRedirect: boolean;
};

export type PinduoduoAuthCheckResult = {
  provider: 'pinduoduo';
  profileKey: string;
  status: PinduoduoAuthCheckStatus;
  loginStatus: PinduoduoAuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  checkedUrl: string;
  finalUrl: string;
  accessStatus: string;
  urlType: PinduoduoAuthUrlType;
  checkMode: 'product_detail' | 'homepage_fallback';
  evidence: PinduoduoProductEvidence;
};

const VERIFICATION_PATTERNS =
  /验证码|滑块|安全验证|访问受限|风险验证|人机验证|请完成验证|拖动.*验证|请按住滑块|异常访问/i;

const EXPLICIT_NOT_LOGGED_IN = /请登录|登录后|需要登录|账号登录|手机登录|验证码登录|立即登录/i;

const APP_GUIDE_PATTERNS =
  /用手机浏览器扫码|扫码在拼多多\s*App|在拼多多\s*App\s*打开|打开拼多多\s*App|请使用.*App\s*打开|去\s*App\s*内|下载拼多多/i;

export function isWechatAuthUrl(href: string): boolean {
  const lower = href.trim().toLowerCase();
  return (
    /open\.weixin\.qq\.com/i.test(lower) ||
    /weixin\.qq\.com\/connect/i.test(lower) ||
    /wx\.qq\.com/i.test(lower)
  );
}

export function isPinduoduoFamilyHost(href: string): boolean {
  try {
    const host = new URL(href).hostname.toLowerCase();
    return (
      host.endsWith('.pinduoduo.com') ||
      host.endsWith('.yangkeduo.com') ||
      host === 'pinduoduo.com' ||
      host === 'yangkeduo.com'
    );
  } catch {
    return false;
  }
}

export function isPinduoduoLoginUrl(href: string): boolean {
  if (isWechatAuthUrl(href)) return true;
  try {
    const u = new URL(href);
    const host = u.hostname.toLowerCase();
    const path = `${u.pathname}${u.search}`.toLowerCase();
    if (/passport|login|auth/i.test(host)) return true;
    if (isPinduoduoFamilyHost(href) && /login|passport|auth/i.test(path)) return true;
  } catch {
    return false;
  }
  return false;
}

export async function evaluatePinduoduoAuthPage(page: Page): Promise<PinduoduoAuthPageSignals> {
  return page.evaluate(
    ({ verificationPattern, explicitNotLoggedInSource, appGuideSource }) => {
      const href = typeof location?.href === 'string' ? location.href : '';
      const title = document.title ?? '';
      const body = document.body?.innerText?.slice(0, 8000) ?? '';
      const lowerHref = href.toLowerCase();

      const wechatAuthHit =
        /open\.weixin\.qq\.com/i.test(lowerHref) ||
        /weixin\.qq\.com\/connect/i.test(lowerHref) ||
        /wx\.qq\.com/i.test(lowerHref);

      const appRedirectHit =
        !wechatAuthHit &&
        (new RegExp(appGuideSource, 'i').test(body) ||
          (/(?:^|\.)mobile\.yangkeduo\.com$/i.test(lowerHref.replace(/\/$/, '')) &&
            !/goods_id|gid=/.test(href + body)));

      const pageAnomaly =
        !wechatAuthHit &&
        !appRedirectHit &&
        /404|页面不存在|找不到页面/.test(body) &&
        body.length < 1200 &&
        !/goods_id|gid=/.test(body);

      const verificationHit =
        !wechatAuthHit &&
        !appRedirectHit &&
        (new RegExp(verificationPattern, 'i').test(body) ||
          /captcha|verify|sec\.pinduoduo|security/i.test(lowerHref));

      const onLoginHost =
        /(?:^|\.)passport\.pinduoduo\.com|login\.pinduoduo|\/login|\/passport/i.test(lowerHref);
      const explicitNotLoggedIn = new RegExp(explicitNotLoggedInSource, 'i').test(body);
      const loginButtonWall =
        !wechatAuthHit &&
        !appRedirectHit &&
        /(?:^|\n)\s*(?:请登录|立即登录|账号登录|手机登录)\s*(?:$|\n)/m.test(body) &&
        body.length < 2500;

      const notLoggedInHit =
        !wechatAuthHit && !appRedirectHit && (onLoginHost || explicitNotLoggedIn || loginButtonWall);

      return {
        loggedInHit: false,
        notLoggedInHit,
        verificationHit,
        wechatAuthHit,
        appRedirectHit,
        pageAnomaly,
        pageTitle: title,
        pageHref: href,
        bodySnippet: body.slice(0, 400),
      };
    },
    {
      verificationPattern: VERIFICATION_PATTERNS.source,
      explicitNotLoggedInSource: EXPLICIT_NOT_LOGGED_IN.source,
      appGuideSource: APP_GUIDE_PATTERNS.source,
    },
  );
}

export async function evaluatePinduoduoProductEvidence(page: Page): Promise<PinduoduoProductEvidence> {
  const snap = await page.evaluate(() => {
    const href = typeof location?.href === 'string' ? location.href : '';
    const body = document.body?.innerText?.slice(0, 6000) ?? '';
    const title = document.title?.trim() ?? '';
    const imgs = document.querySelectorAll('img[src], img[data-src]');
    let imgCount = 0;
    for (const img of imgs) {
      const src = (img as HTMLImageElement).src || img.getAttribute('data-src') || '';
      if (src.startsWith('http') && !/logo|icon|avatar|sprite/i.test(src)) imgCount++;
      if (imgCount >= 1) break;
    }
    const priceMatch = body.match(/[¥￥]\s*[\d,.]+/) ?? body.match(/[\d,.]+\s*元/);
    const h1 = document.querySelector('h1')?.textContent?.trim() ?? '';
    const ogTitle = document.querySelector('meta[property="og:title"]')?.getAttribute('content') ?? '';
    const productTitle =
      (h1 && h1.length >= 4 && h1.length < 200 ? h1 : '') ||
      (ogTitle && ogTitle.length >= 4 ? ogTitle : '') ||
      (title && title.length >= 4 && !/拼多多|批发|首页/.test(title) ? title : '');
    return {
      href,
      body,
      productTitle,
      hasPrice: Boolean(priceMatch),
      hasMainImage: imgCount > 0,
    };
  });

  const loginText = EXPLICIT_NOT_LOGGED_IN.test(snap.body);
  const wechat = isWechatAuthUrl(snap.href);
  const appRedirect =
    APP_GUIDE_PATTERNS.test(snap.body) ||
    isPinduoduoMobileHomeOnly(snap.href);

  return {
    hasProductTitle: Boolean(snap.productTitle && snap.productTitle.length >= 4),
    hasPrice: snap.hasPrice,
    hasMainImage: snap.hasMainImage,
    hasLoginText: loginText,
    hasWechatAuth: wechat,
    hasAppRedirect: appRedirect,
  };
}

function accessStatusFromLoginStatus(status: PinduoduoAuthCheckStatus): string {
  switch (status) {
    case 'ok':
      return 'logged_in';
    case 'not_logged_in':
      return 'login_required';
    case 'wechat_auth_required':
      return 'wechat_auth_required';
    case 'verification_required':
      return 'verify_required';
    case 'app_redirect':
      return 'app_redirect';
    case 'homepage_only':
      return 'homepage_only';
    default:
      return 'unknown';
  }
}

export function resolvePinduoduoAuthResult(input: {
  signals: PinduoduoAuthPageSignals;
  evidence: PinduoduoProductEvidence;
  finalUrl: string;
  checkUrl: string;
  checkMode: 'product_detail' | 'homepage_fallback';
}): {
  status: PinduoduoAuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
} {
  const finalUrl = (input.finalUrl || input.signals.pageHref || '').trim();
  const checkUrl = input.checkUrl.trim();
  const { signals, evidence, checkMode } = input;

  if (evidence.hasWechatAuth || isWechatAuthUrl(finalUrl) || signals.wechatAuthHit) {
    return {
      status: 'wechat_auth_required',
      loggedIn: false,
      needVerification: false,
      message:
        '拼多多可能需要微信扫码授权，请在弹出的采集浏览器中完成授权。系统不会保存账号密码。',
    };
  }

  if (
    evidence.hasAppRedirect ||
    signals.appRedirectHit ||
    isPinduoduoMobileHomeOnly(finalUrl)
  ) {
    return {
      status: 'app_redirect',
      loggedIn: false,
      needVerification: false,
      message:
        '当前打开的是拼多多 App 引导页，无法确认采集浏览器是否已登录。请从具体商品链接或失败任务中打开登录，再重新检测。',
    };
  }

  if (signals.pageAnomaly) {
    return {
      status: 'unknown',
      loggedIn: false,
      needVerification: false,
      message: '暂时无法确认登录状态。请确认采集浏览器中是否已完成登录或微信授权，然后使用具体商品链接重新检测。',
    };
  }

  if (signals.verificationHit) {
    return {
      status: 'verification_required',
      loggedIn: false,
      needVerification: true,
      message: '拼多多页面可能出现验证码或安全验证，请在采集浏览器中手动完成验证后重试',
    };
  }

  if (
    isPinduoduoLoginUrl(finalUrl) ||
    signals.notLoggedInHit ||
    evidence.hasLoginText
  ) {
    return {
      status: 'not_logged_in',
      loggedIn: false,
      needVerification: false,
      message: '请先打开采集浏览器登录拼多多，然后使用商品详情链接重新检测',
    };
  }

  if (checkMode === 'homepage_fallback' || isPifaHomeOnly(checkUrl) || isPifaHomeOnly(finalUrl)) {
    return {
      status: 'homepage_only',
      loggedIn: false,
      needVerification: false,
      message:
        '只能访问拼多多首页，无法确认是否已登录。拼多多首页可能游客也能访问。请从失败任务中打开登录，或在采集弹窗输入具体商品链接后重新检测。',
    };
  }

  const onProductPage =
    isPinduoduoProductDetailUrl(finalUrl) || isPinduoduoProductDetailUrl(checkUrl);
  const hasCoreContent =
    evidence.hasProductTitle || evidence.hasPrice || evidence.hasMainImage;

  if (onProductPage && hasCoreContent && isPinduoduoFamilyHost(finalUrl) && !evidence.hasLoginText) {
    return {
      status: 'ok',
      loggedIn: true,
      needVerification: false,
      message: '已检测到拼多多登录态，可尝试采集需要登录的页面',
    };
  }

  if (isPinduoduoLoginContextUrl(checkUrl)) {
    return {
      status: 'unknown',
      loggedIn: false,
      needVerification: false,
      message:
        '暂时无法确认登录状态。请确认采集浏览器中是否已完成登录或微信授权，然后使用具体商品链接重新检测。',
    };
  }

  return {
    status: 'unknown',
    loggedIn: false,
    needVerification: false,
    message:
      '暂时无法确认登录状态。建议从失败任务或采集弹窗打开具体商品链接登录后再检测。',
  };
}

export function buildPinduoduoAuthCheckResult(input: {
  status: PinduoduoAuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  checkedUrl: string;
  finalUrl: string;
  checkMode: 'product_detail' | 'homepage_fallback';
  evidence: PinduoduoProductEvidence;
}): PinduoduoAuthCheckResult {
  const urlType = authUrlTypeLabel(input.checkedUrl, input.finalUrl);
  return {
    provider: 'pinduoduo',
    profileKey: PINDUODUO_PROFILE_KEY,
    status: input.status,
    loginStatus: input.status,
    loggedIn: input.loggedIn,
    needVerification: input.needVerification,
    message: input.message,
    lastCheckedAt: input.lastCheckedAt,
    checkedUrl: input.checkedUrl,
    finalUrl: input.finalUrl,
    accessStatus: accessStatusFromLoginStatus(input.status),
    urlType,
    checkMode: input.checkMode,
    evidence: input.evidence,
  };
}

export function logPinduoduoAuthDebug(payload: Record<string, unknown>): void {
  if (process.env.COLLECTOR_DEBUG_AUTH !== '1') return;
  console.info('[collector][pinduoduo-auth]', JSON.stringify(payload));
}
