import { ok } from './envelope';
import { E2E_PRODUCT_ID } from './product.fixture';

export function inventoryResponse(path: string) {
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publication-skus`) return ok({ list: [] });
  if (path.includes('/inventory-logs')) return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory') return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory/alerts') return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
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
