import { ok } from './envelope';

export const E2E_SETTLEMENT_ID = '11111111-2222-3333-4444-555555555555';

export const e2eSettlementReconciliation = {
  id: E2E_SETTLEMENT_ID,
  shopId: 'e2e-shop-douyin',
  shopName: 'E2E 抖店旗舰店',
  platform: 'douyin_shop',
  orderId: 'e2e-order-profit-1',
  orderNo: 'SO-E2E-PROFIT-0001',
  currency: 'CNY',
  orderAmountMinor: 10000,
  orderGrossMinor: 10000,
  platformFeeMinor: 1000,
  settlementAmountMinor: 9000,
  transactionCount: 1,
  status: 'matched',
  issues: [],
  firstSettledAt: '2026-09-05T05:00:00Z',
  lastSettledAt: '2026-09-05T05:00:00Z',
  lastImportedAt: '2026-09-06T01:00:00Z',
};

export const e2eSettlementDetail = {
  ...e2eSettlementReconciliation,
  transactions: [
    {
      id: 'e2e-settlement-transaction-1',
      importId: 'e2e-settlement-import-1',
      externalTransactionId: 'SETTLEMENT-E2E-0001',
      orderNo: e2eSettlementReconciliation.orderNo,
      currency: 'CNY',
      orderGrossMinor: 10000,
      platformFeeMinor: 1000,
      settlementAmountMinor: 9000,
      settledAt: '2026-09-05T05:00:00Z',
      createdAt: '2026-09-06T01:00:00Z',
    },
  ],
};

export function settlementResponse(path: string) {
  if (path === '/api/v1/settlement-reconciliation') {
    return ok({ list: [e2eSettlementReconciliation], page: 1, pageSize: 20, total: 1, totalPages: 1 });
  }
  if (path === `/api/v1/settlement-reconciliation/${E2E_SETTLEMENT_ID}`) {
    return ok(e2eSettlementDetail);
  }
  return null;
}
