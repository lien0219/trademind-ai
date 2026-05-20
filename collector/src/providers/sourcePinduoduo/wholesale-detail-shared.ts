import type { ProductSku } from '../../types/product.js';

/** Platform / navigation titles that must not be used as product title. */
export const PIFA_PLATFORM_TITLE_RE =
  /拼多多批发|拼多多官方采购批发平台|多多批发|^登录$|^首页$|购物车|搜索/i;

/** Button / chrome text often concatenated into title containers. */
export const TITLE_NOISE_PHRASES = [
  '分享商品',
  '收藏',
  '加入购物车',
  '立即购买',
  '联系客服',
  '店铺信息',
  '购物车',
  '已售',
  '批发价',
  '原价',
  '去拼单',
  '马上抢',
  '查看详情',
  '客服',
  '进店',
  '关注店铺',
] as const;

export const TITLE_CONTAMINATION_RE =
  /分享商品|加入购物车|立即购买|联系客服|店铺信息|收藏商品/i;

export type PriceRange = {
  priceMin?: number;
  priceMax?: number;
  priceText?: string;
  currency: string;
};

export type WholesaleWarningCode =
  | 'title_maybe_platform_title'
  | 'title_maybe_contaminated'
  | 'description_missing'
  | 'sku_stock_unknown'
  | 'sku_price_fallback_to_min_price'
  | 'attributes_missing'
  | 'sku_parse_failed'
  | 'detail_images_lazy_load'
  | 'description_images_missing'
  | 'main_images_missing'
  | 'no_main_images'
  | 'main_images_too_many'
  | 'main_image_fallback_from_sku'
  | 'main_image_fallback_from_detail'
  | 'main_image_fallback_from_page'
  | 'main_images_fallback_used'
  | 'main_images_maybe_incomplete'
  | 'sku_rows_detected_but_empty'
  | 'images_filtered';

export const WARNING_MESSAGES: Record<WholesaleWarningCode, string> = {
  title_maybe_platform_title: '当前标题可能为平台页标题而非商品标题，请人工核对。',
  title_maybe_contaminated: '标题可能混入了分享、购买等按钮文字，请人工核对。',
  description_missing: '未采集到商品描述文本，可后续使用 AI 生成描述。',
  sku_stock_unknown: '部分规格未能识别库存，请人工核对。',
  sku_price_fallback_to_min_price: '部分规格价格未能识别，已使用商品最低价作为兜底。',
  attributes_missing: '未能识别商品参数，请人工补充。',
  sku_parse_failed: '页面可能存在多个规格，但当前未能完整识别，请人工检查或重新采集。',
  detail_images_lazy_load: '详情图可能未完全加载，请核对商品介绍区域图片。',
  description_images_missing: '未识别到详情图，可手动补充或重新采集。',
  main_images_missing: '未识别到商品主图，请人工补充。',
  no_main_images: '未识别到商品主图，请在图片管理中手动添加后再发布。',
  main_images_too_many: '主图数量偏多，可能混入了详情图，请人工核对。',
  main_image_fallback_from_sku: '主图由规格图自动兜底生成，请发布前确认是否正确。',
  main_image_fallback_from_detail: '主图由详情图自动兜底生成，请发布前确认是否正确。',
  main_image_fallback_from_page: '主图由页面商品图池自动兜底生成，请发布前确认是否正确。',
  main_images_fallback_used: '部分图片由系统自动兜底识别，请发布前确认是否正确。',
  main_images_maybe_incomplete: '主图数量较少，可能未采集完整，请发布前检查图片。',
  sku_rows_detected_but_empty: '页面疑似存在规格行但未解析出 SKU，请人工检查。',
  images_filtered: '已过滤部分店铺图、服务图或无关图片，请核对主图与详情图。',
};

export type ImageSource =
  | 'main_gallery'
  | 'thumbnail_gallery'
  | 'detail_section'
  | 'sku_image'
  | 'shop_info'
  | 'unknown';

export function isPlatformTitle(text: string): boolean {
  const t = text.replace(/\s+/g, ' ').trim();
  if (!t) return true;
  return PIFA_PLATFORM_TITLE_RE.test(t);
}

