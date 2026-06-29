import { getJSON, getWithParams, postJSON } from '@/services/request';

export type OrderSyncPageError = {
  page: number;
  error: string;
  cursor?: string;
  nextPage?: string;
  errorCode?: string;
  retryable?: boolean;
};

export type OrderSyncTaskOutput = {
  totalFetched?: number;
  totalPages?: number;
  successPages?: number;
  failedPages?: number;
  pageErrors?: OrderSyncPageError[];
  createdOrders?: number;
  updatedOrders?: number;
  matchedItems?: number;
  unmatchedItems?: number;
  deductedStockItems?: number;
  hasMore?: boolean;
  retryPagesOnly?: boolean;
};

export type OrderSyncTaskDTO = {
  id: string;
  shopId: string;
  shopName?: string;
  platform: string;
  taskType: string;
  status: string;
  mode: string;
  cursor?: string;
  startedAt?: string;
  finishedAt?: string;
  totalCount: number;
  successCount: number;
  failedCount: number;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function syncShopOrders(
  shopId: string,
  payload: {
    mode?: string;
    start?: string;
    end?: string;
    cursor?: string;
    limit?: number;
  },
): Promise<OrderSyncTaskDTO> {
  return postJSON(`/api/v1/shops/${shopId}/sync-orders`, payload);
}

export async function queryOrderSyncTasks(params: {
  page?: number;
  pageSize?: number;
  shopId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}): Promise<{
  list: OrderSyncTaskDTO[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/order-sync/tasks', params);
}

export async function getOrderSyncTask(id: string): Promise<OrderSyncTaskDTO> {
  return getJSON(`/api/v1/order-sync/tasks/${id}`);
}

export async function retryOrderSyncTask(id: string): Promise<OrderSyncTaskDTO> {
  return postJSON(`/api/v1/order-sync/tasks/${id}/retry`, {});
}
