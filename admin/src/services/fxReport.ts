import { request } from '@umijs/max';
import { getWithParams, postJSON } from '@/services/request';
import { captureTenantContext, isTenantContextCurrent, isTenantSwitching } from '@/utils/tenantContext';

export const FX_CURRENCIES = ['CNY', 'USD', 'JPY', 'EUR', 'GBP', 'KRW', 'SGD', 'AUD', 'CAD'] as const;
export type FXCurrency = (typeof FX_CURRENCIES)[number];

export type FXRate = {
  id: string;
  fromCurrency: FXCurrency;
  toCurrency: FXCurrency;
  rate: string;
  effectiveAt: string;
  evidence: string;
  createdBy: string;
  createdAt: string;
};

export type FXConverted = {
  revenueMinor: number | null;
  productCostMinor: number | null;
  freightMinor: number | null;
  refundMinor: number | null;
  platformFeeMinor: number | null;
  advertisingFeeMinor: number | null;
  warehouseFeeMinor: number | null;
  knownContributionMinor: number | null;
  estimatedProfitMinor: number | null;
};

export type FXReportRow = {
  orderId: string;
  orderNo: string;
  platform: string;
  shopName?: string;
  currency: string;
  status: string;
  conversionStatus: 'complete' | 'source_incomplete' | 'rate_missing' | 'order_date_missing';
  baseCurrency: string;
  rate?: string;
  rateId?: string;
  rateEffectiveAt?: string;
  converted: FXConverted;
  components: { revenue: { amountMinor: number | null } };
  estimatedProfitMinor: number | null;
  orderedAt?: string;
};

export type FXReport = {
  baseCurrency: FXCurrency;
  calculatedAt: string;
  formulaVersion: string;
  totalOrders: number;
  includedOrders: number;
  excludedOrders: number;
  revenueTotalMinor: number;
  estimatedProfitTotalMinor: number;
  rows: FXReportRow[];
};

export type FXReportFilters = {
  baseCurrency: FXCurrency;
  start?: string;
  end?: string;
  platform?: string;
  shopId?: string;
};

// Prefer the current tenant's valid currency setting, without silently
// rewriting unsupported codes or values from another tenant.
export function preferredFXCurrency(value: unknown): FXCurrency | null {
  return typeof value === 'string' && (FX_CURRENCIES as readonly string[]).includes(value)
    ? value as FXCurrency
    : null;
}

export function getFXReport(filters: FXReportFilters): Promise<FXReport> {
  return getWithParams<FXReport>('/api/v1/finance/fx-report', filters);
}

export function listFXRates(baseCurrency: FXCurrency): Promise<{ items: FXRate[] }> {
  return getWithParams<{ items: FXRate[] }>('/api/v1/finance/fx-rates', { baseCurrency });
}

export function createFXRate(rate: {
  fromCurrency: FXCurrency;
  toCurrency: FXCurrency;
  rate: string;
  effectiveAt: string;
  evidence: string;
}): Promise<FXRate> {
  return postJSON<FXRate>('/api/v1/finance/fx-rates', rate);
}

export async function downloadFXReport(filters: FXReportFilters, isCurrent: () => boolean): Promise<boolean> {
  const context = captureTenantContext();
  const safe = () => isCurrent() && !isTenantSwitching() && isTenantContextCurrent(context);
  if (!safe()) return false;
  const params = new URLSearchParams({ format: 'csv', baseCurrency: filters.baseCurrency });
  if (filters.start) params.set('start', filters.start);
  if (filters.end) params.set('end', filters.end);
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.shopId) params.set('shopId', filters.shopId);
  const blob = await request<Blob>('/api/v1/finance/fx-report?' + params.toString(), {
    method: 'GET',
    responseType: 'blob',
  });
  if (!safe()) return false;
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  try {
    anchor.href = url;
    anchor.download = 'order-profit-currency-report.csv';
    anchor.rel = 'noopener';
    document.body.appendChild(anchor);
    anchor.click();
    return true;
  } finally {
    anchor.remove();
    URL.revokeObjectURL(url);
  }
}
