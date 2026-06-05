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

export type TaobaoTmallCollectAlertState = {
  infoMessage: string;
  statusTags: CollectStatusTag[];
  errors: CollectAlertItem[];
  warnings: CollectAlertItem[];
};

const INFO_MESSAGE =
  '该商品来自淘宝/天猫采集，请发布前检查标题、价格、图片、商品规格和库存。';

const CODE_LEVEL: Record<string, CollectAlertLevel> = {
  PRICE_NOT_FOUND: 'error',
  SKU_INCOMPLETE: 'warning',
  DETAIL_IMAGES_INCOMPLETE: 'warning',
  title_missing: 'error',
  price_missing: 'error',
  no_main_images: 'error',
  sku_missing: 'warning',
};

const CODE_MESSAGE: Record<string, string> = {
  PRICE_NOT_FOUND: '未识别到商品价格，请手动填写价格后再发布。',
  SKU_INCOMPLETE: '商品规格识别不完整，请人工核对规格、价格与库存。',
  DETAIL_IMAGES_INCOMPLETE: '详情图可能未完全加载，请核对商品介绍区域图片。',
  title_missing: '商品标题为空，请填写标题后再发布。',
  price_missing: '未识别到商品价格，请手动填写价格。',
  no_main_images: '未识别到商品主图，请在图片管理中手动添加。',
  sku_missing: '未识别到商品规格，请手动新增 SKU。',
};

export function isTaobaoTmallSource(source: string | undefined): boolean {
  const src = source?.trim().toLowerCase();
  return src === 'taobao_tmall' || src === 'taobao';
}

export function taobaoTmallWarningCodesFromRaw(rawData: unknown): string[] {
  if (!rawData || typeof rawData !== 'object') return [];
  const root = rawData as Record<string, unknown>;
  const inner = root.raw;
  if (!inner || typeof inner !== 'object') return [];
  const obj = inner as Record<string, unknown>;
  const out: string[] = [];
  for (const key of ['qualityWarnings', 'warnings']) {
    const w = obj[key];
    if (Array.isArray(w)) {
      for (const item of w) {
        if (typeof item === 'string' && item.trim()) out.push(item.trim());
      }
    }
  }
  return out;
}

function effectiveSkus(skus: ProductSKURow[] | undefined): ProductSKURow[] {
  return (skus ?? []).filter((s) => !String(s.id).startsWith('new_'));
}

function mainImageCount(images: ProductImageRow[] | undefined): number {
  return (images ?? []).filter((i) => i.imageType === 'main').length;
}

function hasProductPrice(data: ProductDetail): boolean {
  const codes = taobaoTmallWarningCodesFromRaw(data.rawData);
  if (!codes.includes('PRICE_NOT_FOUND')) {
    return effectiveSkus(data.skus).some((s) => s.price != null && s.price > 0);
  }
  return false;
}

export function buildTaobaoTmallCollectAlertState(data: ProductDetail): TaobaoTmallCollectAlertState {
  const codes = new Set(taobaoTmallWarningCodesFromRaw(data.rawData));
  if (!data.title?.trim()) codes.add('title_missing');
  if (!hasProductPrice(data)) codes.add('price_missing');
  if (mainImageCount(data.images) === 0) codes.add('no_main_images');
  if (effectiveSkus(data.skus).length === 0) codes.add('sku_missing');

  const errors: CollectAlertItem[] = [];
  const warnings: CollectAlertItem[] = [];
  for (const code of codes) {
    const level = CODE_LEVEL[code] ?? 'warning';
    const message = CODE_MESSAGE[code];
    if (!message) continue;
    const item = { code, message, level };
    if (level === 'error') errors.push(item);
    else warnings.push(item);
  }

  const mainCount = mainImageCount(data.images);
  const skus = effectiveSkus(data.skus);
  const statusTags: CollectStatusTag[] = [
    {
      key: 'title',
      label: data.title?.trim() ? '标题已识别' : '标题未填写',
      tone: data.title?.trim() ? 'success' : 'warning',
    },
    {
      key: 'price',
      label: hasProductPrice(data) ? '价格已识别' : '价格需核对',
      tone: hasProductPrice(data) ? 'success' : 'warning',
    },
    {
      key: 'mainImages',
      label: mainCount > 0 ? '主图已识别' : '主图缺失',
      tone: mainCount > 0 ? 'success' : 'warning',
    },
    {
      key: 'skus',
      label: skus.length > 0 ? '规格已识别' : '规格需补充',
      tone: skus.length > 0 ? 'success' : 'warning',
    },
  ];

  return { infoMessage: INFO_MESSAGE, statusTags, errors, warnings };
}
