import type { Page } from 'playwright';
import { evaluateInPageVoid } from '../../browser/evaluate-in-page.js';
import type { ProductSku } from '../../types/product.js';
import { normalizePriceText } from '../sourceCustom/price-normalize.js';
import type { PinduoduoParseResult } from './parser.js';
import {
  extractPifaWholesaleDetailInPage,
  type PifaWholesaleDomPayload,
} from './wholesale-detail-extract.js';
import {
  collectInteractiveGalleryImages,
  scrollAndCollectDetailImages,
  waitForMainGalleryReady,
} from './wholesale-detail-gallery.js';
import { classifyRegionImages, toImageSummary } from './wholesale-detail-images.js';
import {
  appendWarning,
  buildMainDescription,
  cleanProductTitle,
  isPlatformTitle,
  parsePriceRangeText,
  type PriceRange,
  type WholesaleWarningCode,
  wholesaleRowsToSkus,
} from './wholesale-detail-shared.js';

function mergePayloadImages(
  payload: PifaWholesaleDomPayload,
  interactive: Awaited<ReturnType<typeof collectInteractiveGalleryImages>>,
  scrolledDetails: Awaited<ReturnType<typeof scrollAndCollectDetailImages>>,
): PifaWholesaleDomPayload {
  const mainKeys = new Set(payload.mainImageCandidates.map((c) => c.url.split('?')[0]));
  const detailKeys = new Set(payload.detailImageCandidates.map((c) => c.url.split('?')[0]));
  let orderSeq = Math.max(
    0,
    ...[
      ...payload.mainImageCandidates,
      ...payload.detailImageCandidates,
      ...payload.unknownImageCandidates,
    ].map((c) => c.order),
  );

  const pushMain = (items: typeof payload.mainImageCandidates) => {
    for (const item of items) {
      const key = item.url.split('?')[0] ?? item.url;
      if (mainKeys.has(key)) continue;
      mainKeys.add(key);
      payload.mainImageCandidates.push({ ...item, order: ++orderSeq });
    }
  };

  const pushDetail = (items: typeof payload.detailImageCandidates) => {
    for (const item of items) {
      const key = item.url.split('?')[0] ?? item.url;
      if (detailKeys.has(key) || mainKeys.has(key)) continue;
      detailKeys.add(key);
      payload.detailImageCandidates.push({ ...item, order: ++orderSeq });
    }
  };

  pushMain(interactive.thumbnailCandidates);
  pushMain(interactive.clickedMainImages);
  pushDetail(scrolledDetails);

  return payload;
}

function pickTitle(
  payload: PifaWholesaleDomPayload,
  warningCodes: WholesaleWarningCode[],
  warnings: string[],
): { title: string; titleCleaned: boolean } {
  let titleCleaned = false;

  for (const raw of payload.titleCandidates) {
    const { title, cleaned, contaminated } = cleanProductTitle(raw);
    if (!title || isPlatformTitle(title)) continue;
    if (cleaned) titleCleaned = true;
    if (contaminated) appendWarning(warningCodes, warnings, 'title_maybe_contaminated');
    return { title, titleCleaned };
  }

  const docClean = cleanProductTitle(payload.docTitle);
  if (docClean.title && !isPlatformTitle(docClean.title)) {
    return { title: docClean.title, titleCleaned: docClean.cleaned };
  }

  appendWarning(warningCodes, warnings, 'title_maybe_platform_title');
  return { title: docClean.title.slice(0, 500), titleCleaned: docClean.cleaned };
}

function resolvePriceRange(payload: PifaWholesaleDomPayload): PriceRange {
  const texts = [payload.priceRangeText, ...(payload.priceTexts ?? [])].filter((t): t is string =>
    Boolean(t?.trim()),
  );

  for (const t of texts) {
    const parsed = parsePriceRangeText(t);
    if (parsed.priceMin !== undefined && parsed.priceMin > 0) return parsed;
  }

  for (const t of texts) {
    const norm = normalizePriceText(t);
    if (norm.price && norm.price > 0) {
      return {
        priceMin: norm.price,
        priceMax: norm.price,
        priceText: norm.priceText ?? t,
        currency: norm.currency ?? 'CNY',
      };
    }
  }

  return { currency: 'CNY' };
}

