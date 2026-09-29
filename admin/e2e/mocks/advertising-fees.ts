import { ok } from './envelope';

export const E2E_ADVERTISING_FEE_IMPORT_ID = 'e2e-advertising-fee-import-1';
export const E2E_ADVERTISING_FEE_SPEND_ID = 'e2e-advertising-fee-spend-1';
export const E2E_ADVERTISING_FEE_ALLOCATION_ID = 'e2e-advertising-fee-allocation-1';
export const E2E_ADVERTISING_FEE_ADJUSTMENT_ID = 'e2e-advertising-fee-adjustment-1';

export const e2eAdvertisingFeeAllocation = {
  id: E2E_ADVERTISING_FEE_ALLOCATION_ID,
  importId: E2E_ADVERTISING_FEE_IMPORT_ID,
  spendId: E2E_ADVERTISING_FEE_SPEND_ID,
  shopId: 'e2e-shop-douyin',
  orderId: 'e2e-order-profit-1',
  orderNo: 'SO-E2E-PROFIT-0001',
  paidAt: '2026-09-05T02:00:00Z',
  amountMinor: 280,
  currency: 'CNY',
  policyVersion: 'shop_day_equal_paid_order_v1',
  calculationHash: 'b'.repeat(64),
  attributedAt: '2026-09-08T02:00:00Z',
  spendDate: '2026-09-05',
  shopName: 'E2E 抖店测试店铺',
  platform: 'douyin_shop',
  shopTimezone: 'Asia/Shanghai',
  settlementCoverage: 'excluded',
  baseSpendMinor: 600,
  adjustmentCount: 1,
  adjustmentMinor: 20,
  netAmountMinor: 300,
  profitStatus: 'confirmed',
};

export const e2eAdvertisingFeeDetail = {
  ...e2eAdvertisingFeeAllocation,
  adjustments: [
    {
      id: E2E_ADVERTISING_FEE_ADJUSTMENT_ID,
      allocationId: E2E_ADVERTISING_FEE_ALLOCATION_ID,
      orderId: 'e2e-order-profit-1',
      factType: 'adjustment',
      amountMinor: 20,
      currency: 'CNY',
      reason: '补录站内推广消耗',
      createdAt: '2026-09-08T02:10:00Z',
    },
  ],
};

export const e2eAdvertisingFeePreview = {
  fileName: 'advertising.csv',
  fileHash: 'a'.repeat(64),
  calculationHash: 'b'.repeat(64),
  policyVersion: 'shop_day_equal_paid_order_v1',
  shopId: 'e2e-shop-douyin',
  shopName: 'E2E 抖店测试店铺',
  platform: 'douyin_shop',
  shopTimezone: 'Asia/Shanghai',
  valid: true,
  sourceRows: 1,
  newSpends: 1,
  duplicateSpends: 0,
  allocationCount: 2,
  issues: [],
  warnings: [],
  rows: [
    {
      line: 2,
      spendDate: '2026-09-05',
      currency: 'CNY',
      spendMinor: 600,
      settlementCoverage: 'excluded',
      disposition: 'new',
      eligibleOrderCount: 2,
      excludedOrderCount: 1,
      calculationHash: 'c'.repeat(64),
    },
  ],
  orders: [
    {
      line: 2,
      spendDate: '2026-09-05',
      orderId: 'e2e-order-profit-1',
      orderNo: 'SO-E2E-PROFIT-0001',
      paidAt: '2026-09-05T02:00:00Z',
      currency: 'CNY',
      amountMinor: 300,
      included: true,
    },
    {
      line: 2,
      spendDate: '2026-09-05',
      orderId: 'e2e-order-cancelled-1',
      orderNo: 'SO-E2E-CANCELLED-0001',
      paidAt: '2026-09-05T03:00:00Z',
      currency: 'CNY',
      amountMinor: null,
      included: false,
      reasonCode: 'order_cancelled',
      reason: '订单已取消',
    },
  ],
};

export function advertisingFeeResponse(path: string) {
  if (path === '/api/v1/advertising-fee-imports') {
    return ok({ list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 });
  }
  if (path === '/api/v1/advertising-fees') {
    return ok({ list: [e2eAdvertisingFeeAllocation], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === `/api/v1/advertising-fees/${E2E_ADVERTISING_FEE_ALLOCATION_ID}`) {
    return ok(e2eAdvertisingFeeDetail);
  }
  return null;
}
