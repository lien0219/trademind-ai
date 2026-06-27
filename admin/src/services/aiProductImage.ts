import { getWithParams, postJSON } from '@/services/request';

export type Paginated<T> = {
  list: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
};

export type ImageGenerationOptions = {
  language?: string;
  backgroundStyle?: string;
  keepSubject?: boolean;
  keepBrandLogo?: boolean;
  skipFailedImages?: boolean;
  outputFormat?: string;
  remark?: string;
};

export type CheckBatchItem = {
  productId: string;
  productTitle?: string;
  imageId: string;
  imageType: string;
  imageTypeLabel: string;
  sourceImageUrl?: string;
  operationType: string;
  operationLabel: string;
  status: string;
  statusLabel: string;
  issues: string[];
};

export type CheckBatchResponse = {
  summary: {
    productCount: number;
    imageCount: number;
    itemCount: number;
    readyCount: number;
    warningCount: number;
    blockedCount: number;
  };
  items: CheckBatchItem[];
};

export type AIProductImageBatchRow = {
  id: string;
  batchNo: string;
  status: string;
  statusLabel: string;
  productCount: number;
  imageCount: number;
  itemCount: number;
  successCount: number;
  failedCount: number;
  appliedCount: number;
  operationTypes: string[];
  createdAt: string;
  finishedAt?: string;
};

export type QualityWarning = { code: string; title?: string; message: string };

export type AIProductImageItemRow = {
  id: string;
  productId: string;
  productTitle: string;
  imageId?: string;
  imageType: string;
  imageTypeLabel: string;
  operationType: string;
  operationLabel: string;
  status: string;
  statusLabel: string;
  sourceImageUrl: string;
  resultImageUrl?: string;
  qualityWarnings: QualityWarning[];
  errorMessage?: string;
  imageTaskId?: string;
  applyMode?: string;
  applyModeLabel?: string;
  appliedAt?: string;
  applicationId?: string;
};

export type AIProductImageBatchDetail = AIProductImageBatchRow & {
  items: AIProductImageItemRow[];
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
};

export async function checkAiProductImageBatch(body: {
  productIds: string[];
  imageIds?: string[];
  operationTypes: string[];
  options?: ImageGenerationOptions;
}) {
  return postJSON<CheckBatchResponse>('/api/v1/products/ai-images/batches/check', body);
}

export async function createAiProductImageBatch(body: {
  productIds: string[];
  imageIds: string[];
  operationTypes: string[];
  options?: ImageGenerationOptions;
  idempotencyKey?: string;
}) {
  return postJSON<AIProductImageBatchRow>('/api/v1/products/ai-images/batches', body);
}

export async function fetchAiProductImageBatches(params: { page?: number; pageSize?: number }) {
  return getWithParams<Paginated<AIProductImageBatchRow>>('/api/v1/products/ai-images/batches', params);
}

export async function fetchAiProductImageBatchDetail(id: string, status?: string) {
  return getWithParams<AIProductImageBatchDetail>(`/api/v1/products/ai-images/batches/${id}`, status ? { status } : {});
}

export async function retryAiProductImageBatchFailed(id: string) {
  return postJSON<AIProductImageBatchRow>(`/api/v1/products/ai-images/batches/${id}/retry-failed`, {});
}

export async function cancelAiProductImageBatchPending(id: string) {
  return postJSON<{ cancelled: number }>(`/api/v1/products/ai-images/batches/${id}/cancel-pending`, {});
}

export async function applyAiProductImageSelected(id: string, itemIds: string[], applyMode?: string) {
  return postJSON<{
    successCount: number;
    conflictCount: number;
    failedCount: number;
    items: { itemId: string; productId: string; status: string; statusLabel: string; errorMessage?: string }[];
  }>(`/api/v1/products/ai-images/batches/${id}/apply-selected`, { itemIds, applyMode });
}

export async function undoAiProductImageBatchApplied(id: string) {
  return postJSON<{
    successCount: number;
    conflictCount: number;
    failedCount: number;
    items: { itemId: string; status: string; statusLabel: string; errorMessage?: string }[];
  }>(`/api/v1/products/ai-images/batches/${id}/undo-applied`, {});
}

export async function regenerateAiProductImageItem(id: string) {
  return postJSON<AIProductImageItemRow>(`/api/v1/products/ai-images/items/${id}/regenerate`, {});
}

export async function applyAiProductImageItem(id: string, applyMode: string) {
  return postJSON<{ itemId: string; status: string; statusLabel: string; errorMessage?: string }>(
    `/api/v1/products/ai-images/items/${id}/apply`,
    { applyMode },
  );
}

export async function rejectAiProductImageItem(id: string) {
  return postJSON<{ ok: boolean }>(`/api/v1/products/ai-images/items/${id}/reject`, {});
}
