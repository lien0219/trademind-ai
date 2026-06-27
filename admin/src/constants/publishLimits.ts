/** 批量刊登规模上限（与后端 PUBLISH_BATCH_* 默认值对齐） */
export const PUBLISH_BATCH_MAX_PRODUCTS = 100;
export const PUBLISH_BATCH_MAX_TARGETS = 20;
export const PUBLISH_BATCH_MAX_TASKS = 300;

export const PUBLISH_BATCH_LIMIT_MESSAGE =
  '本次选择的商品和刊登目标较多，请分批创建刊登草稿。';

export function validatePublishBatchMatrix(
  productCount: number,
  targetCount: number,
): string | null {
  if (productCount <= 0 || targetCount <= 0) {
    return null;
  }
  if (productCount > PUBLISH_BATCH_MAX_PRODUCTS) {
    return PUBLISH_BATCH_LIMIT_MESSAGE;
  }
  if (targetCount > PUBLISH_BATCH_MAX_TARGETS) {
    return PUBLISH_BATCH_LIMIT_MESSAGE;
  }
  if (productCount * targetCount > PUBLISH_BATCH_MAX_TASKS) {
    return PUBLISH_BATCH_LIMIT_MESSAGE;
  }
  return null;
}