function skuRowsFromPayload(
  payload: PifaWholesaleDomPayload,
  priceMin?: number,
  codes: WholesaleWarningCode[] = [],
  messages: string[] = [],
): ProductSku[] {
  const rows = payload.skuRows.map((row) => {
    let price: number | undefined;
    if (row.priceText) {
      const norm = normalizePriceText(row.priceText);
      price = norm.price;
    }
    return {
      name: row.name,
      price,
      stock: row.stock,
      imageUrl: row.imageUrl,
    };
  });
  return wholesaleRowsToSkus(rows, priceMin, codes, messages);
}

export function assemblePifaWholesaleProduct(
  sourceUrl: string,
  payload: PifaWholesaleDomPayload,
  galleryMeta?: {
    thumbnailClickedImages?: PifaWholesaleDomPayload['mainImageCandidates'];
    scriptGalleryUrls?: string[];
  },
): PinduoduoParseResult {
  const warningCodes: WholesaleWarningCode[] = [];
  const warnings: string[] = [];

  const { title, titleCleaned } = pickTitle(payload, warningCodes, warnings);

  const priceRange = resolvePriceRange(payload);
  const price = priceRange.priceMin;
  const priceText = priceRange.priceText;

  const skus = skuRowsFromPayload(payload, priceRange.priceMin, warningCodes, warnings);

  if (skus.length === 0 && (payload.skuRows.length > 0 || payload.specButtonCount >= 2)) {
    appendWarning(warningCodes, warnings, 'sku_parse_failed');
    appendWarning(warningCodes, warnings, 'sku_rows_detected_but_empty');
  } else if (skus.length === 0 && payload.specButtonCount >= 2) {
    appendWarning(warningCodes, warnings, 'sku_parse_failed');
  }

  const skuImageUrls = payload.skuRows
    .map((r) => r.imageUrl?.trim())
    .filter((u): u is string => Boolean(u));

  const skuImagesCount = skuImageUrls.length;

  const classified = classifyRegionImages(
    payload.pageUrl,
    { main: payload.mainImageCandidates, detail: payload.detailImageCandidates },
    {
      mainLimit: 10,
      detailLimit: 40,
      ogImageFallback: payload.ogImageUrl,
      skuImageUrls,
      unknownCandidates: payload.unknownImageCandidates,
      thumbnailClickedImages: galleryMeta?.thumbnailClickedImages,
      scriptGalleryUrls: galleryMeta?.scriptGalleryUrls,
    },
  );
  const { mainImages, descriptionImages, summary, imageDebug, fallbackUsed, fallbackReason } =
    classified;
  summary.skuImagesBound = skuImagesCount;

  if (summary.filtered > 0) {
    appendWarning(warningCodes, warnings, 'images_filtered');
  }
  if (mainImages.length === 0) {
    appendWarning(warningCodes, warnings, 'no_main_images');
  } else if (fallbackUsed) {
    appendWarning(warningCodes, warnings, 'main_images_fallback_used');
    if (fallbackReason === 'sku_images' || fallbackReason.startsWith('sku')) {
      appendWarning(warningCodes, warnings, 'main_image_fallback_from_sku');
    } else if (
      fallbackReason === 'detail_first_image' ||
      fallbackReason.includes('detail')
    ) {
      appendWarning(warningCodes, warnings, 'main_image_fallback_from_detail');
    } else if (fallbackReason !== 'main_images_supplement') {
      appendWarning(warningCodes, warnings, 'main_image_fallback_from_page');
    }
  }
  if (mainImages.length > 0 && mainImages.length < 3) {
    appendWarning(warningCodes, warnings, 'main_images_maybe_incomplete');
  }
  const mainRegionCandidateCount = payload.mainImageCandidates.length;
  if (mainRegionCandidateCount > 12) {
    appendWarning(warningCodes, warnings, 'main_images_too_many');
  }
  if (descriptionImages.length === 0) {
    appendWarning(warningCodes, warnings, 'description_images_missing');
    if (payload.introFound) {
      appendWarning(warningCodes, warnings, 'detail_images_lazy_load');
    }
  }

  const imageSummary = toImageSummary(summary, skuImagesCount);

  const attributes: Record<string, string | number | boolean> = {};
  for (const [k, v] of Object.entries(payload.attributes)) {
    if (k && v) attributes[k] = v;
  }
  if (Object.keys(attributes).length === 0) {
    appendWarning(warningCodes, warnings, 'attributes_missing');
  }

  const mainDescription = buildMainDescription({
    introTexts: payload.introTexts,
    title,
    attributes,
  });
  if (!mainDescription.trim()) {
    appendWarning(warningCodes, warnings, 'description_missing');
  }

  return {
    title,
    price,
    currency: priceRange.currency || 'CNY',
    priceText,
    mainDescription,
    mainImages,
    descriptionImages,
    attributes,
    skus,
    warnings,
    blocked: false,
    raw: {
      extractProvider: 'pinduoduo',
      urlType: 'wholesale_detail',
      warnings: warningCodes,
      mainDescription,
      productPrice: price,
      priceMin: priceRange.priceMin,
      priceMax: priceRange.priceMax,
      priceRange: priceText,
      quality: {
        titleCleaned,
        descriptionMissing: !mainDescription.trim(),
        mainImagesCount: imageSummary.mainImagesCount,
        descriptionImagesCount: imageSummary.descriptionImagesCount,
        filteredImagesCount: imageSummary.filteredImagesCount,
        skuCount: skus.length,
        attributesCount: Object.keys(attributes).length,
      },
      imageSummary,
      imageFilterSummary: summary,
      imageDebug,
      pageMeta: { docTitle: payload.docTitle, pageUrl: payload.pageUrl },
      titleCandidates: payload.titleCandidates,
      priceTexts: payload.priceTexts,
      specButtonCount: payload.specButtonCount,
      skuRowCount: payload.skuRows.length,
      introFound: payload.introFound,
      extractedAt: new Date().toISOString(),
    },
  };
}

