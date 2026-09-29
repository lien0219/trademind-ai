import { getJSON, getWithParams, postFormData, postJSON } from '@/services/request';

export type FreightValidationIssue = {
  line: number;
  field?: string;
  code: string;
  message: string;
};

export type FreightChargePreviewRow = {
  line: number;
  externalLineId: string;
  carrier: string;
  trackingNo: string;
  amountMinor: number;
  currency: string;
  billedAt: string;
  disposition: 'new' | 'duplicate';
  orderId?: string;
  shipmentId?: string;
  orderNo?: string;
  orderCurrency?: string;
  status: 'confirmed' | 'mismatch' | 'blocked';
  reasonCode?: string;
  reason?: string;
};

export type FreightImportPreview = {
  fileName: string;
  fileHash: string;
  calculationHash: string;
  policyVersion: string;
  shopId: string;
  shopName: string;
  valid: boolean;
  sourceRows: number;
  newCharges: number;
  duplicateCharges: number;
  rows: FreightChargePreviewRow[];
  issues: FreightValidationIssue[];
  warnings: FreightValidationIssue[];
};

export type FreightImport = {
  id: string;
  shopId: string;
  fileName: string;
  fileHash: string;
  policyVersion: string;
  calculationHash: string;
  sourceRowCount: number;
  importedRows: number;
  duplicateRows: number;
  confirmedAt: string;
};

export type FreightAdjustment = {
  id: string;
  chargeId: string;
  factType: 'adjustment' | 'reversal';
  amountMinor: number;
  currency: string;
  reason: string;
  reversesAdjustmentId?: string;
  actorId?: string;
  createdAt: string;
};

export type FreightCharge = {
  id: string;
  importId: string;
  shopId: string;
  orderId: string;
  shipmentId: string;
  orderNo: string;
  orderCurrency: string;
  carrier: string;
  trackingNo: string;
  externalLineId: string;
  billedAt: string;
  amountMinor: number;
  currency: string;
  importedAt: string;
  adjustmentCount: number;
  adjustmentMinor: number;
  netAmountMinor: number;
  status: 'confirmed' | 'mismatch';
  reasonCode?: string;
  reason?: string;
};

export type FreightChargeDetail = FreightCharge & { adjustments: FreightAdjustment[] };

export type FreightPage<T> = {
  list: T[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type FreightListParams = {
  page?: number;
  pageSize?: number;
  orderNo?: string;
  trackingNo?: string;
  shopId?: string;
  currency?: string;
  status?: FreightCharge['status'];
};

function importForm(file: File, shopId: string, fields?: Record<string, string>) {
  const form = new FormData();
  form.append('file', file);
  form.append('shopId', shopId);
  for (const [key, value] of Object.entries(fields || {})) form.append(key, value);
  return form;
}

export function previewFreightFeeImport(file: File, shopId: string) {
  return postFormData<FreightImportPreview>(
    '/api/v1/freight-fee-imports/preview',
    importForm(file, shopId),
  );
}

export function confirmFreightFeeImport(
  file: File,
  shopId: string,
  expectedFileHash: string,
  expectedCalculationHash: string,
  idempotencyKey: string,
) {
  return postFormData<{ import: FreightImport; replayed: boolean }>(
    '/api/v1/freight-fee-imports',
    importForm(file, shopId, { expectedFileHash, expectedCalculationHash, idempotencyKey }),
  );
}

export function queryFreightFees(params: FreightListParams) {
  return getWithParams<FreightPage<FreightCharge>>('/api/v1/freight-fees', params);
}

export function queryFreightFeeImports(params: { page?: number; pageSize?: number }) {
  return getWithParams<FreightPage<FreightImport>>('/api/v1/freight-fee-imports', params);
}

export function getFreightFee(id: string) {
  return getJSON<FreightChargeDetail>(`/api/v1/freight-fees/${encodeURIComponent(id)}`);
}

export function createFreightFeeAdjustment(
  id: string,
  payload: { amountMinor: number; reason: string; idempotencyKey: string },
) {
  return postJSON<{ adjustment: FreightAdjustment; replayed: boolean }>(
    `/api/v1/freight-fees/${encodeURIComponent(id)}/adjustments`,
    payload,
  );
}

export function reverseFreightFeeAdjustment(
  chargeId: string,
  adjustmentId: string,
  payload: { reason: string; idempotencyKey: string },
) {
  return postJSON<{ adjustment: FreightAdjustment; replayed: boolean }>(
    `/api/v1/freight-fees/${encodeURIComponent(chargeId)}/adjustments/${encodeURIComponent(adjustmentId)}/reverse`,
    payload,
  );
}

export function freightFeeIdempotencyKey(action: string) {
  const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `admin-freight-fee-${action}-${random}`.slice(0, 128);
}

export function downloadFreightFeeCSVTemplate() {
  const header = 'external_line_id,carrier,tracking_no,amount_minor,currency,billed_at';
  const blob = new Blob([`\uFEFF${header}\r\n`], { type: 'text/csv;charset=utf-8' });
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = 'carrier-freight-fee-template.csv';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}
