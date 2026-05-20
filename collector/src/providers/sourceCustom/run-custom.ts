import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import type { CustomAccessReport, ExtractedFieldsSummary, QualityScoreSummary } from '../../types/access-status.js';
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
import { buildQualityScore, SKU_LIMITATION_HINT } from './quality-score.js';
import type { TitleCandidate } from './title-quality.js';

export type CustomRunMode = 'task' | 'rule_test';

export type CustomRunResult = {
  product?: NormalizedProduct;
  report: CustomAccessReport;
};

function fieldSummary(
  product: NormalizedProduct | undefined,
  rule: CustomRuleDecl,
  titleDiag?: TitleCandidate,
): {
  extracted: ExtractedFieldsSummary;
  missing: string[];
  warnings: string[];
  qualityScore: QualityScoreSummary;
} {
  const extracted: ExtractedFieldsSummary = {
    title: false,
    price: false,
    mainImage: false,
    mainImagesCount: 0,
    detailImagesCount: 0,
    attributesCount: 0,
    titleText: '',
    titleSelector: '',
    titleConfidence: '',
    titleSuspectWrong: false,
  };
  const missing: string[] = [];
  const warnings: string[] = [];

  const title = product?.title?.trim() ?? '';
  if (title) {
    extracted.title = true;
    extracted.titleText = title;
    extracted.titleSelector = titleDiag?.selector ?? '';
    extracted.titleConfidence = titleDiag?.confidence ?? '';
    extracted.titleSuspectWrong = titleDiag?.suspectWrongTitle ?? false;
  } else {
    missing.push('title');
  }

  const priceVal = product?.raw?.productPrice;
  if (typeof priceVal === 'number' && priceVal > 0) extracted.price = true;
  else if (rule.price?.selectors?.length) {
    missing.push('price');
  }

  const mainN = product?.mainImages?.length ?? 0;
  extracted.mainImagesCount = mainN;
  extracted.mainImage = mainN > 0;
  if (!extracted.mainImage) missing.push('mainImage');

  extracted.detailImagesCount = product?.descriptionImages?.length ?? 0;

  const attrN = product?.attributes ? Object.keys(product.attributes).length : 0;
  extracted.attributesCount = attrN;

  const rawWarnings = product?.raw?.qualityWarnings;
  if (Array.isArray(rawWarnings)) {
    for (const w of rawWarnings) {
      if (typeof w === 'string' && w.trim()) warnings.push(w.trim());
    }
  }

  if (extracted.title && !extracted.mainImage) {
    warnings.push('title_extracted_but_no_main_image');
  }

  if (titleDiag?.suspectWrongTitle) {
    warnings.push('title_suspect_wrong');
  }

  if (mainN === 1) {
    warnings.push('main_images_single_only');
  }

  if (extracted.detailImagesCount === 0 && rule.descriptionImages?.selectors?.length) {
    warnings.push('description_images_empty');
  }

  if (attrN === 0 && rule.attributes?.mode && rule.attributes.mode !== 'disabled') {
    warnings.push('attributes_empty');
  }

  warnings.push(SKU_LIMITATION_HINT);

  const qs = buildQualityScore(product, titleDiag);
  const qualityScore: QualityScoreSummary = {
    titleOk: qs.titleOk,
    priceOk: qs.priceOk,
    mainImagesOk: qs.mainImagesOk,
    descriptionImagesOk: qs.descriptionImagesOk,
    attributesOk: qs.attributesOk,
    skuSupported: qs.skuSupported,
    score: qs.score,
    hints: qs.hints,
  };

  for (const h of qs.hints) {
    if (!warnings.includes(h)) warnings.push(h);
  }

  const attrSamples = product?.raw?.attributeSamples;
  if (Array.isArray(attrSamples)) {
    extracted.attributeSamples = attrSamples.slice(0, 5) as { key: string; value: string }[];
  }

  return { extracted, missing, warnings: [...new Set(warnings)], qualityScore };
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
      qualityScore: report.qualityScore,
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
  qualityScore: QualityScoreSummary,
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
  if (qualityScore.score < 50 && qualityScore.hints.length) {
    suggestion = `${qualityScore.hints[0]} ${suggestion}`.trim();
  }
  return {
    accessStatus,
    finalUrl: signals.finalUrl,
    httpStatus: signals.httpStatus,
    extractedFields: extracted,
    missingFields: missing,
    warnings,
    qualityScore,
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
    const resp = await page.goto(urlStr, {
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
          mainImagesCount: 0,
          detailImagesCount: 0,
          attributesCount: 0,
        },
        missingFields: ['title', 'mainImage'],
        warnings: [],
        qualityScore: {
          titleOk: false,
          priceOk: false,
          mainImagesOk: false,
          descriptionImagesOk: false,
          attributesOk: false,
          skuSupported: false,
          score: 0,
          hints: [],
        },
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
    let titleDiag: TitleCandidate | undefined;
    try {
      const parsed = await parseCustomProduct(page, urlStr, rule, {
        scrollForDetailImages: opts.scrollForDetailImages ?? mode === 'rule_test',
      });
      product = parsed.product;
      titleDiag = parsed.titleDiagnostics;
    } catch {
      product = undefined;
    }

    const { extracted, missing, warnings, qualityScore } = fieldSummary(product, rule, titleDiag);
    const report = buildReport(signals, extracted, missing, warnings, qualityScore, accessStatus);

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