export function cleanProductTitle(raw: string): {
  title: string;
  cleaned: boolean;
  contaminated: boolean;
} {
  let t = raw.replace(/\s+/g, ' ').trim();
  let cleaned = false;
  const before = t;

  for (const phrase of TITLE_NOISE_PHRASES) {
    const re = new RegExp(phrase.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
    const next = t.replace(re, ' ').replace(/\s+/g, ' ').trim();
    if (next !== t) {
      cleaned = true;
      t = next;
    }
  }

  t = t.replace(/分享商品\s*$/i, '').replace(/\s+分享商品\s*$/i, '').trim();

  if (t !== before) cleaned = true;

  const contaminated = TITLE_CONTAMINATION_RE.test(before) || TITLE_CONTAMINATION_RE.test(t);
  return { title: t.slice(0, 500), cleaned, contaminated };
}

/** Remove stock hints from SKU display name. */
export function cleanSkuName(raw: string): string {
  return raw
    .replace(/\s+/g, ' ')
    .replace(/仅剩\s*\d+\s*件/gi, '')
    .replace(/库存\s*\d+\s*件?/gi, '')
    .replace(/缺货|售罄/gi, '')
    .trim();
}

/** Parse texts like ¥7.75 - 12.65 or ¥7.75~12.65 */
export function parsePriceRangeText(raw: string): PriceRange {
  const text = raw.replace(/\s+/g, ' ').trim();
  const currency = /[¥￥]|元/.test(text) ? 'CNY' : '';
  const nums = [...text.replace(/,/g, '').matchAll(/(\d+(?:\.\d+)?)/g)]
    .map((m) => Number.parseFloat(m[1]))
    .filter((n) => Number.isFinite(n) && n > 0 && n < 999_999);
  if (nums.length === 0) {
    return { currency: currency || 'CNY', priceText: text || undefined };
  }
  const priceMin = Math.min(...nums);
  const priceMax = Math.max(...nums);
  return {
    priceMin,
    priceMax: nums.length > 1 ? priceMax : priceMin,
    priceText: text || undefined,
    currency: currency || 'CNY',
  };
}

export function warningMessage(code: WholesaleWarningCode): string {
  return WARNING_MESSAGES[code] ?? code;
}

export function appendWarning(
  codes: WholesaleWarningCode[],
  messages: string[],
  code: WholesaleWarningCode,
): void {
  if (codes.includes(code)) return;
  codes.push(code);
  messages.push(warningMessage(code));
}

const DESC_NOISE_RE =
  /分享商品|加入购物车|立即购买|联系客服|店铺信息|购物车|批发价|原价|已售|拼多多批发|多多批发/i;

export function isDescriptionNoise(text: string): boolean {
  const t = text.replace(/\s+/g, ' ').trim();
  if (!t || t.length < 12) return true;
  if (DESC_NOISE_RE.test(t)) return true;
  if (/^[¥￥]\s*[\d,.]+/.test(t)) return true;
  if (isPlatformTitle(t)) return true;
  return false;
}

/** Build factual description from page snippets + attributes (no fabrication). */
export function buildMainDescription(input: {
  introTexts: string[];
  title: string;
  attributes: Record<string, string | number | boolean>;
}): string {
  const blocks: string[] = [];
  for (const raw of input.introTexts) {
    const t = raw.replace(/\s+/g, ' ').trim();
    if (!isDescriptionNoise(t)) blocks.push(t);
  }

  const unique = [...new Set(blocks)];
  if (unique.length > 0) {
    return unique.join('\n\n').slice(0, 8000);
  }

  const attrEntries = Object.entries(input.attributes).filter(
    ([k, v]) => k.trim() && String(v).trim() && !DESC_NOISE_RE.test(k),
  );
  if (attrEntries.length > 0 && input.title) {
    const lines = attrEntries.slice(0, 12).map(([k, v]) => `${k}：${String(v).trim()}`);
    return `${input.title}\n${lines.join('\n')}`.slice(0, 8000);
  }

  return '';
}

export type WholesaleSkuRow = {
  name: string;
  price?: number;
  stock?: number;
  imageUrl?: string;
};

export function wholesaleRowsToSkus(
  rows: WholesaleSkuRow[],
  priceMin?: number,
  codes: WholesaleWarningCode[] = [],
  messages: string[] = [],
): ProductSku[] {
  const skus: ProductSku[] = [];
  let stockUnknown = false;
  let priceFallback = false;

  for (const row of rows) {
    const name = cleanSkuName(row.name);
    if (!name || name.length < 2) continue;

    let price = row.price;
    if (price === undefined || price <= 0) {
      if (priceMin !== undefined && priceMin > 0) {
        price = priceMin;
        priceFallback = true;
      }
    }

    const sku: ProductSku = {
      properties: { 规格: name },
      price,
      image: row.imageUrl,
      raw: { name, stockHint: row.stock },
    };
    if (row.stock !== undefined && row.stock > 0) {
      sku.stock = row.stock;
    } else {
      stockUnknown = true;
    }
    skus.push(sku);
  }

  if (stockUnknown && skus.length > 0) {
    appendWarning(codes, messages, 'sku_stock_unknown');
  }
  if (priceFallback) {
    appendWarning(codes, messages, 'sku_price_fallback_to_min_price');
  }

  return skus.slice(0, 80);
}
