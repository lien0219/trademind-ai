import { ok } from './envelope';

export const E2E_WAREHOUSE_ID = 'e2e-warehouse-main';
export const E2E_SUPPLIER_ID = 'e2e-supplier-primary';
export const E2E_SUPPLIER_SKU_ID = 'e2e-supplier-sku-blue';
export const E2E_PRODUCT_SKU_ID = 'e2e-product-sku-blue';
export const E2E_PURCHASE_ORDER_ID = 'e2e-purchase-order-approved';
export const E2E_PURCHASE_ORDER_ITEM_ID = 'e2e-purchase-order-item-blue';
export const E2E_GOODS_RECEIPT_ITEM_ID = 'e2e-goods-receipt-item-blue';
export const E2E_PURCHASE_RETURN_ID = 'e2e-purchase-return-approved';

export const e2eWarehouse = {
  id: E2E_WAREHOUSE_ID,
  code: 'MAIN',
  name: 'E2E 华东主仓',
  status: 'active',
  isDefault: true,
  createdAt: '2026-08-20T01:00:00Z',
  updatedAt: '2026-08-20T01:00:00Z',
};

export const e2eSupplier = {
  id: E2E_SUPPLIER_ID,
  code: 'SUP-001',
  name: 'E2E 核心供应商',
  status: 'active',
  contactName: '测试联系人',
  phone: '138****5678',
  email: 'te***@example.test',
  createdAt: '2026-08-20T01:00:00Z',
  updatedAt: '2026-08-20T01:00:00Z',
};

export const e2eSupplierSKU = {
  id: E2E_SUPPLIER_SKU_ID,
  supplierId: E2E_SUPPLIER_ID,
  productSkuId: E2E_PRODUCT_SKU_ID,
  productTitle: 'E2E 蓝牙耳机',
  skuCode: 'BLUE-01',
  skuName: '蓝色',
  supplierSkuCode: 'VENDOR-BLUE',
  unitCostMinor: 2599,
  currency: 'CNY',
  minOrderQty: 2,
  leadTimeDays: 7,
};

export const e2ePurchaseOrder = {
  id: E2E_PURCHASE_ORDER_ID,
  purchaseOrderNo: 'PO-E2E-0001',
  supplierId: E2E_SUPPLIER_ID,
  warehouseId: E2E_WAREHOUSE_ID,
  status: 'approved',
  currency: 'CNY',
  totalAmountMinor: 12995,
  revision: 3,
  remark: 'E2E 分批收货测试',
  approvedAt: '2026-08-20T02:00:00Z',
  createdAt: '2026-08-20T01:00:00Z',
  updatedAt: '2026-08-20T02:00:00Z',
  items: [
    {
      id: E2E_PURCHASE_ORDER_ITEM_ID,
      purchaseOrderId: E2E_PURCHASE_ORDER_ID,
      productSkuId: E2E_PRODUCT_SKU_ID,
      supplierSkuId: E2E_SUPPLIER_SKU_ID,
      productTitle: 'E2E 蓝牙耳机',
      skuCode: 'BLUE-01',
      skuName: '蓝色',
      quantity: 5,
      receivedQuantity: 0,
      unitCostMinor: 2599,
      lineAmountMinor: 12995,
    },
  ],
};

export const e2eReceivedPurchaseOrder = {
  ...e2ePurchaseOrder,
  status: 'received',
  revision: 4,
  items: [{ ...e2ePurchaseOrder.items[0], receivedQuantity: 5 }],
};

export const e2eReturnableReceiptItem = {
  goodsReceiptItemId: E2E_GOODS_RECEIPT_ITEM_ID,
  goodsReceiptId: 'e2e-goods-receipt-one',
  receiptNo: 'GR-E2E-0001',
  purchaseOrderItemId: E2E_PURCHASE_ORDER_ITEM_ID,
  productSkuId: E2E_PRODUCT_SKU_ID,
  productTitle: 'E2E 蓝牙耳机',
  skuCode: 'BLUE-01',
  skuName: '蓝色',
  receivedQuantity: 5,
  allocatedReturnQuantity: 1,
  remainingQuantity: 4,
};

