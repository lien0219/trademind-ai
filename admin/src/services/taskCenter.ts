import { deleteJSON, getJSON, getWithParams, postJSON } from '@/services/request';

export type UnifiedTaskDTO = {
  id: string;
  taskType: string;
  sourceTable: string;
  sourceId: string;
  title: string;
  platform?: string;
  shopId?: string;
  shopName?: string;
  relatedResourceType?: string;
  relatedResourceId?: string;
  relatedResourceTitle?: string;
  status: string;
  normalizedStatus: string;
  retryable: boolean;
  ignored: boolean;
  handled: boolean;
  errorMessage?: string;
  errorCode?: string;
  retryCount: number;
  maxRetries?: number;
  lockedBy?: string;
  lockedUntil?: string;
  createdAt: string;
  updatedAt: string;
  startedAt?: string;
  finishedAt?: string;
  detailUrl?: string;
  retryAction?: string;
  rawSummary?: string;
};

export type FailuresSummary = {
  totalFailed: number;
  retryingTotal?: number;
  staleTotal?: number;
  leaseExpiredTotal?: number;
  byType: Record<string, number>;
  byPlatform: Record<string, number>;
  retryableCount: number;
  ignoredCount: number;
  handledCount: number;
  latestFailedAt?: string;
};

export type ListFailuresResult = {
  list: UnifiedTaskDTO[];
  total: number;
  summary: FailuresSummary;
};

export type FailureDetailDTO = UnifiedTaskDTO & {
  extra?: Record<string, unknown>;
};

export type BatchRetryOneResult = {
  taskType: string;
  id: string;
  ok: boolean;
  error?: string;
};

export type BatchRetryResponse = {
  successCount: number;
  failedCount: number;
  results: BatchRetryOneResult[];
};

export async function queryTaskFailures(params: Record<string, string | number | boolean | undefined>) {
  return getWithParams<ListFailuresResult>(`/api/v1/task-center/failures`, params as Record<
    string,
    string | number | undefined
  >);
}

export async function queryTaskCenterSummary(params?: Record<string, string | undefined>) {
  return getWithParams<FailuresSummary>(`/api/v1/task-center/summary`, params as Record<
    string,
    string | number | undefined
  >);
}

export async function getTaskFailureDetail(taskType: string, id: string) {
  const enc = encodeURIComponent(taskType);
  return getJSON<FailureDetailDTO>(`/api/v1/task-center/failures/${enc}/${encodeURIComponent(id)}`);
}

export async function retryTaskFailure(taskType: string, id: string) {
  const encType = encodeURIComponent(taskType);
  return postJSON<FailureDetailDTO | { retried?: boolean }>(
    `/api/v1/task-center/failures/${encType}/${encodeURIComponent(id)}/retry`,
  );
}

export async function batchRetryTaskFailures(items: { taskType: string; id: string }[]) {
  return postJSON<BatchRetryResponse>(`/api/v1/task-center/failures/batch-retry`, { items });
}

export async function ignoreTaskFailure(taskType: string, id: string, remark?: string) {
  const encType = encodeURIComponent(taskType);
  return postJSON<{ ok: boolean }>(
    `/api/v1/task-center/failures/${encType}/${encodeURIComponent(id)}/ignore`,
    { remark },
  );
}

export async function handleTaskFailure(taskType: string, id: string, remark?: string) {
  const encType = encodeURIComponent(taskType);
  return postJSON<{ ok: boolean }>(
    `/api/v1/task-center/failures/${encType}/${encodeURIComponent(id)}/handle`,
    { remark },
  );
}

export async function unmarkTaskFailure(taskType: string, id: string) {
  const encType = encodeURIComponent(taskType);
  return deleteJSON<{ ok: boolean }>(
    `/api/v1/task-center/failures/${encType}/${encodeURIComponent(id)}/mark`,
  );
}

export async function batchIgnoreTaskFailures(items: { taskType: string; id: string }[], remark?: string) {
  return postJSON<BatchRetryResponse>(`/api/v1/task-center/failures/batch-ignore`, { items, remark });
}

export async function batchHandleTaskFailures(items: { taskType: string; id: string }[], remark?: string) {
  return postJSON<BatchRetryResponse>(`/api/v1/task-center/failures/batch-handle`, { items, remark });
}
