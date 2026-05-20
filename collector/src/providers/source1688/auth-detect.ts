import type { BrowserContext, Page } from 'playwright';

export type AuthCheckStatus =
  | 'ok'
  | 'not_logged_in'
  | 'verification_required'
  | 'unknown';

export type AuthPageSignals = {
  loggedInHit: boolean;
  notLoggedInHit: boolean;
  verificationHit: boolean;
  pageAnomaly: boolean;
  pageTitle: string;
  pageHref: string;
  bodySnippet: string;
};

const LOGGED_IN_PATTERNS = [
  /下午好|早上好|晚上好|中午好|你好，/,
  /我的阿里/,
  /采购车/,
  /\btb\d{6,}\b/i,
];

const VERIFICATION_PATTERNS =
  /验证码|滑块|安全验证|验证中心|访问受限|风险验证|人机验证|请完成验证|拖动.*验证|请按住滑块/i;

const EXPLICIT_NOT_LOGGED_IN = /请登录后继续操作|您还未登录|请登录后(?:继续|操作)/;

const AUTH_COOKIE_NAME =
  /^(?:_m_h5_tk|_m_h5_c|cookie2|cna|lid|isg|tfstk|sgcookie|unb|taklid|ali_apache_id|login|token|session)/i;

const AUTH_CHECK_URLS = ['https://www.1688.com/', 'https://work.1688.com/'];

export function getAuthCheckUrls(): string[] {
  const custom = process.env.COLLECTOR_1688_AUTH_CHECK_URL?.trim();
  if (custom) return [custom];
  return AUTH_CHECK_URLS;
}

/** 在页面内评估登录/验证/异常信号（已登录特征优先于「登录」关键词）。 */
export async function evaluateAuthPage(page: Page): Promise<AuthPageSignals> {
  return page.evaluate(
    ({ loggedInPatterns, verificationPattern, explicitNotLoggedInSource }) => {
      const href = typeof location?.href === 'string' ? location.href : '';
      const title = document.title ?? '';
      const body = document.body?.innerText?.slice(0, 8000) ?? '';
      const lowerHref = href.toLowerCase();

      const pageAnomaly =
        /wrongpage\.html|page\.1688\.com\/shtml\/static\/wrongpage/i.test(lowerHref) ||
        ((/404|页面不存在|找不到页面/.test(body) || /404/.test(title)) && body.length < 1200);

      const verificationHit =
        new RegExp(verificationPattern, 'i').test(body) ||
        /punish|x5secdata|captcha|_____tmd_____|sec\.1688\.com.*(?:verify|captcha)/i.test(lowerHref);

      let loggedInHit = false;
      for (const src of loggedInPatterns) {
        if (new RegExp(src, 'i').test(body)) {
          loggedInHit = true;
          break;
        }
      }
      if (!loggedInHit) {
        loggedInHit = !!document.querySelector(
          '[class*="member-account"], [class*="memberAccount"], [class*="user-name"], [class*="userName"], [class*="account-name"], [class*="accountName"], [class*="login-info"], [class*="loginInfo"]',
        );
      }
      if (!loggedInHit && /消息/.test(body) && /采购车|我的阿里|下午好|tb\d/i.test(body)) {
        loggedInHit = true;
      }

      const onLoginHost = /(?:^|\.)login\.1688\.com|passport\.1688\.com/i.test(lowerHref);
      const explicitNotLoggedIn = new RegExp(explicitNotLoggedInSource, 'i').test(body);
      const loginButtonWall =
        !loggedInHit &&
        /(?:^|\n)\s*(?:请登录|立即登录|账号登录)\s*(?:$|\n)/m.test(body) &&
        body.length < 2000;

      const notLoggedInHit = !loggedInHit && (onLoginHost || explicitNotLoggedIn || loginButtonWall);

      return {
        loggedInHit,
        notLoggedInHit,
        verificationHit,
        pageAnomaly,
        pageTitle: title,
        pageHref: href,
        bodySnippet: body.slice(0, 240),
      };
    },
    {
      loggedInPatterns: LOGGED_IN_PATTERNS.map((r) => r.source),
      verificationPattern: VERIFICATION_PATTERNS.source,
      explicitNotLoggedInSource: EXPLICIT_NOT_LOGGED_IN.source,
    },
  );
}

export async function countAuthCookies(context: BrowserContext): Promise<number> {
  const cookies = await context.cookies();
  return cookies.filter(
    (c) =>
      /\.1688\.com|\.alibaba\.com/i.test(c.domain) && AUTH_COOKIE_NAME.test(c.name),
  ).length;
}

export function resolveAuthResult(input: {
  signals: AuthPageSignals;
  authCookieCount: number;
}): { status: AuthCheckStatus; loggedIn: boolean; needVerification: boolean; message: string } {
  const { signals, authCookieCount } = input;

  if (signals.pageAnomaly) {
    return {
      status: 'unknown',
      loggedIn: false,
      needVerification: false,
      message: '检测页异常，请重新检测',
    };
  }

  if (signals.verificationHit) {
    return {
      status: 'verification_required',
      loggedIn: false,
      needVerification: true,
      message: '1688 需要完成安全验证',
    };
  }

  const loggedIn = signals.loggedInHit || authCookieCount > 0;
  if (loggedIn) {
    return {
      status: 'ok',
      loggedIn: true,
      needVerification: false,
      message: '1688 登录态正常',
    };
  }

  if (signals.notLoggedInHit) {
    return {
      status: 'not_logged_in',
      loggedIn: false,
      needVerification: false,
      message: '未登录',
    };
  }

  return {
    status: 'unknown',
    loggedIn: false,
    needVerification: false,
    message: '检测页异常，请重新检测',
  };
}

export type AuthDebugLog = {
  userDataDir: string;
  checkUrl: string;
  finalUrl: string;
  pageTitle: string;
  loggedInHit: boolean;
  notLoggedInHit: boolean;
  verificationHit: boolean;
  pageAnomaly: boolean;
  cookieCount: number;
  authCookieCount: number;
  result: AuthCheckStatus;
  message: string;
};

export function logAuthDebug(entry: AuthDebugLog): void {
  console.info('[1688-auth-status]', JSON.stringify(entry));
}
