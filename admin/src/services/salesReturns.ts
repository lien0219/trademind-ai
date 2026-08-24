import { ApiRequestError, getJSON, getWithParams, postJSON } from './request';

export type SalesReturnType = 'refund_only' | 'return_refund';
export type SalesReturnStatus =
  | 'draft'
  | 'pending_approval'
  | 'approved'
  | 'completed'
  | 'cancelled';
export type SalesReturnDisposition = 'sellable' | 'damaged';

export type SalesReturnItem = {
  id: string;
  salesReturnId: string;
  orderItemId: string;
  productSkuId: string;
  quantity: number;
  disposition?: SalesReturnDisposition | string;
  refundAmountMinor: number;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
  deductedQuantity: number;
};

export type SalesReturn = {
  id: string;
  returnNo: string;
  orderId: string;
  orderNo?: string;
  warehouseId: string;
  warehouseName?: string;
  type: SalesReturnType | string;
  status: SalesReturnStatus | string;
  currency: string;
  refundAmountMinor: number;
  revision: number;
  reason: string;
  remark?: string;
  approvedBy?: string;
  approvedAt?: string;
  completedBy?: string;
  completedAt?: string;
  cancelledAt?: string;
  itemCount?: number;
  createdAt: string;
  updatedAt: string;
  items?: SalesReturnItem[];
};

export type ReturnableSalesItem = {
  orderItemId: string;
  productSkuId: string;
  productTitle: string;
  skuCode: string;
  skuName: string;
  unitPrice: number;
  deductedQuantity: number;
  restoredQuantity: number;
  allocatedReturnQuantity: number;
  remainingQuantity: number;
};

export type ReturnableSalesItemResult = {
  orderId: string;
  orderNo: string;
  warehouseId: string;
  currency: string;
  list: ReturnableSalesItem[];
};

export type CreateSalesReturnBody = {
  idempotencyKey: string;
  orderId: string;
  type: SalesReturnType;
  reason: string;
  remark: string;
  items: Array<{
    orderItemId: string;
    quantity: number;
    disposition: SalesReturnDisposition | '';
    refundAmountMinor: number;
  }>;
};

const enc = encodeURIComponent;

export function createSalesReturnIdempotencyKey(action: string) {
  const random =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  return `admin-${action}-${random}`.slice(0, 128);
}

export async function listSalesReturns(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  type?: string;
  orderId?: string;
}) {
  return getWithParams<{
    list: SalesReturn[];
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  }>('/api/v1/sales-returns', params);
}

export async function getSalesReturn(id: string) {
  return getJSON<SalesReturn>(`/api/v1/sales-returns/${enc(id)}`);
}

export async function listReturnableSalesItems(orderId: string) {
  return getJSON<ReturnableSalesItemResult>(
    `/api/v1/orders/${enc(orderId)}/sales-returnable-items`,
  );
}

export async function createSalesReturn(body: CreateSalesReturnBody) {
  return postJSON<SalesReturn>('/api/v1/sales-returns', body);
}

export async function transitionSalesReturn(
  id: string,
  action: 'submit' | 'approve' | 'complete' | 'cancel',
  body: { expectedRevision: number; idempotencyKey: string; reason: string },
) {
  return postJSON<SalesReturn>(
    `/api/v1/sales-returns/${enc(id)}/${action}`,
    body,
  );
}

export type RefundExecutionStatus =
  | 'pending'
  | 'succeeded'
  | 'failed'
  | 'unknown'
  | 'cancelled';

export type RefundPlatformReview = {
  status: PlatformAfterSaleReconciliationStatus | string;
  reason: string;
  platformAfterSaleId?: string;
  platform?: string;
  platformStatus?: string;
  externalAfterSaleId?: string;
  refundAmountMinor?: number;
  currency?: string;
  platformUpdatedAt?: string;
};

export type RefundExecutionEvent = {
  id: string;
  action: 'record_result' | 'confirm_platform' | 'cancel' | string;
  fromStatus: string;
  toStatus: string;
  source?: string;
  externalRefundId?: string;
  reason?: string;
  actorId?: string;
  executedAt?: string;
  createdAt: string;
};

export type RefundExecution = {
  id: string;
  executionNo: string;
  salesReturnId: string;
  returnNo?: string;
  orderId: string;
  orderNo?: string;
  internalShopId?: string;
  platformAfterSaleId?: string;
  status: RefundExecutionStatus | string;
  source?: 'manual' | 'platform_fact' | string;
  currency: string;
  refundAmountMinor: number;
  revision: number;
  externalRefundId?: string;
  resultReason?: string;
  executedAt?: string;
  cancelledAt?: string;
  createdAt: string;
  updatedAt: string;
  platformReview?: RefundPlatformReview;
  events?: RefundExecutionEvent[];
};

export async function listRefundExecutions(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  salesReturnId?: string;
  orderId?: string;
}) {
  return getWithParams<{
    list: RefundExecution[];
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  }>('/api/v1/refund-executions', params);
}

export async function getRefundExecution(id: string) {
  return getJSON<RefundExecution>(`/api/v1/refund-executions/${enc(id)}`);
}

