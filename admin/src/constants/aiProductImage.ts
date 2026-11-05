import { tr } from '@/i18n';
export const AI_IMAGE_BATCH_MAX_PRODUCTS = 50;
export const AI_IMAGE_BATCH_MAX_IMAGES = 300;

export const AI_IMAGE_OPERATION_OPTIONS = [
  { value: 'quality_check', get label() { return tr("仅做图片质量检查"); } },
  { value: 'remove_watermark', get label() { return tr("去水印"); } },
  { value: 'remove_logo', get label() { return tr("去 Logo"); } },
  { value: 'white_background', get label() { return tr("生成白底图"); } },
  { value: 'optimize_background', get label() { return tr("优化背景"); } },
  { value: 'translate_text', get label() { return tr("翻译图片文字"); } },
  { value: 'select_best_main', get label() { return tr("主图优选建议"); } },
];

export const AI_IMAGE_APPLY_MODES = [
  { value: 'save_to_gallery', get label() { return tr("保存到商品图库"); } },
  { value: 'set_main', get label() { return tr("设置为主图"); } },
  { value: 'add_detail', get label() { return tr("添加为详情图"); } },
  { value: 'replace_image', get label() { return tr("替换原图片"); } },
];

export const AI_IMAGE_REVIEW_FILTERS = [
  { value: 'all', get label() { return tr("全部"); } },
  { value: 'pending_review', get label() { return tr("待复核"); } },
  { value: 'applied', get label() { return tr("已应用"); } },
  { value: 'failed', get label() { return tr("处理失败"); } },
  { value: 'conflict', get label() { return tr("图片有冲突"); } },
  { value: 'rejected', get label() { return tr("已放弃"); } },
];

export const AI_IMAGE_IMAGE_FILTERS = [
  { value: 'all', get label() { return tr("全部图片"); } },
  { value: 'main', get label() { return tr("主图"); } },
  { value: 'detail', get label() { return tr("详情图"); } },
  { value: 'sku', get label() { return tr("规格图"); } },
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
