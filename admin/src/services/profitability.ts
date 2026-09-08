import { request } from '@umijs/max';
import { getJSON, getWithParams } from '@/services/request';

export type ProfitabilityStatus = 'complete' | 'pending' | 'mismatch' | 'blocked';
export type ProfitComponentStatus = 'available' | 'missing' | 'pending' | 'mismatch' | 'blocked';

export type ProfitMoneyComponent = {
  amountMinor: number | null;
  knownAmountMinor: number;
  currency: string;
  status: ProfitComponentStatus;
  source: string;
  sourceAt?: string;
  reasonCode?: string;
  reason?: string;
};

export type ProfitIssue = {
  code: string;
  component: string;
  message: string;
};

export type OrderProfit = {
  orderId: string;
  orderNo: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  warehouseId?: string;
  warehouseCode?: string;
  warehouseName?: string;
  currency: string;
  status: ProfitabilityStatus;
  orderStatus: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  knownContributionMinor: number | null;
  estimatedProfitMinor: number | null;
  estimatedMarginBps: number | null;
  components: {
    revenue: ProfitMoneyComponent;
    productCost: ProfitMoneyComponent;
    freight: ProfitMoneyComponent;
    refund: ProfitMoneyComponent;
    platformFee: ProfitMoneyComponent;
    advertisingFee: ProfitMoneyComponent;
    warehouseFee: ProfitMoneyComponent;
  };
  issues: ProfitIssue[];
  related: {
    fulfillmentWaveId?: string;
    supplierIds: string[];
    productCostSnapshotIds: string[];
    refundExecutionIds: string[];
    settlementReconciliationId?: string;
    settlementTransactionIds: string[];
    warehouseFeeSnapshotId?: string;
    warehouseFeeAdjustmentIds: string[];
  };
  orderedAt?: string;
  calculatedAt: string;
  formulaVersion: string;
};

export type ProductCostLine = {
  orderItemId: string;
  productSkuId?: string;
  productTitle: string;
  skuCode?: string;
  quantity: number;
  unitCostMinor: number | null;
  lineCostMinor: number | null;
  currency: string;
  status: ProfitComponentStatus;
  source: string;
  costBasis: 'estimate' | 'snapshot';
  sourceAt?: string;
  snapshotId?: string;
  resolutionStatus?: 'resolved' | 'missing' | 'ambiguous' | 'currency_mismatch' | 'invalid';
  capturedAt?: string;
  supplierId?: string;
  supplierSkuId?: string;
  supplierName?: string;
  supplierSkuCode?: string;
  candidateCount: number;
  reasonCode?: string;
  reason?: string;
};

export type OrderProfitDetail = OrderProfit & {
  productCostLines: ProductCostLine[];
};

export type OrderProfitFormula = {
  version: string;
  revenueSource: string;
  productCostSource: string;
  freightSource: string;
  refundSource: string;
  platformFeeSource: string;
  warehouseFeeSource: string;
  missingFeeBehavior: string;
};

export type OrderProfitList = {
  list: OrderProfit[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
  calculatedAt: string;
  formula: OrderProfitFormula;
};

export type OrderProfitListParams = {
  page?: number;
  pageSize?: number;
  orderNo?: string;
  platform?: string;
  shopId?: string;
  warehouseId?: string;
  currency?: string;
  status?: ProfitabilityStatus;
  start?: string;
  end?: string;
};

export async function queryOrderProfits(params: OrderProfitListParams) {
  return getWithParams<OrderProfitList>('/api/v1/order-profits', params);
}

export async function getOrderProfit(orderId: string) {
  return getJSON<OrderProfitDetail>(`/api/v1/order-profits/${encodeURIComponent(orderId)}`);
}

export async function downloadOrderProfits(params: Omit<OrderProfitListParams, 'page' | 'pageSize'>) {
  const query = new URLSearchParams({ format: 'csv' });
  const entries: Array<[string, string | undefined]> = [
    ['orderNo', params.orderNo?.trim() || undefined],
    ['platform', params.platform],
    ['shopId', params.shopId],
    ['warehouseId', params.warehouseId],
    ['currency', params.currency?.trim().toUpperCase() || undefined],
    ['status', params.status],
    ['start', params.start],
    ['end', params.end],
  ];
  for (const [key, value] of entries) {
    if (value) query.set(key, value);
  }
  const blob = await request<Blob>(`/api/v1/order-profits?${query.toString()}`, {
    method: 'GET',
    responseType: 'blob',
  });
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = 'order-profit-estimates.csv';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}

export function currencyMinorExponent(currency: string) {
  switch (currency.trim().toUpperCase()) {
    case 'BIF':
    case 'CLP':
    case 'DJF':
    case 'GNF':
    case 'JPY':
    case 'KMF':
    case 'KRW':
    case 'MGA':
    case 'PYG':
    case 'RWF':
    case 'UGX':
    case 'VND':
    case 'VUV':
    case 'XAF':
    case 'XOF':
    case 'XPF':
      return 0;
    case 'BHD':
    case 'IQD':
    case 'JOD':
    case 'KWD':
    case 'LYD':
    case 'OMR':
    case 'TND':
      return 3;
    case 'CLF':
    case 'UYW':
      return 4;
    default:
      return 2;
  }
}

export function formatProfitAmount(value: number | null | undefined, currency: string) {
  if (value == null || !Number.isSafeInteger(value)) return '—';
  const exponent = currencyMinorExponent(currency);
  const scale = 10 ** exponent;
  const absolute = Math.abs(value);
  const whole = Math.floor(absolute / scale);
  const fraction = exponent ? `.${String(absolute % scale).padStart(exponent, '0')}` : '';
  return `${currency.trim().toUpperCase()} ${value < 0 ? '-' : ''}${whole}${fraction}`;
}

export function formatMarginBps(value: number | null | undefined) {
  if (value == null || !Number.isSafeInteger(value)) return '—';
  return `${(value / 100).toFixed(2)}%`;
}
