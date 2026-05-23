import type { ProductDetail, ProductImageRow, ProductSKURow } from '@/services/products';

export type CollectAlertLevel = 'error' | 'warning' | 'info';

export type CollectAlertItem = {
  code: string;
  message: string;
  level: CollectAlertLevel;
};

export type CollectStatusTag = {
  key: string;
  label: string;
  tone: 'success' | 'warning' | 'default';
};

export type PinduoduoCollectAlertState = {
  infoMessage: string;
  statusTags: CollectStatusTag[];
  errors: CollectAlertItem[];
  warnings: CollectAlertItem[];
};

const PINDUODUO_INFO_MESSAGE =
  '该商品来自拼多多采集器，请发布前检查价格、图片、规格和库存。';

/** Codes that never appear in yellow/error lists (source notice, fallback notices, etc.). */
const INFO_ONLY_CODES = new Set([
  'source_pinduoduo',
  'pinduoduo_beta_notice',
  'partial_parse_notice',
  'fallback_image_used',
  'main_images_fallback_used',
  'main_image_fallback_from_sku',
  'main_image_fallback_from_detail',
  'main_image_fallback_from_page',
  'images_filtered',
]);

const CODE_LEVEL: Record<string, CollectAlertLevel> = {
  title_missing: 'error',
  price_missing: 'error',
  no_main_images: 'error',
  main_images_missing: 'error',
  sku_missing: 'error',
  sku_price_missing: 'error',
  stock_missing: 'warning',
  description_missing: 'warning',
  description_images_missing: 'warning',
  main_images_maybe_incomplete: 'warning',
  sku_stock_unknown: 'warning',
  attributes_missing: 'warning',
  detail_images_lazy_load: 'warning',
  title_maybe_platform_title: 'warning',
  title_maybe_contaminated: 'warning',
  sku_parse_failed: 'warning',
  sku_rows_detected_but_empty: 'warning',
  main_images_too_many: 'warning',
  sku_price_fallback_to_min_price: 'warning',
  source_pinduoduo: 'info',
  pinduoduo_beta_notice: 'info',
  fallback_image_used: 'info',
  partial_parse_notice: 'info',
  main_images_fallback_used: 'info',
  main_image_fallback_from_sku: 'info',
  main_image_fallback_from_detail: 'info',
  main_image_fallback_from_page: 'info',
  images_filtered: 'info',
};

const CODE_MESSAGE: Record<string, string> = {
  title_missing: '商品标题为空，请填写标题后再发布。',
  price_missing: '未识别到商品价格，请手动填写价格。',
  sku_price_missing: '部分规格缺少有效价格，请核对后再发布。',
  no_main_images: '未识别到商品主图，请在图片管理中手动添加。',
  main_images_missing: '未识别到商品主图，请在图片管理中手动添加。',
  sku_missing: '未识别到商品规格，请手动新增 SKU。',
  stock_missing: '库存未完整识别，请人工核对。',
  description_missing: '未采集到商品描述，可使用 AI 生成描述。',
  description_images_missing: '未识别到详情图，可手动补充。',
  main_images_maybe_incomplete: '主图数量较少，请确认是否完整。',
  sku_stock_unknown: '部分规格库存未识别，请人工核对。',
  attributes_missing: '未识别到商品参数，可后续手动补充。',
  detail_images_lazy_load: '详情图可能未完全加载，请核对商品介绍区域图片。',
  title_maybe_platform_title: '标题可能为平台页标题，请人工核对。',
  title_maybe_contaminated: '标题可能混入按钮文案，请人工核对。',
  sku_parse_failed: '页面可能存在多个规格，但未能完整识别，请人工检查。',
  sku_rows_detected_but_empty: '页面疑似存在规格行但未解析出 SKU，请人工检查。',
  main_images_too_many: '主图数量偏多，可能混入了详情图，请人工核对。',
  sku_price_fallback_to_min_price: '部分规格价格未能识别，已使用商品最低价作为兜底。',
};

