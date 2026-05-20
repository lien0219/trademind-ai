import type { ProductSku } from '../../types/product.js';
import { coerceInt, coerceNumber, trimStr } from './utils.js';
import type { DomSkuTableRow } from './types.js';

export interface DimRow {
  name: string;
  values: string[];
}

const COLOR_VALUE_RE = /颜色|蓝|粉|黄|绿|米白|红|黑|白|灰|卡其|藏青|不锈钢|更衣柜|玻璃|移门|消毒/;
const SIZE_VALUE_RE = /内长|尺码|尺寸|cm|码|鞋底标|mm|厚度/;

export function inferTwoDimNames(dimHints: DimRow[], v1: string, v2: string): [string, string] {
  if (dimHints.length >= 2) return [dimHints[0].name, dimHints[1].name];
  const first = dimHints[0]?.name;
  const second = dimHints[1]?.name;
  if (first && second) return [first, second];
  if (COLOR_VALUE_RE.test(v1) || SIZE_VALUE_RE.test(v2)) return ['颜色', '尺码'];
  if (COLOR_VALUE_RE.test(v2) || SIZE_VALUE_RE.test(v1)) return ['尺码', '颜色'];
  return ['颜色', '尺码'];
}

/**
 * 解析 skuMap 键：支持「颜色:蓝;尺码:M」「蓝色>内长12」等。
 * 1688 常见键为「蓝色【F106】>内长12【鞋底标12.5】」（无维度名，仅两段的值）。
 */
