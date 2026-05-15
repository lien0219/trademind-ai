import { getJSON, getWithParams, postJSON } from './request';

export type CollectTaskRow = {
  id: string;
  source: string;
  sourceUrl: string;
  status: string;
  resultProductId?: string;
  rawResult?: unknown;
  errorMessage?: string;
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

export async function fetchCollectTasks(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  source?: string;
  keyword?: string;
}) {
  return getWithParams<{ list: CollectTaskRow[]; pagination: Pagination }>('/api/v1/collect/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
    source: params.source,
    keyword: params.keyword,
  });
}

export async function fetchCollectTask(id: string) {
  return getJSON<CollectTaskRow>(`/api/v1/collect/tasks/${id}`);
}

export async function createCollectTask(body: { source: string; url: string }) {
  return postJSON<CollectTaskRow>('/api/v1/collect/tasks', body);
}

export async function retryCollectTask(id: string) {
  return postJSON<CollectTaskRow>(`/api/v1/collect/tasks/${id}/retry`);
}
