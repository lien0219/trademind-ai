/** 平台与店铺展示名（扩展 userFriendly.platformLabel） */

export const PLATFORM_DISPLAY_LABEL: Record<string, string> = {
  douyin_shop: '抖店',
  tiktok: 'TikTok Shop',
  shopee: 'Shopee',
  lazada: 'Lazada',
  amazon: 'Amazon',
  mock: '模拟',
  manual: '手动',
};

export const SHOP_AUTH_STATUS_LABEL: Record<string, string> = {
  unauthorized: '未授权',
  authorized: '已授权',
  expired: '授权已过期',
  invalid: '异常',
  need_check: '需要检查',
  error: '异常',
  unsupported: '不支持',
};

export function platformDisplayLabel(platform?: string | null): string {
  const k = (platform ?? '').trim().toLowerCase();
  if (!k) return '—';
  return PLATFORM_DISPLAY_LABEL[k] ?? platform ?? '—';
}

export function shopAuthStatusLabel(status?: string | null): string {
  const k = (status ?? '').trim().toLowerCase();
  return SHOP_AUTH_STATUS_LABEL[k] ?? status ?? '—';
}

/** 商品字段英文名 → 中文 */
export const PRODUCT_FIELD_LABEL: Record<string, string> = {
  specs: '规格',
  sku: '规格编码',
  'main images': '主图',
  'detail images': '详情图',
  'external links': '外链图片',
  main_images: '主图',
  detail_images: '详情图',
  platform: '平台',
  shop: '店铺',
  stock: '库存',
  price: '售价',
  category: '类目',
  attributes: '商品参数',
};

export function productFieldLabel(field?: string | null): string {
  const k = (field ?? '').trim().toLowerCase();
  return PRODUCT_FIELD_LABEL[k] ?? field ?? '—';
}
