import { ok } from './envelope';

export const E2E_WAREHOUSE_ID = 'e2e-warehouse-main';
export const E2E_SUPPLIER_ID = 'e2e-supplier-primary';
export const E2E_SUPPLIER_SKU_ID = 'e2e-supplier-sku-blue';
export const E2E_PRODUCT_SKU_ID = 'e2e-product-sku-blue';
export const E2E_PURCHASE_ORDER_ID = 'e2e-purchase-order-approved';
export const E2E_PURCHASE_ORDER_ITEM_ID = 'e2e-purchase-order-item-blue';

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
  if (path === `/api/v1/purchase-orders/${E2E_PURCHASE_ORDER_ID}`) return ok(e2ePurchaseOrder);
  return null;
}