export async function extractAndAssemblePifaWholesale(
  page: Page,
  sourceUrl: string,
): Promise<PinduoduoParseResult> {
  await waitForMainGalleryReady(page);
  const interactive = await collectInteractiveGalleryImages(page);
  const scrolledDetails = await scrollAndCollectDetailImages(page);

  const payload = await evaluateInPageVoid(page, extractPifaWholesaleDetailInPage);
  const merged = mergePayloadImages(payload, interactive, scrolledDetails);

  return assemblePifaWholesaleProduct(sourceUrl, merged, {
    thumbnailClickedImages: interactive.clickedMainImages,
    scriptGalleryUrls: interactive.scriptGalleryUrls,
  });
}

export function validateWholesaleCollectQuality(
  result: PinduoduoParseResult,
): { ok: boolean; partial: boolean; error?: string } {
  const titleBad =
    !result.title.trim() || isPlatformTitle(result.title) || /分享商品/.test(result.title);
  const hasPrice = result.price !== undefined && result.price > 0;
  const hasSkus = result.skus.length > 0;
  const hasAnyImages =
    result.mainImages.length > 0 ||
    result.descriptionImages.length > 0 ||
    hasSkus;

  if (!hasPrice && !hasAnyImages && !hasSkus) {
    if (titleBad) {
      return { ok: false, partial: false, error: 'PARSE_FAILED_TITLE_MISSING:no_core_fields' };
    }
    return { ok: false, partial: false, error: 'PARSE_FAILED:missing_core_fields' };
  }

  if (titleBad && !hasPrice && !hasAnyImages) {
    return { ok: false, partial: false, error: 'PARSE_FAILED_TITLE_MISSING:platform_or_empty_title' };
  }

  if (!hasPrice) {
    return { ok: false, partial: false, error: 'PARSE_FAILED_PRICE_MISSING:invalid_price' };
  }

  const partial =
    result.warnings.length > 0 ||
    result.mainImages.length === 0 ||
    titleBad;
  return { ok: true, partial };
}
