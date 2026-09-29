import { ok } from './envelope';

export const E2E_WAREHOUSE_FEE_RATE_ID = 'e2e-warehouse-fee-rate-1';
export const E2E_WAREHOUSE_FEE_SNAPSHOT_ID = 'e2e-warehouse-fee-snapshot-1';
export const E2E_WAREHOUSE_FEE_ORDER_ID = 'e2e-order-profit-1';

export const e2eWarehouseFeeRate = {
  id: E2E_WAREHOUSE_FEE_RATE_ID,
  warehouseId: 'e2e-warehouse-main',
  warehouseCode: 'WH-EAST',
  warehouseName: 'E2E 华东主仓',
  code: 'WH-EAST-STD',
  name: '华东仓标准操作费',
  currency: 'CNY',
  outboundBaseFeeMinor: 100,
  pickingFeePerItemMinor: 20,
  packingFeePerPackageMinor: 30,
  status: 'active',
  revision: 2,
  createdAt: '2026-09-07T01:00:00Z',
  updatedAt: '2026-09-08T01:00:00Z',
};

export const e2eWarehouseFeeCandidate = {
  orderId: E2E_WAREHOUSE_FEE_ORDER_ID,
  orderNo: 'SO-E2E-PROFIT-0001',
  shopId: 'e2e-shop-douyin',
  shopName: 'E2E 抖店旗舰店',
  platform: 'douyin_shop',
  currency: 'CNY',
  warehouseId: 'e2e-warehouse-main',
  warehouseCode: 'WH-EAST',
  warehouseName: 'E2E 华东主仓',
  waveId: 'e2e-wave-profit-1',
  waveNo: 'FW-E2E-0001',
  waveRevision: 8,
  waveStatus: 'completed',
  waveOrderId: 'e2e-wave-order-profit-1',
  waveOrderStatus: 'fulfilled',
  shipmentId: 'e2e-shipment-profit-1',
  packVerificationId: 'e2e-pack-verification-profit-1',
  packageCode: 'PKG-E2E-0001',
  itemQuantity: 2,
  packageQuantity: 1,
  fulfilledAt: '2026-09-08T01:00:00Z',
  verifiedAt: '2026-09-08T00:55:00Z',
  confirmed: false,
};

export const e2eWarehouseFeePreview = {
  orderId: E2E_WAREHOUSE_FEE_ORDER_ID,
  orderNo: e2eWarehouseFeeCandidate.orderNo,
  shopId: e2eWarehouseFeeCandidate.shopId,
  warehouseId: e2eWarehouseFeeCandidate.warehouseId,
  warehouseCode: e2eWarehouseFeeCandidate.warehouseCode,
  warehouseName: e2eWarehouseFeeCandidate.warehouseName,
  waveId: e2eWarehouseFeeCandidate.waveId,
  waveNo: e2eWarehouseFeeCandidate.waveNo,
  waveRevision: e2eWarehouseFeeCandidate.waveRevision,
  waveOrderId: e2eWarehouseFeeCandidate.waveOrderId,
  packVerificationId: e2eWarehouseFeeCandidate.packVerificationId,
  packageCode: e2eWarehouseFeeCandidate.packageCode,
  itemQuantity: 2,
  packageQuantity: 1,
  rateCardId: E2E_WAREHOUSE_FEE_RATE_ID,
  rateCardCode: e2eWarehouseFeeRate.code,
  rateCardName: e2eWarehouseFeeRate.name,
  rateCardRevision: 2,
  outboundBaseFeeMinor: 100,
  pickingFeePerItemMinor: 20,
  pickingFeeMinor: 40,
  packingFeePerPackageMinor: 30,
  packingFeeMinor: 30,
  amountMinor: 170,
  currency: 'CNY',
  calculationHash: 'a'.repeat(64),
};

export const e2eWarehouseFeeSnapshot = {
  ...e2eWarehouseFeePreview,
  id: E2E_WAREHOUSE_FEE_SNAPSHOT_ID,
  idempotencyKey: 'e2e-confirm-key',
  confirmedAt: '2026-09-08T01:05:00Z',
  adjustmentCount: 1,
  adjustmentMinor: 20,
  netAmountMinor: 190,
};

export const e2eWarehouseFeeDetail = {
  ...e2eWarehouseFeeSnapshot,
  adjustments: [
    {
      id: 'e2e-warehouse-fee-adjustment-1',
      snapshotId: E2E_WAREHOUSE_FEE_SNAPSHOT_ID,
      orderId: E2E_WAREHOUSE_FEE_ORDER_ID,
      factType: 'adjustment',
      amountMinor: 20,
      currency: 'CNY',
      reason: '加固包装材料补收',
      createdAt: '2026-09-08T01:10:00Z',
    },
  ],
};

export function warehouseFeeResponse(path: string) {
  if (path === '/api/v1/warehouse-fee-rate-cards') return ok({ list: [e2eWarehouseFeeRate] });
  if (path === `/api/v1/warehouse-fee-rate-cards/${E2E_WAREHOUSE_FEE_RATE_ID}`) {
    return ok({
      ...e2eWarehouseFeeRate,
      revisions: [
        { ...e2eWarehouseFeeRate, rateCardId: E2E_WAREHOUSE_FEE_RATE_ID },
        { ...e2eWarehouseFeeRate, id: 'e2e-rate-revision-1', rateCardId: E2E_WAREHOUSE_FEE_RATE_ID, revision: 1, outboundBaseFeeMinor: 90, createdAt: '2026-09-07T01:00:00Z' },
      ],
    });
  }
  if (path === '/api/v1/warehouse-operation-fees/candidates') {
    return ok({ list: [e2eWarehouseFeeCandidate], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === '/api/v1/warehouse-operation-fees') {
    return ok({ list: [e2eWarehouseFeeSnapshot], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === `/api/v1/warehouse-operation-fees/${E2E_WAREHOUSE_FEE_SNAPSHOT_ID}`) {
    return ok(e2eWarehouseFeeDetail);
  }
  return null;
}
