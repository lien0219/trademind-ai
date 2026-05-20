import { getJSON, getWithParams, postJSON } from './request';

export type CollectTaskRow = {
  id: string;
  batchId?: string;
  source: string;
  sourceUrl: string;
  status: string;
  resultProductId?: string;
  rawResult?: unknown;
  errorMessage?: string;
  collectorErrorCode?: string;
  retryable?: boolean;
  failureHint?: string;
  sameUrlSucceededElsewhere?: boolean;
  retryCount?: number;
  maxRetries?: number;
  nextRetryAt?: string;
  retryEnqueuedAt?: string;
  createdBy?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type Pagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type CollectTaskEventRow = {
  id: string;
  taskId: string;
  batchId?: string;
  eventType: string;
  fromStatus?: string | null;
  toStatus?: string | null;
  message?: string;
  errorMessage?: string;
  retryCount?: number;
  maxRetries?: number;
  nextRetryAt?: string | null;
  payload?: unknown;
  createdAt: string;
};

export async function fetchCollectTasks(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  source?: string;
  keyword?: string;
  batchId?: string;
}) {
  return getWithParams<{ list: CollectTaskRow[]; pagination: Pagination }>('/api/v1/collect/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
    source: params.source,
    keyword: params.keyword,
    batchId: params.batchId,
  });
}

export async function fetchCollectTask(id: string) {
  return getJSON<CollectTaskRow>(`/api/v1/collect/tasks/${id}`);
}

export async function createCollectTask(body: {
  source: string;
  url: string;
  ruleId?: string;
  profileId?: string;
  useBrowserProfile?: boolean;
}) {
  return postJSON<CollectTaskRow>('/api/v1/collect/tasks', body);
}

export async function retryCollectTask(id: string) {
  return postJSON<CollectTaskRow>(`/api/v1/collect/tasks/${id}/retry`);
}

export async function queryCollectTaskEvents(
  taskId: string,
  params?: { page?: number; pageSize?: number },
) {
  return getWithParams<{ list: CollectTaskEventRow[]; pagination: Pagination }>(
    `/api/v1/collect/tasks/${taskId}/events`,
    {
      page: params?.page ?? 1,
      pageSize: params?.pageSize ?? 50,
    },
  );
}
