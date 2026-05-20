import type { Page, Response } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import type { CustomAccessReport, ExtractedFieldsSummary } from '../../types/access-status.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import { prepare1688OfferPage } from '../source1688/page-prep.js';
import { assert1688PageCollectible, is1688CollectHost } from '../shared/page-guard.js';
import {
  accessStatusToErrorCode,
  buildAccessSuggestion,
  evaluateGenericPageAccess,
  resolveAccessStatusFromSignals,
} from './access-detect.js';
import type { CustomCollectOptions, CustomRuleDecl } from './types.js';
import { normalizeCustomRuleDecl } from './normalize-rule.js';
import { parseCustomProduct } from './parser.js';
import type { NormalizedProduct } from '../../types/product.js';
import { CustomCollectError, throwCustomError } from './errors.js';

export type CustomRunMode = 'task' | 'rule_test';

export type CustomRunResult = {
  product?: NormalizedProduct;
  report: CustomAccessReport;
};

function fieldSummary(product: NormalizedProduct | undefined, rule: CustomRuleDecl): {
  extracted: ExtractedFieldsSummary;
  missing: string[];
  warnings: string[];
} {
  const extracted: ExtractedFieldsSummary = {
    title: false,
    price: false,
    mainImage: false,
    detailImagesCount: 0,
    attributesCount: 0,
  };
  const missing: string[] = [];
  const warnings: string[] = [];

  const title = product?.title?.trim() ?? '';
  if (title) extracted.title = true;
  else missing.push('title');

  const priceVal = product?.raw?.productPrice;
  if (typeof priceVal === 'number' && priceVal > 0) extracted.price = true;
  else if ((rule as { price?: { selectors?: string[] } }).price?.selectors?.length) {
    missing.push('price');
  }

  const mainN = product?.mainImages?.length ?? 0;
  extracted.mainImage = mainN > 0;
  if (!extracted.mainImage) missing.push('mainImage');
  extracted.detailImagesCount = product?.descriptionImages?.length ?? 0;

  const attrN = product?.attributes ? Object.keys(product.attributes).length : 0;
  extracted.attributesCount = attrN;

  if (extracted.title && !extracted.mainImage) {
    warnings.push('title_extracted_but_no_main_image');
  }

  return { extracted, missing, warnings };
}

function attachReportToProduct(
  product: NormalizedProduct,
  report: CustomAccessReport,
  opts: CustomCollectOptions,
): NormalizedProduct {
  return {
    ...product,
    raw: {
      ...(product.raw ?? {}),
      accessReport: report,
      customUseBrowserProfile: Boolean(opts.useBrowserProfile && opts.profileKey),
      customBrowserProfileName: opts.profileKey ?? '',
      customCookieProfileId: opts.profileId ?? '',
    },
  };
}

function buildReport(
  signals: Awaited<ReturnType<typeof evaluateGenericPageAccess>>,
  extracted: ExtractedFieldsSummary,
  missing: string[],
  warnings: string[],
  overrideStatus?: import('../../types/access-status.js').AccessStatus,
): CustomAccessReport {
  let accessStatus = overrideStatus ?? resolveAccessStatusFromSignals(signals);
  if (accessStatus === 'public' && missing.includes('title')) {
    accessStatus = 'unknown';
  }
  const errorCode = accessStatusToErrorCode(accessStatus);
  let suggestion = buildAccessSuggestion(accessStatus, missing);
  if (accessStatus === 'public' && missing.length > 0) {
    suggestion = buildAccessSuggestion('unknown', missing);
  }
  return {
    accessStatus,
    finalUrl: signals.finalUrl,
    httpStatus: signals.httpStatus,
    extractedFields: extracted,
    missingFields: missing,
    warnings,
    errorCode:
      errorCode ??
      (missing.includes('title') ? 'PARSE_FAILED_TITLE_MISSING' : undefined),
    suggestion,
    customUseBrowserProfile: false,
    customBrowserProfileName: '',
    customCookieProfileId: '',
  };
}