/** Suppress in yellow/error when message duplicates blue info or generic collector copy. */
const SUPPRESS_MESSAGE_RE =
  /该商品来自拼多多采集器|采集质量提示|商品规格和库存来自页面|商品规格来自页面自动识别|主图由规格图自动兜底|主图由详情图自动兜底|主图由页面商品图|部分图片由系统自动兜底|部分规格未能识别库存/i;

export function isPinduoduoSource(source: string | undefined): boolean {
  const src = source?.trim().toLowerCase();
  return src === 'pinduoduo' || src === 'pdd';
}

export function pinduoduoWarningCodesFromRaw(rawData: unknown): string[] {
  if (!rawData || typeof rawData !== 'object') return [];
  const root = rawData as Record<string, unknown>;
  const inner = root.raw;
  if (!inner || typeof inner !== 'object') return [];
  const w = (inner as Record<string, unknown>).warnings;
  if (!Array.isArray(w)) return [];
  return w.filter((x): x is string => typeof x === 'string' && x.trim().length > 0);
}

function effectiveSkus(skus: ProductSKURow[] | undefined): ProductSKURow[] {
  return (skus ?? []).filter((s) => !String(s.id).startsWith('new_'));
}

function mainImageCount(images: ProductImageRow[] | undefined): number {
  return (images ?? []).filter((i) => i.imageType === 'main').length;
}

function detailImageCount(images: ProductImageRow[] | undefined): number {
  return (images ?? []).filter(
    (i) => i.imageType === 'detail' || i.imageType === 'description',
  ).length;
}

function productPriceFromRaw(rawData: unknown): number | undefined {
  if (!rawData || typeof rawData !== 'object') return undefined;
  const inner = (rawData as Record<string, unknown>).raw;
  if (!inner || typeof inner !== 'object') return undefined;
  const p = (inner as Record<string, unknown>).productPrice;
  return typeof p === 'number' && p > 0 ? p : undefined;
}

function hasTitle(data: ProductDetail): boolean {
  return !!(data.title?.trim() || data.originalTitle?.trim());
}

function hasProductPrice(data: ProductDetail): boolean {
  if (productPriceFromRaw(data.rawData) != null) return true;
  return effectiveSkus(data.skus).some((s) => s.price != null && s.price > 0);
}

function hasSkuPriceGap(data: ProductDetail): boolean {
  const skus = effectiveSkus(data.skus);
  if (skus.length === 0) return false;
  return skus.some((s) => s.price == null || s.price <= 0);
}

function hasPartialUnknownStock(data: ProductDetail): boolean {
  const skus = effectiveSkus(data.skus);
  if (skus.length === 0) return false;
  const missing = skus.filter((s) => s.stock == null);
  return missing.length > 0;
}

function hasOnlyDefaultSku(skus: ProductSKURow[] | undefined): boolean {
  const rows = effectiveSkus(skus);
  return rows.length === 1 && rows[0]?.skuName?.trim() === '默认规格';
}

