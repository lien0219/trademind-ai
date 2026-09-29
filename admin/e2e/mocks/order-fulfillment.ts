import { ok } from './envelope';

export const E2E_FULFILL_ORDER_ID = 'e2e-order-fulfillment-v1';
export const E2E_FULFILL_ORDER_ITEM_ID = 'e2e-order-fulfillment-item-blue';
export const E2E_FULFILL_SHIPMENT_ID = 'e2e-shipment-fulfillment-v1';

export const e2eFulfillmentShipment = {
  id: E2E_FULFILL_SHIPMENT_ID,
  orderId: E2E_FULFILL_ORDER_ID,
  carrier: 'E2E 物流',
  trackingNo: 'E2E-TRACK-0001',
  trackingUrl: 'https://tracking.example.test/E2E-TRACK-0001',
  status: 'in_transit',
  shippedAt: '2026-08-23T01:10:00Z',
  createdAt: '2026-08-23T01:10:00Z',
  updatedAt: '2026-08-23T02:00:00Z',
};

export const e2eFulfillmentTrackingEvents = [
  {
    id: 'e2e-shipment-event-in-transit',
    tenantId: 1,
    orderId: E2E_FULFILL_ORDER_ID,
    shipmentId: E2E_FULFILL_SHIPMENT_ID,
    eventKey: 'e2e-event-in-transit',
    status: 'in_transit',
    occurredAt: '2026-08-23T02:00:00Z',
    location: '深圳转运中心',
    description: '包裹已离开转运中心',
    source: 'local',
  },
  {
    id: 'e2e-shipment-event-shipped',
    tenantId: 1,
    orderId: E2E_FULFILL_ORDER_ID,
    shipmentId: E2E_FULFILL_SHIPMENT_ID,
    eventKey: 'e2e-event-shipped',
    status: 'shipped',
    occurredAt: '2026-08-23T01:10:00Z',
    location: 'E2E 仓库',
    description: '包裹已交承运商',
    source: 'local',
  },
];

export const e2eFulfillmentOrder = {
  id: E2E_FULFILL_ORDER_ID,
  tenantId: 1,
  platform: 'manual',
  warehouseId: 'e2e-warehouse-main',
  orderNo: 'SO-E2E-FULFILL-0001',
  customerName: 'E2E 履约客户',
  status: 'paid',
  paymentStatus: 'paid',
  fulfillmentStatus: 'unfulfilled',
  currency: 'CNY',
  totalAmount: 99.99,
  orderedAt: '2026-08-23T01:00:00Z',
  createdAt: '2026-08-23T01:00:00Z',
  updatedAt: '2026-08-23T01:00:00Z',
  items: [
    {
      id: E2E_FULFILL_ORDER_ITEM_ID,
      orderId: E2E_FULFILL_ORDER_ID,
      productId: 'e2e-product-blue',
      productSkuId: 'e2e-product-sku-blue',
      productTitle: 'E2E 履约商品',
      skuCode: 'BLUE-01',
      skuName: '蓝色',
      quantity: 1,
      unitPrice: 99.99,
      totalPrice: 99.99,
      createdAt: '2026-08-23T01:00:00Z',
      updatedAt: '2026-08-23T01:00:00Z',
    },
  ],
  shipments: [],
  inventorySummary: {
    hasReservationSuccess: true,
    hasReleaseSuccess: false,
    hasDeductionSuccess: false,
    hasRestoreSuccess: false,
    fullyRestored: false,
  },
};

export function fulfillmentOrderResponse(path: string) {
  if (path === `/api/v1/orders/${E2E_FULFILL_ORDER_ID}`)
    return ok(e2eFulfillmentOrder);
  if (path === `/api/v1/orders/${E2E_FULFILL_ORDER_ID}/shipments`)
    return ok({ list: [e2eFulfillmentShipment] });
  if (path === `/api/v1/orders/${E2E_FULFILL_ORDER_ID}/shipments/${E2E_FULFILL_SHIPMENT_ID}/events`)
    return ok({
      shipment: e2eFulfillmentShipment,
      events: e2eFulfillmentTrackingEvents,
      provider: 'local',
    });
  if (path === `/api/v1/orders/${E2E_FULFILL_ORDER_ID}/sku-matches`)
    return ok({
      items: [
        {
          orderItemId: E2E_FULFILL_ORDER_ITEM_ID,
          productSkuId: 'e2e-product-sku-blue',
          localSkuCode: 'BLUE-01',
          matchStatus: 'matched',
          confidence: 1,
        },
      ],
    });
  if (path === `/api/v1/orders/${E2E_FULFILL_ORDER_ID}/inventory-effects`)
    return ok({
      list: [],
      pagination: { page: 1, pageSize: 100, total: 0, totalPages: 0 },
    });
  return null;
}
