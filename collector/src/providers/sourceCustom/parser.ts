import type { Page } from 'playwright';
import { evaluateInPage, evaluateInPageVoid } from '../../browser/evaluate-in-page.js';
import type { NormalizedProduct } from '../../types/product.js';
import type { CustomAttributesRule, CustomFieldRule, CustomRuleDecl } from './types.js';
import { normalizeImageList, type ImageFilters } from './image-utils.js';
import { normalizeCustomRuleDecl } from './normalize-rule.js';
import { extractDomImageCandidates } from './page-images.js';
import { fixMisplacedPriceInCurrency, normalizePriceText } from './price-normalize.js';
import { extractJsonLdHints } from './jsonld.js';
import { extractMetaImageHints } from './meta-images.js';
import { extractOpenGraphHints } from './opengraph.js';
import { extractSelectorStrings } from './selectors.js';
import {
  evaluateTitleCandidate,
  pickBestTitle,
  type TitleCandidate,
  TITLE_SUSPECT_HINT,
} from './title-quality.js';
import { DESCRIPTION_IMAGES_EMPTY_HINT } from './quality-score.js';

/** Built-in selectors when rule mainImages miss (e.g. jd.com lazy gallery). */
function builtinMainImageSelectors(pageUrl: string): string[] {
  let host = '';
  try {
    host = new URL(pageUrl).hostname.toLowerCase();
  } catch {
    return [];
  }
  const common = [
    'meta[property="og:image"]',
    'meta[itemprop="image"]',
    '#spec-img',
    'img#spec-img',
    '#spec-img img',
    '.spec-img img',
    '#main-img',
    'img[data-origin]',
    'img[data-lazy-img]',
    '.image-zoom img',
    '#preview img',
    '.product-gallery img',
    '[property="og:image"]',
  ];
  if (host.includes('jd.com')) {
    return [
      '#spec-list img',
      '#spec-n1 img',
      '.spec-items img',
      '#spec-img',
      'img[data-img]',
      'img[data-origin]',
      'img[data-lazy-img]',
      ...common,
    ];
  }
  if (host.includes('tmall.com') || host.includes('taobao.com')) {
    return ['#J_UlThumb img', '#J_ImgBooth img', ...common, 'img[src*="alicdn"]'];
  }
  return common;
}

async function extractFieldText(
  page: Page,
  pageUrl: string,
  field: CustomFieldRule | undefined,
): Promise<{ values: string[]; fb?: string; hitSelector?: string }> {
  if (!field) return { values: [] };
  const sels = Array.isArray(field.selectors) ? field.selectors : [];
  const vals = await extractSelectorStrings(
    page,
    sels,
    typeof field.attr === 'string' ? field.attr : 'text',
    !!field.multiple,
  );
  const fb = typeof field.fallback === 'string' ? field.fallback.trim() : '';
  const hitSelector = vals.length > 0 && sels.length > 0 ? sels[0] : undefined;
  return { values: vals.map((v) => v.trim()).filter(Boolean), fb, hitSelector };
}

async function extractFieldImages(
  page: Page,
  pageUrl: string,
  field: CustomFieldRule | undefined,
  defaultLimit: number,
): Promise<string[]> {
  if (!field) return [];
  const lim =
    typeof field.limit === 'number' && field.limit > 0 ? Math.min(field.limit, defaultLimit) : defaultLimit;
  const sels = Array.isArray(field.selectors) ? field.selectors : [];
  const vals = await extractSelectorStrings(page, sels, typeof field.attr === 'string' ? field.attr : 'src', true);
  const filters = field.filters as ImageFilters | undefined;
  return normalizeImageList(pageUrl, vals, lim, filters);
}

