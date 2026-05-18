import { getWithParams, postJSON } from '@/services/request';

export type Paginated<T> = {
  list: T[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
};

export type AIOperationBatchRow = {
  id: string;
  batchNo: string;
  operationType: string;
  status: string;
  productCount: number;
  taskCount: number;
  successCount: number;
  failedCount: number;
  skippedCount: number;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
  startedAt?: string;
  finishedAt?: string;
};

export type ProductFilters = {
  keyword?: string;
  status?: string;
  source?: string;
  onlyMissingAiTitle?: boolean;
  onlyMissingAiDescription?: boolean;
  onlyHasMainImage?: boolean;
};

export async function createProductTextBatch(body: {
  operationType: string;
  productIds: string[];
  filters: ProductFilters;
  options: {
    language?: string;
    platform?: string;
    maxLength?: number;
    tone?: string;
  };
  applyMode?: string;
  confirmAll?: boolean;
}) {
  return postJSON<AIOperationBatchRow>('/api/v1/ai/batches/product-text', body);
}

export async function createProductImagesBatch(body: {
  operationType: string;
  productIds: string[];
  filters: ProductFilters;
  options: {
    provider?: string;
    prompt?: string;
    backgroundPrompt?: string;
    style?: string;
  };
  confirmAll?: boolean;
}) {
  return postJSON<AIOperationBatchRow>('/api/v1/ai/batches/product-images', body);
}

export async function fetchAiBatches(params: {
  page?: number;
  pageSize?: number;
  operationType?: string;
  status?: string;
  createdBy?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<Paginated<AIOperationBatchRow>>('/api/v1/ai/batches', params);
}

export type AiBatchDetail = {
  batch: AIOperationBatchRow;
  recentAiTasks: unknown[];
  recentImageTasks: unknown[];
};

export async function fetchAiBatchDetail(id: string) {
  return getWithParams<AiBatchDetail>(`/api/v1/ai/batches/${id}`, {});
}

export async function fetchAiBatchTasks(id: string, params: { page?: number; pageSize?: number }) {
  return getWithParams<{ kind: string; list: unknown[]; pagination: Paginated<unknown>['pagination'] }>(
    `/api/v1/ai/batches/${id}/tasks`,
    params,
  );
}

export async function retryAiBatchFailed(id: string) {
  return postJSON<AIOperationBatchRow>(`/api/v1/ai/batches/${id}/retry-failed`, {});
}

export async function applyAiBatchResults(id: string, body: { target: string; productIds?: string[] }) {
  return postJSON<{ applied: number }>(`/api/v1/ai/batches/${id}/apply-results`, body);
}
