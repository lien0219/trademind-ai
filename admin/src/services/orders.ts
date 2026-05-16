import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';

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
  orderedAt?: string;
  createdAt: string;
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
  status?: string;
  fulfillmentStatus?: string;
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
