import type { TaobaoAssembled } from './parser.js';

export type TaobaoQualityStatus = 'ok' | 'warning' | 'failed';

export type TaobaoQualityReport = {
  status: TaobaoQualityStatus;
  score: number;
  warnings: string[];
  errors: string[];
};

const FAILED_CODES = new Set([
  'TITLE_NOT_FOUND',
  'MAIN_IMAGES_EMPTY',
  'ITEM_NOT_FOUND',
  'LOGIN_REQUIRED',
  'VERIFY_REQUIRED',
  'PAGE_LOAD_TIMEOUT',
]);

const WARNING_CODES = new Set([
  'PRICE_NOT_FOUND',
  'SKU_INCOMPLETE',
  'DETAIL_IMAGES_INCOMPLETE',
  'ATTRIBUTES_EMPTY',
  'STOCK_UNKNOWN',
]);

function clamp01(n: number): number {
  if (!Number.isFinite(n)) return 0;
  return Math.max(0, Math.min(1, n));
}

export function buildTaobaoQualityReport(assembled: TaobaoAssembled): TaobaoQualityReport {
  const warnings = [...new Set(assembled.warnings)];
  const errors: string[] = [];

  if (!assembled.title.trim()) {
    errors.push('TITLE_NOT_FOUND');
  }
  if (assembled.mainImages.length === 0) {
    errors.push('MAIN_IMAGES_EMPTY');
  }

  let score = 1;
  if (!assembled.title.trim()) score -= 0.35;
  if (assembled.mainImages.length === 0) score -= 0.35;
  else if (assembled.mainImages.length < 2) score -= 0.05;

  if (!assembled.price || assembled.price <= 0) score -= 0.12;
  if (assembled.descriptionImages.length === 0) score -= 0.08;
  else if (assembled.descriptionImages.length < 3) score -= 0.03;

  if (assembled.skus.length === 0) score -= 0.08;
  if (Object.keys(assembled.attributes).length === 0) score -= 0.05;

  for (const w of warnings) {
    if (WARNING_CODES.has(w)) score -= 0.04;
  }

  const status: TaobaoQualityStatus =
    errors.length > 0 ? 'failed' : warnings.length > 0 ? 'warning' : 'ok';

  return {
    status,
    score: Math.round(clamp01(score) * 100) / 100,
    warnings,
    errors,
  };
}

export function validateTaobaoCollectQuality(assembled: TaobaoAssembled): {
  ok: boolean;
  partial: boolean;
  error?: string;
  quality: TaobaoQualityReport;
} {
  const quality = buildTaobaoQualityReport(assembled);
  if (!assembled.title.trim()) {
    return { ok: false, partial: false, error: 'TITLE_NOT_FOUND:missing_title', quality };
  }
  if (assembled.mainImages.length === 0) {
    return { ok: false, partial: false, error: 'MAIN_IMAGES_EMPTY:no_main_images', quality };
  }
  return { ok: true, partial: quality.warnings.length > 0, quality };
}
