import type { BrowserExtractPayload } from './types.js';
import type { ProductSku } from '../../types/product.js';
import { coerceNumber, trimStr } from './utils.js';

const PRICE_JSON_KEYS = new Set([
  'price',
  'priceRange',
  'priceRanges',
  'priceTiers',
  'skuPrice',
  'skuPriceMap',
  'salePrice',
  'wholesalePrice',
  'discountPrice',
  'offerPrice',
  'finalPrice',
  'priceDisplay',
  'referencePrice',
  'promotionPrice',
  'minPrice',
  'maxPrice',
  'consignPrice',
  'retailPrice',
  'originPrice',
]);

/** 非价格数值字段 — 避免 unitWeight(39) 等被误识别为单价 */
const NON_PRICE_KEYS = new Set([
  'unitWeight',
  'weight',
  'netWeight',
  'grossWeight',
  'pieceWeight',
  'volume',
  'length',
  'width',
  'height',
  'depth',
  'diameter',
  'quantity',
  'minOrder',
  'moq',
  'amount',
  'count',
  'stock',
  'canBookCount',
  'amountOnSale',
  'skuId',
  'specId',
  'offerId',
  'componentId',
  'version',
  'seed',
  'bookCount',
  'saleCount',
  'skuVid',
  'id',
]);

function isPriceKey(key: string): boolean {
  if (NON_PRICE_KEYS.has(key)) return false;
  if (PRICE_JSON_KEYS.has(key)) return true;
  return /price|Price|批发|wholesale|discount|sale/i.test(key);
}

function readPriceValue(v: unknown): number | undefined {
  if (typeof v === 'number') {
    return v > 0 && v < 1_000_000 ? v : undefined;
  }
  if (typeof v === 'string') {
    const n = coerceNumber(v);
    return n !== undefined && n > 0 && n < 1_000_000 ? n : undefined;
  }
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    const o = v as Record<string, unknown>;
    return (
      readPriceValue(o.value) ??
      readPriceValue(o.number) ??
      readPriceValue(o.minPrice) ??
      readPriceValue(o.maxPrice)
    );
  }
  return undefined;
}

function walkPrice(root: unknown, depth: number, parentKey = ''): number | undefined {
  if (depth > 18 || root == null) return undefined;

  if (typeof root === 'number') {
    return isPriceKey(parentKey) && root > 0 && root < 1_000_000 ? root : undefined;
  }

  if (typeof root === 'string') {
    if (!isPriceKey(parentKey)) return undefined;
    return readPriceValue(root);
  }

  if (Array.isArray(root)) {
    if (parentKey === 'priceRange' || parentKey === 'priceRanges' || parentKey === 'priceTiers') {
      for (const item of root) {
        if (item && typeof item === 'object') {
          const tier = item as Record<string, unknown>;
          const n = readPriceValue(tier.price ?? tier.value ?? tier.minPrice ?? tier.maxPrice);
          if (n !== undefined) return n;
        }
      }
    }
    for (const item of root) {
      const hit = walkPrice(item, depth + 1, parentKey);
      if (hit !== undefined) return hit;
    }
    return undefined;
  }

  if (typeof root !== 'object') return undefined;
  const o = root as Record<string, unknown>;

  for (const [k, v] of Object.entries(o)) {
    if (!isPriceKey(k)) continue;
    if (k === 'priceRange' || k === 'priceRanges' || k === 'priceTiers') {
      if (Array.isArray(v) && v.length > 0) {
        const first = v[0];
        if (first && typeof first === 'object') {
          const tier = first as Record<string, unknown>;
          const n = readPriceValue(tier.price ?? tier.value ?? tier.minPrice ?? tier.maxPrice);
          if (n !== undefined) return n;
        }
      }
      if (typeof v === 'string') {
        const m = /([\d.]+)/.exec(v);
        if (m) {
          const n = coerceNumber(m[1]);
          if (n !== undefined && n > 0) return n;
        }
      }
    }
    const direct = readPriceValue(v);
    if (direct !== undefined) return direct;
    const hit = walkPrice(v, depth + 1, k);
    if (hit !== undefined) return hit;
  }

  for (const [k, v] of Object.entries(o)) {
    if (isPriceKey(k) || NON_PRICE_KEYS.has(k)) continue;
    const hit = walkPrice(v, depth + 1, k);
    if (hit !== undefined) return hit;
  }
  return undefined;
}

export function extractPriceFromJsonRoots(roots: unknown[]): number | undefined {
  for (const r of roots) {
    const p = walkPrice(r, 0);
    if (p !== undefined) return p;
  }
  return undefined;
}

/** DOM 文本区域提取价格（¥ / 批发价 / 起批） */
export function extractPriceFromDomText(domPriceTexts: string[]): number | undefined {
  const candidates: number[] = [];
  for (const blob of domPriceTexts) {
    const text = trimStr(blob);
    if (!text) continue;
    const matches = text.matchAll(/(?:¥|￥)\s*([\d,]+(?:\.\d{1,2})?)/g);
    for (const m of matches) {
      const n = coerceNumber(m[1]);
      if (n !== undefined && n >= 0.01 && n < 1_000_000) candidates.push(n);
    }
    const labeled =
      /(?:批发价|拿货价|价格|起批价|新人价|分销价)[：:\s]*([\d,]+(?:\.\d{1,2})?)/.exec(text);
    if (labeled?.[1]) {
      const n = coerceNumber(labeled[1]);
      if (n !== undefined && n >= 0.01 && n < 1_000_000) candidates.push(n);
    }
  }
  if (candidates.length === 0) return undefined;
  return Math.min(...candidates);
}

export function applyProductPriceToSkus(skus: ProductSku[], productPrice?: number): ProductSku[] {
  if (productPrice === undefined) return skus;
  return skus.map((s) => ({
    ...s,
    price: s.price ?? productPrice,
  }));
}

export function hasAnySkuPrice(skus: ProductSku[]): boolean {
  return skus.some((s) => typeof s.price === 'number' && s.price > 0);
}

export function collectDomPriceTexts(payload: BrowserExtractPayload): string[] {
  return payload.domPriceTexts ?? [];
}