async function scrollToDetailArea(page: Page, field: CustomFieldRule | undefined): Promise<void> {
  const sels = field?.selectors ?? [];
  const firstSel = sels.find((s) => typeof s === 'string' && s.trim());
  if (!firstSel) {
    await evaluateInPageVoid(page, () => {
      window.scrollTo(0, document.body.scrollHeight);
    }).catch(() => undefined);
    await new Promise((r) => setTimeout(r, 600));
    return;
  }
  await evaluateInPage(
    page,
    (sel: string) => {
      const el = document.querySelector(sel);
      if (el) el.scrollIntoView({ behavior: 'instant', block: 'center' });
      else window.scrollTo(0, document.body.scrollHeight);
    },
    firstSel,
  ).catch(() => undefined);
  await new Promise((r) => setTimeout(r, 800));
}

function parseTextAllAttributes(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  const parts = text.split(/[/|；;\n]+/);
  for (const part of parts) {
    const m = part.match(/^(.{1,40}?)[：:]\s*(.+)$/);
    if (!m) continue;
    const k = m[1].trim();
    const v = m[2].trim();
    if (k && v && !(k in out)) out[k] = v;
  }
  return out;
}

async function extractPairsAttributes(page: Page, ar: CustomAttributesRule): Promise<Record<string, string>> {
  if (!ar || ar.mode === 'disabled') return {};
  const rowSel = typeof ar.rowSelector === 'string' ? ar.rowSelector : '';
  const keySel = typeof ar.keySelector === 'string' ? ar.keySelector : '';
  const valSel = typeof ar.valueSelector === 'string' ? ar.valueSelector : '';
  if (!rowSel || !keySel || !valSel) return {};

  const pairs = await evaluateInPage(
    page,
    ({ rowSelector, keySelector, valueSelector }) => {
      const rec: Record<string, string> = {};
      try {
        const rows = Array.from(document.querySelectorAll(rowSelector)).slice(0, 400);
        for (const row of rows) {
          const kEl = row.querySelector(keySelector);
          const vEl = row.querySelector(valueSelector);
          const k = (kEl?.textContent ?? '').trim();
          const v = (vEl?.textContent ?? '').trim();
          if (k && v && !(k in rec)) rec[k] = v;
        }
      } catch {
        /** ignore */
      }
      return rec;
    },
    { rowSelector: rowSel, keySelector: keySel, valueSelector: valSel },
  );
  return pairs;
}

async function extractTextAllAttributes(page: Page, ar: CustomAttributesRule): Promise<Record<string, string>> {
  const sel = typeof ar.textSelector === 'string' ? ar.textSelector.trim() : '';
  if (!sel) return {};
  const text = await evaluateInPage(
    page,
    (s: string) => {
      const el = document.querySelector(s);
      return (el?.textContent ?? '').trim();
    },
    sel,
  );
  if (!text) return {};
  return parseTextAllAttributes(text);
}

async function extractAttributes(page: Page, ar: CustomAttributesRule | undefined): Promise<Record<string, string>> {
  if (!ar || ar.mode === 'disabled') return {};
  const mode = ar.mode ?? 'pairs';
  if (mode === 'text_all') return extractTextAllAttributes(page, ar);
  return extractPairsAttributes(page, ar);
}

export type ParseCustomProductResult = {
  product: NormalizedProduct;
  titleDiagnostics?: TitleCandidate;
  qualityWarnings: string[];
};

