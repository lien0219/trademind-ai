export type LocalizedLabel = {
  source: string;
  color?: 'default' | 'processing' | 'success' | 'error' | 'warning' | 'blue' | 'cyan' | 'gold' | 'purple' | 'orange' | 'red';
  descriptionSource?: string;
};

export const OPERATION_TASK_STATUS_LABELS: Record<string, LocalizedLabel> = {
  suggested: { source: '待处理建议', color: 'default', descriptionSource: '任务已创建，等待准备草稿。' },
  draft_preparing: { source: '草稿准备中', color: 'processing', descriptionSource: '正在冻结待审核的草稿内容。' },
  pending_review: { source: '待审核', color: 'warning', descriptionSource: '需要人工审核最新草稿。' },
  approved: { source: '已批准', color: 'success', descriptionSource: '最新草稿已获人工批准，可进行受控草稿生成。' },
  rejected: { source: '已拒绝', color: 'error', descriptionSource: '审核已拒绝，任务不会继续执行。' },
  execution_queued: { source: '等待执行', color: 'processing', descriptionSource: '执行请求已排队，不代表真实发布。' },
  executing: { source: '执行中', color: 'processing', descriptionSource: '正在执行已批准的草稿请求。' },
  draft_written: { source: '草稿已创建', color: 'success', descriptionSource: '草稿创建完成，不代表商品发布或上架。' },
  result_unknown: { source: '结果待核对', color: 'warning', descriptionSource: '平台结果不明确，必须人工核对，禁止直接重试。' },
  execution_failed: { source: '执行失败', color: 'error', descriptionSource: '执行失败，可按后端允许状态人工重试。' },
  cancelled: { source: '已取消', color: 'default', descriptionSource: '任务已取消。' },
};

export const OPERATION_DRAFT_STATUS_LABELS: Record<string, LocalizedLabel> = {
  editable: { source: '可编辑', color: 'processing' },
  pending_review: { source: '待审核', color: 'warning' },
  approved: { source: '已批准', color: 'success' },
  superseded: { source: '历史版本', color: 'default' },
  written: { source: '草稿已生成', color: 'success' },
  failed: { source: '失败', color: 'error' },
  result_unknown: { source: '结果待核对', color: 'warning' },
};

export const OPERATION_ATTEMPT_STATUS_LABELS: Record<string, LocalizedLabel> = {
  queued: { source: '排队中', color: 'processing' },
  running: { source: '运行中', color: 'processing' },
  succeeded: { source: '已完成', color: 'success' },
  failed: { source: '失败', color: 'error' },
  cancelled: { source: '已取消', color: 'default' },
};

export const OPERATION_PRIORITY_LABELS: Record<string, LocalizedLabel> = {
  low: { source: '低', color: 'default' },
  normal: { source: '普通', color: 'blue' },
  high: { source: '高', color: 'orange' },
  urgent: { source: '紧急', color: 'red' },
};

export const OPERATION_TASK_TYPE_LABELS: Record<string, string> = {
  product_content: '商品内容',
  order_exception: '订单异常',
  product_publish: '商品刊登',
  inventory_sync: '库存同步',
  customer_reply: '客服回复',
  ai_text: 'AI 文案',
  ai_image: 'AI 图片',
  manual_review: '人工复核',
};

export const OPERATION_SOURCE_TYPE_LABELS: Record<string, string> = {
  ai_suggestion: '智能建议',
  manual: '人工创建',
  rule_engine: '规则引擎',
  order_exception: '订单异常',
  product_content: '商品内容',
};

export const OPERATION_PLATFORM_LABELS: Record<string, string> = {
  local: '本地',
  douyin: '抖音',
  amazon: 'Amazon',
  lazada: 'Lazada',
  shopee: 'Shopee',
  tiktok: 'TikTok',
  woocommerce: 'WooCommerce',
  custom: '自定义',
};

export const OPERATION_ADAPTER_MODE_LABELS: Record<string, string> = {
  local_draft_only: '仅生成本地草稿',
  mock: '模拟草稿',
  sandbox: '沙箱草稿',
  production_draft: '平台草稿',
};

export const OPERATION_RESULT_TYPE_LABELS: Record<string, string> = {
  local_draft: '本地草稿',
  mock_draft: '模拟草稿',
  sandbox_fixture: '沙箱夹具',
  platform_draft: '平台草稿',
  result_unknown: '结果待核对',
};

export const OPERATION_EVENT_TYPE_LABELS: Record<string, LocalizedLabel> = {
  task_created: { source: '任务已创建' },
  draft_generated: { source: '草稿已生成' },
  draft_updated: { source: '草稿已更新' },
  review_requested: { source: '请求审核' },
  approved: { source: '已批准' },
  rejected: { source: '已拒绝' },
  execution_queued: { source: '执行已排队' },
  execution_started: { source: '执行已开始' },
  draft_written: { source: '草稿已生成' },
  execution_failed: { source: '执行失败' },
  result_unknown: { source: '结果待核对' },
  retry_requested: { source: '请求人工重试' },
  cancelled: { source: '已取消' },
  permission_denied: { source: '权限被拒绝' },
  production_capability_blocked: { source: '生产能力已拦截' },
};

export const OPERATION_ACTOR_TYPE_LABELS: Record<string, string> = {
  user: '后台用户',
  system: '系统',
  ai: '智能服务',
  rule: '规则引擎',
};

export const OPERATION_ERROR_LABELS: Record<string, string> = {
  validation_error: '提交内容未通过校验，请检查后重试。',
  permission_denied: '当前账号无权执行该操作。',
  not_found: '任务不存在或当前账号无权查看。',
  state_conflict: '任务状态已变化，请刷新后重新操作。',
  revision_conflict: '任务版本已变化，请查看最新内容后重新操作。',
  invalid_transition: '当前状态不允许执行该操作。',
  idempotency_payload_conflict: '该操作请求与之前提交内容不一致，请重新发起操作。',
  retry_limit_exceeded: '已达到最大重试限制。',
  production_capability_forbidden: '生产平台能力未启用，操作已被安全拦截。',
  execution_mode_forbidden: '当前任务不允许使用该执行模式。',
  draft_not_latest: '草稿已更新，请查看最新版本后重新审核。',
  draft_version_mismatch: '草稿版本不一致，请刷新后重试。',
  draft_hash_mismatch: '草稿内容校验不一致，请刷新后重试。',
  internal_error: '系统暂时无法完成操作，请稍后重试。',
};

export const OPERATION_METADATA_ALLOWLIST = new Set([
  'platform',
  'adapterMode',
  'taskType',
  'taskStatusBefore',
  'taskStatusAfter',
  'draftVersion',
  'payloadHash',
  'attemptNumber',
  'errorCategory',
  'retryable',
  'resultType',
  'capabilityBlocked',
]);

export const NON_PRODUCTION_BOUNDARY_COPY = {
  titleSource: '非生产边界',
  descriptionSource: '当前仅允许生成本地、模拟或沙箱草稿，不会调用真实平台发布或上架能力。',
  realWriteDisabled: '真实平台写入：未启用',
  autoPublishDisabled: '自动发布：未启用',
  autoListingDisabled: '自动上架：未启用',
};

export function operationLabel(map: Record<string, LocalizedLabel>, value?: string | null) {
  const key = (value ?? '').trim();
  return key ? map[key]?.source ?? key : '—';
}

export function operationLabelColor(map: Record<string, LocalizedLabel>, value?: string | null) {
  const key = (value ?? '').trim();
  return map[key]?.color ?? 'default';
}
