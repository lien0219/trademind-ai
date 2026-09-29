import { request } from '@umijs/max';
import { getJSON, getWithParams, postFormData } from '@/services/request';

export type SettlementStatus = 'matched' | 'pending' | 'mismatch' | 'blocked';
export type SettlementDisposition = 'new' | 'duplicate';

export type SettlementCSVRow = {
  line: number;
  externalTransactionId: string;
  orderNo: string;
  currency: string;
  orderGrossMinor: number;
  platformFeeMinor: number;
  settlementAmountMinor: number;
  settledAt: string;
  disposition: SettlementDisposition;
};

export type SettlementValidationIssue = {
  line: number;
  field?: string;
  code: string;
  message: string;
};

export type SettlementImportPreview = {
  fileName: string;
  fileHash: string;
  shopId: string;
  shopName: string;
  platform: string;
  valid: boolean;
  sourceRows: number;
  newRows: number;
  duplicateRows: number;
  rows: SettlementCSVRow[];
  issues: SettlementValidationIssue[];
};

export type SettlementImport = {
  id: string;
  shopId: string;
  platform: string;
  fileName: string;
  fileHash: string;
  sourceRowCount: number;
  importedRows: number;
  duplicateRows: number;
  confirmedAt: string;
};

export type SettlementImportResult = {
  import: SettlementImport;
  replayed: boolean;
};

export type SettlementIssue = {
  code: string;
  message: string;
};

export type SettlementReconciliation = {
  id: string;
  shopId: string;
  shopName: string;
  platform: string;
  orderId?: string;
  orderNo: string;
  currency: string;
  orderAmountMinor: number | null;
  orderGrossMinor: number | null;
  platformFeeMinor: number | null;
  settlementAmountMinor: number | null;
  transactionCount: number;
  status: SettlementStatus;
  issues: SettlementIssue[];
  firstSettledAt: string;
  lastSettledAt: string;
  lastImportedAt: string;
};

export type SettlementTransaction = {
  id: string;
  importId: string;
  externalTransactionId: string;
  orderNo: string;
  currency: string;
  orderGrossMinor: number;
  platformFeeMinor: number;
  settlementAmountMinor: number;
  settledAt: string;
  createdAt: string;
};

export type SettlementReconciliationDetail = SettlementReconciliation & {
  transactions: SettlementTransaction[];
};

export type SettlementListResult = {
  list: SettlementReconciliation[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type SettlementListParams = {
  page?: number;
  pageSize?: number;
  orderNo?: string;
  platform?: string;
  shopId?: string;
  currency?: string;
  status?: SettlementStatus;
  start?: string;
  end?: string;
};

function importForm(file: File, shopId: string, fields?: Record<string, string>) {
  const form = new FormData();
  form.append('file', file);
  form.append('shopId', shopId);
  for (const [key, value] of Object.entries(fields || {})) form.append(key, value);
  return form;
}

export async function previewSettlementImport(file: File, shopId: string) {
  return postFormData<SettlementImportPreview>(
    '/api/v1/settlement-imports/preview',
    importForm(file, shopId),
  );
}

export async function confirmSettlementImport(
  file: File,
  shopId: string,
  expectedFileHash: string,
  idempotencyKey: string,
) {
  return postFormData<SettlementImportResult>(
    '/api/v1/settlement-imports',
    importForm(file, shopId, { expectedFileHash, idempotencyKey }),
  );
}

export async function querySettlementReconciliation(params: SettlementListParams) {
  return getWithParams<SettlementListResult>('/api/v1/settlement-reconciliation', params);
}

export async function getSettlementReconciliation(id: string) {
  return getJSON<SettlementReconciliationDetail>(
    `/api/v1/settlement-reconciliation/${encodeURIComponent(id)}`,
  );
}

export async function downloadSettlementReconciliation(
  params: Omit<SettlementListParams, 'page' | 'pageSize'>,
) {
  const query = new URLSearchParams({ format: 'csv' });
  const entries: Array<[string, string | undefined]> = [
    ['orderNo', params.orderNo?.trim() || undefined],
    ['platform', params.platform],
    ['shopId', params.shopId],
    ['currency', params.currency?.trim().toUpperCase() || undefined],
    ['status', params.status],
    ['start', params.start],
    ['end', params.end],
  ];
  for (const [key, value] of entries) if (value) query.set(key, value);
  const blob = await request<Blob>(
    `/api/v1/settlement-reconciliation?${query.toString()}`,
    { method: 'GET', responseType: 'blob' },
  );
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = 'settlement-reconciliation.csv';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}

export function settlementImportIdempotencyKey() {
  const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `settlement-import-${random}`;
}

export function downloadSettlementCSVTemplate() {
  const header = [
    'external_transaction_id',
    'order_no',
    'currency',
    'order_gross_minor',
    'platform_fee_minor',
    'settlement_amount_minor',
    'settled_at',
  ].join(',');
  const blob = new Blob([`\uFEFF${header}\r\n`], { type: 'text/csv;charset=utf-8' });
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = 'settlement-import-template.csv';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}
