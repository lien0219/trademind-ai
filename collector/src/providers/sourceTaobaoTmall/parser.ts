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
import { buildTaobaoQualityReport, validateTaobaoCollectQuality } from './quality.js';
import {
  collectSkuPricesByClick,
  mergeSkuResults,
  toTaobaoSkuGroups,
  type TaobaoSkuCollectOptions,
  type TaobaoSkuGroup,
} from './sku-collect.js';
import { cleanTaobaoTitle } from './title-utils.js';

export type TaobaoPriceInfo = {
  price?: number;
  priceMin?: number;
  priceMax?: number;
  currency: string;
  priceText: string;
  priceSource: 'page_display' | 'sku' | 'json' | 'unknown';
};

export type TaobaoShopInfo = {
  shopName: string;
  shopUrl: string;
  sellerName: string;
};

export type TaobaoAssembled = {
  title: string;
  originalTitle: string;
  price?: number;
  priceMin?: number;
  priceMax?: number;
  priceText: string;
  priceRange: string;
  priceSource: TaobaoPriceInfo['priceSource'];
  currency: string;
  shopName: string;
  shopUrl: string;
  sellerName: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string>;
  skuGroups: TaobaoSkuGroup[];
  skus: ProductSku[];
  warnings: string[];
  raw: Record<string, unknown>;
};

function skuLooksIncomplete(groups: TaobaoSkuGroup[], skus: ProductSku[]): boolean {
  if (groups.length === 0) return true;
  if (groups.some((g) => g.options.length === 0)) return true;
  if (skus.length === 0) return true;
  return false;
}

function resolvePriceInfo(payload: TaobaoPagePayload, skus: ProductSku[]): TaobaoPriceInfo {
  const base = resolvePriceFromPayload(payload);
  const skuPrices = skus.map((s) => s.price).filter((p): p is number => !!p && p > 0);
  let priceMin = base.price;
  let priceMax = base.price;
  let priceSource: TaobaoPriceInfo['priceSource'] = 'unknown';

  if (base.price && base.price > 0) {
    priceSource = 'page_display';
  }
  if (skuPrices.length) {
    const min = Math.min(...skuPrices);
    const max = Math.max(...skuPrices);
    priceMin = min;
    priceMax = max;
    if (!base.price || base.price <= 0) {
      priceSource = 'sku';
    }
  }

  const rangeMatch = (payload.priceRange || payload.priceText || '').match(
    /(\d+(?:\.\d{1,2})?)\s*[-–—~至]\s*(\d+(?:\.\d{1,2})?)/,
  );
  if (rangeMatch) {
    priceMin = Number(rangeMatch[1]);
    priceMax = Number(rangeMatch[2]);
    if (!base.price) priceSource = 'page_display';
  }

  return {
    price: base.price ?? (skuPrices.length === 1 ? skuPrices[0] : undefined),
    priceMin,
    priceMax,
    currency: 'CNY',
    priceText: payload.priceRange || payload.priceText,
    priceSource,
  };
}

export async function assembleTaobaoProduct(
  page: Page,
  sourceUrl: string,
  payload: TaobaoPagePayload,
  skuOptions: TaobaoSkuCollectOptions,
  detailWaitMs: number,
): Promise<TaobaoAssembled> {
  const warnings: string[] = [];
  const interactiveMain = await collectMainImagesInteractive(page);
  const scrolledDetail = await scrollAndCollectDetailImages(page, detailWaitMs);

  let mainImages = dedupeUrls([...payload.mainImages, ...interactiveMain]);
  let descriptionImages = dedupeUrls([...payload.descriptionImages, ...scrolledDetail]);

  const title = cleanTaobaoTitle(payload.title.trim());
  const originalTitle = (payload.originalTitle || payload.title).trim();
  let skuGroups = toTaobaoSkuGroups(payload);
  let skus = payload.skus;

  if (skuOptions.enabled && skuGroups.length > 0) {
    const clicked = await collectSkuPricesByClick(page, skuGroups, skuOptions);
    const merged = mergeSkuResults(skus, clicked);
    skus = merged.skus;
  }

  const priceInfo = resolvePriceInfo(payload, skus);
  if (!priceInfo.price || priceInfo.price <= 0) {
    warnings.push('PRICE_NOT_FOUND');
  }

  if (descriptionImages.length === 0) {
    warnings.push('DETAIL_IMAGES_INCOMPLETE');
  }

  if (skuLooksIncomplete(skuGroups, skus)) {
    warnings.push('SKU_INCOMPLETE');
  }

  if (Object.keys(payload.attributes).length === 0) {
    warnings.push('ATTRIBUTES_EMPTY');
  }

  const hasUnknownStock = skus.some((s) => s.stock == null);
  if (skus.length > 0 && hasUnknownStock) {
    warnings.push('STOCK_UNKNOWN');
  }

  const skusFinal: ProductSku[] = skus.map((s) => ({
    ...s,
    price:
      s.price && s.price > 0
        ? s.price
        : priceInfo.price && priceInfo.price > 0
          ? priceInfo.price
          : s.price,
  }));

  const shopUrl = await page
    .evaluate(() => {
      const a =
        document.querySelector('[class*="ShopHeader"] a[href*="shop"]') ??
        document.querySelector('.tb-shop-name a') ??
        document.querySelector('a[href*="shop.taobao.com"]');
      return (a as HTMLAnchorElement | null)?.href ?? '';
    })
    .catch(() => '');

  const assembled: TaobaoAssembled = {
    title,
    originalTitle,
    price: priceInfo.price,
    priceMin: priceInfo.priceMin,
    priceMax: priceInfo.priceMax,
    priceText: priceInfo.priceText,
    priceRange: payload.priceRange || priceInfo.priceText,
    priceSource: priceInfo.priceSource,
    currency: 'CNY',
    shopName: payload.shopName,
    shopUrl: shopUrl.trim(),
    sellerName: payload.shopName,
    mainImages,
    descriptionImages,
    attributes: payload.attributes,
    skuGroups,
    skus: skusFinal,
    warnings: [...new Set(warnings)],
    raw: {},
  };

  const quality = buildTaobaoQualityReport(assembled);
  assembled.raw = {
    extractProvider: 'taobao_tmall',
    warnings: assembled.warnings,
    qualityWarnings: assembled.warnings,
    quality,
    productPrice: priceInfo.price,
    priceMin: priceInfo.priceMin,
    priceMax: priceInfo.priceMax,
    priceText: priceInfo.priceText,
    priceRange: assembled.priceRange,
    priceSource: priceInfo.priceSource,
    originalTitle,
    shopName: payload.shopName,
    shopUrl: assembled.shopUrl,
    sellerName: assembled.sellerName,
    skuGroups: assembled.skuGroups,
    debug: payload.debug,
    finalUrl: page.url(),
    sourceUrl,
  };

  return assembled;
}

export { validateTaobaoCollectQuality };

export async function extractAndAssembleTaobao(
  page: Page,
  sourceUrl: string,
  skuOptions: TaobaoSkuCollectOptions,
  detailWaitMs: number,
): Promise<TaobaoAssembled> {
  const domPayload = await extractTaobaoPagePayload(page);
  const jsonPatch = await extractTaobaoJsonPatch(page).catch(() => ({
    mainImages: [] as string[],
    descriptionImages: [] as string[],
    attributes: {} as Record<string, string>,
    skuGroups: [] as TaobaoPagePayload['skuGroups'],
    skus: [] as ProductSku[],
  }));
  const payload = mergeTaobaoPayload(domPayload, jsonPatch);
  return assembleTaobaoProduct(page, sourceUrl, payload, skuOptions, detailWaitMs);
}
