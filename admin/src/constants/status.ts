/** 商品草稿状态（与后端约定保持一致） */
export const PRODUCT_STATUS = {
  draft: { text: '草稿', color: 'default' as const },
  ai_processing: { text: 'AI 处理中', color: 'processing' as const },
  ready: { text: '可用', color: 'success' as const },
  published: { text: '已发布', color: 'blue' as const },
  archived: { text: '已归档', color: 'default' as const },
};

/** 采集 / 异步任务统一状态 */
export const COLLECT_TASK_STATUS = {
  pending: { text: '等待处理', color: 'processing' as const },
  running: { text: '处理中', color: 'processing' as const },
  success: { text: '成功', color: 'success' as const },
  failed: { text: '失败', color: 'error' as const },
  cancelled: { text: '已取消', color: 'default' as const },
  retrying: { text: '等待重试', color: 'warning' as const },
  success_with_warnings: { text: '成功（有警告）', color: 'warning' as const },
  success_with_review: { text: '成功（建议检查）', color: 'warning' as const },
  low_quality: { text: '失败：质量不达标', color: 'error' as const },
  need_manual_review: { text: '需人工检查', color: 'warning' as const },
  failed_render_validation: { text: '渲染校验失败', color: 'error' as const },
  obsolete: { text: '已过期', color: 'default' as const },
};

/** 采集批次聚合状态 */
export const COLLECT_BATCH_STATUS = {
  pending: { text: '待开始', color: 'default' as const },
  running: { text: '进行中', color: 'processing' as const },
  partial_success: { text: '部分成功', color: 'warning' as const },
  success: { text: '全部成功', color: 'success' as const },
  failed: { text: '全部失败', color: 'error' as const },
  cancelled: { text: '已取消', color: 'default' as const },
};

/** 客服会话状态（后端 customer_conversations.status） */
export const CUSTOMER_CONVERSATION_STATUS = {
  open: { text: '进行中', color: 'processing' as const },
  pending_reply: { text: '待回复', color: 'warning' as const },
  replied: { text: '已回复', color: 'success' as const },
  closed: { text: '已关闭', color: 'default' as const },
};

/** 手工订单 orders.status（与后端 order/constants 对齐） */
export const ORDER_STATUS = {
  pending: { text: '待处理', color: 'default' as const },
  paid: { text: '已付款', color: 'success' as const },
  processing: { text: '处理中', color: 'processing' as const },
  shipped: { text: '已发货', color: 'blue' as const },
  delivered: { text: '已送达', color: 'cyan' as const },
  cancelled: { text: '已取消', color: 'warning' as const },
  refunded: { text: '已退款', color: 'warning' as const },
  closed: { text: '已关闭', color: 'default' as const },
};

export const ORDER_PAYMENT_STATUS = {
  unpaid: { text: '未支付', color: 'default' as const },
  paid: { text: '已支付', color: 'success' as const },
  partially_refunded: { text: '部分退款', color: 'warning' as const },
  refunded: { text: '已退款', color: 'warning' as const },
};

export const ORDER_FULFILLMENT_STATUS = {
  unfulfilled: { text: '未履约', color: 'default' as const },
  partial: { text: '部分履约', color: 'processing' as const },
  fulfilled: { text: '已履约', color: 'success' as const },
  returned: { text: '已退货', color: 'warning' as const },
};

export const ORDER_SHIPMENT_STATUS = {
  pending: { text: '待发货', color: 'default' as const },
  shipped: { text: '已发货', color: 'blue' as const },
  in_transit: { text: '运输中', color: 'processing' as const },
  delivered: { text: '已签收', color: 'success' as const },
  exception: { text: '异常', color: 'error' as const },
  returned: { text: '已退回', color: 'warning' as const },
};

/** 统一店铺 shops.status */
export const SHOP_STATUS = {
  active: { text: '启用', color: 'success' as const },
  disabled: { text: '停用', color: 'default' as const },
};

/** 统一店铺 shops.auth_status */
export const SHOP_AUTH_STATUS = {
  unauthorized: { text: '未授权', color: 'default' as const },
  authorized: { text: '已授权', color: 'success' as const },
  expired: { text: '已过期', color: 'warning' as const },
  invalid: { text: '异常', color: 'error' as const },
  need_check: { text: '待检查', color: 'warning' as const },
  error: { text: '异常', color: 'error' as const },
  unsupported: { text: '不支持', color: 'default' as const },
};

/** Platform Provider 元信息 status（采集与店铺_provider 语义一致） */
export const PLATFORM_PROVIDER_STATUS = {
  available: { text: '可用', color: 'success' as const },
  beta: { text: '测试', color: 'processing' as const },
  planned: { text: '规划中', color: 'default' as const },
  disabled: { text: '停用', color: 'default' as const },
};

/** 订单同步任务 order_sync_tasks.status */
export const ORDER_SYNC_TASK_STATUS = {
  pending: { text: '等待处理', color: 'processing' as const },
  running: { text: '处理中', color: 'processing' as const },
  partial_success: { text: '部分成功', color: 'warning' as const },
  success: { text: '成功', color: 'success' as const },
  failed: { text: '失败', color: 'error' as const },
  cancelled: { text: '已取消', color: 'default' as const },
};

/** 客服消息同步任务 customer_message_sync_tasks.status（与订单同步任务状态语义一致） */
export const CUSTOMER_MESSAGE_SYNC_TASK_STATUS = ORDER_SYNC_TASK_STATUS;