async function navigatePage(page: Page, urlStr: string): Promise<{ httpStatus?: number; navError?: string }> {
  const gotoTimeout = getDefaultNavigationTimeoutMs();
  let httpStatus: number | undefined;
  try {
    const resp: Response | null = await page.goto(urlStr, {
      waitUntil: 'domcontentloaded',
      timeout: gotoTimeout,
    });
    httpStatus = resp?.status();
  } catch (e) {
    const err = e instanceof Error ? e.message : String(e);
    if (/timeout/i.test(err)) {
      return { navError: `TIMEOUT:${err}` };
    }
    return { navError: `NAVIGATION_FAILED:${err}` };
  }
  await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);
  return { httpStatus };
}

export async function runCustomCollect(
  browser: BrowserManager,
  urlStr: string,
  opts: CustomCollectOptions,
  mode: CustomRunMode,
): Promise<CustomRunResult> {
  const rule = normalizeCustomRuleDecl(opts.rule);
  if (!rule.title?.selectors?.length) {
    throw new Error('CUSTOM_RULE_INVALID:title selector is required');
  }

  const host = (() => {
    try {
      return new URL(urlStr).hostname.toLowerCase();
    } catch {
      return '';
    }
  })();
  const use1688Session = is1688CollectHost(host);

  const run = async (page: Page): Promise<CustomRunResult> => {
    const { httpStatus, navError } = await navigatePage(page, urlStr);
    if (navError) {
      const isTimeout = navError.startsWith('TIMEOUT:');
      const report: CustomAccessReport = {
        accessStatus: isTimeout ? 'timeout' : 'navigation_failed',
        finalUrl: urlStr,
        extractedFields: {
          title: false,
          price: false,
          mainImage: false,
          detailImagesCount: 0,
          attributesCount: 0,
        },
        missingFields: ['title', 'mainImage'],
        warnings: [],
        errorCode: isTimeout ? 'TIMEOUT' : 'NAVIGATION_FAILED',
        suggestion: buildAccessSuggestion(isTimeout ? 'timeout' : 'navigation_failed', [
          'title',
          'mainImage',
        ]),
      };
      if (mode === 'rule_test') return { report };
      throw new CustomCollectError(report.errorCode ?? 'NAVIGATION_FAILED', report, navError);
    }

    if (use1688Session) {
      await prepare1688OfferPage(page, false);
    } else {
      await page.waitForSelector('body', { timeout: 8000 }).catch(() => undefined);
      await page
        .evaluate(() => {
          window.scrollTo(0, Math.min(600, document.body.scrollHeight));
        })
        .catch(() => undefined);
      await new Promise((r) => setTimeout(r, 400));
    }

    const signals = await evaluateGenericPageAccess(page, httpStatus);
    let accessStatus = resolveAccessStatusFromSignals(signals);

    if (use1688Session && accessStatus === 'public') {
      try {
        await assert1688PageCollectible(page);
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        if (/PAGE_BLOCKED|VERIFY|verification/i.test(msg)) {
          accessStatus = 'verify_required';
        }
      }
    }

    let product: NormalizedProduct | undefined;
    try {
      product = await parseCustomProduct(page, urlStr, rule);
    } catch {
      product = undefined;
    }

    const { extracted, missing, warnings } = fieldSummary(product, rule);
    const report = buildReport(signals, extracted, missing, warnings, accessStatus);

    if (mode === 'rule_test') {
      if (product?.title?.trim()) {
        return { product: attachReportToProduct(product, report, opts), report };
      }
      return { report };
    }

    if (report.accessStatus === 'login_required') {
      throwCustomError('LOGIN_REQUIRED', report);
    }
    if (report.accessStatus === 'verify_required' || report.accessStatus === 'blocked') {
      throwCustomError('PAGE_BLOCKED_OR_VERIFY_REQUIRED', report);
    }
    if (!product?.title?.trim()) {
      throwCustomError('PARSE_FAILED_TITLE_MISSING', report);
    }
    if (!extracted.mainImage) {
      throwCustomError('PARSE_FAILED_IMAGE_MISSING', report);
    }

    return {
      product: attachReportToProduct(product, report, opts),
      report,
    };
  };

  const profileKey = opts.profileKey?.trim() ?? '';
  const useProfile = Boolean(opts.useBrowserProfile && profileKey);
  if (useProfile) {
    return browser.withCustomProfilePage(profileKey, run);
  }
  return use1688Session ? browser.with1688Page(run) : browser.withPage(run);
}
