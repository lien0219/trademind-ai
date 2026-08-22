import { request } from '@umijs/max';
import { ApiRequestError, getJSON, getWithParams, postJSON, putJSON } from './request';

export type Warehouse = {
  id: string;
  code: string;
  name: string;
  status: 'active' | 'inactive' | string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
};

export type Supplier = {
  id: string;
  code: string;
  name: string;
  status: 'active' | 'inactive' | string;
  contactName?: string;
  phone?: string;
  email?: string;
  createdAt: string;
  updatedAt: string;
};

export type SupplierSKU = {
  id: string;
  supplierId: string;
  productSkuId: string;
  productTitle: string;
  skuCode: string;
  skuName: string;
  supplierSkuCode?: string;
  unitCostMinor: number;
  currency: string;
  minOrderQty: number;
  leadTimeDays: number;
};

export type PurchaseOrderStatus =
  | 'draft'
  | 'pending_approval'
  | 'approved'
  | 'partially_received'
  | 'received'
  | 'closed'
  | 'cancelled';

export type PurchaseOrderItem = {
  id: string;
  purchaseOrderId: string;
  productSkuId: string;
  supplierSkuId?: string;
  quantity: number;
  receivedQuantity: number;
  unitCostMinor: number;
  lineAmountMinor: number;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
};

export type PurchaseOrder = {
  id: string;
  purchaseOrderNo: string;
  supplierId: string;
  warehouseId: string;
  status: PurchaseOrderStatus | string;
  currency: string;
  totalAmountMinor: number;
  revision: number;
  remark?: string;
  approvedAt?: string;
  closedAt?: string;
  cancelledAt?: string;
  createdAt: string;
  updatedAt: string;
  items?: PurchaseOrderItem[];
};

export type GoodsReceipt = {
  id: string;
  receiptNo: string;
  purchaseOrderId: string;
  warehouseId: string;
  receivedAt: string;
};

export type PurchaseReturnStatus = 'draft' | 'pending_approval' | 'approved' | 'completed' | 'cancelled';

export type PurchaseReturnItem = {
  id: string;
  purchaseReturnId: string;
  goodsReceiptItemId: string;
  purchaseOrderItemId: string;
  productSkuId: string;
  quantity: number;
  receiptNo?: string;
  receiptQuantity: number;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
};

export type PurchaseReturn = {
  id: string;
  returnNo: string;
  purchaseOrderId: string;
  purchaseOrderNo?: string;
  supplierId: string;
  supplierName?: string;
  warehouseId: string;
  warehouseName?: string;
  status: PurchaseReturnStatus | string;
  revision: number;
  reason?: string;
  remark?: string;
  approvedBy?: string;
  approvedAt?: string;
  completedBy?: string;
  completedAt?: string;
  cancelledAt?: string;
  itemCount?: number;
  createdAt: string;
  updatedAt: string;
  items?: PurchaseReturnItem[];
};

