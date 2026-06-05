import type { Page } from 'playwright';
import { evaluateTaobaoAuthPage, resolveTaobaoAuthResult } from './auth-detect.js';

const NOT_FOUND_RE = /商品不存在|已下架|找不到该商品|页面不存在|404|宝贝不存在|该商品已失效/i;
const ACCESS_DENIED_RE = /访问被拒绝|无权访问|拒绝访问|access denied/i;

export type TaobaoAccessStatus =
  | 'public'
  | 'login_required'
  | 'verify_required'
  | 'not_found'
  | 'access_denied';

export type TaobaoAccessReport = {
  status: TaobaoAccessStatus;
  finalUrl: string;
  pageTitle: string;
  bodySnippet: string;
  errorCode?: string;
};

export async function detectTaobaoAccessStatus(
  page: Page,
  _sourceUrl: string,
): Promise<TaobaoAccessReport> {
  const finalUrl = page.url();
  const pageTitle = await page.title().catch(() => '');
  const bodySnippet = await page
    .evaluate(() => document.body?.innerText?.slice(0, 1200) ?? '')
    .catch(() => '');

  if (NOT_FOUND_RE.test(bodySnippet) && bodySnippet.length < 2500) {
    return {
      status: 'not_found',
      finalUrl,
      pageTitle,
      bodySnippet: bodySnippet.slice(0, 240),
      errorCode: 'ITEM_NOT_FOUND',
    };
  }
  if (ACCESS_DENIED_RE.test(bodySnippet)) {
    return {
      status: 'access_denied',
      finalUrl,
      pageTitle,
      bodySnippet: bodySnippet.slice(0, 240),
      errorCode: 'ACCESS_DENIED',
    };
  }

  const signals = await evaluateTaobaoAuthPage(page);
  const resolved = resolveTaobaoAuthResult(signals);

  if (resolved.status === 'verify_required') {
    return {
      status: 'verify_required',
      finalUrl,
      pageTitle,
      bodySnippet: bodySnippet.slice(0, 240),
      errorCode: 'VERIFY_REQUIRED',
    };
  }
  if (resolved.status === 'login_required') {
    return {
      status: 'login_required',
      finalUrl,
      pageTitle,
      bodySnippet: bodySnippet.slice(0, 240),
      errorCode: 'LOGIN_REQUIRED',
    };
  }

  return {
    status: 'public',
    finalUrl,
    pageTitle,
    bodySnippet: bodySnippet.slice(0, 240),
  };
}

export function throwAccessError(report: TaobaoAccessReport): never {
  const code = report.errorCode ?? 'UNKNOWN_COLLECT_ERROR';
  throw new Error(`${code}:${report.bodySnippet || report.pageTitle || code}`);
}
