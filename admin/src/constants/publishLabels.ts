/** 刊登能力分级（与后端 productpublish 对齐） */
export const PUBLISH_CAPABILITY_LABEL: Record<string, string> = {
  real_draft_create: '可创建平台草稿',
  local_draft_only: '仅生成本地草稿',
  not_configured: '尚未配置',
  not_authorized: '店铺未授权',
  disabled: '已停用',
};

export const PUBLISH_TARGET_STATUS_LABEL: Record<string, string> = {
  ready: '可以创建草稿',
  warning: '需要检查',
  blocked: '暂不能创建草稿',
  skipped: '已跳过',
};

export const PUBLISH_BATCH_STATUS_LABEL: Record<string, string> = {
  pending: '等待处理',
  running: '处理中',
  success: '全部成功',
  partial_success: '部分成功',
  failed: '失败',
  cancelled: '已取消',
};

export const PUBLISH_BATCH_STATUS: Record<
  string,
  { text: string; color: 'default' | 'processing' | 'success' | 'warning' | 'error' }
> = {
  pending: { text: '等待处理', color: 'default' },
  running: { text: '处理中', color: 'processing' },
  success: { text: '全部成功', color: 'success' },
  partial_success: { text: '部分成功', color: 'warning' },
  failed: { text: '失败', color: 'error' },
  cancelled: { text: '已取消', color: 'default' },
};

export function publishCapabilityLabel(cap?: string | null): string {
  const k = (cap ?? '').trim();
  return PUBLISH_CAPABILITY_LABEL[k] ?? '—';
}

export function publishTargetStatusLabel(status?: string | null): string {
  const k = (status ?? '').trim().toLowerCase();
  return PUBLISH_TARGET_STATUS_LABEL[k] ?? '—';
}

export function publishBatchStatusLabel(status?: string | null): string {
  const k = (status ?? '').trim().toLowerCase();
  return PUBLISH_BATCH_STATUS_LABEL[k] ?? '—';
}

export function publishBatchStatusTag(status?: string | null, label?: string) {
  const k = (status ?? '').trim().toLowerCase();
  const meta = PUBLISH_BATCH_STATUS[k];
  const text = label || meta?.text || publishBatchStatusLabel(k);
  return { text, color: meta?.color ?? ('default' as const) };
}

/** 统一刊登配置字段标签 */
export const COMMON_PUBLISH_CONFIG_LABEL: Record<string, string> = {
  title: '统一标题',
  description: '统一描述',
  priceRule: '统一价格规则',
  imageStrategy: '统一图片策略',
  packageWeight: '统一包裹重量',
  packageSize: '统一包裹尺寸',
  stockStrategy: '统一库存策略',
  remark: '备注',
};