export const e2ePurchaseReturn = {
  id: E2E_PURCHASE_RETURN_ID,
  returnNo: 'PR-E2E-0001',
  purchaseOrderId: E2E_PURCHASE_ORDER_ID,
  purchaseOrderNo: 'PO-E2E-0001',
  supplierId: E2E_SUPPLIER_ID,
  supplierName: 'E2E 核心供应商',
  warehouseId: E2E_WAREHOUSE_ID,
  warehouseName: 'E2E 华东主仓',
  status: 'approved',
  revision: 3,
  reason: '到货质量异常',
  remark: 'E2E 采购退货',
  approvedBy: 'e2e-reviewer',
  approvedAt: '2026-08-22T02:00:00Z',
  itemCount: 1,
  createdAt: '2026-08-22T01:00:00Z',
  updatedAt: '2026-08-22T02:00:00Z',
  items: [{
    id: 'e2e-purchase-return-item-blue',
    purchaseReturnId: E2E_PURCHASE_RETURN_ID,
    goodsReceiptItemId: E2E_GOODS_RECEIPT_ITEM_ID,
    purchaseOrderItemId: E2E_PURCHASE_ORDER_ITEM_ID,
    productSkuId: E2E_PRODUCT_SKU_ID,
    quantity: 1,
    receiptNo: 'GR-E2E-0001',
    receiptQuantity: 5,
    productTitle: 'E2E 蓝牙耳机',
    skuCode: 'BLUE-01',
    skuName: '蓝色',
  }],
};

export const e2eProductSkuHit = {
  productId: 'e2e-product-blue',
  productTitle: 'E2E 蓝牙耳机',
  productSkuId: E2E_PRODUCT_SKU_ID,
  skuCode: 'BLUE-01',
  skuName: '蓝色',
  stock: 10,
};

export function procurementResponse(path: string) {
  if (path === '/api/v1/warehouses') return ok({ list: [e2eWarehouse] });
  if (path === '/api/v1/suppliers') return ok({ list: [e2eSupplier] });
  if (path === `/api/v1/suppliers/${E2E_SUPPLIER_ID}/skus`) return ok({ list: [e2eSupplierSKU] });
  if (path === '/api/v1/product-skus/search') return ok({ list: [e2eProductSkuHit] });
  if (path === '/api/v1/purchase-orders') {
    return ok({ list: [{ ...e2ePurchaseOrder, items: undefined }], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === '/api/v1/procurement/replenishment-suggestions') {
    return ok({
      warehouseId: E2E_WAREHOUSE_ID,
      warehouseCode: e2eWarehouse.code,
      warehouseName: e2eWarehouse.name,
      list: [{
        warehouseId: E2E_WAREHOUSE_ID,
        warehouseCode: e2eWarehouse.code,
        warehouseName: e2eWarehouse.name,
        productId: 'e2e-product-blue',
        productTitle: 'E2E 补货耳机',
        productSkuId: E2E_PRODUCT_SKU_ID,
        skuCode: 'BLUE-01',
        skuName: '蓝色',
        availableStock: 2,
        inTransitTransfer: 1,
        pendingPurchase: 0,
        warningStock: 10,
        safetyStock: 4,
        deficit: 7,
        suggestedQuantity: 8,
        minOrderQty: 4,
        unitCostMinor: 2599,
        currency: 'CNY',
        leadTimeDays: 7,
        supplierId: E2E_SUPPLIER_ID,
        supplierName: e2eSupplier.name,
        status: 'actionable',
        inventoryOnHandTotal: 2,
        inventoryBalanceCount: 1,
      }],
      page: 1,
      pageSize: 20,
      total: 1,
      totalPages: 1,
    });
  }
  if (path === `/api/v1/purchase-orders/${E2E_PURCHASE_ORDER_ID}`) return ok(e2ePurchaseOrder);
  if (path === `/api/v1/purchase-orders/${E2E_PURCHASE_ORDER_ID}/returnable-receipt-items`) return ok({ list: [e2eReturnableReceiptItem] });
  if (path === '/api/v1/purchase-returns') {
    return ok({ list: [{ ...e2ePurchaseReturn, items: undefined }], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === `/api/v1/purchase-returns/${E2E_PURCHASE_RETURN_ID}`) return ok(e2ePurchaseReturn);
  return null;
}
