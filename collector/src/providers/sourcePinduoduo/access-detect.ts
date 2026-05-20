import type { Page } from 'playwright';
import {
  evaluateGenericPageAccess,
  resolveAccessStatusFromSignals,
  accessStatusToErrorCode,
} from '../sourceCustom/access-detect.js';
import { isPinduoduoHost } from './validate-url.js';

const APP_GUIDE_RE =
  /打开.*app|请在app|下载拼多多|去app内|打开拼多多|扫码下载|请在客户端|继续访问/i;
const NOT_FOUND_RE = /商品不存在|已下架|找不到商品|页面不存在|404|商品已售罄/i;
const PDD_LOGIN_HOST_RE = /passport|login|auth/i;

export type PinduoduoAccessStatus =
  | 'public'
  | 'login_required'
  | 'verify_required'
  | 'blocked'
  | 'app_guide'
  | 'not_found';

export type PinduoduoAccessReport = {
  status: PinduoduoAccessStatus;
  finalUrl: string;
  pageTitle: string;
  bodySnippet: string;
  errorCode?: string;
};

function isPinduoduoLoginUrl(href: string): boolean {
  try {
    const u = new URL(href);
    const host = u.hostname.toLowerCase();
    const path = `${u.pathname}${u.search}`.toLowerCase();
    if (PDD_LOGIN_HOST_RE.test(host) || PDD_LOGIN_HOST_RE.test(path)) return true;
    if (isPinduoduoHost(host) && /login|passport|auth/i.test(path)) return true;
  } catch {
    return false;
  }
  return false;
}

export async function detectPinduoduoAccessStatus(page: Page): Promise<PinduoduoAccessReport> {
  const finalUrl = page.url();
  const signals = await evaluateGenericPageAccess(page);
  const snap = await page.evaluate(() => ({
    title: document.title?.trim() ?? '',
    body: document.body?.innerText?.slice(0, 5000) ?? '',
  }));

  if (isPinduoduoLoginUrl(finalUrl) || signals.loginRedirect) {
    return {
      status: 'login_required',
      finalUrl,
      pageTitle: snap.title,
      bodySnippet: snap.body.slice(0, 400),
      errorCode: 'LOGIN_REQUIRED',
    };
  }

  if (signals.verifyRedirect || signals.verifyTextHit) {
    return {
      status: 'verify_required',
      finalUrl,
      pageTitle: snap.title,
      bodySnippet: snap.body.slice(0, 400),
      errorCode: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED',
    };
  }

  if (NOT_FOUND_RE.test(snap.body) && snap.body.length < 1200) {
    return {
      status: 'not_found',
      finalUrl,
      pageTitle: snap.title,
      bodySnippet: snap.body.slice(0, 400),
      errorCode: 'PRODUCT_NOT_FOUND',
    };
  }

  if (APP_GUIDE_RE.test(snap.body) && snap.body.length < 3500) {
    const hasProductHint = /goods_id|商品详情|¥|￥/.test(snap.body);
    if (!hasProductHint) {
      return {
        status: 'app_guide',
        finalUrl,
        pageTitle: snap.title,
        bodySnippet: snap.body.slice(0, 400),
        errorCode: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED',
      };
    }
  }

  const generic = resolveAccessStatusFromSignals(signals);
  if (generic !== 'public') {
    return {
      status: generic as PinduoduoAccessStatus,
      finalUrl,
      pageTitle: snap.title,
      bodySnippet: snap.body.slice(0, 400),
      errorCode: accessStatusToErrorCode(generic),
    };
  }

  return {
    status: 'public',
    finalUrl,
    pageTitle: snap.title,
    bodySnippet: snap.body.slice(0, 400),
  };
}

export function throwAccessError(report: PinduoduoAccessReport): never {
  const code = report.errorCode ?? 'PAGE_BLOCKED_OR_VERIFY_REQUIRED';
  throw new Error(`${code}:${report.status}`);
}
