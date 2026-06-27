import { getWithParams, postJSON } from '@/services/request';

export type Paginated<T> = {
  list: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
};

export type TextGenerationOptions = {
  language?: string;
  platform?: string;
  tone?: string;
  maxLength?: number;
  titleStyle?: string;
  highlightSelling?: boolean;
  keepBrandWords?: boolean;
  keepSpecWords?: boolean;
  removeCollectNoise?: boolean;
  descStyle?: string;
  descStructure?: string;
  highlightScenarios?: boolean;
  generateBullets?: boolean;
  keepOriginalParams?: boolean;
  crossBorderReady?: boolean;
  keywords?: string[];
  forbiddenWords?: string[];
  remark?: string;
};

export type CheckBatchItem = {
  productId: string;
  productTitle: string;
  operationType: string;
  operationLabel: string;
  status: string;
  statusLabel: string;
  currentContent?: string;
  issues: string[];
};

export type CheckBatchResponse = {
  summary: {
    productCount: number;
    itemCount: number;
    readyCount: number;
    warningCount: number;
    blockedCount: number;
  };
  items: CheckBatchItem[];
};

export type AIProductTextBatchRow = {
  id: string;
  batchNo: string;
  status: string;
  statusLabel: string;
  productCount: number;
  itemCount: number;
  successCount: number;
  failedCount: number;
  appliedCount: number;
  operationTypes: string[];
  createdAt: string;
  finishedAt?: string;
};

export type QualityWarning = { code: string; message: string };

export type AIProductTextItemRow = {
  id: string;
  productId: string;
  productTitle: string;
  operationType: string;
  operationLabel: string;
  status: string;
  statusLabel: string;
  currentContent: string;
  generatedText: string;
  editedText: string;
  prepareApplyText: string;
  qualityWarnings: QualityWarning[];
  errorMessage?: string;
  aiTaskId?: string;
  sourceSnapshotHash?: string;
  productUpdatedAt?: string;
  appliedAt?: string;
  applicationId?: string;
};

export type AIProductTextBatchDetail = AIProductTextBatchRow & {
  items: AIProductTextItemRow[];
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
};

export async function checkAiProductTextBatch(body: {
  productIds: string[];
  operationTypes: string[];
  options?: TextGenerationOptions;
}) {
  return postJSON<CheckBatchResponse>('/api/v1/products/ai-text/batches/check', body);
}

export async function createAiProductTextBatch(body: {
  productIds: string[];
  operationTypes: string[];
  options?: TextGenerationOptions;
  idempotencyKey?: string;
}) {
  return postJSON<AIProductTextBatchRow>('/api/v1/products/ai-text/batches', body);
}

export async function fetchAiProductTextBatches(params: { page?: number; pageSize?: number }) {
  return getWithParams<Paginated<AIProductTextBatchRow>>('/api/v1/products/ai-text/batches', params);
}

export async function fetchAiProductTextBatchDetail(id: string, status?: string) {
  return getWithParams<AIProductTextBatchDetail>(`/api/v1/products/ai-text/batches/${id}`, status ? { status } : {});
}

export async function retryAiProductTextBatchFailed(id: string) {
  return postJSON<AIProductTextBatchRow>(`/api/v1/products/ai-text/batches/${id}/retry-failed`, {});
}

export async function cancelAiProductTextBatchPending(id: string) {
  return postJSON<{ cancelled: number }>(`/api/v1/products/ai-text/batches/${id}/cancel-pending`, {});
}

export async function applyAiProductTextSelected(id: string, itemIds: string[]) {
  return postJSON<{
    successCount: number;
    conflictCount: number;
    failedCount: number;
    items: { itemId: string; productId: string; status: string; statusLabel: string; errorMessage?: string }[];
  }>(`/api/v1/products/ai-text/batches/${id}/apply-selected`, { itemIds });
}

export async function undoAiProductTextBatchApplied(id: string) {
  return postJSON<{
    successCount: number;
    conflictCount: number;
    failedCount: number;
    items: { itemId: string; status: string; statusLabel: string; errorMessage?: string }[];
  }>(`/api/v1/products/ai-text/batches/${id}/undo-applied`, {});
}

export async function regenerateAiProductTextItem(id: string) {
  return postJSON<AIProductTextItemRow>(`/api/v1/products/ai-text/items/${id}/regenerate`, {});
}

export async function updateAiProductTextEditedText(id: string, editedText: string) {
  return postJSON<{ ok: boolean }>(`/api/v1/products/ai-text/items/${id}/update-edited-text`, { editedText });
}

export async function applyAiProductTextItem(id: string, text?: string) {
  return postJSON<{ itemId: string; status: string; statusLabel: string; errorMessage?: string }>(
    `/api/v1/products/ai-text/items/${id}/apply`,
    text ? { text } : {},
  );
}

export async function rejectAiProductTextItem(id: string) {
  return postJSON<{ ok: boolean }>(`/api/v1/products/ai-text/items/${id}/reject`, {});
}
