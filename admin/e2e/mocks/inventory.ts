import { ok } from './envelope';
import { E2E_PRODUCT_ID } from './product.fixture';

export function inventoryResponse(path: string) {
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publication-skus`) return ok({ list: [] });
  if (path.includes('/inventory-logs')) return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory') return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory/alerts') return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory/warehouse-transfers') return ok({ list: [{ id: 'e2e-transfer-1', transferNo: 'TRF-E2E-0001', sourceWarehouseId: 'e2e-warehouse-main', targetWarehouseId: 'e2e-warehouse-east', sourceWarehouseCode: 'MAIN', sourceWarehouseName: 'E2E 华东主仓', targetWarehouseCode: 'EAST', targetWarehouseName: 'E2E 华东备仓', status: 'approved', revision: 3, idempotencyKey: 'e2e-transfer-key', itemCount: 1, createdAt: '2026-08-22T01:00:00Z', updatedAt: '2026-08-22T01:00:00Z' }], total: 1, page: 1, pageSize: 20, totalPages: 1 });
  if (path === '/api/v1/inventory/warehouse-transfers/e2e-transfer-1') return ok({ id: 'e2e-transfer-1', transferNo: 'TRF-E2E-0001', sourceWarehouseId: 'e2e-warehouse-main', targetWarehouseId: 'e2e-warehouse-east', sourceWarehouseName: 'E2E 华东主仓', targetWarehouseName: 'E2E 华东备仓', status: 'approved', revision: 3, idempotencyKey: 'e2e-transfer-key', items: [{ id: 'e2e-transfer-item-1', productId: E2E_PRODUCT_ID, productSkuId: 'e2e-product-sku-blue', quantity: 4, receivedQuantity: 0, skuCode: 'BLUE-01' }], createdAt: '2026-08-22T01:00:00Z', updatedAt: '2026-08-22T01:00:00Z' });
  if (path === '/api/v1/inventory/stocktakes') return ok({ list: [{ id: 'e2e-stocktake-1', stocktakeNo: 'STK-E2E-0001', warehouseId: 'e2e-warehouse-main', warehouseCode: 'MAIN', warehouseName: 'E2E 华东主仓', status: 'counting', revision: 1, idempotencyKey: 'e2e-stocktake-key', itemCount: 1, createdAt: '2026-08-22T02:00:00Z', updatedAt: '2026-08-22T02:00:00Z' }], total: 1, page: 1, pageSize: 20, totalPages: 1 });
  if (path === '/api/v1/inventory/stocktakes/e2e-stocktake-1') return ok({ id: 'e2e-stocktake-1', stocktakeNo: 'STK-E2E-0001', warehouseId: 'e2e-warehouse-main', warehouseCode: 'MAIN', warehouseName: 'E2E 华东主仓', status: 'counting', revision: 1, idempotencyKey: 'e2e-stocktake-key', items: [{ id: 'e2e-stocktake-item-1', productId: E2E_PRODUCT_ID, productSkuId: 'e2e-product-sku-blue', snapshotOnHand: 10, snapshotReserved: 1, snapshotInTransit: 0, snapshotDamaged: 0, snapshotVersion: 1, skuCode: 'BLUE-01' }], createdAt: '2026-08-22T02:00:00Z', updatedAt: '2026-08-22T02:00:00Z' });
  if (path.endsWith('/warehouse-balances')) {
    return ok({
      list: [
        {
          warehouseId: 'e2e-warehouse-main',
          warehouseCode: 'MAIN',
          warehouseName: 'E2E 华东主仓',
          isDefault: true,
          onHand: 10,
          reserved: 0,
          inTransit: 0,
          damaged: 0,
          available: 10,
          version: 1,
        },
      ],
    });
  }
  if (path === '/api/v1/inventory/warehouse-ledger/reconciliation') {
    return ok({
      list: [
        {
          productId: E2E_PRODUCT_ID,
          productTitle: 'E2E 库存账商品',
          productSkuId: 'e2e-product-sku-blue',
          skuCode: 'BLUE-01',
          skuName: '蓝色',
          aggregateStock: 10,
          warehouseOnHand: 10,
          difference: 0,
          balanceCount: 1,
          status: 'matched',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
      totalPages: 1,
      matched: 1,
      unmigrated: 0,
      mismatch: 0,
    });
  }
  return null;
}
