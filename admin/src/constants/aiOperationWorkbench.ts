/** AI 商品运营工作台待办类型（用户可见中文见 typeLabel） */
export const WORKBENCH_TODO_TYPES = [
  { value: 'ai_text_review', label: 'AI 文案待复核' },
  { value: 'ai_text_conflict', label: 'AI 文案内容冲突' },
  { value: 'ai_image_review', label: 'AI 图片待复核' },
  { value: 'ai_image_conflict', label: 'AI 图片有冲突' },
  { value: 'publish_check_failed', label: '发布检查未通过' },
  { value: 'publish_check_warning', label: '发布检查建议处理' },
  { value: 'publish_batch_failed', label: '刊登任务失败' },
  { value: 'publish_batch_partial_success', label: '刊登任务部分成功' },
  { value: 'taskcenter_failure', label: '系统失败任务' },
] as const;

export const WORKBENCH_PRIORITY_OPTIONS = [
  { value: 'P0', label: '紧急', color: 'red' },
  { value: 'P1', label: '阻断', color: 'red' },
  { value: 'P2', label: '建议处理', color: 'orange' },
  { value: 'P3', label: '普通提醒', color: 'blue' },
] as const;

export function workbenchTypeLabel(type?: string) {
  const key = (type ?? '').trim();
  return WORKBENCH_TODO_TYPES.find((x) => x.value === key)?.label ?? '待处理事项';
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
    title: '待复核 AI 文案',
    filterType: 'ai_text_review',
    actionLabel: '去复核',
    link: '/ai/text-batches',
  },
  {
    key: 'aiImageReviewCount',
    highKey: 'aiImageReviewHighPriority',
    todayKey: 'aiImageReviewTodayNew',
    title: '待复核 AI 图片',
    filterType: 'ai_image_review',
    actionLabel: '去复核',
    link: '/ai/image-batches',
  },
  {
    key: 'publishCheckIssueCount',
    highKey: 'publishCheckHighPriority',
    todayKey: 'publishCheckTodayNew',
    title: '发布检查问题',
    filterType: 'publish_check_failed',
    actionLabel: '去处理',
    link: '/product/drafts?readiness=blocked',
  },
  {
    key: 'publishTaskIssueCount',
    highKey: 'publishTaskIssueHighPriority',
    todayKey: 'publishTaskIssueTodayNew',
    title: '刊登任务异常',
    filterType: 'publish_batch_failed',
    actionLabel: '查看批次',
    link: '/product/publish-tasks?status=failed',
  },
  {
    key: 'todayResolvedCount',
    title: '今日已处理',
    actionLabel: '刷新待办',
    link: '',
  },
] as const;
