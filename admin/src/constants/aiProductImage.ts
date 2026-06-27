export const AI_IMAGE_BATCH_MAX_PRODUCTS = 50;
export const AI_IMAGE_BATCH_MAX_IMAGES = 300;

export const AI_IMAGE_OPERATION_OPTIONS = [
  { value: 'quality_check', label: '仅做图片质量检查' },
  { value: 'remove_watermark', label: '去水印' },
  { value: 'remove_logo', label: '去 Logo' },
  { value: 'white_background', label: '生成白底图' },
  { value: 'optimize_background', label: '优化背景' },
  { value: 'translate_text', label: '翻译图片文字' },
  { value: 'select_best_main', label: '主图优选建议' },
];

export const AI_IMAGE_APPLY_MODES = [
  { value: 'save_to_gallery', label: '保存到商品图库' },
  { value: 'set_main', label: '设置为主图' },
  { value: 'add_detail', label: '添加为详情图' },
  { value: 'replace_image', label: '替换原图片' },
];

export const AI_IMAGE_REVIEW_FILTERS = [
  { value: 'all', label: '全部' },
  { value: 'pending_review', label: '待复核' },
  { value: 'applied', label: '已应用' },
  { value: 'failed', label: '处理失败' },
  { value: 'conflict', label: '图片有冲突' },
  { value: 'rejected', label: '已放弃' },
];

export const AI_IMAGE_IMAGE_FILTERS = [
  { value: 'all', label: '全部图片' },
  { value: 'main', label: '主图' },
  { value: 'detail', label: '详情图' },
  { value: 'sku', label: '规格图' },
];

export function aiImageBatchStatusTag(status: string, label?: string) {
  const text = label || status;
  if (status === 'success') return { color: 'green', text };
  if (status === 'partial_success') return { color: 'orange', text };
  if (status === 'running') return { color: 'processing', text };
  if (status === 'failed') return { color: 'red', text };
  return { color: 'default', text };
}

export function aiImageItemStatusTag(status: string, label?: string) {
  const text = label || status;
  if (status === 'pending_review' || status === 'success') return { color: 'blue', text };
  if (status === 'applied') return { color: 'green', text };
  if (status === 'failed') return { color: 'red', text };
  if (status === 'conflict') return { color: 'orange', text };
  if (status === 'rejected') return { color: 'default', text };
  return { color: 'default', text };
}

export function aiImageOperationLabel(op: string) {
  return AI_IMAGE_OPERATION_OPTIONS.find((x) => x.value === op)?.label || op;
}
