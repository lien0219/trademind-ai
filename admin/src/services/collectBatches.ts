import { getJSON, getWithParams, postJSON } from './request';
import type { CollectTaskRow, Pagination } from './collectTasks';

export type CollectBatchRow = {
  id: string;
  source: string;
  totalCount: number;
  pendingCount: number;
  runningCount: number;
  successCount: number;
  failedCount: number;
  cancelledCount: number;
  status: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  finishedAt?: string;
};

export async function createCollectBatch(payload: { source: string; urls: string[] }) {
  return postJSON<{ batch: CollectBatchRow; taskCount: number }>('/api/v1/collect/batches', payload);
}

export async function queryCollectBatches(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  source?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<{ list: CollectBatchRow[]; pagination: Pagination }>('/api/v1/collect/batches', {
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
    source: params.source,
    start: params.start,
    end: params.end,
  });
}

export async function getCollectBatch(id: string) {
  return getJSON<CollectBatchRow>(`/api/v1/collect/batches/${id}`);
}

export async function queryCollectBatchTasks(
  batchId: string,
  params: { page?: number; pageSize?: number; status?: string },
) {
  return getWithParams<{ list: CollectTaskRow[]; pagination: Pagination }>(
    `/api/v1/collect/batches/${batchId}/tasks`,
    {
      page: params.page,
      pageSize: params.pageSize,
      status: params.status,
    },
  );
}

export async function retryFailedBatchTasks(id: string) {
  return postJSON<{ retried: number }>(`/api/v1/collect/batches/${id}/retry-failed`);
}
