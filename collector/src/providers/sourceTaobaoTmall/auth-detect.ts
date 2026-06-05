import type { Page } from 'playwright';

export type TaobaoAuthCheckStatus =
  | 'logged_in'
  | 'login_required'
  | 'verify_required'
  | 'unknown';

export type TaobaoAuthPageSignals = {
  loggedInHit: boolean;
  loginRequiredHit: boolean;
  verifyRequiredHit: boolean;
  pageAnomaly: boolean;
  pageTitle: string;
  pageHref: string;
  bodySnippet: string;
};

const VERIFICATION_PATTERNS =
  /验证码|滑块|安全验证|验证中心|访问受限|风险验证|人机验证|请完成验证|拖动.*验证|请按住滑块|punish|x5secdata|captcha|_____tmd_____/i;

const LOGIN_URL_PATTERNS =
  /(?:^|\.)login\.(?:taobao|tmall)\.com|passport\.(?:taobao|tmall)\.com|login\.m\.taobao\.com/i;

const LOGGED_IN_PATTERNS = [
  /我的淘宝/,
  /已登录/,
  /会员名/,
  /nick\s*[:：]/i,
  /Hi,\s*\S+/,
];

export async function evaluateTaobaoAuthPage(page: Page): Promise<TaobaoAuthPageSignals> {
  return page.evaluate(
    ({ loggedInPatterns, verificationPattern, loginUrlPattern }) => {
      const href = typeof location?.href === 'string' ? location.href : '';
      const title = document.title ?? '';
      const body = document.body?.innerText?.slice(0, 8000) ?? '';
      const lowerHref = href.toLowerCase();

      const pageAnomaly =
        /404|页面不存在|找不到页面|商品不存在|已下架/.test(body) &&
        body.length < 2000 &&
        !document.querySelector('[class*="ItemHeader"], [class*="MainTitle"], #J_Title, h1');

      const verifyRequiredHit =
        new RegExp(verificationPattern, 'i').test(body) ||
        /punish|x5secdata|captcha|_____tmd_____|sec\.taobao\.com|sec\.tmall\.com/i.test(lowerHref);

      let loggedInHit = false;
      for (const src of loggedInPatterns) {
        if (new RegExp(src, 'i').test(body)) {
          loggedInHit = true;
          break;
        }
      }
      if (!loggedInHit) {
        loggedInHit = !!document.querySelector(
          '[class*="member"], [class*="Member"], [class*="user-nick"], [class*="UserNick"], a[href*="member.taobao"], a[href*="i.taobao.com/my"]',
        );
      }

      const onLoginHost = new RegExp(loginUrlPattern, 'i').test(lowerHref);
      const explicitLogin =
        /请登录|立即登录|账号登录|登录后查看|登录淘宝|登录天猫/.test(body) &&
        !loggedInHit &&
        body.length < 4000;
      const loginRequiredHit = !loggedInHit && (onLoginHost || explicitLogin);

      return {
        loggedInHit,
        loginRequiredHit,
        verifyRequiredHit,
        pageAnomaly,
        pageTitle: title,
        pageHref: href,
        bodySnippet: body.slice(0, 240),
      };
    },
    {
      loggedInPatterns: LOGGED_IN_PATTERNS.map((r) => r.source),
      verificationPattern: VERIFICATION_PATTERNS.source,
      loginUrlPattern: LOGIN_URL_PATTERNS.source,
    },
  );
}

export function resolveTaobaoAuthResult(signals: TaobaoAuthPageSignals): {
  status: TaobaoAuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
} {
  if (signals.verifyRequiredHit) {
    return {
      status: 'verify_required',
      loggedIn: false,
      needVerification: true,
      message: '页面出现安全验证、滑块或验证码，请在采集浏览器中手动完成验证后重试。',
    };
  }
  if (signals.loginRequiredHit) {
    return {
      status: 'login_required',
      loggedIn: false,
      needVerification: false,
      message: '页面需要登录后才能访问商品详情，请打开淘宝/天猫采集浏览器完成登录后重试。',
    };
  }
  if (signals.loggedInHit) {
    return {
      status: 'logged_in',
      loggedIn: true,
      needVerification: false,
      message: '已检测到登录态，可尝试采集需要登录的商品。',
    };
  }
  if (signals.pageAnomaly) {
    return {
      status: 'unknown',
      loggedIn: false,
      needVerification: false,
      message: '检测页异常或无法确认登录状态，请使用具体商品链接重新检测。',
    };
  }
  return {
    status: 'unknown',
    loggedIn: false,
    needVerification: false,
    message: '暂时无法确认登录状态。若商品页可正常浏览，可直接尝试采集；若跳转登录页请先登录。',
  };
}

export type TaobaoAuthCheckResult = {
  provider: string;
  profileKey: string;
  status: TaobaoAuthCheckStatus;
  loginStatus: TaobaoAuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  checkedUrl?: string;
  finalUrl?: string;
};

export function buildTaobaoAuthCheckResult(input: {
  status: TaobaoAuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  checkedUrl?: string;
  finalUrl?: string;
}): TaobaoAuthCheckResult {
  return {
    provider: 'taobao_tmall',
    profileKey: 'taobao_tmall',
    status: input.status,
    loginStatus: input.status,
    loggedIn: input.loggedIn,
    needVerification: input.needVerification,
    message: input.message,
    lastCheckedAt: input.lastCheckedAt,
    checkedUrl: input.checkedUrl,
    finalUrl: input.finalUrl,
  };
}

export function logTaobaoAuthDebug(info: Record<string, unknown>): void {
  console.info('[collector][taobao_tmall][auth]', JSON.stringify(info));
}
