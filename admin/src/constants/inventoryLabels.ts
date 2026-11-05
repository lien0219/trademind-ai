import { tr } from '@/i18n';
/** 库存域统一文案（F3）：阻断提示、状态、来源、推荐操作 */

export const INVENTORY_SKU_NOT_BOUND_MESSAGE =
  '当前 SKU 尚未绑定平台 SKU，暂不能同步平台库存。请先完成 SKU 绑定后再重试。';

export const INVENTORY_SKU_AMBIGUOUS_MESSAGE =
  '当前 SKU 存在多个可能匹配项，需要人工确认后才能继续同步库存。';

export const INVENTORY_SYNC_DISABLED_MESSAGE =
  '当前店铺未开启平台库存同步。开启后，系统可在人工确认后将本地库存同步到平台。';

export const INVENTORY_STOCK_STATUS: Record<string, { text: string; color: string }> = {
  normal: { get text() { return tr("正常"); }, color: 'green' },
  low_stock: { get text() { return tr("低库存"); }, color: 'orange' },
  out_of_stock: { get text() { return tr("库存为 0"); }, color: 'red' },
  below_safety_stock: { get text() { return tr("低于安全线"); }, color: 'gold' },
  stock_unknown: { get text() { return tr("库存未知"); }, color: 'default' },
};

export const INVENTORY_ALERT_STATUS: Record<string, { text: string; color: string }> = {
  normal: { get text() { return tr("正常"); }, color: 'green' },
  out_of_stock: { get text() { return tr("库存为 0"); }, color: 'red' },
  low_stock: { get text() { return tr("低库存"); }, color: 'orange' },
  below_safety_stock: { get text() { return tr("低于安全线"); }, color: 'gold' },
  platform_stock_unknown: { get text() { return tr("库存未知"); }, color: 'default' },
  platform_stock_mismatch: { get text() { return tr("平台不一致"); }, color: 'orange' },
  inventory_sync_failed: { get text() { return tr("同步失败"); }, color: 'red' },
  sku_unbound: { get text() { return tr("SKU 未绑定"); }, color: 'volcano' },
};

export const INVENTORY_BIND_STATUS: Record<string, { text: string; color: string }> = {
  bound: { get text() { return tr("已绑定"); }, color: 'green' },
  unbound: { get text() { return tr("未绑定"); }, color: 'orange' },
  ambiguous: { get text() { return tr("待确认"); }, color: 'gold' },
  none: { get text() { return tr("无刊登"); }, color: 'default' },
};

export const INVENTORY_SYNC_STATUS: Record<string, { text: string; color: string }> = {
  success: { get text() { return tr("成功"); }, color: 'green' },
  failed: { get text() { return tr("失败"); }, color: 'red' },
  pending: { get text() { return tr("等待中"); }, color: 'default' },
  running: { get text() { return tr("执行中"); }, color: 'processing' },
  blocked: { get text() { return tr("被阻断"); }, color: 'volcano' },
  disabled: { get text() { return tr("未开启"); }, color: 'default' },
  none: { get text() { return tr("未同步"); }, color: 'default' },
  partial_success: { get text() { return tr("部分成功"); }, color: 'warning' },
  cancelled: { get text() { return tr("已取消"); }, color: 'default' },
};

export const INVENTORY_RECONCILIATION_STATUS: Record<string, { text: string; color: string }> = {
  matched: { get text() { return tr("投影一致"); }, color: 'green' },
  unmigrated: { get text() { return tr("待建仓库账"); }, color: 'gold' },
  mismatch: { get text() { return tr("需要对账"); }, color: 'red' },
};

export const INVENTORY_DEDUCT_SOURCE: Record<string, string> = {
  deduct: '订单同步扣减',
  restore: '系统回滚',
  manual_adjust: '人工修正',
  order_deduct: '订单同步扣减',
  order_cancel_restore: '系统回滚',
};

export const INVENTORY_DEDUCT_STATUS: Record<string, { text: string; color: string }> = {
  success: { get text() { return tr("成功"); }, color: 'green' },
  failed: { get text() { return tr("失败"); }, color: 'red' },
  skipped: { get text() { return tr("跳过"); }, color: 'default' },
  pending: { get text() { return tr("等待中"); }, color: 'processing' },
};

export const INVENTORY_SYNC_BLOCK_REASON: Record<string, string> = {
  sku_not_bound: 'SKU 未绑定',
  sku_ambiguous: 'SKU 绑定冲突',
  product_not_bound: '平台商品未创建',
  platform_sku_missing: '平台 SKU 缺失',
  sync_disabled: '库存同步开关未开启',
  permission_denied: '平台权限不足',
  stock_invalid: '库存值非法',
};

export const INVENTORY_FAILURE_CATEGORY_LABEL: Record<string, string> = {
  inventory_deduct_failed: '库存扣减失败',
  inventory_sync_failed: '库存同步失败',
  inventory_sync_partial_success: '库存同步部分成功',
  inventory_sku_not_bound: 'SKU 未绑定阻断',
  inventory_sku_ambiguous: 'SKU 绑定歧义阻断',
  inventory_stock_invalid: '库存值非法',
  inventory_platform_permission_denied: '平台库存权限不足',
  inventory_product_not_bound: '平台商品未绑定',
  inventory_platform_sku_missing: '平台 SKU 缺失',
};

export function inventoryTagFromMap(
  raw: string,
  map: Record<string, { text: string; color: string }>,
) {
  const k = (raw || '').trim();
  const cfg = map[k];
  if (!cfg) return { text: k || '—', color: 'default' as const };
  return cfg;
}

export function inventoryBindBlockHint(status?: string): string | undefined {
  const s = (status || '').trim().toLowerCase();
  if (s === 'unbound' || s === 'unmatched') return INVENTORY_SKU_NOT_BOUND_MESSAGE;
  if (s === 'ambiguous') return INVENTORY_SKU_AMBIGUOUS_MESSAGE;
  return undefined;
}
