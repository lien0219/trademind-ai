import { ok } from './envelope';

export const E2E_ORDER_PROFIT_ORDER_ID = 'e2e-order-profit-1';

const available = (amountMinor: number, source: string, sourceAt: string) => ({
  amountMinor,
  knownAmountMinor: amountMinor,
  currency: 'CNY',
  status: 'available',
  source,
  sourceAt,
});

const missing = (source: string, reasonCode: string, reason: string) => ({
  amountMinor: null,
  knownAmountMinor: 0,
  currency: 'CNY',
  status: 'missing',
  source,
  reasonCode,
  reason,
});

export const e2eOrderProfit = {
  orderId: E2E_ORDER_PROFIT_ORDER_ID,
  orderNo: 'SO-E2E-PROFIT-0001',
  platform: 'douyin_shop',
  shopId: 'e2e-shop-douyin',
  shopName: 'E2E 抖店旗舰店',
  warehouseId: 'e2e-warehouse-main',
  warehouseCode: 'WH-EAST',
  warehouseName: 'E2E 华东主仓',
  currency: 'CNY',
  status: 'pending',
  orderStatus: 'shipped',
  paymentStatus: 'paid',
  fulfillmentStatus: 'fulfilled',
  knownContributionMinor: 4500,
  estimatedProfitMinor: null,
  estimatedMarginBps: null,
  components: {
    revenue: available(10000, 'order_total', '2026-09-05T02:00:00Z'),
    productCost: available(4000, 'current_supplier_catalog', '2026-09-04T02:00:00Z'),
    freight: available(1000, 'confirmed_local_freight_quote', '2026-09-05T03:00:00Z'),
    refund: available(500, 'refund_execution_ledger', '2026-09-05T04:00:00Z'),
    platformFee: missing('platform_settlement', 'platform_fee_missing', '缺少平台结算费用'),
    advertisingFee: missing('advertising_ledger', 'advertising_fee_missing', '缺少订单归属广告费用'),
    warehouseFee: missing('warehouse_fee_ledger', 'warehouse_fee_missing', '缺少仓储作业费用'),
  },
  issues: [
    { code: 'platform_fee_missing', component: 'platformFee', message: '缺少平台结算费用' },
    { code: 'advertising_fee_missing', component: 'advertisingFee', message: '缺少订单归属广告费用' },
    { code: 'warehouse_fee_missing', component: 'warehouseFee', message: '缺少仓储作业费用' },
  ],
  related: {
    fulfillmentWaveId: 'e2e-wave-profit-1',
    supplierIds: ['e2e-supplier-primary'],
    refundExecutionIds: ['e2e-refund-profit-1'],
  },
  orderedAt: '2026-09-05T01:00:00Z',
  calculatedAt: '2026-09-06T01:00:00Z',
  formulaVersion: 'order_profit_estimate_v1',
};

export const e2eOrderProfitDetail = {
  ...e2eOrderProfit,
  productCostLines: [
    {
      orderItemId: 'e2e-order-profit-item-1',
      productSkuId: 'e2e-product-sku-blue',
      productTitle: 'E2E 蓝牙耳机',
      skuCode: 'BLUE-01',
      quantity: 2,
      unitCostMinor: 2000,
      lineCostMinor: 4000,
      currency: 'CNY',
      status: 'available',
      source: 'current_supplier_catalog',
      sourceAt: '2026-09-04T02:00:00Z',
      supplierId: 'e2e-supplier-primary',
      supplierName: 'E2E 核心供应商',
    },
  ],
};

export function orderProfitResponse(path: string) {
  if (path === '/api/v1/order-profits') {
    return ok({
      list: [e2eOrderProfit],
      page: 1,
      pageSize: 20,
      total: 1,
      totalPages: 1,
      calculatedAt: e2eOrderProfit.calculatedAt,
      formula: {
        version: 'order_profit_estimate_v1',
        revenueSource: 'orders.total_amount',
        productCostSource: 'current supplier catalog',
        freightSource: 'confirmed local quote',
        refundSource: 'succeeded refund execution',
        missingFeeBehavior: 'missing fees remain null',
      },
    });
  }
  if (path === `/api/v1/order-profits/${E2E_ORDER_PROFIT_ORDER_ID}`) {
    return ok(e2eOrderProfitDetail);
  }
  return null;
}
