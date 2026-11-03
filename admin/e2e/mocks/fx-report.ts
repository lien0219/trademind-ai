import { ok } from './envelope';
import { e2eOrderProfit } from './order-profits';

export function fxReportResponse(path: string) {
  if (path === '/api/v1/finance/fx-rates') {
    return ok({
      items: [{
        id: 'e2e-fx-rate-001',
        fromCurrency: 'USD',
        toCurrency: 'CNY',
        rate: '7.25',
        effectiveAt: '2026-09-01T00:00:00Z',
        evidence: 'Isolated E2E sample rate',
        createdBy: 'e2e-admin',
        createdAt: '2026-09-01T00:00:00Z',
      }],
    });
  }
  if (path === '/api/v1/finance/fx-report') {
    return ok({
      baseCurrency: 'CNY',
      calculatedAt: '2026-09-08T05:00:00Z',
      formulaVersion: 'order_profit_estimate_v6',
      totalOrders: 1,
      includedOrders: 1,
      excludedOrders: 0,
      revenueTotalMinor: 10000,
      estimatedProfitTotalMinor: 3010,
      rows: [{
        ...e2eOrderProfit,
        baseCurrency: 'CNY',
        conversionStatus: 'complete',
        rate: '1',
        converted: {
          revenueMinor: 10000,
          productCostMinor: 4000,
          freightMinor: 1000,
          refundMinor: 500,
          platformFeeMinor: 1000,
          advertisingFeeMinor: 300,
          warehouseFeeMinor: 190,
          knownContributionMinor: 3010,
          estimatedProfitMinor: 3010,
        },
      }],
    });
  }
  return null;
}
