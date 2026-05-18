import { deleteJSON, getWithParams, postJSON } from '@/services/request';

export type OrderExceptionSummary = {
  totalOpen: number;
  skuUnmatched: number;
  skuAmbiguous: number;
  insufficientStock: number;
  inventoryDeductFailed: number;
  inventoryRestoreFailed: number;
  inventorySyncFailed: number;
};

export type OrderExceptionRow = {
  id: string;
  exceptionType: string;
  severity: string;
  status: string;
  sourceType: string;
  sourceId: string;
  orderId?: string;
  orderNo?: string;
  externalOrderId?: string;
  platform?: string;
  shopId?: string;
  shopName?: string;
  orderItemId?: string;
  externalItemId?: string;
  externalSkuId?: string;
  skuCode?: string;
  skuName?: string;
  productId?: string;
  productSkuId?: string;
  productTitle?: string;
  localSkuCode?: string;
  quantity?: number;
  errorMessage?: string;
  suggestedAction?: string;
  createdAt: string;
  updatedAt: string;
  detailUrl?: string;
  handled: boolean;
  ignored: boolean;
};

export type ListOrderExceptionsResponse = {
  list: OrderExceptionRow[];
  total: number;
  summary: OrderExceptionSummary;
};

/** POST .../bind-sku 摘要（与后端 map 对齐） */
export type OrderExceptionBindSkuResult = {
  bind?: string;
  inventoryDeduction?: Record<string, unknown>;
  inventoryDeductionError?: string;
  inventorySyncTasks?: string;
};

export async function queryOrderExceptions(params: {
  page?: number;
  pageSize?: number;
  exceptionType?: string;
  severity?: string;
  platform?: string;
  shopId?: string;
  orderId?: string;
  keyword?: string;
  handled?: boolean;
  ignored?: boolean;
  start?: string;
  end?: string;
}): Promise<ListOrderExceptionsResponse> {
  return getWithParams<ListOrderExceptionsResponse>('/api/v1/orders/exceptions', {
    page: params.page,
    pageSize: params.pageSize,
    exceptionType: params.exceptionType,
    severity: params.severity,
    platform: params.platform,
    shopId: params.shopId,
    orderId: params.orderId,
    keyword: params.keyword,
    handled: params.handled === undefined ? undefined : params.handled ? 'true' : 'false',
    ignored: params.ignored === undefined ? undefined : params.ignored ? 'true' : 'false',
    start: params.start,
    end: params.end,
  });
}

export async function postOrderExceptionHandle(
  sourceType: string,
  sourceId: string,
  body: { exceptionType: string; remark?: string },
) {
  return postJSON(`/api/v1/orders/exceptions/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}/handle`, body);
}

export async function postOrderExceptionIgnore(
  sourceType: string,
  sourceId: string,
  body: { exceptionType: string; remark?: string },
) {
  return postJSON(`/api/v1/orders/exceptions/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}/ignore`, body);
}

export async function deleteOrderExceptionMark(sourceType: string, sourceId: string) {
  return deleteJSON(`/api/v1/orders/exceptions/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}/mark`);
}

export async function postOrderExceptionBindSku(
  sourceType: string,
  sourceId: string,
  body: {
    exceptionType: string;
    productSkuId: string;
    deductInventory?: boolean | null;
    syncInventory?: boolean | null;
    autoMarkHandled?: boolean | null;
    candidateConfidence?: number | null;
    candidateSource?: string;
  },
): Promise<OrderExceptionBindSkuResult> {
  return postJSON(`/api/v1/orders/exceptions/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}/bind-sku`, body);
}

export async function postOrderExceptionRetryDeduct(sourceType: string, sourceId: string, syncPlatforms?: boolean) {
  return postJSON(`/api/v1/orders/exceptions/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}/retry-deduct`, {
    syncPlatforms: !!syncPlatforms,
  });
}

export async function postOrderExceptionRetryInventorySync(sourceType: string, sourceId: string) {
  return postJSON(
    `/api/v1/orders/exceptions/${encodeURIComponent(sourceType)}/${encodeURIComponent(sourceId)}/retry-inventory-sync`,
    {},
  );
}
