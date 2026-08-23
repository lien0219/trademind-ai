import { ok } from './envelope';

export const E2E_SALES_ORDER_ID = 'e2e-sales-order-returnable';
export const E2E_SALES_ORDER_ITEM_ID = 'e2e-sales-order-item-blue';
export const E2E_SALES_RETURN_ID = 'e2e-sales-return-approved';
export const E2E_PLATFORM_AFTER_SALE_ID = 'e2e-platform-after-sale-1';

export const e2eSalesOrder = {
  id: E2E_SALES_ORDER_ID,
  tenantId: 1,
  platform: 'manual',
  warehouseId: 'e2e-warehouse-main',
  orderNo: 'SO-E2E-0001',
  customerName: 'E2E 买家',
  status: 'delivered',
  paymentStatus: 'paid',
  fulfillmentStatus: 'fulfilled',
  currency: 'CNY',
  totalAmount: 199.98,
  orderedAt: '2026-08-21T01:00:00Z',
  createdAt: '2026-08-21T01:00:00Z',
  updatedAt: '2026-08-21T03:00:00Z',
  items: [
    {
      id: E2E_SALES_ORDER_ITEM_ID,
      orderId: E2E_SALES_ORDER_ID,
      productId: 'e2e-product-blue',
      productSkuId: 'e2e-product-sku-blue',
      productTitle: 'E2E 售后蓝牙耳机',
      skuCode: 'BLUE-01',
      skuName: '蓝色',
      quantity: 2,
      unitPrice: 99.99,
      totalPrice: 199.98,
      createdAt: '2026-08-21T01:00:00Z',
      updatedAt: '2026-08-21T01:00:00Z',
    },
  ],
  shipments: [],
  inventorySummary: {
    hasReservationSuccess: true,
    hasReleaseSuccess: false,
    hasDeductionSuccess: true,
    hasRestoreSuccess: false,
    fullyRestored: false,
  },
};

export const e2eReturnableSalesItem = {
  orderItemId: E2E_SALES_ORDER_ITEM_ID,
  productSkuId: 'e2e-product-sku-blue',
  productTitle: 'E2E 售后蓝牙耳机',
  skuCode: 'BLUE-01',
  skuName: '蓝色',
  unitPrice: 99.99,
  deductedQuantity: 2,
  restoredQuantity: 0,
  allocatedReturnQuantity: 1,
  remainingQuantity: 1,
};

export const e2eSalesReturn = {
  id: E2E_SALES_RETURN_ID,
  returnNo: 'SR-E2E-0001',
  orderId: E2E_SALES_ORDER_ID,
  orderNo: 'SO-E2E-0001',
  warehouseId: 'e2e-warehouse-main',
  warehouseName: 'E2E 华东主仓',
  type: 'return_refund',
  status: 'approved',
  currency: 'CNY',
  refundAmountMinor: 9999,
  revision: 3,
  reason: '客户七天无理由退货',
  remark: 'E2E 销售售后',
  approvedBy: 'e2e-reviewer',
  approvedAt: '2026-08-22T02:00:00Z',
  itemCount: 1,
  createdAt: '2026-08-22T01:00:00Z',
  updatedAt: '2026-08-22T02:00:00Z',
  items: [
    {
      id: 'e2e-sales-return-item-blue',
      salesReturnId: E2E_SALES_RETURN_ID,
      orderItemId: E2E_SALES_ORDER_ITEM_ID,
      productSkuId: 'e2e-product-sku-blue',
      quantity: 1,
      disposition: 'sellable',
      refundAmountMinor: 9999,
      productTitle: 'E2E 售后蓝牙耳机',
      skuCode: 'BLUE-01',
      skuName: '蓝色',
      deductedQuantity: 2,
    },
  ],
};

export const e2ePlatformAfterSale = {
  id: E2E_PLATFORM_AFTER_SALE_ID,
  platform: 'douyin_shop',
  platformShopId: 'e2e-douyin-shop',
  externalAfterSaleId: 'e2e-after-sale-1',
  externalOrderId: 'e2e-platform-order-1',
  platformType: 'refund_only',
  platformStatus: 'success',
  refundAmountMinor: 9999,
  currency: 'CNY',
  platformUpdatedAt: '2026-08-22T03:00:00Z',
  orderId: E2E_SALES_ORDER_ID,
  orderNo: e2eSalesOrder.orderNo,
  salesReturnId: E2E_SALES_RETURN_ID,
  returnNo: e2eSalesReturn.returnNo,
  reconciliationStatus: 'matched',
  reconciliationReason: 'matched_order_and_sales_return',
  lastEventId: 'e2e-after-sale-event-1',
  events: [
    {
      id: 'e2e-platform-after-sale-event-row-1',
      eventId: 'e2e-after-sale-event-1',
      eventType: 'refund_success',
      platformAfterSaleId: E2E_PLATFORM_AFTER_SALE_ID,
      applied: true,
      createdAt: '2026-08-22T03:00:00Z',
    },
  ],
  createdAt: '2026-08-22T02:00:00Z',
  updatedAt: '2026-08-22T03:00:00Z',
};

export function salesReturnResponse(path: string) {
  if (path === `/api/v1/orders/${E2E_SALES_ORDER_ID}`) return ok(e2eSalesOrder);
  if (path === `/api/v1/orders/${E2E_SALES_ORDER_ID}/sku-matches`)
    return ok({ items: [] });
  if (path === `/api/v1/orders/${E2E_SALES_ORDER_ID}/inventory-effects`)
    return ok({
      list: [],
      pagination: { page: 1, pageSize: 100, total: 0, totalPages: 0 },
    });
  if (path === `/api/v1/orders/${E2E_SALES_ORDER_ID}/sales-returnable-items`) {
    return ok({
      orderId: E2E_SALES_ORDER_ID,
      orderNo: e2eSalesOrder.orderNo,
      warehouseId: e2eSalesOrder.warehouseId,
      currency: e2eSalesOrder.currency,
      list: [e2eReturnableSalesItem],
    });
  }
  if (path === '/api/v1/sales-returns') {
    return ok({
      list: [{ ...e2eSalesReturn, items: undefined }],
      page: 1,
      pageSize: 20,
      total: 1,
      totalPages: 1,
    });
  }
  if (path === `/api/v1/sales-returns/${E2E_SALES_RETURN_ID}`)
    return ok(e2eSalesReturn);
  if (path === '/api/v1/sales-return-reconciliation')
    return ok({ list: [e2ePlatformAfterSale], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  if (path === `/api/v1/sales-return-reconciliation/${E2E_PLATFORM_AFTER_SALE_ID}`)
    return ok(e2ePlatformAfterSale);
  return null;
}
