import { getJSON, getWithParams, postJSON } from '@/services/request';

export type PaginatedInventory<T> = {
  list: T[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
};

export type PublicationSkuListingRow = {
  publicationSkuId: string;
  publicationId: string;
  productSkuId?: string;
  shopId: string;
  shopName?: string;
  platform: string;
  externalProductId?: string;
  externalSkuId?: string;
  skuCode?: string;
  platformStock?: number | null;
  inventorySyncCapability?: string;
};

export type InventoryChangeLogRow = {
  id: string;
  createdAt: string;
  changeType: string;
  beforeStock: number;
  afterStock: number;
  delta: number;
  reason?: string;
  remark?: string;
  createdBy?: string | null;
  refOrderId?: string;
  refOrderItemId?: string;
};

export type OrderInventoryEffectRow = {
  id: string;
  createdAt: string;
  updatedAt: string;
  orderId: string;
  orderNo?: string;
  orderItemId: string;
  productId?: string;
  productSkuId: string;
  effectType: string;
  quantity: number;
  status: string;
  beforeStock?: number;
  afterStock?: number;
  reason?: string;
  errorMessage?: string;
  inventoryChangeLogId?: string;
};

export type InventorySyncTaskDTO = {
  id: string;
  productId: string;
  productTitle?: string;
  productSkuId?: string;
  skuCode?: string;
  publicationId?: string;
  publicationSkuId?: string;
  shopId: string;
  shopName?: string;
  platform: string;
  taskType: string;
  status: string;
  mode: string;
  targetStock: number;
  startedAt?: string;
  finishedAt?: string;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string | null;
  createdAt: string;
  updatedAt: string;
};

export type AdjustStockPayload = {
  stock: number;
  reason?: string;
  remark?: string;
  sync?: boolean;
};

export async function adjustSkuStock(productId: string, skuId: string, payload: AdjustStockPayload) {
  return postJSON<Record<string, unknown>>(`/api/v1/products/${productId}/skus/${skuId}/adjust-stock`, payload);
}

export async function querySkuInventoryLogs(
  productId: string,
  skuId: string,
  params?: { page?: number; pageSize?: number },
) {
  return getWithParams<PaginatedInventory<InventoryChangeLogRow>>(
    `/api/v1/products/${productId}/skus/${skuId}/inventory-logs`,
    {
      page: params?.page,
      pageSize: params?.pageSize,
    },
  );
}

export async function listProductPublicationSkus(productId: string, params?: { productSkuId?: string }) {
  return getWithParams<{ list: PublicationSkuListingRow[] }>(`/api/v1/products/${productId}/publication-skus`, {
    ...(params?.productSkuId ? { productSkuId: params.productSkuId } : {}),
  });
}

export async function syncPublicationSkuInventory(
  publicationSkuId: string,
  payload: { stock: number; options?: Record<string, unknown> },
) {
  return postJSON<InventorySyncTaskDTO>(`/api/v1/product-publication-skus/${publicationSkuId}/sync-inventory`, payload);
}

export async function syncProductInventory(
  productId: string,
  payload: { shopId: string; skuIds: string[]; options?: Record<string, unknown>; useLocal?: boolean },
) {
  return postJSON<{ list: InventorySyncTaskDTO[] }>(`/api/v1/products/${productId}/sync-inventory`, payload);
}

export async function queryInventorySyncTasks(params?: {
  page?: number;
  pageSize?: number;
  productId?: string;
  productSkuId?: string;
  shopId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<PaginatedInventory<InventorySyncTaskDTO>>('/api/v1/inventory-sync/tasks', params);
}

export async function getInventorySyncTask(id: string) {
  return getJSON<InventorySyncTaskDTO>(`/api/v1/inventory-sync/tasks/${id}`);
}

export async function retryInventorySyncTask(id: string) {
  return postJSON<InventorySyncTaskDTO>(`/api/v1/inventory-sync/tasks/${id}/retry`, {});
}

export async function queryGlobalInventoryLogs(params?: {
  page?: number;
  pageSize?: number;
  productId?: string;
  productSkuId?: string;
  orderId?: string;
  changeType?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<PaginatedInventory<InventoryChangeLogRow>>('/api/v1/inventory/logs', params);
}

export async function queryGlobalInventoryEffects(params?: {
  page?: number;
  pageSize?: number;
  orderId?: string;
  productSkuId?: string;
  effectType?: string;
  status?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<PaginatedInventory<OrderInventoryEffectRow>>('/api/v1/inventory/effects', params);
}
