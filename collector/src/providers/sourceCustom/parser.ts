import type { Page } from 'playwright';
import { evaluateInPage, evaluateInPageVoid } from '../../browser/evaluate-in-page.js';
import type { NormalizedProduct } from '../../types/product.js';
import type { CustomAttributesRule, CustomFieldRule, CustomRuleDecl } from './types.js';
import { normalizeCustomRuleDecl } from './normalize-rule.js';
import { extractDomImageCandidates } from './page-images.js';
import { extractJsonLdHints } from './jsonld.js';
import { extractMetaImageHints } from './meta-images.js';
import { extractOpenGraphHints } from './opengraph.js';
import { extractSelectorStrings } from './selectors.js';

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
      ...common,
      '#spec-n1 img',
      '.spec-list img',
      'img[data-img]',
      'img[src*="360buyimg"]',
      'img[src*="jfs"]',
    ];
  }
  if (host.includes('tmall.com') || host.includes('taobao.com')) {
    return [...common, '#J_ImgBooth img', '#J_UlThumb img', 'img[src*="alicdn"]'];
  }
  return common;
}

function resolveUrl(pageUrl: string, raw: string): string {
  const s = raw.trim();
  if (!s) return '';
  try {
    return new URL(s, pageUrl).href;
  } catch {
    return s;
  }
}

function isJunkImageUrl(u: string): boolean {
  const s = u.toLowerCase();
  if (!s.startsWith('http://') && !s.startsWith('https://') && !s.startsWith('data:')) return false;
  if (s.startsWith('data:')) return true;
  if (s.includes('favicon')) return true;
  if (s.includes('pixel') && (s.includes('1x1') || s.includes('tracking'))) return true;
  if (s.includes('placeholder')) return true;
  if (s.includes('/logo.') || s.endsWith('/logo.png') || s.includes('logo.svg')) return true;
  if (s.includes('icon.') || s.includes('/icon/')) return true;
  if (s.includes('spacer.gif')) return true;
  if (s.includes('blank.gif')) return true;
  return false;
}

function normalizeImages(pageUrl: string, urls: string[], limit: number): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of urls) {
    const abs = resolveUrl(pageUrl, raw);
    if (!abs || !abs.startsWith('http')) continue;
    if (isJunkImageUrl(abs)) continue;
    if (seen.has(abs)) continue;
    seen.add(abs);
    out.push(abs);
    if (out.length >= limit) break;
  }
  return out;
}

async function extractFieldText(
  page: Page,
  pageUrl: string,
  field: CustomFieldRule | undefined,
): Promise<{ values: string[]; fb?: string }> {
  if (!field) return { values: [] };
  const sels = Array.isArray(field.selectors) ? field.selectors : [];
  const vals = await extractSelectorStrings(page, sels, typeof field.attr === 'string' ? field.attr : 'text', !!field.multiple);
  const fb = typeof field.fallback === 'string' ? field.fallback.trim() : '';
  return { values: vals.map((v) => v.trim()).filter(Boolean), fb };
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
  return normalizeImages(pageUrl, vals, lim);
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

export async function parseCustomProduct(page: Page, pageUrl: string, ruleInput: CustomRuleDecl): Promise<NormalizedProduct> {
  const rule = normalizeCustomRuleDecl(ruleInput);
  const fb = rule.fallbacks ?? {};
  const useJsonLd = fb.jsonLd !== false;
  const useOg = fb.openGraph !== false;

  const jsonLd = useJsonLd ? await extractJsonLdHints(page) : null;
  const og = useOg ? await extractOpenGraphHints(page) : { images: [] as string[] };

  const docTitle =
    (await evaluateInPageVoid(page, () => document.title?.trim() || ''))?.trim() ?? '';

  const titleSel = await extractFieldText(page, pageUrl, rule.title);
  const currencySel = await extractFieldText(page, pageUrl, rule.currency);
  const priceSel = await extractFieldText(page, pageUrl, (ruleInput as { price?: CustomFieldRule }).price);

  let title =
    titleSel.values[0] ||
    titleSel.fb ||
    jsonLd?.title ||
    og.title ||
    docTitle;

  title = title?.trim() ?? '';

  let currency =
    currencySel.values[0] ||
    currencySel.fb ||
    jsonLd?.currency ||
    og.currency ||
    '';

  currency = currency.trim();

  const priceText = priceSel.values[0] || priceSel.fb || '';
  let productPrice: number | undefined;
  if (priceText) {
    const m = priceText.replace(/,/g, '').match(/(\d+(?:\.\d+)?)/);
    if (m) productPrice = Number.parseFloat(m[1]);
  }
  if (productPrice === undefined && jsonLd?.priceAmount !== undefined) {
    productPrice = jsonLd.priceAmount;
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
    };
    mainImages = await extractFieldImages(page, pageUrl, builtin, mainLimit);
  }
  if (mainImages.length === 0 && jsonLd?.images?.length) {
    mainImages = normalizeImages(pageUrl, jsonLd.images, mainLimit);
  }
  if (mainImages.length === 0 && og.images?.length) {
    mainImages = normalizeImages(pageUrl, og.images, mainLimit);
  }
  if (mainImages.length === 0 && fb.meta !== false) {
    const metaImgs = await extractMetaImageHints(page);
    if (metaImgs.length) mainImages = normalizeImages(pageUrl, metaImgs, mainLimit);
  }
  if (mainImages.length === 0) {
    const domImgs = await extractDomImageCandidates(page, mainLimit * 2);
    if (domImgs.length) mainImages = normalizeImages(pageUrl, domImgs, mainLimit);
  }

  let descriptionImages = await extractFieldImages(page, pageUrl, rule.descriptionImages, descLimit);

  let attributes: Record<string, string | number | boolean> = {};
  try {
    const ar = rule.attributes;
    if (ar && ar.mode !== 'disabled') {
      const pairs = await extractPairsAttributes(page, ar);
      attributes = pairs as Record<string, string | number | boolean>;
    }
  } catch {
    attributes = {};
  }

  const skuMode = rule.skus?.mode ?? 'disabled';
  const skus = skuMode === 'simple' || skuMode === 'disabled' ? [] : [];

  const finalUrl = page.url() || pageUrl;

  const raw: Record<string, unknown> = {
    extractProvider: 'custom',
    productPrice,
    pageUrl: finalUrl,
    stateDigest: {
      jsonLd: !!jsonLd?.title || (jsonLd?.images?.length ?? 0) > 0,
      openGraph: !!(og.title || og.images?.length),
      meta: fb.meta !== false ? !!(og.description || og.currency) : false,
      titleSource:
        titleSel.values[0] ? 'selector'
        : jsonLd?.title ? 'jsonLd'
        : og.title ? 'openGraph'
        : docTitle ? 'documentTitle'
        : 'none',
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
  };

  return {
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
}
