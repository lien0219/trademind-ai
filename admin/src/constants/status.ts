import { tr } from '@/i18n';

/** 商品草稿状态（与后端约定保持一致） */
export const PRODUCT_STATUS = {
  draft: { get text() { return tr('草稿'); }, color: 'default' as const },
  ai_processing: { get text() { return tr('AI 处理中'); }, color: 'processing' as const },
  ready: { get text() { return tr('可用'); }, color: 'success' as const },
  published: { get text() { return tr('已发布'); }, color: 'blue' as const },
  archived: { get text() { return tr('已归档'); }, color: 'default' as const },
};

/** 采集 / 异步任务统一状态 */
export const COLLECT_TASK_STATUS = {
  pending: { get text() { return tr('等待处理'); }, color: 'processing' as const },
  running: { get text() { return tr('处理中'); }, color: 'processing' as const },
  success: { get text() { return tr('成功'); }, color: 'success' as const },
  failed: { get text() { return tr('失败'); }, color: 'error' as const },
  cancelled: { get text() { return tr('已取消'); }, color: 'default' as const },
  retrying: { get text() { return tr('等待重试'); }, color: 'warning' as const },
  success_with_warnings: { get text() { return tr('成功（有警告）'); }, color: 'warning' as const },
  success_with_review: { get text() { return tr('成功（建议检查）'); }, color: 'warning' as const },
  low_quality: { get text() { return tr('失败：质量不达标'); }, color: 'error' as const },
  need_manual_review: { get text() { return tr('需人工检查'); }, color: 'warning' as const },
  failed_render_validation: { get text() { return tr('渲染校验失败'); }, color: 'error' as const },
  obsolete: { get text() { return tr('已过期'); }, color: 'default' as const },
};

/** 采集任务事件类型（collect_task_events.event_type） */
export const COLLECT_TASK_EVENT_TYPE: Record<string, string> = {
  get 'task.success'() { return tr('任务成功'); },
  get 'task.failed'() { return tr('任务失败'); },
  get 'task.retry_exhausted'() { return tr('重试次数已用尽'); },
  get 'task.running'() { return tr('任务执行中'); },
  get 'task.auto_retry_scheduled'() { return tr('已安排自动重试'); },
  get 'task.auto_retry_enqueued'() { return tr('自动重试已入队'); },
  get 'task.manual_retry'() { return tr('手动重试'); },
  get 'batch.delay.applied'() { return tr('批次延迟已生效'); },
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
  pending: { get text() { return tr('待开始'); }, color: 'default' as const },
  running: { get text() { return tr('进行中'); }, color: 'processing' as const },
  partial_success: { get text() { return tr('部分成功'); }, color: 'warning' as const },
  success: { get text() { return tr('全部成功'); }, color: 'success' as const },
  failed: { get text() { return tr('全部失败'); }, color: 'error' as const },
  cancelled: { get text() { return tr('已取消'); }, color: 'default' as const },
};

/** 客服会话状态（后端 customer_conversations.status） */
export const CUSTOMER_CONVERSATION_STATUS = {
  open: { get text() { return tr('进行中'); }, color: 'processing' as const },
  pending_reply: { get text() { return tr('待回复'); }, color: 'warning' as const },
  replied: { get text() { return tr('已回复'); }, color: 'success' as const },
  closed: { get text() { return tr('已关闭'); }, color: 'default' as const },
};

/** AI 回复建议状态 */
export const CUSTOMER_SUGGESTION_STATUS = {
  generated: { get text() { return tr('待确认'); }, color: 'processing' as const },
  edited: { get text() { return tr('已编辑'); }, color: 'blue' as const },
  accepted: { get text() { return tr('已发送'); }, color: 'success' as const },
  discarded: { get text() { return tr('已废弃'); }, color: 'default' as const },
  rejected: { get text() { return tr('已拒绝'); }, color: 'default' as const },
  generate_failed: { get text() { return tr('生成失败'); }, color: 'error' as const },
  send_failed: { get text() { return tr('发送失败'); }, color: 'error' as const },
};

/** 会话发送状态（列表汇总） */
export const CUSTOMER_SEND_STATUS = {
  none: { text: '—', color: 'default' as const },
  pending_confirm: { get text() { return tr('待确认'); }, color: 'processing' as const },
  sent: { get text() { return tr('已发送'); }, color: 'success' as const },
  failed: { get text() { return tr('发送失败'); }, color: 'error' as const },
};

