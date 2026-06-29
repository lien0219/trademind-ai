import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';
import type { OrderInventoryEffectRow, PaginatedInventory } from '@/services/inventory';

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
  hasException?: boolean;
  start?: string;
  end?: string;
}): Promise<{
  list: OrderListRow[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/orders', params);
}

export async function createOrder(payload: Record<string, unknown>): Promise<OrderDetailDTO> {
  return postJSON('/api/v1/orders', payload);
}

export async function getOrder(id: string): Promise<OrderDetailDTO> {
  return getJSON(`/api/v1/orders/${id}`);
}

export async function updateOrder(id: string, payload: Record<string, unknown>): Promise<OrderDetailDTO> {
  return putJSON(`/api/v1/orders/${id}`, payload);
}

export async function deleteOrder(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${id}`);
}

export async function createOrderItem(orderId: string, payload: Record<string, unknown>): Promise<OrderItemRow> {
  return postJSON(`/api/v1/orders/${orderId}/items`, payload);
}

export async function updateOrderItem(
  orderId: string,
  itemId: string,
  payload: Record<string, unknown>,
): Promise<OrderItemRow> {
  return putJSON(`/api/v1/orders/${orderId}/items/${itemId}`, payload);
}

export async function deleteOrderItem(orderId: string, itemId: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${orderId}/items/${itemId}`);
}

export async function createOrderShipment(orderId: string, payload: Record<string, unknown>): Promise<OrderShipmentRow> {
  return postJSON(`/api/v1/orders/${orderId}/shipments`, payload);
}

export async function updateOrderShipment(
  orderId: string,
  shipmentId: string,
  payload: Record<string, unknown>,
): Promise<OrderShipmentRow> {
  return putJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}`, payload);
}

export async function deleteOrderShipment(orderId: string, shipmentId: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/orders/${orderId}/shipments/${shipmentId}`);
}

export async function deductOrderInventory(
  orderId: string,
  body?: { syncInventory?: boolean },
): Promise<{ order: OrderDetailDTO; inventoryDeduction: Record<string, unknown> }> {
  return postJSON(`/api/v1/orders/${orderId}/deduct-inventory`, body ?? {});
}

export async function restoreOrderInventory(
  orderId: string,
  body?: { syncInventory?: boolean; reason?: string },
): Promise<{ order: OrderDetailDTO; inventoryRestoration: Record<string, unknown> }> {
  return postJSON(`/api/v1/orders/${orderId}/restore-inventory`, body ?? {});
}

export async function getOrderInventoryEffects(
  orderId: string,
  params?: { page?: number; pageSize?: number },
): Promise<{ list: OrderInventoryEffectRow[]; pagination: PaginatedInventory<OrderInventoryEffectRow>['pagination'] }> {
  return getWithParams(`/api/v1/orders/${orderId}/inventory-effects`, params ?? {});
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

export async function getOrderSKUMatches(orderId: string): Promise<{ items: OrderSkuMatchRow[] }> {
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
): Promise<{ item: OrderItemRow; inventoryDeduction?: Record<string, unknown> }> {
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
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/order-item-sku-matches', params);
}