function resolveEffectiveCodes(data: ProductDetail): Set<string> {
  const codes = new Set(pinduoduoWarningCodesFromRaw(data.rawData));
  const mainCount = mainImageCount(data.images);
  const detailCount = detailImageCount(data.images);
  const skus = effectiveSkus(data.skus);

  if (hasTitle(data)) {
    codes.delete('title_missing');
  } else {
    codes.add('title_missing');
  }

  if (hasProductPrice(data) && !hasSkuPriceGap(data)) {
    codes.delete('price_missing');
    codes.delete('sku_price_missing');
  } else {
    if (!hasProductPrice(data)) codes.add('price_missing');
    if (hasSkuPriceGap(data)) codes.add('sku_price_missing');
  }

  if (mainCount >= 1) {
    codes.delete('no_main_images');
    codes.delete('main_images_missing');
    if (mainCount < 3) codes.add('main_images_maybe_incomplete');
  } else {
    codes.add('no_main_images');
    codes.delete('main_images_maybe_incomplete');
  }

  if (detailCount === 0) codes.add('description_images_missing');
  else codes.delete('description_images_missing');

  if (data.description?.trim()) codes.delete('description_missing');
  else codes.add('description_missing');

  if (skus.length > 0) {
    codes.delete('sku_missing');
    if (!hasOnlyDefaultSku(data.skus)) {
      codes.delete('sku_parse_failed');
      codes.delete('sku_rows_detected_but_empty');
    }
  } else {
    codes.add('sku_missing');
  }

  if (hasPartialUnknownStock(data) || codes.has('sku_stock_unknown')) {
    codes.add('sku_stock_unknown');
  }

  for (const infoCode of INFO_ONLY_CODES) {
    codes.delete(infoCode);
  }

  return codes;
}

function messageForCode(code: string): string | undefined {
  return CODE_MESSAGE[code];
}

function dedupeItems(items: CollectAlertItem[]): CollectAlertItem[] {
  const seenCodes = new Set<string>();
  const seenMessages = new Set<string>();
  const out: CollectAlertItem[] = [];
  for (const item of items) {
    if (seenCodes.has(item.code) || seenMessages.has(item.message)) continue;
    if (SUPPRESS_MESSAGE_RE.test(item.message)) continue;
    seenCodes.add(item.code);
    seenMessages.add(item.message);
    out.push(item);
  }
  return out;
}

export function buildStatusTags(data: ProductDetail): CollectStatusTag[] {
  const mainCount = mainImageCount(data.images);
  const skus = effectiveSkus(data.skus);
  const tags: CollectStatusTag[] = [];

  tags.push({
    key: 'title',
    label: hasTitle(data) ? '标题已识别' : '标题未填写',
    tone: hasTitle(data) ? 'success' : 'warning',
  });

  tags.push({
    key: 'price',
    label: hasProductPrice(data) && !hasSkuPriceGap(data) ? '价格已识别' : '价格需核对',
    tone: hasProductPrice(data) && !hasSkuPriceGap(data) ? 'success' : 'warning',
  });

  tags.push({
    key: 'mainImages',
    label: mainCount > 0 ? '主图已识别' : '主图缺失',
    tone: mainCount > 0 ? 'success' : 'warning',
  });

  tags.push({
    key: 'skus',
    label: skus.length > 0 ? '规格已识别' : '规格需补充',
    tone: skus.length > 0 ? 'success' : 'warning',
  });

  const stockNeedsCheck = skus.length === 0 || hasPartialUnknownStock(data);
  tags.push({
    key: 'stock',
    label: stockNeedsCheck ? '库存需核对' : '库存已识别',
    tone: stockNeedsCheck ? 'warning' : 'success',
  });

  tags.push({
    key: 'description',
    label: data.description?.trim() ? '描述已填写' : '描述未填写',
    tone: data.description?.trim() ? 'success' : 'default',
  });

  return tags;
}

export function buildPinduoduoCollectAlertState(data: ProductDetail): PinduoduoCollectAlertState {
  const codes = resolveEffectiveCodes(data);
  const errors: CollectAlertItem[] = [];
  const warnings: CollectAlertItem[] = [];

  for (const code of codes) {
    if (INFO_ONLY_CODES.has(code)) continue;
    const level = CODE_LEVEL[code] ?? 'warning';
    if (level === 'info') continue;
    const message = messageForCode(code);
    if (!message) continue;
    const item: CollectAlertItem = { code, message, level };
    if (level === 'error') errors.push(item);
    else warnings.push(item);
  }

  return {
    infoMessage: PINDUODUO_INFO_MESSAGE,
    statusTags: buildStatusTags(data),
    errors: dedupeItems(errors),
    warnings: dedupeItems(warnings),
  };
}
