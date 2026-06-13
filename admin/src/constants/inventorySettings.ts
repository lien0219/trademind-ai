/** 库存预警开关项 */
export const INVENTORY_ALERT_TOGGLES = [
  {
    name: 'enable_inventory_alerts',
    label: '启用库存预警',
    extra: '关闭后不再生成新的库存预警记录',
  },
  {
    name: 'alert_out_of_stock',
    label: '售罄预警',
    extra: '本地库存为 0 或低于安全线时提示',
  },
  {
    name: 'alert_platform_stock_mismatch',
    label: '平台库存不一致预警',
    extra: '平台侧库存与本地库存差异超过阈值时提示',
  },
] as const;

/** 订单与库存联动开关项 */
export const INVENTORY_ORDER_TOGGLES = [
  {
    name: 'auto_match_order_skus',
    label: '平台订单自动规格匹配',
    extra: '入库后按平台规格编码匹配本地规格；失败不影响订单同步',
    link: { href: '/orders/sku-matches', text: '全局匹配记录' },
  },
  {
    name: 'auto_deduct_after_sku_match',
    label: '匹配成功后允许自动扣库存',
    extra: '默认关闭；需同时开启「平台订单自动扣库存」且订单行已绑定本地规格',
  },
  {
    name: 'auto_deduct_manual_orders',
    label: '手工订单创建后扣库存',
    extra: '与新建订单弹窗内的「创建后扣库存」任一为真则生效',
  },
  {
    name: 'auto_deduct_platform_orders',
    label: '平台订单自动扣库存',
    extra: '平台同步订单到达后，按策略尝试扣减本地库存',
  },
  {
    name: 'auto_restore_cancelled_orders',
    label: '取消 / 关闭订单回滚库存',
    extra: '订单取消或关闭时尝试恢复已扣减的本地库存',
  },
  {
    name: 'inventory_sync_after_deduct',
    label: '扣库后同步平台库存',
    extra: '本地扣减成功后排队同步到各平台（需商品已刊登且开启库存同步）',
  },
  {
    name: 'allow_manual_sku_bind_after_deduct',
    label: '扣库后仍允许人工绑定',
    extra: '已有成功扣库记录时，仍允许人工绑定未匹配行并再扣',
  },
  {
    name: 'allow_negative_stock',
    label: '允许负库存',
    extra: '开启后本地库存可扣成负数（谨慎使用）',
  },
] as const;

/** 平台限流配额字段 */
export const INVENTORY_PLATFORM_RATE_LIMITS = [
  { name: 'inventory_sync_platform_rate_limit_per_minute_tiktok', label: 'TikTok' },
  { name: 'inventory_sync_platform_rate_limit_per_minute_shopee', label: 'Shopee' },
  { name: 'inventory_sync_platform_rate_limit_per_minute_lazada', label: 'Lazada' },
  { name: 'inventory_sync_platform_rate_limit_per_minute_amazon', label: 'Amazon' },
] as const;
