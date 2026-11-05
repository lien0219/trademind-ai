import { ok } from './envelope';

export const E2E_AI_TEXT_BATCH_ID = 'e2e-ai-text-batch-1';
export const E2E_AI_TEXT_ITEM_ID = 'e2e-ai-text-item-1';
export const E2E_AI_IMAGE_BATCH_ID = 'e2e-ai-image-batch-1';
export const E2E_AI_IMAGE_ITEM_ID = 'e2e-ai-image-item-1';

const e2eProductTitle = 'E2E 商品草稿长标题用于回归测试响应式和换行能力';

export const e2eAiTextBatchDetail = {
  id: E2E_AI_TEXT_BATCH_ID,
  batchNo: 'AI-TEXT-E2E-0001',
  status: 'partial_success',
  statusLabel: '部分成功',
  productCount: 1,
  itemCount: 1,
  successCount: 1,
  failedCount: 0,
  appliedCount: 0,
  operationTypes: ['title_optimize'],
  createdAt: '2026-01-03T00:00:00Z',
  finishedAt: '2026-01-03T00:05:00Z',
  items: [
    {
      id: E2E_AI_TEXT_ITEM_ID,
      productId: 'e2e-product-draft',
      productTitle: e2eProductTitle,
      operationType: 'title_optimize',
      operationLabel: '标题优化',
      status: 'pending_review',
      statusLabel: '待复核',
      currentContent: 'E2E 原始商品标题',
      generatedText: 'E2E AI 优化标题',
      editedText: 'E2E AI 优化标题',
      prepareApplyText: 'E2E AI 优化标题',
      qualityWarnings: [],
      aiTaskId: 'e2e-ai-task-1',
      sourceSnapshotHash: 'e2e-ai-text-snapshot',
      productUpdatedAt: '2026-01-03T00:00:00Z',
    },
  ],
  input: { language: 'zh-CN', operationTypes: ['title_optimize'] },
  output: { provider: 'e2e-provider', model: 'e2e-model' },
};

export const e2eAiImageBatchDetail = {
  id: E2E_AI_IMAGE_BATCH_ID,
  batchNo: 'AI-IMAGE-E2E-0001',
  status: 'partial_success',
  statusLabel: '部分成功',
  productCount: 1,
  imageCount: 1,
  itemCount: 1,
  successCount: 1,
  failedCount: 0,
  appliedCount: 0,
  operationTypes: ['remove_background'],
  createdAt: '2026-01-03T00:00:00Z',
  finishedAt: '2026-01-03T00:05:00Z',
  overview: {
    successCount: 1,
    warningCount: 0,
    failedCount: 0,
    appliedCount: 0,
    recoverableCount: 0,
    nonRecoverableCount: 0,
    providerReadiness: {
      provider: 'removebg',
      status: 'ready',
      statusLabel: '已就绪',
      summary: 'E2E image provider is ready.',
    },
    nextActions: [],
    summaryMessage: 'E2E image batch is ready for review.',
  },
  items: [
    {
      id: E2E_AI_IMAGE_ITEM_ID,
      productId: 'e2e-product-draft',
      productTitle: e2eProductTitle,
      imageId: 'e2e-image-main',
      imageType: 'main',
      imageTypeLabel: '主图',
      operationType: 'remove_background',
      operationLabel: '去背景',
      status: 'pending_review',
      statusLabel: '待复核',
      sourceImageUrl: 'https://example.test/e2e-main.jpg',
      resultImageUrl: 'https://example.test/e2e-result.png',
      qualityWarnings: [],
      recoverable: false,
      needsConfig: false,
      applyMode: 'save_to_gallery',
      applyModeLabel: '保存到商品图库',
    },
  ],
  input: { operationTypes: ['remove_background'] },
  output: { provider: 'removebg' },
};

export function aiProductBatchResponse(path: string) {
  if (path === `/api/v1/products/ai-text/batches/${E2E_AI_TEXT_BATCH_ID}`) {
    return ok(e2eAiTextBatchDetail);
  }
  if (path === `/api/v1/products/ai-images/batches/${E2E_AI_IMAGE_BATCH_ID}`) {
    return ok(e2eAiImageBatchDetail);
  }
  return null;
}
