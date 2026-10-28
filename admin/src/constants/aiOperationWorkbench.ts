import { tr } from '@/i18n';
/** AI 商品运营工作台待办类型（用户可见中文见 typeLabel） */
export const WORKBENCH_TODO_TYPES = [
  { value: 'ai_text_review', get label() { return tr("AI 文案待复核"); } },
  { value: 'ai_text_conflict', get label() { return tr("AI 文案内容冲突"); } },
  { value: 'ai_image_review', get label() { return tr("AI 图片待复核"); } },
  { value: 'ai_image_conflict', get label() { return tr("AI 图片有冲突"); } },
  { value: 'publish_check_failed', get label() { return tr("发布检查未通过"); } },
  { value: 'publish_check_warning', get label() { return tr("发布检查建议处理"); } },
  { value: 'publish_batch_failed', get label() { return tr("刊登任务失败"); } },
  { value: 'publish_batch_partial_success', get label() { return tr("刊登任务部分成功"); } },
  { value: 'taskcenter_failure', get label() { return tr("系统失败任务"); } },
] as const;

export const WORKBENCH_PRIORITY_OPTIONS = [
  { value: 'P0', get label() { return tr("紧急"); }, color: 'red' },
  { value: 'P1', get label() { return tr("阻断"); }, color: 'red' },
  { value: 'P2', get label() { return tr("建议处理"); }, color: 'orange' },
  { value: 'P3', get label() { return tr("普通提醒"); }, color: 'blue' },
] as const;

export function workbenchTypeLabel(type?: string) {
  const key = (type ?? '').trim();
  return WORKBENCH_TODO_TYPES.find((x) => x.value === key)?.label ?? tr("待处理事项");
}

export function workbenchPriorityMeta(priority?: string) {
  const key = (priority ?? '').trim();
  return WORKBENCH_PRIORITY_OPTIONS.find((x) => x.value === key) ?? { value: key, label: key || '—', color: 'default' };
}

export const WORKBENCH_SUMMARY_CARDS = [
  {
    key: 'aiTextReviewCount',
    highKey: 'aiTextReviewHighPriority',
    todayKey: 'aiTextReviewTodayNew',
    get title() { return tr("待复核 AI 文案"); },
    filterType: 'ai_text_review',
    get actionLabel() { return tr("去复核"); },
    link: '/ai/text-batches?source=ai_workbench',
  },
  {
    key: 'aiImageReviewCount',
    highKey: 'aiImageReviewHighPriority',
    todayKey: 'aiImageReviewTodayNew',
    get title() { return tr("待复核 AI 图片"); },
    filterType: 'ai_image_review',
    get actionLabel() { return tr("去复核"); },
    link: '/ai/image-batches?source=ai_workbench',
  },
  {
    key: 'publishCheckIssueCount',
    highKey: 'publishCheckHighPriority',
    todayKey: 'publishCheckTodayNew',
    get title() { return tr("发布检查问题"); },
    filterType: 'publish_check_failed',
    get actionLabel() { return tr("去处理"); },
    link: '/product/drafts?readiness=blocked',
  },
  {
    key: 'publishTaskIssueCount',
    highKey: 'publishTaskIssueHighPriority',
    todayKey: 'publishTaskIssueTodayNew',
    get title() { return tr("刊登任务异常"); },
    filterType: 'publish_batch_failed',
    get actionLabel() { return tr("查看批次"); },
    link: '/product/publish-tasks?status=failed&tab=batches&source=ai_workbench',
  },
  {
    key: 'todayResolvedCount',
    get title() { return tr("今日已处理"); },
    get actionLabel() { return tr("刷新待办"); },
    link: '',
  },
] as const;