/** 手工订单 orders.status（与后端 order/constants 对齐） */
export const ORDER_STATUS = {
  pending: { get text() { return tr('待处理'); }, color: 'default' as const },
  paid: { get text() { return tr('已付款'); }, color: 'success' as const },
  processing: { get text() { return tr('处理中'); }, color: 'processing' as const },
  shipped: { get text() { return tr('已发货'); }, color: 'blue' as const },
  delivered: { get text() { return tr('已送达'); }, color: 'cyan' as const },
  cancelled: { get text() { return tr('已取消'); }, color: 'warning' as const },
  refunded: { get text() { return tr('已退款'); }, color: 'warning' as const },
  closed: { get text() { return tr('已关闭'); }, color: 'default' as const },
};

export const ORDER_PAYMENT_STATUS = {
  unpaid: { get text() { return tr('未支付'); }, color: 'default' as const },
  paid: { get text() { return tr('已支付'); }, color: 'success' as const },
  partially_refunded: { get text() { return tr('部分退款'); }, color: 'warning' as const },
  refunded: { get text() { return tr('已退款'); }, color: 'warning' as const },
};

export const ORDER_FULFILLMENT_STATUS = {
  unfulfilled: { get text() { return tr('未履约'); }, color: 'default' as const },
  partial: { get text() { return tr('部分履约'); }, color: 'processing' as const },
  fulfilled: { get text() { return tr('已履约'); }, color: 'success' as const },
  returned: { get text() { return tr('已退货'); }, color: 'warning' as const },
};

export const ORDER_SHIPMENT_STATUS = {
  pending: { get text() { return tr('待发货'); }, color: 'default' as const },
  shipped: { get text() { return tr('已发货'); }, color: 'blue' as const },
  in_transit: { get text() { return tr('运输中'); }, color: 'processing' as const },
  delivered: { get text() { return tr('已签收'); }, color: 'success' as const },
  exception: { get text() { return tr('异常'); }, color: 'error' as const },
  returned: { get text() { return tr('已退回'); }, color: 'warning' as const },
};

/** 统一店铺 shops.status */
export const SHOP_STATUS = {
  active: { get text() { return tr('启用'); }, color: 'success' as const },
  disabled: { get text() { return tr('停用'); }, color: 'default' as const },
};

/** 统一店铺 shops.auth_status */
export const SHOP_AUTH_STATUS = {
  unauthorized: { get text() { return tr('未授权'); }, color: 'default' as const },
  authorized: { get text() { return tr('已授权'); }, color: 'success' as const },
  expired: { get text() { return tr('授权已过期'); }, color: 'warning' as const },
  invalid: { get text() { return tr('异常'); }, color: 'error' as const },
  need_check: { get text() { return tr('需要检查'); }, color: 'warning' as const },
  error: { get text() { return tr('异常'); }, color: 'error' as const },
  unsupported: { get text() { return tr('不支持'); }, color: 'default' as const },
};

/** 平台接入元信息 status */
export const PLATFORM_PROVIDER_STATUS = {
  available: { get text() { return tr('可用'); }, color: 'success' as const },
  beta: { get text() { return tr('测试中'); }, color: 'processing' as const },
  planned: { get text() { return tr('规划中'); }, color: 'default' as const },
  disabled: { get text() { return tr('停用'); }, color: 'default' as const },
};

/** 规格绑定状态 */
export const SKU_BIND_STATUS = {
  bound: { get text() { return tr('已绑定'); }, color: 'success' as const },
  unmatched: { get text() { return tr('未匹配'); }, color: 'default' as const },
  ambiguous: { get text() { return tr('需要人工确认'); }, color: 'warning' as const },
  skipped: { get text() { return tr('已跳过'); }, color: 'default' as const },
};

/** 配置开关状态 */
export const CONFIG_TOGGLE_STATUS = {
  enabled: { get text() { return tr('已开启'); }, color: 'success' as const },
  disabled: { get text() { return tr('未开启'); }, color: 'default' as const },
  configured: { get text() { return tr('已配置'); }, color: 'success' as const },
  unconfigured: { get text() { return tr('未配置'); }, color: 'default' as const },
};

