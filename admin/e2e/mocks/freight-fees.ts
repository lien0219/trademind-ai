import { E2E_SHOP_ID } from './product.fixture';
import { ok } from './envelope';

export const E2E_FREIGHT_FEE_IMPORT_ID = 'e2e-freight-fee-import-1';
export const E2E_FREIGHT_FEE_CHARGE_ID = 'e2e-freight-fee-charge-1';
export const E2E_FREIGHT_FEE_ADJUSTMENT_ID = 'e2e-freight-fee-adjustment-1';

export const e2eFreightFeeCharge = {
  id: E2E_FREIGHT_FEE_CHARGE_ID,
  importId: E2E_FREIGHT_FEE_IMPORT_ID,
  shopId: E2E_SHOP_ID,
  orderId: 'e2e-order-profit-1',
  shipmentId: 'e2e-shipment-profit-1',
  orderNo: 'SO-E2E-PROFIT-0001',
  orderCurrency: 'CNY',
  carrier: 'Carrier A',
  trackingNo: 'TRACK-E2E-FREIGHT-1',
  externalLineId: 'FREIGHT-E2E-0001',
  billedAt: '2026-09-28T01:00:00Z',
  amountMinor: 500,
  currency: 'CNY',
  importedAt: '2026-09-28T01:05:00Z',
  adjustmentCount: 1,
  adjustmentMinor: 25,
  netAmountMinor: 525,
  status: 'confirmed' as const,
};

export const e2eFreightFeeDetail = {
  ...e2eFreightFeeCharge,
  adjustments: [
    {
      id: E2E_FREIGHT_FEE_ADJUSTMENT_ID,
      chargeId: E2E_FREIGHT_FEE_CHARGE_ID,
      factType: 'adjustment' as const,
      amountMinor: 25,
      currency: 'CNY',
      reason: 'E2E 包装附加费',
      createdAt: '2026-09-28T01:10:00Z',
    },
  ],
};

export const e2eFreightFeePreview = {
  fileName: 'freight.csv',
  fileHash: 'a'.repeat(64),
  calculationHash: 'b'.repeat(64),
  policyVersion: 'carrier_shipment_final_bill_v1',
  shopId: E2E_SHOP_ID,
  shopName: 'E2E 抖店测试店铺',
  valid: true,
  sourceRows: 1,
  newCharges: 1,
  duplicateCharges: 0,
  issues: [],
  warnings: [],
  rows: [
    {
      line: 2,
      externalLineId: 'FREIGHT-E2E-0001',
      carrier: 'Carrier A',
      trackingNo: 'TRACK-E2E-FREIGHT-1',
      amountMinor: 500,
      currency: 'CNY',
      billedAt: '2026-09-28T01:00:00Z',
      disposition: 'new',
      orderId: e2eFreightFeeCharge.orderId,
      shipmentId: e2eFreightFeeCharge.shipmentId,
      orderNo: e2eFreightFeeCharge.orderNo,
      orderCurrency: 'CNY',
      status: 'confirmed',
    },
  ],
};

export function freightFeeResponse(path: string) {
  if (path === '/api/v1/freight-fee-imports') {
    return ok({ list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 });
  }
  if (path === '/api/v1/freight-fees') {
    return ok({ list: [e2eFreightFeeCharge], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === `/api/v1/freight-fees/${E2E_FREIGHT_FEE_CHARGE_ID}`) {
    return ok(e2eFreightFeeDetail);
  }
  return null;
}
