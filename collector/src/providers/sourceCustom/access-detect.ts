import type { Page } from 'playwright';
import type { AccessStatus } from '../../types/access-status.js';
import { isCaptchaRedirectUrl, isLoginRedirectUrl, isVerificationPlaceholderTitle } from '../shared/page-guard.js';

const LOGIN_TEXT_RE =
  /(?:^|\s)(?:请登录|立即登录|账号登录|密码登录|扫码登录|欢迎登录|sign\s*in|log\s*in|login)(?:\s|$)/i;
const VERIFY_TEXT_RE =
  /验证码|安全验证|滑块|人机验证|captcha|verify\s+you|please\s+verify|拖动.*验证/i;
const BLOCKED_TEXT_RE = /access\s+denied|forbidden|访问受限|403\s+forbidden|permission\s+denied/i;

export type GenericAccessSignals = {
  finalUrl: string;
  httpStatus?: number;
  pageTitle: string;
  bodySnippet: string;
  loginRedirect: boolean;
  verifyRedirect: boolean;
  loginTextHit: boolean;
  verifyTextHit: boolean;
  blockedTextHit: boolean;
};

export async function evaluateGenericPageAccess(
  page: Page,
  httpStatus?: number,
): Promise<GenericAccessSignals> {
  const finalUrl = page.url();
  const loginRedirect = isLoginRedirectUrl(finalUrl);
  const verifyRedirect = isCaptchaRedirectUrl(finalUrl);

  const snap = await page.evaluate(() => {
    const title = document.title?.trim() ?? '';
    const body = document.body?.innerText?.slice(0, 5000) ?? '';
    return { title, body };
  });

  const body = snap.body;
  const loginTextHit =
    LOGIN_TEXT_RE.test(body) ||
    LOGIN_TEXT_RE.test(snap.title) ||
    (/登录/.test(body) && body.length < 2800);
  const verifyTextHit = VERIFY_TEXT_RE.test(body) || VERIFY_TEXT_RE.test(snap.title);
  const blockedTextHit =
    BLOCKED_TEXT_RE.test(body) ||
    httpStatus === 401 ||
    httpStatus === 403;

  return {
    finalUrl,
    httpStatus,
    pageTitle: snap.title,
    bodySnippet: body.slice(0, 400),
    loginRedirect,
    verifyRedirect,
    loginTextHit,
    verifyTextHit,
    blockedTextHit,
  };
}

export function resolveAccessStatusFromSignals(signals: GenericAccessSignals): AccessStatus {
  if (signals.verifyRedirect || signals.verifyTextHit) {
    return 'verify_required';
  }
  if (signals.loginRedirect || signals.loginTextHit) {
    return 'login_required';
  }
  if (signals.blockedTextHit) {
    return 'blocked';
  }
  if (isVerificationPlaceholderTitle(signals.pageTitle)) {
    return 'verify_required';
  }
  return 'public';
}

export function accessStatusToErrorCode(status: AccessStatus): string | undefined {
  switch (status) {
    case 'login_required':
      return 'LOGIN_REQUIRED';
    case 'verify_required':
      return 'PAGE_BLOCKED_OR_VERIFY_REQUIRED';
    case 'blocked':
      return 'PAGE_BLOCKED_OR_VERIFY_REQUIRED';
    case 'timeout':
      return 'TIMEOUT';
    case 'navigation_failed':
      return 'NAVIGATION_FAILED';
    default:
      return undefined;
  }
}

export function buildAccessSuggestion(
  status: AccessStatus,
  missingFields: string[],
): string {
  const parts: string[] = [];
  switch (status) {
    case 'login_required':
      parts.push(
        '页面疑似需要登录，请确认该商品页是否公开可访问；自定义采集器使用未登录浏览器，带登录态 Profile 能力预留后续扩展。',
      );
      break;
    case 'verify_required':
      parts.push('页面疑似触发验证码或风控，请稍后重试或降低采集频率。');
      break;
    case 'blocked':
      parts.push('页面返回访问受限（401/403 或 Access denied），请确认链接权限。');
      break;
    case 'timeout':
      parts.push('页面加载超时，请检查网络或稍后重试。');
      break;
    case 'navigation_failed':
      parts.push('页面无法打开，请检查链接是否有效。');
      break;
    default:
      break;
  }
  if (missingFields.includes('title')) {
    parts.push('标题未提取到，请检查 title selector。');
  }
  if (missingFields.includes('mainImage')) {
    parts.push('主图未提取到，请检查 mainImage / mainImages selector。');
  }
  if (missingFields.includes('price')) {
    parts.push('价格未提取到，请检查 price selector（可选）。');
  }
  if (status === 'public' && parts.length === 0 && missingFields.length === 0) {
    parts.push('页面可访问，核心字段已提取。');
  }
  return parts.join(' ') || '请根据 missingFields 调整采集规则选择器。';
}
