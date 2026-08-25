import {
  ApiRequestError,
  deleteJSON,
  getJSON,
  getWithParams,
  postJSON,
  putJSON,
} from "@/services/request";
import type {
  OrderInventoryEffectRow,
  PaginatedInventory,
} from "@/services/inventory";

export type OrderShipmentRow = {
  id: string;
  orderId: string;
  carrier: string;
  trackingNo: string;
  trackingUrl?: string;
  status: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type OrderShipmentEventRow = {
  id: string;
  tenantId: number;
  orderId: string;
  shipmentId: string;
  eventKey: string;
  status: string;
  occurredAt: string;
  location?: string;
  description?: string;
  source: string;
  rawData?: Record<string, unknown>;
};

export type OrderShipmentEventsResponse = {
  shipment: OrderShipmentRow;
  events: OrderShipmentEventRow[];
  provider: string;
};

export type OrderItemRow = {
  id: string;
  orderId: string;
  productId?: string;
  productSkuId?: string;
  externalItemId?: string;
  externalSkuId?: string;
  sellerSku?: string;
  productTitle: string;
  skuName?: string;
  skuCode?: string;
  quantity: number;
  unitPrice: number;
  totalPrice: number;
  imageUrl?: string;
  attrs?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type OrderShopSummary = {
  id: string;
  platform: string;
  shopName: string;
  shopCode?: string;
  status: string;
  authStatus: string;
};

/** Order inventory flags from backend `inventory_summary` projection. */
export type OrderInventorySummary = {
  hasReservationSuccess: boolean;
  hasReleaseSuccess: boolean;
  hasDeductionSuccess: boolean;
  hasRestoreSuccess: boolean;
  fullyRestored: boolean;
};

/** GET /orders/:id response (flattened header + nested children) */
export type OrderDetailDTO = {
  id: string;
  tenantId: number;
  platform: string;
  shopId?: string;
  warehouseId?: string;
  shopSummary?: OrderShopSummary | null;
  externalOrderId?: string;
  orderNo: string;
  customerName: string;
  customerEmail?: string;
  customerPhone?: string;
  status: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  currency: string;
  totalAmount: number;
  paidAt?: string;
  orderedAt?: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  items: OrderItemRow[];
  shipments: OrderShipmentRow[];
  inventorySummary?: OrderInventorySummary | null;
};

export type PartialOrderCreate = {
  orderId: string;
  order: OrderDetailDTO;
  inventoryDeduction?: Record<string, unknown>;
};

export function partialOrderCreateFromError(
  error: unknown,
): PartialOrderCreate | null {
  if (
    !(error instanceof ApiRequestError) ||
    !error.data ||
    typeof error.data !== "object"
  ) {
    return null;
  }
  const data = error.data as Partial<PartialOrderCreate>;
  if (
    typeof data.orderId !== "string" ||
    !data.order ||
    typeof data.order !== "object" ||
    data.order.id !== data.orderId
  ) {
    return null;
  }
  return data as PartialOrderCreate;
}

export type OrderListRow = {
  id: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  shopPlatform?: string;
  orderNo: string;
  customerName: string;
  status: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  currency: string;
  totalAmount: number;
  itemCount?: number;
  skuMatchStatus?: string;
  skuMatchedCount?: number;
  skuTotalCount?: number;
  inventoryDeductStatus?: string;
  syncStatus?: string;
  openExceptionCount?: number;
  detailUrl?: string;
  orderedAt?: string;
  createdAt: string;
  updatedAt?: string;
  latestShipmentStatus?: string;
  externalOrderId?: string;
  reconciliationStatus?: FulfillmentReconciliationStatus;
};

export type WarehouseAllocationBlock = {
  code: string;
  message: string;
};

export type WarehouseAllocationCandidateLine = {
  productSkuId: string;
  skuCode?: string;
  skuName?: string;
  required: number;
  available: number;
  shortage: number;
};

export type WarehouseAllocationRecommendationReason = {
  code: string;
  message: string;
};

export type WarehouseAllocationCandidate = {
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  isDefault: boolean;
  eligible: boolean;
  shortageCount: number;
  recommendationRank?: number;
  recommendationReasons?: WarehouseAllocationRecommendationReason[];
  revision: string;
  lines: WarehouseAllocationCandidateLine[];
};

export type WarehouseAllocationStatus = "allocated" | "allocatable" | "blocked";

export type WarehouseAllocation = {
  orderId: string;
  status: WarehouseAllocationStatus;
  warehouseId?: string;
  warehouseCode?: string;
  warehouseName?: string;
  recommendationPolicy?: string;
  recommendedWarehouseId?: string;
  recommendedWarehouseReasons?: WarehouseAllocationRecommendationReason[];
  candidateCount: number;
  eligibleCandidateCount: number;
  blocks: WarehouseAllocationBlock[];
  candidates: WarehouseAllocationCandidate[];
};

export type WarehouseAllocationListRow = OrderListRow & {
  allocationStatus: WarehouseAllocationStatus;
  warehouseId?: string;
  warehouseCode?: string;
  warehouseName?: string;
  recommendedWarehouseId?: string;
  recommendedWarehouseCode?: string;
  recommendedWarehouseName?: string;
  recommendedWarehouseReasons?: WarehouseAllocationRecommendationReason[];
  candidateCount: number;
  eligibleCandidateCount: number;
  blocks: WarehouseAllocationBlock[];
};

export type ConfirmWarehouseAllocationPayload = {
  warehouseId: string;
  expectedRevision: string;
  idempotencyKey: string;
};

export type ConfirmWarehouseAllocationResponse = {
  allocation: WarehouseAllocation;
  inventoryReserve?: Record<string, unknown>;
};

export type FulfillmentReconciliationStatus =
  | "matched"
  | "pending"
  | "mismatch"
  | "blocked";

export type FulfillmentActionSummary = {
  expected: number;
  actual: number;
};

export type FulfillmentTimelineEntry = {
  id: string;
  type: string;
  action: string;
  status?: string;
  quantity?: number;
  createdAt: string;
};

export type FulfillmentReconciliation = {
  orderId: string;
  orderNo: string;
  status: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  warehouseId?: string;
  reserve: FulfillmentActionSummary;
  deduct: FulfillmentActionSummary;
  release: FulfillmentActionSummary;
  restore: FulfillmentActionSummary;
  shipmentCount: number;
  effectCount: number;
  lastInventoryActionAt?: string;
  reconciliationStatus: FulfillmentReconciliationStatus;
  issues?: string[];
  timeline?: FulfillmentTimelineEntry[];
};

export async function queryOrders(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  shopId?: string;
  orderNo?: string;
  customerName?: string;
  keyword?: string;
  status?: string;
  paymentStatus?: string;
  fulfillmentStatus?: string;
  skuMatchStatus?: string;
  inventoryDeductStatus?: string;
  syncStatus?: string;
  reconciliationStatus?: FulfillmentReconciliationStatus;
  hasException?: boolean;
  start?: string;
  end?: string;
}): Promise<{
  list: OrderListRow[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
}> {
  const query: Record<string, string | number | undefined> = {
    ...params,
    hasException:
      params.hasException === undefined
        ? undefined
        : params.hasException
          ? 1
          : 0,
  };
  return getWithParams("/api/v1/orders", query);
}

export async function queryWarehouseAllocations(params: {
  page?: number;
  pageSize?: number;
  keyword?: string;
  assignment?: "all" | "allocated" | "unallocated";
}): Promise<{
  list: WarehouseAllocationListRow[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
}> {
  return getWithParams("/api/v1/orders/warehouse-allocations", params);
}

export async function getWarehouseAllocation(
  orderId: string,
): Promise<WarehouseAllocation> {
  return getJSON(
    `/api/v1/orders/${encodeURIComponent(orderId)}/warehouse-allocation`,
  );
}

export async function confirmWarehouseAllocation(
  orderId: string,
  payload: ConfirmWarehouseAllocationPayload,
): Promise<ConfirmWarehouseAllocationResponse> {
  return postJSON(
    `/api/v1/orders/${encodeURIComponent(orderId)}/warehouse-allocation`,
    payload,
  );
}

export function createOrderAllocationIdempotencyKey() {
  const random =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  return `admin-order-warehouse-allocation-${random}`.slice(0, 128);
}

export async function createOrder(
  payload: Record<string, unknown>,
): Promise<OrderDetailDTO> {
  return postJSON("/api/v1/orders", payload);
}

export async function getOrder(id: string): Promise<OrderDetailDTO> {
  return getJSON(`/api/v1/orders/${id}`);
}

export async function getOrderFulfillmentReconciliation(
  id: string,
): Promise<FulfillmentReconciliation> {
  return getJSON(`/api/v1/orders/${id}/fulfillment-reconciliation`);
}

export async function updateOrder(
  id: string,
  payload: Record<string, unknown>,
): Promise<OrderDetailDTO> {
  return putJSON(`/api/v1/orders/${id}`, payload);
}

export async function deleteOrder(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${id}`);
}

export async function createOrderItem(
  orderId: string,
  payload: Record<string, unknown>,
): Promise<OrderItemRow> {
  return postJSON(`/api/v1/orders/${orderId}/items`, payload);
}

export async function updateOrderItem(
  orderId: string,
  itemId: string,
  payload: Record<string, unknown>,
): Promise<OrderItemRow> {
  return putJSON(`/api/v1/orders/${orderId}/items/${itemId}`, payload);
}

export async function deleteOrderItem(
  orderId: string,
  itemId: string,
): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${orderId}/items/${itemId}`);
}

export async function createOrderShipment(
  orderId: string,
  payload: Record<string, unknown>,
): Promise<OrderShipmentRow> {
  return postJSON(`/api/v1/orders/${orderId}/shipments`, payload);
}

export async function updateOrderShipment(
  orderId: string,
  shipmentId: string,
  payload: Record<string, unknown>,
): Promise<OrderShipmentRow> {
  return putJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}`, payload);
}

export async function deleteOrderShipment(
  orderId: string,
  shipmentId: string,
): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}`);
}

