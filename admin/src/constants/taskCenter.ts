/** 失败任务 / 告警 · 严重等级（与后端 failureclassifier 一致） */
export const TASK_FAILURE_SEVERITY: Record<string, { text: string; color: string }> = {
  low: { text: '低', color: 'default' },
  medium: { text: '中', color: 'blue' },
  high: { text: '高', color: 'orange' },
  critical: { text: '紧急', color: 'red' },
};

export const TASK_FAILURE_SEVERITY_OPTIONS = Object.entries(TASK_FAILURE_SEVERITY).map(([value, m]) => ({
  label: m.text,
  value,
}));

/** 失败归类（与 backend failureclassifier 类别常量一致） */
export const TASK_FAILURE_CATEGORY_LABEL: Record<string, string> = {
  platform_auth: '平台授权失败',
  platform_permission: '平台权限不足',
  platform_rate_limit: '平台限流',
  platform_api_error: '平台 API 错误',
  platform_config_incomplete: '平台配置不完整',
  network_timeout: '网络超时',
  collector_blocked: '采集被拦截',
  collector_platform_login: '平台登录/验证失效',
  login_required: '需要登录',
  collector_missing_images: '图片缺失',
  collector_missing_price: '价格字段缺失',
  collector_evaluate_script: '采集脚本执行错误',
  collector_invalid_url: '采集链接无效',
  ai_provider_error: 'AI 服务错误',
  ai_config_incomplete: 'AI 配置不完整',
  image_provider_error: '图片服务错误',
  storage_error: '存储错误',
  validation_error: '校验失败',
  inventory_mapping_missing: '库存映射缺失',
  sku_mapping_missing: '规格绑定缺失',
  worker_lease_expired: '后台任务执行超时',
  system_error: '系统错误',
  unknown: '未知',
};

export function failureCategoryLabel(cat?: string): string {
  const k = (cat || '').trim();
  if (!k) return '—';
  return TASK_FAILURE_CATEGORY_LABEL[k] || k;
}

export function failureSeverityLabel(sev?: string): string {
  const k = (sev || '').trim().toLowerCase();
  if (!k) return '—';
  return TASK_FAILURE_SEVERITY[k]?.text || sev;
}

/** 任务类型（失败任务中心列表） */
export const TASK_CENTER_TASK_TYPE_LABEL: Record<string, string> = {
  collect: '采集',
  image: 'AI 图片',
  order_sync: '订单同步',
  customer_message_sync: '客服消息同步',
  product_publish: '商品刊登',
  inventory_sync: '库存同步',
};

/** 抖店任务恢复状态 → 用户可见文案（不展示 stale / result_unknown 等内部值） */
export const TASK_RECOVERY_STATUS_LABEL: Record<string, string> = {
  stale: '任务执行时间过长',
  result_unknown: '平台结果暂时无法确认',
  recovery_required: '需要人工检查',
  recovery_failed: '恢复失败',
  superseded: '已被新任务取代',
  skipped: '已跳过',
};

export const TASK_RECOVERY_STATUS_OPTIONS = Object.entries(TASK_RECOVERY_STATUS_LABEL).map(
  ([value, label]) => ({ value, label }),
);

export function recoveryStatusLabel(status?: string | null): string {
  const k = (status ?? '').trim();
  if (!k) return '—';
  return TASK_RECOVERY_STATUS_LABEL[k] || '—';
}

/** Worker 进程有效状态（监控页） */
export const WORKER_EFFECTIVE_STATUS: Record<string, { text: string; color: string }> = {
  running: { text: '运行中', color: 'success' },
  stale: { text: '心跳超时', color: 'warning' },
  stopped: { text: '已停止', color: 'default' },
};

export const WORKER_STATUS_METRIC: Record<
  'running' | 'stale' | 'stopped',
  { text: string; valueStyle: string }
> = {
  running: { text: '运行中', valueStyle: '#52c41a' },
  stale: { text: '心跳超时', valueStyle: '#faad14' },
  stopped: { text: '已停止', valueStyle: 'rgba(0, 0, 0, 0.45)' },
};

/** Worker 监控按类型分组（与后端 byType 键一致） */
export const WORKER_MONITOR_TYPE_KEYS = [
  'collect',
  'image',
  'order_sync',
  'customer_message_sync',
  'product_publish',
  'inventory_sync',
] as const;

export type WorkerMonitorTypeKey = (typeof WORKER_MONITOR_TYPE_KEYS)[number];

export function workerTypeLabel(type?: string): string {
  const k = (type || '').trim();
  if (!k) return '—';
  return TASK_CENTER_TASK_TYPE_LABEL[k] || k;
}

/** 归一化状态 */
export const TASK_NORMALIZED_STATUS: Record<string, { text: string; color: string }> = {
  failed: { text: '失败', color: 'error' },
  retrying: { text: '重试中', color: 'processing' },
  stale: { text: '停滞', color: 'warning' },
  lease_expired: { text: '执行超时', color: 'warning' },
  running: { text: '执行中', color: 'processing' },
  pending: { text: '排队', color: 'default' },
  success: { text: '成功', color: 'success' },
  cancelled: { text: '取消', color: 'default' },
};