export async function createRefundExecution(
  salesReturnId: string,
  body: { idempotencyKey: string; platformAfterSaleId?: string },
) {
  return postJSON<RefundExecution>(
    `/api/v1/sales-returns/${enc(salesReturnId)}/refund-execution`,
    body,
  );
}

export async function recordRefundResult(
  id: string,
  body: {
    expectedRevision: number;
    idempotencyKey: string;
    result: 'succeeded' | 'failed' | 'unknown';
    externalRefundId: string;
    executedAt: string;
    reason: string;
  },
) {
  return postJSON<RefundExecution>(
    `/api/v1/refund-executions/${enc(id)}/result`,
    body,
  );
}

export async function confirmRefundFromPlatform(
  id: string,
  body: {
    expectedRevision: number;
    idempotencyKey: string;
    platformAfterSaleId: string;
    reason: string;
  },
) {
  return postJSON<RefundExecution>(
    `/api/v1/refund-executions/${enc(id)}/confirm-from-platform`,
    body,
  );
}

export async function cancelRefundExecution(
  id: string,
  body: { expectedRevision: number; idempotencyKey: string; reason: string },
) {
  return postJSON<RefundExecution>(
    `/api/v1/refund-executions/${enc(id)}/cancel`,
    body,
  );
}

export type SalesReturnAPIError = {
  code: number;
  message: string;
  traceId?: string;
};

export type PlatformAfterSaleReconciliationStatus =
  | 'matched'
  | 'pending'
  | 'mismatch'
  | 'blocked';

export type PlatformAfterSale = {
  id: string;
  platform: string;
  platformShopId: string;
  externalAfterSaleId: string;
  externalOrderId: string;
  platformType: string;
  platformStatus: string;
  refundAmountMinor: number;
  currency: string;
  platformUpdatedAt?: string;
  orderId?: string;
  orderNo?: string;
  salesReturnId?: string;
  returnNo?: string;
  reconciliationStatus: PlatformAfterSaleReconciliationStatus | string;
  reconciliationReason: string;
  lastEventId: string;
  events?: PlatformAfterSaleEvent[];
  createdAt: string;
  updatedAt: string;
};

export type PlatformAfterSaleEvent = {
  id: string;
  eventId: string;
  eventType: string;
  platformAfterSaleId: string;
  applied: boolean;
  ignoredReason?: string;
  createdAt: string;
};

export type PlatformAfterSaleListParams = {
  page?: number;
  pageSize?: number;
  platform?: string;
  platformShopId?: string;
  shopId?: string;
  orderNo?: string;
  platformStatus?: string;
  reconciliationStatus?: string;
  start?: string;
  end?: string;
};

export async function listPlatformAfterSaleReconciliation(
  params: PlatformAfterSaleListParams,
) {
  return getWithParams<{
    list: PlatformAfterSale[];
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  }>('/api/v1/sales-return-reconciliation', params);
}

export async function getPlatformAfterSaleReconciliation(id: string) {
  return getJSON<PlatformAfterSale>(
    `/api/v1/sales-return-reconciliation/${enc(id)}`,
  );
}

export function extractSalesReturnAPIError(
  error: unknown,
): SalesReturnAPIError {
  if (error instanceof ApiRequestError)
    return { code: error.code, message: error.message, traceId: error.traceId };
  if (error instanceof Error) return { code: -1, message: error.message };
  return { code: -1, message: 'request_failed' };
}

export function salesReturnErrorMessage(
  error: SalesReturnAPIError,
  fallback = '操作失败，请稍后重试。',
) {
  const message = error.message.toLowerCase();
  if (message.includes('sales return revision conflict'))
    return '售后单已被其他人更新，请确认最新状态。';
  if (message.includes('sales return transition'))
    return '当前售后单状态不允许执行此操作。';
  if (message.includes('sales return idempotency'))
    return '相同操作编号已用于其他内容，请关闭窗口后重新发起。';
  if (message.includes('exceeds deducted'))
    return '退货数量超过订单明细的剩余可退数量，请重新核对。';
  if (message.includes('approver cannot receive'))
    return '审批人与退货收货人必须为不同账号。';
  if (message.includes('warehouse unavailable'))
    return '原订单仓库当前不可用，售后单尚未执行。';
  if (message.includes('refund execution revision conflict'))
    return '退款执行单已被其他人更新，请确认最新状态。';
  if (message.includes('refund execution transition'))
    return '当前退款执行状态不允许执行此操作。';
  if (message.includes('refund execution idempotency'))
    return '相同退款操作编号已用于其他内容，请关闭窗口后重新发起。';
  if (message.includes('refund execution already exists'))
    return '该售后单已创建退款执行单，请前往退款执行工作台查看。';
  if (message.includes('approver cannot record refund'))
    return '售后审批人与退款结果登记人必须为不同账号。';
  if (message.includes('platform refund fact does not match'))
    return '平台退款事实与本地售后金额、币种或类型不一致，未执行确认。';
  if (message.includes('platform refund fact is not final'))
    return '平台退款状态尚未终结，请等待新的平台事实后再确认。';
  if (error.code === 403 || error.code === 40301)
    return '当前账号没有执行此操作的权限。';
  return fallback;
}

export function formatSalesReturnAmount(amountMinor: number, currency: string) {
  return `${currency || 'CNY'} ${(Number(amountMinor || 0) / 100).toFixed(2)}`;
}
