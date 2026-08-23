import { ok } from './envelope';

export const E2E_FULFILL_ORDER_ID = 'e2e-order-fulfillment-v1';
export const E2E_FULFILL_ORDER_ITEM_ID = 'e2e-order-fulfillment-item-blue';

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
