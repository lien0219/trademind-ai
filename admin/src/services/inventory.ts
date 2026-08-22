import { getJSON, getWithParams, postJSON } from "@/services/request";

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
  bindStatus?: string;
  bindConfidence?: number;
  bindMessage?: string;
  lastSyncedAt?: string;
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
  warehouseId?: string;
  orderNo?: string;
  orderItemId: string;
  productId?: string;
  productTitle?: string;
  productSkuId: string;
  skuCode?: string;
  skuName?: string;
  effectType: string;
  quantity: number;
  status: string;
  beforeStock?: number;
  afterStock?: number;
  reason?: string;
  errorMessage?: string;
  inventoryChangeLogId?: string;
};

export type InventoryCenterRow = InventoryAlertRow & {
  availableStock: number;
  skuBindStatus: string;
  platformSyncStatus: string;
  lastDeductAt?: string;
  exceptionCount: number;
  affectedOrderCount?: number;
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
  batchId?: string | null;
  batchNo?: string;
  createdAt: string;
  updatedAt: string;
};

export type AdjustStockPayload = {
  warehouseId: string;
  stock: number;
  idempotencyKey: string;
  reason?: string;
  remark?: string;
  sync?: boolean;
};

export async function adjustSkuStock(
  productId: string,
  skuId: string,
  payload: AdjustStockPayload,
) {
  return postJSON<Record<string, unknown>>(
    `/api/v1/products/${productId}/skus/${skuId}/adjust-stock`,
    payload,
  );
}

export type WarehouseBalance = {
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  isDefault: boolean;
  onHand: number;
  reserved: number;
  inTransit: number;
  damaged: number;
  available: number;
  version: number;
};

export type InventoryWarehouse = {
  id: string;
  code: string;
  name: string;
  status: string;
  isDefault: boolean;
};

export async function listInventoryWarehouses() {
  return getJSON<{ list: InventoryWarehouse[] }>("/api/v1/warehouses");
}