export async function getOrderShipments(
  orderId: string,
): Promise<{ list: OrderShipmentRow[] }> {
  return getJSON(`/api/v1/orders/${orderId}/shipments`);
}

export async function getOrderShipmentEvents(
  orderId: string,
  shipmentId: string,
): Promise<OrderShipmentEventsResponse> {
  return getJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}/events`);
}

export type OrderShipmentEventPayload = {
  eventKey: string;
  status: string;
  occurredAt?: string;
  location?: string;
  description?: string;
  source?: string;
  rawData?: Record<string, unknown>;
};

export async function appendOrderShipmentEvent(
  orderId: string,
  shipmentId: string,
  payload: OrderShipmentEventPayload,
): Promise<{
  event: OrderShipmentEventRow;
  shipment: OrderShipmentRow;
  replay: boolean;
}> {
  return postJSON(
    `/api/v1/orders/${orderId}/shipments/${shipmentId}/events`,
    payload,
  );
}

export type FulfillOrderPayload = {
  idempotencyKey: string;
  warehouseId?: string;
  carrier: string;
  trackingNo: string;
  trackingUrl?: string;
};

export type FulfillOrderResponse = {
  order: OrderDetailDTO;
  shipment: OrderShipmentRow;
  inventoryDeduction?: Record<string, unknown> | null;
};

export async function fulfillOrder(
  orderId: string,
  payload: FulfillOrderPayload,
): Promise<FulfillOrderResponse> {
  return postJSON(`/api/v1/orders/${orderId}/fulfill`, payload);
}

export type BatchFulfillOrderItemPayload = {
  orderId: string;
  warehouseId?: string;
  carrier: string;
  trackingNo: string;
  trackingUrl?: string;
};

export type BatchFulfillmentItemResult = {
  orderId: string;
  orderNo: string;
  warehouseId?: string;
  status: "succeeded" | "blocked" | "in_progress" | "failed";
  shipment?: OrderShipmentRow;
  error?: string;
};

export type BatchFulfillmentResult = {
  batchIdempotencyKey: string;
  summary: {
    requested: number;
    succeeded: number;
    blocked: number;
    inProgress: number;
    failed: number;
  };
  items: BatchFulfillmentItemResult[];
  pickList: Array<{
    warehouseId: string;
    warehouseCode?: string;
    warehouseName?: string;
    productSkuId: string;
    skuCode?: string;
    skuName?: string;
    productTitle?: string;
    quantity: number;
    orderCount: number;
  }>;
};

export type BatchFulfillOrdersPayload = {
  batchIdempotencyKey: string;
  items: BatchFulfillOrderItemPayload[];
};

export async function batchFulfillOrders(
  payload: BatchFulfillOrdersPayload,
): Promise<BatchFulfillmentResult> {
  return postJSON("/api/v1/orders/fulfillment-batch", payload);
}

export function createOrderBatchFulfillmentIdempotencyKey() {
  const random =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  return `admin-order-fulfillment-batch-${random}`.slice(0, 128);
}

export async function deductOrderInventory(
  orderId: string,
  body?: { syncInventory?: boolean; warehouseId?: string },
): Promise<{
  order: OrderDetailDTO;
  inventoryDeduction: Record<string, unknown>;
}> {
  return postJSON(`/api/v1/orders/${orderId}/deduct-inventory`, body ?? {});
}

export async function restoreOrderInventory(
  orderId: string,
  body?: { syncInventory?: boolean; reason?: string; warehouseId?: string },
): Promise<{
  order: OrderDetailDTO;
  inventoryRestoration: Record<string, unknown>;
}> {
  return postJSON(`/api/v1/orders/${orderId}/restore-inventory`, body ?? {});
}

export async function getOrderInventoryEffects(
  orderId: string,
  params?: { page?: number; pageSize?: number },
): Promise<{
  list: OrderInventoryEffectRow[];
  pagination: PaginatedInventory<OrderInventoryEffectRow>["pagination"];
}> {
  return getWithParams(
    `/api/v1/orders/${orderId}/inventory-effects`,
    params ?? {},
  );
}

export type OrderSkuMatchRow = {
  id?: string;
  orderId?: string;
  orderItemId?: string;
  platform?: string;
  externalSkuId?: string;
  sellerSku?: string;
  skuCode?: string;
  matchStatus?: string;
  matchType?: string;
  confidence?: number;
  reason?: string;
  productId?: string;
  productSkuId?: string;
  productTitle?: string;
  localSkuCode?: string;
  externalOrderId?: string;
  candidateSkus?: Array<{
    productSkuId: string;
    productId: string;
    skuCode: string;
    skuName?: string;
    productTitle?: string;
  }>;
};

export async function getOrderSKUMatches(
  orderId: string,
): Promise<{ items: OrderSkuMatchRow[] }> {
  return getJSON(`/api/v1/orders/${orderId}/sku-matches`);
}

export async function matchOrderSKUs(
  orderId: string,
  body?: { overwrite?: boolean; force?: boolean },
): Promise<{ summary: Record<string, unknown> }> {
  return postJSON(`/api/v1/orders/${orderId}/match-skus`, body ?? {});
}

export async function bindOrderItemSku(
  itemId: string,
  body: {
    productSkuId: string;
    deductInventory?: boolean;
    syncInventory?: boolean;
    candidateConfidence?: number | null;
    candidateSource?: string;
  },
): Promise<{
  item: OrderItemRow;
  inventoryDeduction?: Record<string, unknown>;
}> {
  return postJSON(`/api/v1/order-items/${itemId}/bind-sku`, body);
}

export type OrderSkuMatchListRow = OrderSkuMatchRow & {
  shopName?: string;
  orderNo?: string;
  productTitle?: string;
  localSkuCode?: string;
};

export async function queryOrderSkuMatches(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  shopId?: string;
  matchStatus?: string;
  matchType?: string;
  orderId?: string;
  productSkuId?: string;
  start?: string;
  end?: string;
}): Promise<{
  list: OrderSkuMatchListRow[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
}> {
  return getWithParams("/api/v1/order-item-sku-matches", params);
}
