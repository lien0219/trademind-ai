import type { Page } from 'playwright';
import type { ProductSku } from '../../types/product.js';
import { dedupeUrls } from './image-utils.js';
import {
  collectMainImagesInteractive,
  extractTaobaoPagePayload,
  resolvePriceFromPayload,
  scrollAndCollectDetailImages,
  type TaobaoPagePayload,
} from './page-extract.js';
import { extractTaobaoJsonPatch, mergeTaobaoPayload } from './json-extract.js';

export type TaobaoAssembled = {
  title: string;
  originalTitle: string;
  price?: number;
  priceText: string;
  priceRange: string;
  currency: string;
  shopName: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string>;
  skus: ProductSku[];
  warnings: string[];
  raw: Record<string, unknown>;
};

function skuLooksIncomplete(payload: TaobaoPagePayload): boolean {
  if (payload.skuGroups.length === 0) return true;
  return payload.skuGroups.some((g) => g.options.length === 0);
}

export async function assembleTaobaoProduct(
  page: Page,
  sourceUrl: string,
  payload: TaobaoPagePayload,
): Promise<TaobaoAssembled> {
  const warnings: string[] = [];
  const interactiveMain = await collectMainImagesInteractive(page);
  const scrolledDetail = await scrollAndCollectDetailImages(page);

  let mainImages = dedupeUrls([...payload.mainImages, ...interactiveMain]);
  let descriptionImages = dedupeUrls([...payload.descriptionImages, ...scrolledDetail]);

  const title = payload.title.trim();
  const originalTitle = (payload.originalTitle || title).trim();
  const { price, priceText, priceRange } = resolvePriceFromPayload(payload);

  if (!price || price <= 0) {
    warnings.push('PRICE_NOT_FOUND');
  }

  if (descriptionImages.length === 0) {
    warnings.push('DETAIL_IMAGES_INCOMPLETE');
  }

  if (skuLooksIncomplete(payload)) {
    warnings.push('SKU_INCOMPLETE');
  }

  const skus: ProductSku[] = payload.skus.map((s) => ({
    ...s,
    price: s.price && s.price > 0 ? s.price : price && price > 0 ? price : s.price,
  }));

  return {
    title,
    originalTitle,
    price,
    priceText,
    priceRange,
    currency: 'CNY',
    shopName: payload.shopName,
    mainImages,
    descriptionImages,
    attributes: payload.attributes,
    skus,
    warnings: [...new Set(warnings)],
    raw: {
      extractProvider: 'taobao_tmall',
      warnings: [...new Set(warnings)],
      productPrice: price,
      priceText,
      priceRange,
      originalTitle,
      shopName: payload.shopName,
      skuGroups: payload.skuGroups,
      debug: payload.debug,
      finalUrl: page.url(),
      sourceUrl,
    },
  };
}

export function validateTaobaoCollectQuality(assembled: TaobaoAssembled): {
  ok: boolean;
  partial: boolean;
  error?: string;
} {
  if (!assembled.title.trim()) {
    return { ok: false, partial: false, error: 'PARSE_FAILED:missing_title' };
  }
  if (assembled.mainImages.length === 0) {
    return { ok: false, partial: false, error: 'MAIN_IMAGES_EMPTY:no_main_images' };
  }
  const partial = assembled.warnings.length > 0;
  return { ok: true, partial };
}

export async function extractAndAssembleTaobao(
  page: Page,
  sourceUrl: string,
): Promise<TaobaoAssembled> {
  const domPayload = await extractTaobaoPagePayload(page);
  const jsonPatch = await extractTaobaoJsonPatch(page).catch(() => ({
    mainImages: [] as string[],
    descriptionImages: [] as string[],
    attributes: {} as Record<string, string>,
    skuGroups: [] as TaobaoPagePayload['skuGroups'],
    skus: [] as TaobaoAssembled['skus'],
  }));
  const payload = mergeTaobaoPayload(domPayload, jsonPatch);
  return assembleTaobaoProduct(page, sourceUrl, payload);
}
