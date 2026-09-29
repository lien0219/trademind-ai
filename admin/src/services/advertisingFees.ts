import { getJSON, getWithParams, postFormData, postJSON } from '@/services/request';

export type AdvertisingSettlementCoverage = 'excluded' | 'included' | 'unknown';
export type AdvertisingDisposition = 'new' | 'duplicate';

export type AdvertisingValidationIssue = {
  line: number;
  field?: string;
  code: string;
  message: string;
};

export type AdvertisingSpendPreview = {
  line: number;
  spendDate: string;
  currency: string;
  spendMinor: number;
  settlementCoverage: AdvertisingSettlementCoverage;
  disposition: AdvertisingDisposition;
  eligibleOrderCount: number;
  excludedOrderCount: number;
  calculationHash?: string;
};

export type AdvertisingPreviewOrder = {
  line: number;
  spendDate: string;
  orderId: string;
  orderNo: string;
  paidAt?: string;
  currency: string;
  amountMinor: number | null;
  included: boolean;
  reasonCode?: string;
  reason?: string;
};

export type AdvertisingImportPreview = {
  fileName: string;
  fileHash: string;
  calculationHash: string;
  policyVersion: string;
  shopId: string;
  shopName: string;
  platform: string;
  shopTimezone: string;
  valid: boolean;
  sourceRows: number;
  newSpends: number;
  duplicateSpends: number;
  allocationCount: number;
  rows: AdvertisingSpendPreview[];
  orders: AdvertisingPreviewOrder[];
  issues: AdvertisingValidationIssue[];
  warnings: AdvertisingValidationIssue[];
};

export type AdvertisingImport = {
  id: string;
  shopId: string;
  platform: string;
  shopTimezone: string;
  fileName: string;
  fileHash: string;
  policyVersion: string;
  calculationHash: string;
  sourceRowCount: number;
  importedSpends: number;
  duplicateSpends: number;
  allocationCount: number;
  confirmedAt: string;
};

export type AdvertisingAdjustment = {
  id: string;
  allocationId: string;
  orderId: string;
  factType: 'adjustment' | 'reversal';
  amountMinor: number;
  currency: string;
  reason: string;
  reversesAdjustmentId?: string;
  actorId?: string;
  createdAt: string;
};

export type AdvertisingAllocation = {
  id: string;
  importId: string;
  spendId: string;
  shopId: string;
  orderId: string;
  orderNo: string;
  paidAt: string;
  amountMinor: number;
  currency: string;
  policyVersion: string;
  calculationHash: string;
  attributedAt: string;
  spendDate: string;
  shopName: string;
  platform: string;
  shopTimezone: string;
  settlementCoverage: AdvertisingSettlementCoverage;
  baseSpendMinor: number;
  adjustmentCount: number;
  adjustmentMinor: number;
  netAmountMinor: number;
  profitStatus: 'confirmed' | 'mismatch' | 'blocked';
  reasonCode?: string;
  reason?: string;
};

export type AdvertisingAllocationDetail = AdvertisingAllocation & {
  adjustments: AdvertisingAdjustment[];
};

export type AdvertisingPage<T> = {
  list: T[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type AdvertisingListParams = {
  page?: number;
  pageSize?: number;
  orderNo?: string;
  shopId?: string;
  currency?: string;
  settlementCoverage?: AdvertisingSettlementCoverage;
};

function importForm(file: File, shopId: string, fields?: Record<string, string>) {
  const form = new FormData();
  form.append('file', file);
  form.append('shopId', shopId);
  for (const [key, value] of Object.entries(fields || {})) form.append(key, value);
  return form;
}

export function previewAdvertisingFeeImport(file: File, shopId: string) {
  return postFormData<AdvertisingImportPreview>(
    '/api/v1/advertising-fee-imports/preview',
    importForm(file, shopId),
  );
}

export function confirmAdvertisingFeeImport(
  file: File,
  shopId: string,
  expectedFileHash: string,
  expectedCalculationHash: string,
  idempotencyKey: string,
) {
  return postFormData<{ import: AdvertisingImport; replayed: boolean }>(
    '/api/v1/advertising-fee-imports',
    importForm(file, shopId, { expectedFileHash, expectedCalculationHash, idempotencyKey }),
  );
}

export function queryAdvertisingFees(params: AdvertisingListParams) {
  return getWithParams<AdvertisingPage<AdvertisingAllocation>>('/api/v1/advertising-fees', params);
}

export function getAdvertisingFee(id: string) {
  return getJSON<AdvertisingAllocationDetail>(
    `/api/v1/advertising-fees/${encodeURIComponent(id)}`,
  );
}

export function createAdvertisingFeeAdjustment(
  allocationId: string,
  payload: { amountMinor: number; reason: string; idempotencyKey: string },
) {
  return postJSON<{ adjustment: AdvertisingAdjustment; replayed: boolean }>(
    `/api/v1/advertising-fees/${encodeURIComponent(allocationId)}/adjustments`,
    payload,
  );
}

export function reverseAdvertisingFeeAdjustment(
  allocationId: string,
  adjustmentId: string,
  payload: { reason: string; idempotencyKey: string },
) {
  return postJSON<{ adjustment: AdvertisingAdjustment; replayed: boolean }>(
    `/api/v1/advertising-fees/${encodeURIComponent(allocationId)}/adjustments/${encodeURIComponent(adjustmentId)}/reverse`,
    payload,
  );
}

export function advertisingFeeIdempotencyKey(action: string) {
  const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `admin-advertising-fee-${action}-${random}`.slice(0, 128);
}

export function downloadAdvertisingFeeCSVTemplate() {
  const header = 'spend_date,currency,spend_minor,settlement_coverage';
  const blob = new Blob([`\uFEFF${header}\r\n`], { type: 'text/csv;charset=utf-8' });
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = 'advertising-fee-import-template.csv';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}