export function parseComboKey(raw: string, dimHints: DimRow[] = []): Record<string, string> {
  const props: Record<string, string> = {};
  const normalized = raw.replace(/&gt;/gi, '>').replace(/&amp;/g, '&').trim();
  const segs = normalized.split(/[;；#]/).map((x) => x.trim()).filter(Boolean);
  for (const seg of segs) {
    const m = seg.match(/^([^:：#\s]{1,24})\s*[#:：]\s*(.+)$/);
    if (!m) continue;
    const k = trimStr(m[1]);
    const v = trimStr(m[2]);
    if (k && v) props[k] = v;
  }
  if (Object.keys(props).length > 0) return props;

  const gtParts = normalized.split(/>|»/).map((x) => trimStr(x)).filter(Boolean);
  if (gtParts.length === 2) {
    const [a, b] = gtParts;
    const [n1, n2] = inferTwoDimNames(dimHints, a, b);
    return { [n1]: a, [n2]: b };
  }
  if (gtParts.length >= 4 && gtParts.length % 2 === 0) {
    for (let i = 0; i < gtParts.length; i += 2) {
      const k = gtParts[i];
      const v = gtParts[i + 1];
      if (k && v) props[k] = v;
    }
    return props;
  }
  return props;
}

/** 修正误把颜色值当作属性名的结构 */
export function normalizeSkuPropertyKeys(
  props: Record<string, string>,
  dimNames: [string, string],
): Record<string, string> {
  const keys = Object.keys(props);
  if (keys.length !== 1) return props;
  const onlyKey = keys[0];
  const val = props[onlyKey] ?? '';
  if (/颜色|尺码|规格/.test(onlyKey)) return props;
  if (COLOR_VALUE_RE.test(onlyKey) && SIZE_VALUE_RE.test(val)) {
    return { [dimNames[0]]: onlyKey, [dimNames[1]]: val };
  }
  if (SIZE_VALUE_RE.test(onlyKey) && COLOR_VALUE_RE.test(val)) {
    return { [dimNames[0]]: val, [dimNames[1]]: onlyKey };
  }
  return props;
}

export function skuNameFromProps(props: Record<string, string>): string {
  const keys = Object.keys(props).sort();
  return keys.map((k) => `${props[k]}`).join(' / ');
}

/** 过滤 DOM 维度值中的标签/价格/库存拼接噪声 */
export function isValidSkuDimensionValue(value: string, dimName: string): boolean {
  const v = trimStr(value);
  if (!v || v.length < 2 || v.length > 100) return false;
  if (v === dimName) return false;
  if (/^(颜色|尺寸|尺码|规格|库存|价格|数量|厚度)$/.test(v)) return false;
  if (/¥|￥/.test(v)) return false;
  if (/库存\s*\d+/.test(v)) return false;
  if (/^库存\d+/.test(v)) return false;
  if (/\d+(?:\.\d+)?\s*mm\s*¥|\d+mm.*¥.*库存/i.test(v)) return false;
  if (/^尺寸[\d.]+\s*mm/i.test(v)) return false;
  return true;
}

export function extractSkuBucketPrice(bucket: Record<string, unknown>): number | undefined {
  const direct =
    bucket.price ??
    bucket.priceMoney ??
    bucket.discountPrice ??
    bucket.originPrice ??
    bucket.consignPrice ??
    bucket.retailPrice;
  let price = coerceNumber(direct);
  if (price === undefined && typeof bucket.price === 'object' && bucket.price) {
    price =
      coerceNumber((bucket.price as Record<string, unknown>).value) ??
      coerceNumber((bucket.price as Record<string, unknown>).number);
  }
  if (price === undefined) {
    const promo = bucket.promotionPrices;
    if (promo && typeof promo === 'object') {
      price =
        coerceNumber((promo as Record<string, unknown>).finalPrice) ??
        coerceNumber((promo as Record<string, unknown>).salePriceMoney);
    }
  }
  if (price === undefined && bucket.specAttrs) {
    price = extractFromSpecAttrs(bucket.specAttrs).price;
  }
  return price;
}

export function extractSkuBucketStock(bucket: Record<string, unknown>): number | undefined {
  let stock =
    coerceInt(bucket.canBookCount) ??
    coerceInt(bucket.amountOnSale) ??
    coerceInt(bucket.saleCount) ??
    coerceInt(bucket.amount) ??
    coerceInt(bucket.bookCount) ??
    coerceInt(bucket.stock);
  if ((stock === undefined || stock === 0) && bucket.specAttrs) {
    const fromAttrs = extractFromSpecAttrs(bucket.specAttrs).stock;
    if (fromAttrs !== undefined && fromAttrs > 0) stock = fromAttrs;
  }
  return stock;
}

function extractFromSpecAttrs(raw: unknown): { price?: number; stock?: number } {
  let o: Record<string, unknown> | null = null;
  if (typeof raw === 'string') {
    try {
      o = JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return {};
    }
  } else if (raw && typeof raw === 'object') {
    o = raw as Record<string, unknown>;
  }
  if (!o) return {};
  return {
    price: coerceNumber(o.price ?? o.discountPrice ?? o.salePrice),
    stock: coerceInt(o.canBookCount ?? o.amountOnSale ?? o.stock),
  };
}

/** 在 JSON 树中查找形如 skuMap 的对象（值为含 canBookCount/skuId 的桶） */
export function findSkuMapLikeObjects(root: unknown, depth = 0, out: Record<string, unknown>[] = []): Record<string, unknown>[] {
  if (depth > 16 || !root || typeof root !== 'object') return out;
  if (Array.isArray(root)) {
    for (const i of root) findSkuMapLikeObjects(i, depth + 1, out);
    return out;
  }
  const o = root as Record<string, unknown>;
  const entries = Object.entries(o);
  if (entries.length >= 2 && entries.length <= 120) {
    let skuLike = 0;
    for (const [, v] of entries.slice(0, 8)) {
      if (v && typeof v === 'object' && !Array.isArray(v)) {
        const b = v as Record<string, unknown>;
        if ('canBookCount' in b || 'skuId' in b || 'specId' in b) skuLike++;
      }
    }
    if (skuLike >= 2) {
      out.push(o);
      return out;
    }
  }
  for (const v of Object.values(o)) findSkuMapLikeObjects(v, depth + 1, out);
  return out;
}

/** 用 DOM 尺码表（价/库存）补全 JSON 中缺失或为 0 的字段 */
export function enrichSkusFromDomTable(skus: ProductSku[], rows: DomSkuTableRow[]): ProductSku[] {
  if (!rows.length || !skus.length) return skus;
  return skus.map((sku) => {
    const vals = Object.values(sku.properties ?? {});
    const sizePart = vals.find((v) => SIZE_VALUE_RE.test(v)) ?? vals[vals.length - 1] ?? '';
    const row = rows.find((r) => {
      if (!sizePart) return false;
      const normSize = sizePart.replace(/\s+/g, '').toLowerCase();
      const normLabel = r.label.replace(/\s+/g, '').toLowerCase();
      if (normLabel.includes(normSize) || normSize.includes(normLabel)) return true;
      const needle = sizePart.slice(0, Math.min(8, sizePart.length));
      return r.label.includes(needle);
    });
    if (!row) return sku;
    const price = sku.price ?? (row.priceText ? coerceNumber(row.priceText) : undefined);
    const stock =
      sku.stock && sku.stock > 0 ? sku.stock : row.stockText ? coerceInt(row.stockText) : sku.stock;
    return { ...sku, price, stock: stock && stock > 0 ? stock : undefined };
  });
}
