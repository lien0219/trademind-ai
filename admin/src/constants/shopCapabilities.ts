/** 店铺 / 平台能力标识 → 中文展示（与 backend/internal/providers/platform/types.go 对齐） */
export const SHOP_CAPABILITY_LABELS: Record<string, string> = {
  order_sync: '订单同步',
  product_publish: '商品刊登',
  customer_message: '客服消息',
  inventory_sync: '库存同步',
  logistics_sync: '物流同步',
  refund_after_sale: '退款售后',
  shop_info: '店铺信息',
  manual_manage: '手工管理',
};

export function shopCapabilityLabel(key: string): string {
  const k = (key ?? '').trim();
  return SHOP_CAPABILITY_LABELS[k] ?? k;
}