export async function parseCustomProduct(
  page: Page,
  pageUrl: string,
  ruleInput: CustomRuleDecl,
  opts?: { scrollForDetailImages?: boolean },
): Promise<ParseCustomProductResult> {
  const rule = normalizeCustomRuleDecl(ruleInput);
  const fb = rule.fallbacks ?? {};
  const useJsonLd = fb.jsonLd !== false;
  const useOg = fb.openGraph !== false;
  const qualityWarnings: string[] = [];

  const jsonLd = useJsonLd ? await extractJsonLdHints(page) : null;
  const og = useOg ? await extractOpenGraphHints(page) : { images: [] as string[] };

  const docTitle =
    (await evaluateInPageVoid(page, () => document.title?.trim() || ''))?.trim() ?? '';

  const titleSel = await extractFieldText(page, pageUrl, rule.title);
  const currencySel = await extractFieldText(page, pageUrl, rule.currency);
  const priceSel = await extractFieldText(page, pageUrl, rule.price);

  const titleCandidates: TitleCandidate[] = [];
  for (const [i, val] of titleSel.values.entries()) {
    const sel = rule.title?.selectors?.[i] ?? rule.title?.selectors?.[0];
    titleCandidates.push(evaluateTitleCandidate(val, 'selector', sel));
  }
  if (titleSel.fb) titleCandidates.push(evaluateTitleCandidate(titleSel.fb, 'fallback'));
  if (jsonLd?.title) titleCandidates.push(evaluateTitleCandidate(jsonLd.title, 'jsonLd'));
  if (og.title) titleCandidates.push(evaluateTitleCandidate(og.title, 'openGraph'));
  if (docTitle) titleCandidates.push(evaluateTitleCandidate(docTitle, 'documentTitle'));

  const bestTitle = pickBestTitle(titleCandidates);
  const title = bestTitle?.text?.trim() ?? '';
  const titleDiagnostics = bestTitle;

  if (bestTitle?.suspectWrongTitle) {
    qualityWarnings.push(TITLE_SUSPECT_HINT);
  }

  let currency = currencySel.values[0] || currencySel.fb || jsonLd?.currency || og.currency || '';
  currency = currency.trim();

  let productPrice: number | undefined;
  let priceText = priceSel.values[0] || priceSel.fb || '';

  if (priceText) {
    const norm = normalizePriceText(priceText);
    productPrice = norm.price;
    if (norm.currency && !currency) currency = norm.currency;
    priceText = norm.priceText ?? priceText;
  }

  if (currency) {
    const fixed = fixMisplacedPriceInCurrency(currency, productPrice);
    currency = fixed.currency;
    if (fixed.price !== undefined && productPrice === undefined) productPrice = fixed.price;
    if (fixed.priceText && !priceText) priceText = fixed.priceText;
  }

  if (productPrice === undefined && jsonLd?.priceAmount !== undefined) {
    productPrice = jsonLd.priceAmount;
    if (!currency && jsonLd.currency) currency = jsonLd.currency;
  }

  const mainLimit =
    typeof rule.mainImages?.limit === 'number' && rule.mainImages.limit > 0
      ? Math.min(rule.mainImages.limit, 10)
      : 10;
  const descLimit =
    typeof rule.descriptionImages?.limit === 'number' && rule.descriptionImages.limit > 0
      ? Math.min(rule.descriptionImages.limit, 30)
      : 30;

  let mainImages = await extractFieldImages(page, pageUrl, rule.mainImages, mainLimit);
  if (mainImages.length === 0) {
    const builtin: CustomFieldRule = {
      selectors: builtinMainImageSelectors(pageUrl),
      attr: 'src',
      multiple: true,
      limit: mainLimit,
      filters: { minWidth: 200, minHeight: 200, dedupeByImageKey: true },
    };
    mainImages = await extractFieldImages(page, pageUrl, builtin, mainLimit);
  }
  if (mainImages.length === 0 && jsonLd?.images?.length) {
    mainImages = normalizeImageList(pageUrl, jsonLd.images, mainLimit, { dedupeByImageKey: true });
  }
  if (mainImages.length === 0 && og.images?.length) {
    mainImages = normalizeImageList(pageUrl, og.images, mainLimit, { dedupeByImageKey: true });
  }
  if (mainImages.length === 0 && fb.meta !== false) {
    const metaImgs = await extractMetaImageHints(page);
    if (metaImgs.length) mainImages = normalizeImageList(pageUrl, metaImgs, mainLimit, { dedupeByImageKey: true });
  }
  if (mainImages.length === 0) {
    const domImgs = await extractDomImageCandidates(page, mainLimit * 2, pageUrl);
    if (domImgs.length) mainImages = normalizeImageList(pageUrl, domImgs, mainLimit, { dedupeByImageKey: true });
  }

  if (mainImages.length === 1) {
    qualityWarnings.push('主图仅识别 1 张，轮播图可能未抓全，建议检查主图区域规则。');
  }

  const shouldScroll =
    opts?.scrollForDetailImages !== false &&
    (rule.descriptionImages?.scrollIntoView !== false || opts?.scrollForDetailImages === true);
  if (shouldScroll && rule.descriptionImages?.selectors?.length) {
    await scrollToDetailArea(page, rule.descriptionImages);
  }

  let descriptionImages = await extractFieldImages(page, pageUrl, rule.descriptionImages, descLimit);
  if (descriptionImages.length === 0 && rule.descriptionImages?.selectors?.length) {
    qualityWarnings.push(DESCRIPTION_IMAGES_EMPTY_HINT);
  }

  let attributes: Record<string, string | number | boolean> = {};
  try {
    const ar = rule.attributes;
    if (ar && ar.mode !== 'disabled') {
      const pairs = await extractAttributes(page, ar);
      attributes = pairs as Record<string, string | number | boolean>;
    }
  } catch {
    attributes = {};
  }

  if (Object.keys(attributes).length === 0 && rule.attributes?.mode && rule.attributes.mode !== 'disabled') {
    qualityWarnings.push('商品参数未识别，可补充 attributes 规则或手动填写。');
  }

  const skuMode = rule.skus?.mode ?? 'disabled';
  const skus = skuMode === 'simple' || skuMode === 'disabled' ? [] : [];

  const finalUrl = page.url() || pageUrl;

  const raw: Record<string, unknown> = {
    extractProvider: 'custom',
    productPrice,
    priceText: priceText || undefined,
    pageUrl: finalUrl,
    qualityWarnings: qualityWarnings.length ? qualityWarnings : undefined,
    titleDiagnostics: titleDiagnostics
      ? {
          text: titleDiagnostics.text,
          selector: titleDiagnostics.selector,
          confidence: titleDiagnostics.confidence,
          suspectWrongTitle: titleDiagnostics.suspectWrongTitle,
          hint: titleDiagnostics.hint,
          source: titleDiagnostics.source,
        }
      : undefined,
    stateDigest: {
      jsonLd: !!jsonLd?.title || (jsonLd?.images?.length ?? 0) > 0,
      openGraph: !!(og.title || og.images?.length),
      meta: fb.meta !== false ? !!(og.description || og.currency) : false,
      titleSource: titleDiagnostics?.source ?? 'none',
      selectorTitleHits: titleSel.values.length,
      jsonLdSnippet:
        jsonLd?.descriptionSnippet ??
        (jsonLd?.brand ? `brand=${jsonLd.brand.slice(0, 120)}` : undefined),
      ogSnippet: og.description ? og.description.slice(0, 240) : undefined,
    },
    jsonLdDigest:
      jsonLd ?
        {
          imageCount: jsonLd.images?.length ?? 0,
          hasPrice: jsonLd.priceAmount !== undefined,
        }
      : null,
    attributeSamples: Object.entries(attributes)
      .slice(0, 5)
      .map(([k, v]) => ({ key: k, value: String(v) })),
  };

  const product: NormalizedProduct = {
    source: 'custom',
    sourceUrl: finalUrl,
    title,
    currency,
    mainImages,
    descriptionImages,
    attributes,
    skus,
    raw,
  };

  return { product, titleDiagnostics, qualityWarnings };
}

/** @deprecated Use parseCustomProduct return type; kept for callers expecting NormalizedProduct only. */
export async function parseCustomProductLegacy(
  page: Page,
  pageUrl: string,
  ruleInput: CustomRuleDecl,
): Promise<NormalizedProduct> {
  const { product } = await parseCustomProduct(page, pageUrl, ruleInput);
  return product;
}
