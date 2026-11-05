import { tr } from '@/i18n';
/** Phase A3.1 批量 AI 文案任务状态与文案标签 */

export const AI_TEXT_ITEM_STATUS = {
  pending: { get label() { return tr("等待处理"); }, color: 'default' },
  running: { get label() { return tr("生成中"); }, color: 'processing' },
  success: { get label() { return tr("待复核"); }, color: 'blue' },
  pending_review: { get label() { return tr("待复核"); }, color: 'blue' },
  failed: { get label() { return tr("生成失败"); }, color: 'error' },
  applied: { get label() { return tr("已应用"); }, color: 'success' },
  rejected: { get label() { return tr("已放弃"); }, color: 'default' },
  conflict: { get label() { return tr("内容有冲突"); }, color: 'warning' },
  cancelled: { get label() { return tr("已取消"); }, color: 'default' },
} as const;

export const AI_TEXT_BATCH_STATUS = {
  pending: { get label() { return tr("等待处理"); }, color: 'default' },
  running: { get label() { return tr("处理中"); }, color: 'processing' },
  success: { get label() { return tr("已完成"); }, color: 'success' },
  partial_success: { get label() { return tr("部分成功"); }, color: 'warning' },
  failed: { get label() { return tr("失败"); }, color: 'error' },
  cancelled: { get label() { return tr("已取消"); }, color: 'default' },
} as const;

export const AI_TEXT_OPERATION_OPTIONS = [
  { value: 'title', get label() { return tr("优化商品标题"); } },
  { value: 'description', get label() { return tr("生成商品描述"); } },
  { value: 'both', get label() { return tr("标题和描述都处理"); } },
];

export const AI_TEXT_REVIEW_FILTERS = [
  { value: 'all', get label() { return tr("全部"); } },
  { value: 'pending_review', get label() { return tr("待复核"); } },
  { value: 'applied', get label() { return tr("已应用"); } },
  { value: 'failed', get label() { return tr("生成失败"); } },
  { value: 'conflict', get label() { return tr("内容有冲突"); } },
  { value: 'rejected', get label() { return tr("已放弃"); } },
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
