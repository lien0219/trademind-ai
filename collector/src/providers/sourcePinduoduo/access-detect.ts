import type { Page } from 'playwright';
import { CustomCollectError } from '../sourceCustom/errors.js';
import type { CustomAccessReport } from '../../types/access-status.js';
import {
  evaluateGenericPageAccess,
  resolveAccessStatusFromSignals,
  accessStatusToErrorCode,
} from '../sourceCustom/access-detect.js';
import { isPinduoduoHost } from './validate-url.js';
import { classifyPinduoduoUrl, type PinduoduoUrlType, wholesaleLoginSuggestion } from './url-type.js';

const APP_GUIDE_RE =
  /打开.*app|请在app|下载拼多多|去app内|打开拼多多|扫码下载|请在客户端|继续访问/i;
const NOT_FOUND_RE = /商品不存在|已下架|找不到商品|页面不存在|404|商品已售罄/i;
const PDD_LOGIN_HOST_RE = /passport|login|auth/i;
const PIFA_LOGIN_TEXT_RE = /请登录|登录后|需要登录|账号登录|手机登录|验证码登录/i;

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
  urlType?: PinduoduoUrlType;
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

function loginSuggestionForUrlType(urlType: PinduoduoUrlType): string {
  if (urlType === 'wholesale_detail') {
    return wholesaleLoginSuggestion();
  }
  return '该页面需要登录后才能采集。请打开采集浏览器登录拼多多后重试，或换用公开商品详情页链接。';
}

function toAccessStatus(status: PinduoduoAccessStatus): CustomAccessReport['accessStatus'] {
  if (status === 'app_guide' || status === 'not_found') {
    return status === 'not_found' ? 'blocked' : 'verify_required';
  }
  return status;
}

function toCustomAccessReport(
  report: PinduoduoAccessReport,
  sourceUrl: string,
): CustomAccessReport {
  const urlType = report.urlType ?? classifyPinduoduoUrl(sourceUrl);
  const code =
    report.errorCode ??
    (report.status === 'app_guide' || report.status === 'not_found'
      ? 'PAGE_BLOCKED_OR_VERIFY_REQUIRED'
      : accessStatusToErrorCode(toAccessStatus(report.status)));
  return {
    accessStatus: toAccessStatus(report.status),
    finalUrl: report.finalUrl,
    extractedFields: {
      title: false,
      price: false,
      mainImage: false,
      detailImagesCount: 0,
      attributesCount: 0,
    },
    missingFields: ['title'],
    warnings: [],
    errorCode: code,
    suggestion: loginSuggestionForUrlType(urlType),
  };
}

export async function detectPinduoduoAccessStatus(
  page: Page,
  sourceUrl: string,
): Promise<PinduoduoAccessReport> {
  const urlType = classifyPinduoduoUrl(sourceUrl);
  const finalUrl = page.url();
  const signals = await evaluateGenericPageAccess(page);
  const snap = await page.evaluate(() => ({
    title: document.title?.trim() ?? '',
    body: document.body?.innerText?.slice(0, 5000) ?? '',
  }));

  const base = { finalUrl, pageTitle: snap.title, bodySnippet: snap.body.slice(0, 400), urlType };

  if (isPinduoduoLoginUrl(finalUrl) || signals.loginRedirect) {
    return {
      ...base,
      status: 'login_required',
      errorCode: 'LOGIN_REQUIRED',
    };
  }

  if (
    urlType === 'wholesale_detail' &&
    (PIFA_LOGIN_TEXT_RE.test(snap.body) || PIFA_LOGIN_TEXT_RE.test(snap.title)) &&
    !/goods_id|商品详情|¥|￥/.test(snap.body)
  ) {
    return {
      ...base,
      status: 'login_required',
      errorCode: 'LOGIN_REQUIRED',
    };
  }

  if (signals.verifyRedirect || signals.verifyTextHit) {
    return {
      ...base,
      status: 'verify_required',
      errorCode: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED',
    };
  }

  if (NOT_FOUND_RE.test(snap.body) && snap.body.length < 1200) {
    return {
      ...base,
      status: 'not_found',
      errorCode: 'PRODUCT_NOT_FOUND',
    };
  }

  if (APP_GUIDE_RE.test(snap.body) && snap.body.length < 3500) {
    const hasProductHint = /goods_id|商品详情|¥|￥|gid=/.test(snap.body);
    if (!hasProductHint) {
      return {
        ...base,
        status: 'app_guide',
        errorCode: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED',
      };
    }
  }

  const generic = resolveAccessStatusFromSignals(signals);
  if (generic !== 'public') {
    return {
      ...base,
      status: generic as PinduoduoAccessStatus,
      errorCode: accessStatusToErrorCode(generic),
    };
  }

  return {
    ...base,
    status: 'public',
  };
}

export function throwAccessError(report: PinduoduoAccessReport, sourceUrl: string): never {
  const code = report.errorCode ?? 'PAGE_BLOCKED_OR_VERIFY_REQUIRED';
  const custom = toCustomAccessReport(report, sourceUrl);
  throw new CustomCollectError(code, custom, `${code}:${report.status}`);
}
