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

/** 采集任务事件类型（collect_task_events.event_type） */
export const COLLECT_TASK_EVENT_TYPE: Record<string, string> = {
  'task.success': '任务成功',
  'task.failed': '任务失败',
  'task.retry_exhausted': '重试次数已用尽',
  'task.running': '任务执行中',
  'task.auto_retry_scheduled': '已安排自动重试',
  'task.auto_retry_enqueued': '自动重试已入队',
  'task.manual_retry': '手动重试',
  'batch.delay.applied': '批次延迟已生效',
};

export function collectTaskEventLabel(eventType?: string | null): string {
  const k = (eventType ?? '').trim();
  if (!k) return '—';
  return COLLECT_TASK_EVENT_TYPE[k] ?? k;
}

export function collectTaskStatusTransition(from?: string | null, to?: string | null): string {
  const fromM = from ? COLLECT_TASK_STATUS[from as keyof typeof COLLECT_TASK_STATUS] : null;
  const toM = to ? COLLECT_TASK_STATUS[to as keyof typeof COLLECT_TASK_STATUS] : null;
  const fromText = fromM?.text ?? from ?? '—';
  const toText = toM?.text ?? to ?? '—';
  if (!from && !to) return '—';
  return `${fromText} → ${toText}`;
}

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
  expired: { text: '授权已过期', color: 'warning' as const },
  invalid: { text: '异常', color: 'error' as const },
  need_check: { text: '需要检查', color: 'warning' as const },
  error: { text: '异常', color: 'error' as const },
  unsupported: { text: '不支持', color: 'default' as const },
};

/** 平台接入元信息 status */
export const PLATFORM_PROVIDER_STATUS = {
  available: { text: '可用', color: 'success' as const },
  beta: { text: '测试中', color: 'processing' as const },
  planned: { text: '规划中', color: 'default' as const },
  disabled: { text: '停用', color: 'default' as const },
};

/** 规格绑定状态 */
export const SKU_BIND_STATUS = {
  bound: { text: '已绑定', color: 'success' as const },
  unmatched: { text: '未匹配', color: 'default' as const },
  ambiguous: { text: '需要人工确认', color: 'warning' as const },
  skipped: { text: '已跳过', color: 'default' as const },
};

/** 配置开关状态 */
export const CONFIG_TOGGLE_STATUS = {
  enabled: { text: '已开启', color: 'success' as const },
  disabled: { text: '未开启', color: 'default' as const },
  configured: { text: '已配置', color: 'success' as const },
  unconfigured: { text: '未配置', color: 'default' as const },
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

/** 订单列表 SKU 匹配汇总 */
export const ORDER_SKU_MATCH_SUMMARY = {
  all_matched: { text: '已全部匹配', color: 'success' as const },
  partial: { text: '部分匹配', color: 'processing' as const },
  unmatched: { text: '未匹配', color: 'error' as const },
  ambiguous: { text: '候选待确认', color: 'warning' as const },
  none: { text: '无明细', color: 'default' as const },
};

/** 订单列表库存扣减汇总 */
export const ORDER_INVENTORY_DEDUCT_SUMMARY = {
  none: { text: '未扣减', color: 'default' as const },
  success: { text: '已扣减', color: 'success' as const },
  failed: { text: '扣减失败', color: 'error' as const },
  partial: { text: '部分扣减', color: 'warning' as const },
  blocked: { text: 'SKU 未就绪', color: 'warning' as const },
};

/** 订单来源/同步状态 */
export const ORDER_SYNC_SUMMARY = {
  manual: { text: '手工订单', color: 'default' as const },
  synced: { text: '平台已同步', color: 'success' as const },
  unknown: { text: '待确认', color: 'warning' as const },
};

/** 订单行 SKU 匹配状态 */
export const ORDER_ITEM_SKU_MATCH_STATUS = {
  matched: { text: '已匹配', color: 'success' as const },
  manual_bound: { text: '人工绑定', color: 'processing' as const },
  ambiguous: { text: '候选待确认', color: 'warning' as const },
  unmatched: { text: '未匹配', color: 'error' as const },
  skipped: { text: '已跳过', color: 'default' as const },
};

/** 客服消息同步任务 customer_message_sync_tasks.status（与订单同步任务状态语义一致） */
export const CUSTOMER_MESSAGE_SYNC_TASK_STATUS = ORDER_SYNC_TASK_STATUS;
