/** Phase A3.1 批量 AI 文案任务状态与文案标签 */

export const AI_TEXT_ITEM_STATUS = {
  pending: { label: '等待处理', color: 'default' },
  running: { label: '生成中', color: 'processing' },
  success: { label: '待复核', color: 'blue' },
  pending_review: { label: '待复核', color: 'blue' },
  failed: { label: '生成失败', color: 'error' },
  applied: { label: '已应用', color: 'success' },
  rejected: { label: '已放弃', color: 'default' },
  conflict: { label: '内容有冲突', color: 'warning' },
  cancelled: { label: '已取消', color: 'default' },
} as const;

export const AI_TEXT_BATCH_STATUS = {
  pending: { label: '等待处理', color: 'default' },
  running: { label: '处理中', color: 'processing' },
  success: { label: '已完成', color: 'success' },
  partial_success: { label: '部分成功', color: 'warning' },
  failed: { label: '失败', color: 'error' },
  cancelled: { label: '已取消', color: 'default' },
} as const;

export const AI_TEXT_OPERATION_OPTIONS = [
  { value: 'title', label: '优化商品标题' },
  { value: 'description', label: '生成商品描述' },
  { value: 'both', label: '标题和描述都处理' },
];

export const AI_TEXT_REVIEW_FILTERS = [
  { value: 'all', label: '全部' },
  { value: 'pending_review', label: '待复核' },
  { value: 'applied', label: '已应用' },
  { value: 'failed', label: '生成失败' },
  { value: 'conflict', label: '内容有冲突' },
  { value: 'rejected', label: '已放弃' },
];

export function aiTextItemStatusTag(status: string, statusLabel?: string) {
  const k = (status || '').trim().toLowerCase();
  const meta = AI_TEXT_ITEM_STATUS[k as keyof typeof AI_TEXT_ITEM_STATUS];
  return { color: meta?.color || 'default', text: statusLabel || meta?.label || status };
}

export function aiTextBatchStatusTag(status: string, statusLabel?: string) {
  const k = (status || '').trim().toLowerCase();
  const meta = AI_TEXT_BATCH_STATUS[k as keyof typeof AI_TEXT_BATCH_STATUS];
  return { color: meta?.color || 'default', text: statusLabel || meta?.label || status };
}

export const AI_TEXT_BATCH_MAX_PRODUCTS = 100;