export type WarehouseTransferStatus = 'draft' | 'pending_approval' | 'approved' | 'in_transit' | 'received' | 'cancelled' | string;
export type WarehouseTransferItem = {
  id: string;
  productId: string;
  productSkuId: string;
  quantity: number;
  receivedQuantity: number;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
};
export type WarehouseTransfer = {
  id: string;
  transferNo: string;
  sourceWarehouseId: string;
  targetWarehouseId: string;
  sourceWarehouseCode?: string;
  sourceWarehouseName?: string;
  targetWarehouseCode?: string;
  targetWarehouseName?: string;
  status: WarehouseTransferStatus;
  revision: number;
  idempotencyKey: string;
  reason?: string;
  remark?: string;
  createdAt: string;
  updatedAt: string;
  items?: WarehouseTransferItem[];
  itemCount?: number;
};
export type WarehouseTransferList = {
  list: WarehouseTransfer[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
};
export type CreateWarehouseTransferPayload = {
  idempotencyKey: string;
  sourceWarehouseId: string;
  targetWarehouseId: string;
  reason?: string;
  remark?: string;
  items: Array<{ productSkuId: string; quantity: number }>;
};
export type WarehouseTransferActionPayload = {
  expectedRevision: number;
  idempotencyKey: string;
  reason?: string;
};

export async function queryWarehouseTransfers(params?: { page?: number; pageSize?: number; status?: string }) {
  return getWithParams<WarehouseTransferList>('/api/v1/inventory/warehouse-transfers', params);
}

export async function getWarehouseTransfer(id: string) {
  return getJSON<WarehouseTransfer>(`/api/v1/inventory/warehouse-transfers/${encodeURIComponent(id)}`);
}

export async function createWarehouseTransfer(payload: CreateWarehouseTransferPayload) {
  return postJSON<WarehouseTransfer>('/api/v1/inventory/warehouse-transfers', payload);
}

export async function actOnWarehouseTransfer(id: string, action: 'submit' | 'approve' | 'dispatch' | 'receive' | 'cancel', payload: WarehouseTransferActionPayload) {
  return postJSON<WarehouseTransfer>(`/api/v1/inventory/warehouse-transfers/${encodeURIComponent(id)}/${action}`, payload);
}

export async function listSkuWarehouseBalances(
  productId: string,
  skuId: string,
) {
  return getJSON<{ list: WarehouseBalance[] }>(
    `/api/v1/products/${encodeURIComponent(productId)}/skus/${encodeURIComponent(skuId)}/warehouse-balances`,
  );
}

export type WarehouseLedgerReconciliationRow = {
  productId: string;
  productTitle: string;
  productSkuId: string;
  skuCode: string;
  skuName: string;
  aggregateStock: number;
  warehouseOnHand: number;
  difference: number;
  balanceCount: number;
  status: "matched" | "unmigrated" | "mismatch" | string;
};

export type WarehouseLedgerReconciliation = {
  list: WarehouseLedgerReconciliationRow[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
  matched: number;
  unmigrated: number;
  mismatch: number;
};

export async function queryWarehouseLedgerReconciliation(params?: {
  page?: number;
  pageSize?: number;
  status?: string;
}) {
  return getWithParams<WarehouseLedgerReconciliation>(
    "/api/v1/inventory/warehouse-ledger/reconciliation",
    params,
  );
}

export type LegacyStockMigrationResult = {
  warehouseId: string;
  warehouseCode: string;
  migratedCount: number;
  remainingCount: number;
};

export async function migrateLegacyStock(limit = 100) {
  return postJSON<LegacyStockMigrationResult>(
    "/api/v1/inventory/warehouse-ledger/migrate-legacy",
    { limit },
  );
}

export function createInventoryIdempotencyKey(action: string) {
  const random =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  return `admin-${action}-${random}`.slice(0, 128);
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

export async function listProductPublicationSkus(
  productId: string,
  params?: { productSkuId?: string },
) {
  return getWithParams<{ list: PublicationSkuListingRow[] }>(
    `/api/v1/products/${productId}/publication-skus`,
    {
      ...(params?.productSkuId ? { productSkuId: params.productSkuId } : {}),
    },
  );
}

export async function syncPublicationSkuInventory(
  publicationSkuId: string,
  payload: {
    stock: number;
    options?: Record<string, unknown>;
    fromInventoryAlert?: boolean;
  },
) {
  return postJSON<InventorySyncTaskDTO>(
    `/api/v1/product-publication-skus/${publicationSkuId}/sync-inventory`,
    payload,
  );
}

export async function syncProductInventory(
  productId: string,
  payload: {
    shopId: string;
    skuIds: string[];
    options?: Record<string, unknown>;
    useLocal?: boolean;
  },
) {
  return postJSON<{ list: InventorySyncTaskDTO[] }>(
    `/api/v1/products/${productId}/sync-inventory`,
    payload,
  );
}

export async function queryInventorySyncTasks(params?: {
  page?: number;
  pageSize?: number;
  productId?: string;
  productSkuId?: string;
  shopId?: string;
  batchId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<PaginatedInventory<InventorySyncTaskDTO>>(
    "/api/v1/inventory-sync/tasks",
    params,
  );
}

export async function getInventorySyncTask(id: string) {
  return getJSON<InventorySyncTaskDTO>(`/api/v1/inventory-sync/tasks/${id}`);
}

export async function retryInventorySyncTask(id: string) {
  return postJSON<InventorySyncTaskDTO>(
    `/api/v1/inventory-sync/tasks/${id}/retry`,
    {},
  );
}

export type PlatformStockAlertEntry = {
  publicationSkuId: string;
  shopId: string;
  shopName?: string;
  platform: string;
  externalProductId?: string;
  externalSkuId?: string;
  platformStock?: number | null;
  platformStockStatus: string;
  lastSyncedAt?: string;
  lastSyncTaskId?: string;
  lastSyncStatus?: string;
  lastSyncError?: string;
  lastSyncAt?: string;
};

export type InventoryAlertRow = {
  productId: string;
  productTitle: string;
  productSkuId: string;
  skuCode: string;
  skuName: string;
  stock: number;
  warningStock: number;
  safetyStock: number;
  stockStatus: string;
  alertTypes: string[];
  publicationCount: number;
  platformStocks: PlatformStockAlertEntry[];
  lastInventoryChangeAt?: string;
  lastSyncTaskId?: string;
  lastSyncStatus?: string;
  lastSyncError?: string;
  lastSyncAt?: string;
};

export async function queryInventoryCenter(params?: {
  keyword?: string;
  productId?: string;
  productSkuId?: string;
  platform?: string;
  shopId?: string;
  stockStatus?: string;
  alertStatus?: string;
  skuBindStatus?: string;
  syncStatus?: string;
  hasException?: boolean;
  page?: number;
  pageSize?: number;
}) {
  const q: Record<string, string | number | undefined> = {
    keyword: params?.keyword?.trim() || undefined,
    productId: params?.productId,
    productSkuId: params?.productSkuId,
    platform: params?.platform?.trim() || undefined,
    shopId: params?.shopId,
    stockStatus: params?.stockStatus?.trim() || undefined,
    alertStatus: params?.alertStatus?.trim() || undefined,
    skuBindStatus: params?.skuBindStatus?.trim() || undefined,
    syncStatus: params?.syncStatus?.trim() || undefined,
    page: params?.page,
    pageSize: params?.pageSize,
  };
  if (params?.hasException) {
    q.hasException = "true";
  }
  return getWithParams<{
    list: InventoryCenterRow[];
    pagination: PaginatedInventory<InventoryCenterRow>["pagination"];
  }>("/api/v1/inventory", q);
}

export async function queryInventoryAlerts(params?: {
  keyword?: string;
  productId?: string;
  productSkuId?: string;
  platform?: string;
  shopId?: string;
  alertType?: string;
  stockStatus?: string;
  onlyPublished?: boolean;
  includeNormal?: boolean;
  page?: number;
  pageSize?: number;
}) {
  const q: Record<string, string | number | undefined> = {
    keyword: params?.keyword?.trim() || undefined,
    productId: params?.productId,
    productSkuId: params?.productSkuId,
    platform: params?.platform?.trim() || undefined,
    shopId: params?.shopId,
    alertType: params?.alertType?.trim() || undefined,
    stockStatus: params?.stockStatus?.trim() || undefined,
    page: params?.page,
    pageSize: params?.pageSize,
  };
  if (params?.onlyPublished) {
    q.onlyPublished = "true";
  }
  if (params?.includeNormal) {
    q.includeNormal = "true";
  }
  return getWithParams<{
    list: InventoryAlertRow[];
    pagination: PaginatedInventory<InventoryAlertRow>["pagination"];
  }>("/api/v1/inventory/alerts", q);
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
  return getWithParams<PaginatedInventory<InventoryChangeLogRow>>(
    "/api/v1/inventory/logs",
    params,
  );
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
  return getWithParams<PaginatedInventory<OrderInventoryEffectRow>>(
    "/api/v1/inventory/effects",
    params,
  );
}

export type InventorySyncBatchDTO = {
  id: string;
  batchNo: string;
  source: string;
  status: string;
  platform?: string;
  shopId?: string | null;
  shopName?: string;
  productId?: string | null;
  totalCount: number;
  pendingCount: number;
  runningCount: number;
  successCount: number;
  failedCount: number;
  skippedCount: number;
  skippedReason?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string | null;
  startedAt?: string | null;
  finishedAt?: string | null;
  createdAt: string;
  updatedAt: string;
  recentTasks?: InventorySyncTaskDTO[];
};

export type CreateInventorySyncBatchPayload = {
  source: string;
  platform?: string;
  shopId?: string;
  productId?: string;
  productSkuIds?: string[];
  publicationSkuIds?: string[];
  onlyAlerts?: boolean;
  alertTypes?: string[];
  onlyPublished?: boolean;
  confirmAll?: boolean;
  force?: boolean;
  options?: Record<string, unknown>;
};

export async function createInventorySyncBatch(
  payload: CreateInventorySyncBatchPayload,
) {
  return postJSON<InventorySyncBatchDTO>(
    "/api/v1/inventory-sync/batches",
    payload,
  );
}

export async function queryInventorySyncBatches(params?: {
  page?: number;
  pageSize?: number;
  source?: string;
  status?: string;
  platform?: string;
  shopId?: string;
  productId?: string;
  start?: string;
  end?: string;
}) {
  return getWithParams<{
    items: InventorySyncBatchDTO[];
    pagination: PaginatedInventory<InventorySyncBatchDTO>["pagination"];
  }>("/api/v1/inventory-sync/batches", params);
}

export async function getInventorySyncBatch(
  id: string,
  params?: { recentTasks?: number },
) {
  return getWithParams<InventorySyncBatchDTO>(
    `/api/v1/inventory-sync/batches/${encodeURIComponent(id)}`,
    {
      recentTasks: params?.recentTasks,
    },
  );
}

export async function queryInventorySyncBatchTasks(
  batchId: string,
  params?: {
    page?: number;
    pageSize?: number;
    status?: string;
    platform?: string;
    productId?: string;
    productSkuId?: string;
    shopId?: string;
    start?: string;
    end?: string;
  },
) {
  return getWithParams<PaginatedInventory<InventorySyncTaskDTO>>(
    `/api/v1/inventory-sync/batches/${encodeURIComponent(batchId)}/tasks`,
    params,
  );
}

export async function retryInventorySyncBatchFailed(batchId: string) {
  return postJSON<InventorySyncBatchDTO>(
    `/api/v1/inventory-sync/batches/${encodeURIComponent(batchId)}/retry-failed`,
    {},
  );
}

export async function retryInventorySyncTasksBatch(taskIds: string[]) {
  return postJSON<InventorySyncBatchDTO>(
    "/api/v1/inventory-sync/batches/retry-failed-tasks",
    { taskIds },
  );
}

export type BatchStockSettingsPreviewPayload = {
  productId?: string;
  productSkuIds?: string[];
  platform?: string;
  shopId?: string;
  stockStatus?: string;
  alertTypes?: string[];
  keyword?: string;
  onlyPublished?: boolean;
  includeNormal?: boolean;
  page?: number;
  pageSize?: number;
};

export type BatchStockSettingsSampleSku = {
  productId: string;
  productSkuId: string;
  skuCode?: string;
  productTitle?: string;
};

export type BatchStockSettingsPreviewResult = {
  matchedCount: number;
  sampleSkus: BatchStockSettingsSampleSku[];
  page: number;
  pageSize: number;
  totalPages: number;
};

export async function previewBatchStockSettings(
  payload: BatchStockSettingsPreviewPayload,
) {
  return postJSON<BatchStockSettingsPreviewResult>(
    "/api/v1/inventory/stock-settings/batch-preview",
    payload,
  );
}

export type BatchStockSettingsUpdatePayload =
  BatchStockSettingsPreviewPayload & {
    warningStock: number;
    safetyStock: number;
    confirm: boolean;
    confirmLarge?: boolean;
    confirmAll?: boolean;
  };

export type BatchStockSettingsUpdateResult = {
  matchedCount: number;
  updatedCount: number;
  summary: string;
};

export async function batchUpdateStockSettings(
  payload: BatchStockSettingsUpdatePayload,
) {
  return postJSON<BatchStockSettingsUpdateResult>(
    "/api/v1/inventory/stock-settings/batch-update",
    payload,
  );
}