/** 订单同步任务 order_sync_tasks.status */
export const ORDER_SYNC_TASK_STATUS = {
  pending: { get text() { return tr('等待处理'); }, color: 'processing' as const },
  running: { get text() { return tr('处理中'); }, color: 'processing' as const },
  partial_success: { get text() { return tr('部分成功'); }, color: 'warning' as const },
  success: { get text() { return tr('成功'); }, color: 'success' as const },
  failed: { get text() { return tr('失败'); }, color: 'error' as const },
  cancelled: { get text() { return tr('已取消'); }, color: 'default' as const },
};

/** 订单列表 SKU 匹配汇总 */
export const ORDER_SKU_MATCH_SUMMARY = {
  all_matched: { get text() { return tr('已全部匹配'); }, color: 'success' as const },
  partial: { get text() { return tr('部分匹配'); }, color: 'processing' as const },
  unmatched: { get text() { return tr('未匹配'); }, color: 'error' as const },
  ambiguous: { get text() { return tr('候选待确认'); }, color: 'warning' as const },
  none: { get text() { return tr('无明细'); }, color: 'default' as const },
};

/** 订单列表库存扣减汇总 */
export const ORDER_INVENTORY_DEDUCT_SUMMARY = {
  none: { get text() { return tr('未扣减'); }, color: 'default' as const },
  success: { get text() { return tr('已扣减'); }, color: 'success' as const },
  failed: { get text() { return tr('扣减失败'); }, color: 'error' as const },
  partial: { get text() { return tr('部分扣减'); }, color: 'warning' as const },
  blocked: { get text() { return tr('SKU 未就绪'); }, color: 'warning' as const },
};

/** 订单库存履约对账状态（只读工作台） */
export const ORDER_FULFILLMENT_RECONCILIATION_STATUS = {
  matched: { get text() { return tr("已匹配"); }, color: "success" as const },
  pending: { get text() { return tr("待确认"); }, color: "processing" as const },
  mismatch: { get text() { return tr("账实不一致"); }, color: "error" as const },
  blocked: { get text() { return tr("人工阻断"); }, color: "warning" as const },
};

export const ORDER_FULFILLMENT_RECONCILIATION_ISSUES: Record<string, string> = {
  get inventory_effect_pending() { return tr("库存 effect 待处理"); },
  get inventory_effect_failed() { return tr("库存 effect 失败"); },
  get duplicate_effect() { return tr("重复库存 effect"); },
  get inventory_history_not_migrated() { return tr("历史库存未迁移"); },
  get sku_not_bound() { return tr("SKU 未绑定"); },
  get warehouse_missing() { return tr("履约仓库缺失"); },
  get inventory_balance_missing() { return tr("库存余额缺失"); },
  get reserve_quantity_mismatch() { return tr("预占数量不一致"); },
  get deduct_quantity_mismatch() { return tr("出库数量不一致"); },
  get release_quantity_mismatch() { return tr("释放数量不一致"); },
  get restore_quantity_mismatch() { return tr("回补数量不一致"); },
  get inventory_movement_quantity_mismatch() { return tr("库存流水数量不一致"); },
  get shipment_missing() { return tr("发货单缺失"); },
  get shipment_state_conflict() { return tr("发货状态冲突"); },
};

/** 订单来源/同步状态 */
export const ORDER_SYNC_SUMMARY = {
  manual: { get text() { return tr('手工订单'); }, color: 'default' as const },
  synced: { get text() { return tr('平台已同步'); }, color: 'success' as const },
  unknown: { get text() { return tr('待确认'); }, color: 'warning' as const },
};

/** 订单行 SKU 匹配状态 */
export const ORDER_ITEM_SKU_MATCH_STATUS = {
  matched: { get text() { return tr('已匹配'); }, color: 'success' as const },
  manual_bound: { get text() { return tr('人工绑定'); }, color: 'processing' as const },
  ambiguous: { get text() { return tr('候选待确认'); }, color: 'warning' as const },
  unmatched: { get text() { return tr('未匹配'); }, color: 'error' as const },
  skipped: { get text() { return tr('已跳过'); }, color: 'default' as const },
};

/** 客服消息同步任务 customer_message_sync_tasks.status（与订单同步任务状态语义一致） */
export const CUSTOMER_MESSAGE_SYNC_TASK_STATUS = ORDER_SYNC_TASK_STATUS;
