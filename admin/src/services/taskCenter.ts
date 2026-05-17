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
  failureCategory?: string;
  severity?: string;
  classificationReason?: string;
  matchedRule?: string;
  suggestedAction?: string;
  alertStatus?: string;
  relatedAlertId?: string;
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

/** In-site alerts */

export type TaskAlertDTO = {
  id: string;
  taskType: string;
  sourceId: string;
  sourceTable?: string;
  platform?: string;
  failureCategory: string;
  severity: string;
  title: string;
  message?: string;
  suggestedAction?: string;
  status: string;
  alertCount: number;
  firstSeenAt: string;
  lastSeenAt: string;
  handledAt?: string;
};

export type TaskFailureCategoriesResp = {
  categories: string[];
  severities: string[];
};

export type ScanAlertsSummary = {
  scannedCount: number;
  generatedCount: number;
  updatedCount: number;
  ignoredCount: number;
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

export async function queryTaskFailureCategories() {
  return getJSON<TaskFailureCategoriesResp>(`/api/v1/task-center/failure-categories`);
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

export async function generateTaskFailureAlert(taskType: string, id: string) {
  const encType = encodeURIComponent(taskType);
  return postJSON<TaskAlertDTO>(
    `/api/v1/task-center/failures/${encType}/${encodeURIComponent(id)}/generate-alert`,
    {},
  );
}

export async function queryTaskAlerts(
  params: Record<string, string | number | boolean | undefined>,
) {
  type Resp = {
    list: TaskAlertDTO[];
    pagination?: { page: number; pageSize: number; total: number };
  };
  const data = await getWithParams<Resp>(`/api/v1/task-center/alerts`, params as Record<
    string,
    string | number | undefined
  >);
  return {
    list: data.list ?? [],
    total: data.pagination?.total ?? 0,
  };
}

export async function scanTaskAlerts() {
  return postJSON<ScanAlertsSummary>(`/api/v1/task-center/alerts/scan`, {});
}

export async function markTaskAlertHandled(id: string) {
  return postJSON<{ ok: boolean }>(
    `/api/v1/task-center/alerts/${encodeURIComponent(id)}/handle`,
    {},
  );
}

export async function markTaskAlertIgnored(id: string) {
  return postJSON<{ ok: boolean }>(
    `/api/v1/task-center/alerts/${encodeURIComponent(id)}/ignore`,
    {},
  );
}

export async function unmarkTaskAlertRecord(id: string) {
  return deleteJSON<{ ok: boolean }>(
    `/api/v1/task-center/alerts/${encodeURIComponent(id)}/mark`,
  );
}
