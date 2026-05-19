import type { ProductSku } from '../../types/product.js';
import { coerceInt, coerceNumber, dedupeStrings, isLikelyJunkImage, normalizeImageUrl, trimStr } from './utils.js';
import type { DimRow } from './sku-helpers.js';
import {
  extractSkuBucketPrice,
  extractSkuBucketStock,
  findSkuMapLikeObjects,
  inferTwoDimNames,
  normalizeSkuPropertyKeys,
  parseComboKey,
  skuNameFromProps,
} from './sku-helpers.js';

const BASE = 'https://detail.1688.com/offer/';

/** 从 window.context / result.data 等模块树定位 1688 详情 data 节点 */
export function find1688ResultData(roots: unknown[]): Record<string, unknown> | null {
  function walk(x: unknown, depth: number): Record<string, unknown> | null {
    if (depth > 10 || !x || typeof x !== 'object') return null;
    const o = x as Record<string, unknown>;
    const result = o.result;
    if (result && typeof result === 'object') {
      const data = (result as Record<string, unknown>).data;
      if (data && typeof data === 'object') return data as Record<string, unknown>;
    }
    if (o.gallery && typeof o.gallery === 'object') return o;
    if (Array.isArray(x)) {
      for (const i of x) {
        const hit = walk(i, depth + 1);
        if (hit) return hit;
      }
      return null;
    }
    for (const v of Object.values(o)) {
      const hit = walk(v, depth + 1);
      if (hit) return hit;
    }
    return null;
  }
  for (const r of roots) {
    const hit = walk(r, 0);
    if (hit) return hit;
  }
  return null;
}

function normalize1688Img(raw: string, baseUrl: string): string | null {
  let u = trimStr(raw);
  if (!u) return null;
  if (!/\.(jpg|jpeg|png|webp|gif)(\?|$)/i.test(u)) {
    u = `${u.replace(/[_.]+$/, '')}.jpg`;
  }
  const abs = normalizeImageUrl(u, baseUrl);
  if (!abs || isLikelyJunkImage(abs)) return null;
  return abs;
}

/** 主图：gallery.fields.mainImage / offerImgList */
export function extractMainImagesFrom1688Data(data: Record<string, unknown>, baseUrl: string): string[] {
  const urls: string[] = [];
  const gallery = data.gallery;
  if (gallery && typeof gallery === 'object') {
    const fields = (gallery as Record<string, unknown>).fields;
    if (fields && typeof fields === 'object') {
      const f = fields as Record<string, unknown>;
      const main = f.mainImage;
      if (Array.isArray(main)) {
        for (const item of main) {
          if (typeof item === 'string') {
            const abs = normalize1688Img(item, baseUrl);
            if (abs) urls.push(abs);
          }
        }
      }
      const offerList = f.offerImgList;
      if (Array.isArray(offerList)) {
        for (const item of offerList) {
          if (typeof item === 'string') {
            const abs = normalize1688Img(item, baseUrl);
            if (abs && !urls.some((u) => u.split('?')[0] === abs.split('?')[0])) urls.push(abs);
          }
        }
      }
    }
  }
  return dedupeStrings(urls, 12);
}

