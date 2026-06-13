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
  // 抖店平台级站内告警（douyinruntime/alert.go）
  douyin_token_refresh_failed: 'Token 刷新失败',
  douyin_auth_expiring: '店铺授权即将过期',
  douyin_auth_expired: '店铺授权已过期',
  douyin_auth_need_check: '店铺授权需检查',
  douyin_product_draft_failures: '商品草稿失败积压',
  douyin_product_result_unknown: '商品结果暂时无法确认',
  douyin_product_recovery_failed: '商品任务恢复失败',
  douyin_image_upload_failure_rate: '图片上传失败率过高',
  douyin_storage_public_failed: 'Storage 公网访问异常',
  douyin_order_sync_failed: '订单同步失败',
  douyin_order_partial_stale: '订单同步部分停滞',
  douyin_inventory_sync_failed: '库存同步失败',
  douyin_inventory_stale: '库存同步停滞',
  douyin_runtime_emergency_disabled: '抖店紧急停用',
  douyin_stale_tasks_high: '停滞任务过多',
  douyin_failure_backlog: '失败任务积压',
  douyin_rate_limit_spike: '平台限流激增',
};

export function failureCategoryLabel(cat?: string): string {
  const k = (cat || '').trim();
  if (!k) return '—';
  return TASK_FAILURE_CATEGORY_LABEL[k] || k;
}

export function failureSeverityLabel(sev?: string): string {
  const k = (sev || '').trim().toLowerCase();
  if (!k) return '—';
  return TASK_FAILURE_SEVERITY[k]?.text || k;
}

/** 失败任务中心可查询详情的任务类型（与后端 parseTaskType 一致） */
export const TASK_CENTER_FAILURE_TASK_TYPES = [
  'collect',
  'image',
  'order_sync',
  'customer_message_sync',
  'product_publish',
  'inventory_sync',
] as const;

export type TaskCenterFailureTaskType = (typeof TASK_CENTER_FAILURE_TASK_TYPES)[number];

/** 平台级站内告警 taskType（sourceId 非业务任务 UUID，不可走失败详情接口） */
export const PLATFORM_ALERT_TASK_TYPES = ['douyin_platform'] as const;

const TASK_FAILURE_DETAIL_ID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** 任务类型（失败任务中心列表） */
export const TASK_CENTER_TASK_TYPE_LABEL: Record<string, string> = {
  collect: '采集',
  image: 'AI 图片',
  order_sync: '订单同步',
  customer_message_sync: '客服消息同步',
  product_publish: '商品刊登',
  inventory_sync: '库存同步',
  douyin_platform: '抖店平台告警',
};

export function isTaskCenterFailureTaskType(taskType?: string | null): boolean {
  const k = (taskType || '').trim().toLowerCase();
  return (TASK_CENTER_FAILURE_TASK_TYPES as readonly string[]).includes(k);
}

export function isPlatformAlertTaskType(taskType?: string | null): boolean {
  const k = (taskType || '').trim().toLowerCase();
  return (PLATFORM_ALERT_TASK_TYPES as readonly string[]).includes(k);
}

export function isTaskFailureDetailId(id?: string | null): boolean {
  const k = (id || '').trim();
  return TASK_FAILURE_DETAIL_ID_RE.test(k);
}

/** 是否可打开 GET /task-center/failures/:taskType/:id */
export function canOpenFailureDetail(taskType?: string | null, sourceId?: string | null): boolean {
  return isTaskCenterFailureTaskType(taskType) && isTaskFailureDetailId(sourceId);
}

/** 告警中心「相关入口」深链：平台告警走运维页，业务失败任务走详情深链 */
export function resolveAlertRelatedLink(alert: {
  taskType: string;
  sourceId: string;
  platform?: string;
}): { href: string; label: string } {
  const taskType = (alert.taskType || '').trim();
  const sourceId = (alert.sourceId || '').trim();

  if (taskType === 'douyin_platform') {
    return { href: '/ops/platform-runtime?platform=douyin_shop', label: '平台运维' };
  }
  if (isPlatformAlertTaskType(taskType)) {
    return { href: '/ops/task-center/alerts', label: '告警中心' };
  }
  if (canOpenFailureDetail(taskType, sourceId)) {
    const sp = new URLSearchParams({ taskType, jumpId: sourceId });
    return { href: `/ops/task-center/failures?${sp.toString()}`, label: '失败任务' };
  }
  if ((alert.platform || '').trim()) {
    const sp = new URLSearchParams({ platform: alert.platform!.trim() });
    return { href: `/ops/task-center/failures?${sp.toString()}`, label: '失败任务' };
  }
  return { href: '/ops/task-center/failures', label: '失败任务' };
}

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

/** 告警中心任务类型显示名（与 workerTypeLabel 一致） */
export function taskCenterTaskTypeLabel(taskType?: string): string {
  return workerTypeLabel(taskType);
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