export type PurchaseReturnList = {
  list: PurchaseReturn[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type ReturnableReceiptItem = {
  goodsReceiptItemId: string;
  goodsReceiptId: string;
  receiptNo: string;
  purchaseOrderItemId: string;
  productSkuId: string;
  productTitle: string;
  skuCode: string;
  skuName: string;
  receivedQuantity: number;
  allocatedReturnQuantity: number;
  remainingQuantity: number;
};

export type PurchaseOrderList = {
  list: PurchaseOrder[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type CreatePurchaseOrderBody = {
  idempotencyKey: string;
  supplierId: string;
  warehouseId: string;
  currency: string;
  remark: string;
  items: Array<{
    productSkuId: string;
    supplierSkuId?: string;
    quantity: number;
    unitCostMinor: number;
  }>;
};

export type ReplenishmentSuggestionStatus =
  | 'actionable'
  | 'not_needed'
  | 'blocked_inventory_mismatch'
  | 'blocked_inventory_unmigrated'
  | 'blocked_supplier_missing'
  | 'blocked_supplier_selection'
  | string;

export type ReplenishmentSuggestion = {
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  productId: string;
  productTitle: string;
  productSkuId: string;
  skuCode: string;
  skuName: string;
  availableStock: number;
  inTransitTransfer: number;
  pendingPurchase: number;
  warningStock: number;
  safetyStock: number;
  deficit: number;
  suggestedQuantity: number;
  minOrderQty: number;
  unitCostMinor: number;
  currency: string;
  leadTimeDays: number;
  supplierId?: string;
  supplierName?: string;
  status: ReplenishmentSuggestionStatus;
  blockReasonCode?: string;
  blockReason?: string;
  inventoryOnHandTotal: number;
  inventoryBalanceCount: number;
};

export type ReplenishmentSuggestionList = {
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  list: ReplenishmentSuggestion[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

const enc = encodeURIComponent;

export function createProcurementIdempotencyKey(action: string) {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  return `admin-${action}-${random}`.slice(0, 128);
}

export async function listWarehouses() {
  return getJSON<{ list: Warehouse[] }>('/api/v1/warehouses');
}

export async function queryReplenishmentSuggestions(params: {
  warehouseId: string;
  keyword?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}) {
  return getWithParams<ReplenishmentSuggestionList>('/api/v1/procurement/replenishment-suggestions', {
    warehouseId: params.warehouseId,
    keyword: params.keyword?.trim() || undefined,
    status: params.status || undefined,
    page: params.page,
    pageSize: params.pageSize,
  });
}

export async function downloadReplenishmentSuggestions(params: { warehouseId: string; keyword?: string; status?: string }) {
  const query = new URLSearchParams({ warehouseId: params.warehouseId, format: 'csv' });
  if (params.keyword?.trim()) query.set('keyword', params.keyword.trim());
  if (params.status) query.set('status', params.status);
  const blob = await request<Blob>(`/api/v1/procurement/replenishment-suggestions?${query.toString()}`, {
    method: 'GET',
    responseType: 'blob',
  });
  const anchor = document.createElement('a');
  const objectURL = URL.createObjectURL(blob);
  anchor.href = objectURL;
  anchor.download = 'replenishment-suggestions.csv';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}

export async function createWarehouse(body: { code: string; name: string; isDefault: boolean }) {
  return postJSON<Warehouse>('/api/v1/warehouses', body);
}

export async function updateWarehouse(id: string, body: { name: string; status: string; isDefault: boolean }) {
  return putJSON<Warehouse, typeof body>(`/api/v1/warehouses/${enc(id)}`, body);
}

export async function listSuppliers() {
  return getJSON<{ list: Supplier[] }>('/api/v1/suppliers');
}

export async function createSupplier(body: {
  code: string;
  name: string;
  contactName: string;
  phone: string;
  email: string;
}) {
  return postJSON<Supplier>('/api/v1/suppliers', body);
}

export async function updateSupplier(id: string, body: {
  name: string;
  status: string;
  contactName: string;
  phone?: string;
  email?: string;
}) {
  return putJSON<Supplier, typeof body>(`/api/v1/suppliers/${enc(id)}`, body);
}

export async function listSupplierSKUs(supplierId: string) {
  return getJSON<{ list: SupplierSKU[] }>(`/api/v1/suppliers/${enc(supplierId)}/skus`);
}

export async function bindSupplierSKU(supplierId: string, body: {
  productSkuId: string;
  supplierSkuCode: string;
  unitCostMinor: number;
  currency: string;
  minOrderQty: number;
  leadTimeDays: number;
}) {
  return postJSON<SupplierSKU>(`/api/v1/suppliers/${enc(supplierId)}/skus`, body);
}

export async function listPurchaseOrders(params: { page?: number; pageSize?: number }) {
  return getWithParams<PurchaseOrderList>('/api/v1/purchase-orders', params);
}

export async function getPurchaseOrder(id: string) {
  return getJSON<PurchaseOrder>(`/api/v1/purchase-orders/${enc(id)}`);
}

export async function createPurchaseOrder(body: CreatePurchaseOrderBody) {
  return postJSON<PurchaseOrder>('/api/v1/purchase-orders', body);
}

export async function transitionPurchaseOrder(
  id: string,
  action: 'submit' | 'approve' | 'cancel' | 'close',
  body: { expectedRevision: number; reason: string },
) {
  return postJSON<PurchaseOrder>(`/api/v1/purchase-orders/${enc(id)}/${action}`, body);
}

export async function receivePurchaseOrder(id: string, body: {
  expectedRevision: number;
  idempotencyKey: string;
  items: Array<{ purchaseOrderItemId: string; quantity: number }>;
}) {
  return postJSON<{ purchaseOrder: PurchaseOrder; receipt: GoodsReceipt }>(
    `/api/v1/purchase-orders/${enc(id)}/receipts`,
    body,
  );
}

export async function listReturnableReceiptItems(purchaseOrderId: string) {
  return getJSON<{ list: ReturnableReceiptItem[] }>(
    `/api/v1/purchase-orders/${enc(purchaseOrderId)}/returnable-receipt-items`,
  );
}

export async function listPurchaseReturns(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  purchaseOrderId?: string;
}) {
  return getWithParams<PurchaseReturnList>('/api/v1/purchase-returns', params);
}

export async function getPurchaseReturn(id: string) {
  return getJSON<PurchaseReturn>(`/api/v1/purchase-returns/${enc(id)}`);
}

export async function createPurchaseReturn(body: {
  idempotencyKey: string;
  purchaseOrderId: string;
  reason: string;
  remark: string;
  items: Array<{ goodsReceiptItemId: string; quantity: number }>;
}) {
  return postJSON<PurchaseReturn>('/api/v1/purchase-returns', body);
}

export async function transitionPurchaseReturn(
  id: string,
  action: 'submit' | 'approve' | 'complete' | 'cancel',
  body: { expectedRevision: number; idempotencyKey: string; reason: string },
) {
  return postJSON<PurchaseReturn>(`/api/v1/purchase-returns/${enc(id)}/${action}`, body);
}

export type ProcurementAPIError = {
  code: number;
  message: string;
  traceId?: string;
};

export function extractProcurementAPIError(error: unknown): ProcurementAPIError {
  if (error instanceof ApiRequestError) {
    return { code: error.code, message: error.message, traceId: error.traceId };
  }
  if (error instanceof Error) {
    return { code: -1, message: error.message };
  }
  return { code: -1, message: 'request_failed' };
}

export function procurementErrorMessage(error: ProcurementAPIError, fallback = '操作失败，请稍后重试') {
  const message = error.message.toLowerCase();
  if (message.includes('purchase return revision conflict')) return '采购退货单已被其他人更新，页面已重新加载，请确认最新状态。';
  if (message.includes('purchase return transition')) return '当前采购退货单状态不允许执行此操作。';
  if (message.includes('purchase return idempotency')) return '相同退货操作编号已用于其他内容，请关闭窗口后重新发起。';
  if (message.includes('revision conflict')) return '采购单已被其他人更新，页面已重新加载，请确认最新状态后再操作。';
  if (message.includes('invalid transition')) return '当前采购单状态不允许执行此操作，请重新加载后确认。';
  if (message.includes('exceeds remaining')) return '收货数量超过未收数量，请核对后重试。';
  if (message.includes('exceeds received')) return '退货数量超过原收货记录的剩余可退数量，请重新加载后核对。';
  if (message.includes('insufficient warehouse stock')) return '当前仓库可用库存不足，采购退货尚未执行，请先核对库存占用。';
  if (message.includes('approver cannot complete')) return '审批人与退货执行人必须为不同账号。';
  if (message.includes('idempotency key')) return '相同操作编号已用于其他内容，请关闭窗口后重新发起。';
  if (message.includes('warehouse conflict')) return '仓库编码已存在，请更换编码。';
  if (message.includes('supplier conflict')) return '供应商编码已存在，请更换编码。';
  if (error.code === 403 || error.code === 40301) return '当前账号没有执行此操作的权限。';
  return fallback;
}