/** 详情图：detail 模块或 description 富文本字段中的 ibank 图 */
export function extractDetailImagesFrom1688Data(data: Record<string, unknown>, baseUrl: string): string[] {
  const urls: string[] = [];
  function collectStrings(x: unknown, depth: number, keyHint: string): void {
    if (depth > 14 || x == null) return;
    if (typeof x === 'string') {
      const hint = keyHint.toLowerCase();
      if (!hint.includes('detail') && !hint.includes('desc') && !hint.includes('content')) return;
      const re = /(https?:\/\/[^\s"'<>]+\.(?:jpg|jpeg|png|webp))/gi;
      let m: RegExpExecArray | null;
      while ((m = re.exec(x))) {
        const abs = normalize1688Img(m[1], baseUrl);
        if (abs) urls.push(abs);
      }
      return;
    }
    if (typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const i of x) collectStrings(i, depth + 1, keyHint);
      return;
    }
    for (const [k, v] of Object.entries(x as Record<string, unknown>)) {
      collectStrings(v, depth + 1, `${keyHint}.${k}`);
    }
  }
  for (const key of Object.keys(data)) {
    if (/detail|desc|content|template/i.test(key)) collectStrings(data[key], 0, key);
  }
  return dedupeStrings(urls.filter(isLikelyProductDetailUrl), 30);
}

function isLikelyProductDetailUrl(url: string): boolean {
  if (isLikelyJunkImage(url)) return false;
  return /\/img\/ibank\//i.test(url);
}

/** 默认单价（阶梯价取第一档） */
export function extractDefaultOfferPrice(data: Record<string, unknown>): number | undefined {
  function walk(x: unknown, depth: number): number | undefined {
    if (depth > 12 || x == null || typeof x !== 'object') return undefined;
    if (Array.isArray(x)) {
      for (const i of x) {
        const hit = walk(i, depth + 1);
        if (hit !== undefined) return hit;
      }
      return undefined;
    }
    const o = x as Record<string, unknown>;
    for (const k of ['price', 'discountPrice', 'priceDisplay', 'salePrice', 'offerPrice', 'finalPrice']) {
      const n = coerceNumber(o[k]);
      if (n !== undefined && n > 0 && n < 1_000_000) return n;
    }
    const priceMoney = o.priceMoney ?? o.salePriceMoney;
    if (priceMoney && typeof priceMoney === 'object') {
      const n = coerceNumber((priceMoney as Record<string, unknown>).value);
      if (n !== undefined && n > 0) return n;
    }
    for (const v of Object.values(o)) {
      const hit = walk(v, depth + 1);
      if (hit !== undefined) return hit;
    }
    return undefined;
  }
  for (const mod of Object.values(data)) {
    const hit = walk(mod, 0);
    if (hit !== undefined) return hit;
  }
  return undefined;
}

function walkSkuPropArrays(data: Record<string, unknown>): unknown[] {
  const out: unknown[] = [];
  function walk(x: unknown, depth: number): void {
    if (depth > 14 || !x || typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const i of x) walk(i, depth + 1);
      return;
    }
    const o = x as Record<string, unknown>;
    for (const k of Object.keys(o)) {
      if (/^sku_props$/i.test(k) || k === 'saleProp' || k === 'saleProps' || k === 'skuProps') {
        out.push(o[k]);
      }
    }
    for (const v of Object.values(o)) walk(v, depth + 1);
  }
  walk(data, 0);
  return out;
}

export function extractAttributesFrom1688Data(data: Record<string, unknown>): Record<string, string> {
  const attrs: Record<string, string> = {};
  function walk(x: unknown, depth: number): void {
    if (depth > 14 || x == null || typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const row of x) {
        if (!row || typeof row !== 'object') continue;
        const r = row as Record<string, unknown>;
        const k = trimStr(String(r.name ?? r.attributeName ?? r.fname ?? ''));
        const v = trimStr(String(r.value ?? r.attributeValue ?? r.text ?? ''));
        if (k && v && !attrs[k]) attrs[k] = v;
      }
      return;
    }
    for (const v of Object.values(x as Record<string, unknown>)) walk(v, depth + 1);
  }
  for (const key of Object.keys(data)) {
    if (/attr|param|feature/i.test(key)) walk(data[key], 0);
  }
  return attrs;
}

/** 从 1688 context data 模块解析 SKU 列表 */
export function mineSkusFrom1688Data(
  data: Record<string, unknown>,
  dimRows: DimRow[],
  defaultPrice?: number,
): ProductSku[] {
  const maps = findSkuMapLikeObjects(data);
  if (!maps.length) return [];
  const skuMapMerged: Record<string, unknown> = Object.assign({}, ...maps);
  const dimNames = inferTwoDimNames(dimRows, '', '');
  const out: ProductSku[] = [];

  for (const rawKey of Object.keys(skuMapMerged).slice(0, 60)) {
    const bucket = (skuMapMerged[rawKey] ?? {}) as Record<string, unknown>;
    let props = normalizeSkuPropertyKeys(parseComboKey(rawKey, dimRows), dimNames);
    if (Object.keys(props).length === 0) continue;

    let price = extractSkuBucketPrice(bucket) ?? defaultPrice;
    let stock = extractSkuBucketStock(bucket);
    const skuCode = trimStr(String(bucket.specId ?? bucket.skuId ?? ''));

    let imageUrl: string | undefined;
    const imgRaw =
      (typeof bucket.pic === 'string' && bucket.pic) ||
      (typeof bucket.skuPictureUrl === 'string' && bucket.skuPictureUrl);
    if (imgRaw) {
      imageUrl = normalize1688Img(imgRaw, BASE) ?? undefined;
    }

    out.push({
      skuCode,
      properties: props,
      price,
      stock: stock && stock > 0 ? stock : undefined,
      image: imageUrl,
      raw: { source: '1688-context-data', skuMapKeySample: rawKey.slice(0, 120) },
    });
  }

  return out.map((ln) => ({
    ...ln,
    raw: { ...(ln.raw ?? {}), skuNameHint: skuNameFromProps(ln.properties ?? {}) },
  }));
}

export { walkSkuPropArrays };
