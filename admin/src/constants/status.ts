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
  pending: { text: '处理中', color: 'processing' as const },
  running: { text: '处理中', color: 'processing' as const },
  success: { text: '成功', color: 'success' as const },
  failed: { text: '失败', color: 'error' as const },
  cancelled: { text: '已取消', color: 'default' as const },
  retrying: { text: '处理中', color: 'processing' as const },
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
