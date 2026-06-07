import { request } from '@umijs/max';
import type { ApiResponse } from '@/services/request';
import { getWithParams, getJSON, postJSON } from '@/services/request';
import type { ProductReadinessResult } from '@/services/productReadiness';

export type ProductPublicationRow = {
  id: string;
  productId: string;
  shopId: string;
  shopName?: string;
  platform: string;
  publishTaskId?: string;
  externalProductId?: string;
  externalUrl?: string;
  status: string;
  publishStatus: string;
  publishedAt?: string;
  lastSyncedAt?: string;
  skuBindingSyncedAt?: string;
  skuMappingsSummary?: string[];
};

export type DouyinSkuBindingRow = {
  publicationSkuId: string;
  productSkuId?: string;
  skuCode?: string;
  specName?: string;
  externalSkuId?: string;
  platformSkuName?: string;
  bindStatus?: string;
  bindConfidence?: number;
  bindMessage?: string;
  lastSyncedAt?: string;
  price?: number;
  stock?: number;
};

export type DouyinPlatformSkuCandidate = {
  platformSkuId: string;
  specName?: string;
  priceYuan?: number;
  stock?: number;
  boundToPublicationSkuId?: string;
};

export type DouyinSkuBindingSummary = {
  publicationId: string;
  externalProductId?: string;
  skuBindingSyncedAt?: string;
  total: number;
  bound: number;
  skipped: number;
  unmatched: number;
  ambiguous: number;
  failed: number;
  rows: DouyinSkuBindingRow[];
  platformSkus?: DouyinPlatformSkuCandidate[];
  inventorySyncReady?: boolean;
  inventorySyncBlockReason?: string;
  errorCode?: string;
  errorMessage?: string;
};

export type ProductPublishTaskDTO = {
  id: string;
  productId: string;
  shopId: string;
  targetStoreId?: string;
  shopName?: string;
  productTitle?: string;
  platform: string;
  targetPlatform?: string;
  taskType: string;
  status: string;
  publishStatus?: string;
  mode: string;
  publishMode?: string;
  title?: string;
  description?: string;
  images?: unknown;
  skus?: unknown;
  price?: number;
  currency?: string;
  checkResult?: unknown;
  platformPayload?: unknown;
  platformResult?: unknown;
  platformProductId?: string;
  platformRawError?: unknown;
  retryable?: boolean;
  requestId?: string;
  mappingSnapshot?: unknown;
  startedAt?: string;
  finishedAt?: string;
  errorCode?: string;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  readiness?: ProductReadinessResult;
};

export async function publishProduct(
  productId: string,
  body: { shopId: string; options?: Record<string, unknown>; force?: boolean },
): Promise<ProductPublishTaskDTO> {
  const res = await request<ApiResponse<ProductPublishTaskDTO>>(`/api/v1/products/${encodeURIComponent(productId)}/publish`, {
    method: 'POST',
    data: body,
  });
  if (res.code !== 0) {
    const err = new Error(res.message || 'publish_failed') as Error & { businessCode?: number; data?: unknown };
    err.businessCode = res.code;
    err.data = res.data;
    throw err;
  }
  return res.data as ProductPublishTaskDTO;
}

export async function listProductPublications(productId: string): Promise<{ list: ProductPublicationRow[] }> {
  return getJSON(`/api/v1/products/${productId}/publications`);
}

export async function queryProductPublishTasks(params: {
  page?: number;
  pageSize?: number;
  productId?: string;
  shopId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}): Promise<{
  list: ProductPublishTaskDTO[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/product-publish/tasks', params);
}

export async function getProductPublishTask(id: string): Promise<ProductPublishTaskDTO> {
  return getJSON(`/api/v1/product-publish/tasks/${id}`);
}

export async function retryProductPublishTask(id: string): Promise<ProductPublishTaskDTO> {
  return postJSON(`/api/v1/product-publish/tasks/${id}/retry`, {});
}

export async function cancelProductPublishTask(id: string): Promise<ProductPublishTaskDTO> {
  return postJSON(`/api/v1/product-publish/tasks/${id}/cancel`, {});
}

export async function createDouyinProductDraft(
  productId: string,
  body: { shopId: string; publishMode?: string; force?: boolean },
): Promise<ProductPublishTaskDTO> {
  const res = await request<ApiResponse<ProductPublishTaskDTO>>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/create-draft`,
    { method: 'POST', data: { publishMode: 'save_as_platform_draft', ...body } },
  );
  if (res.code !== 0) {
    const err = new Error(res.message || 'create_draft_failed') as Error & { businessCode?: number; data?: unknown };
    err.businessCode = res.code;
    err.data = res.data;
    throw err;
  }
  return res.data as ProductPublishTaskDTO;
}

export async function getDouyinSkuBindings(publicationId: string): Promise<DouyinSkuBindingSummary> {
  return getJSON(`/api/v1/product-publications/${encodeURIComponent(publicationId)}/douyin/sku-bindings`);
}

export async function syncDouyinSkuBindings(publicationId: string): Promise<DouyinSkuBindingSummary> {
  return postJSON(`/api/v1/product-publications/${encodeURIComponent(publicationId)}/douyin/sync-sku-bindings`, {});
}

export async function bindDouyinSku(
  publicationSkuId: string,
  body: { platformSkuId: string; platformSkuName?: string; bindReason?: string },
): Promise<DouyinSkuBindingRow> {
  return postJSON(`/api/v1/product-publication-skus/${encodeURIComponent(publicationSkuId)}/douyin/bind-sku`, {
    bindReason: 'manual',
    ...body,
  });
}

export async function unbindDouyinSku(
  publicationSkuId: string,
  body?: { reason?: string },
): Promise<DouyinSkuBindingRow> {
  return postJSON(`/api/v1/product-publication-skus/${encodeURIComponent(publicationSkuId)}/douyin/unbind-sku`, {
    reason: body?.reason ?? 'manual_unbind',
  });
}

export async function listDouyinPublishTasks(
  productId: string,
  params?: { page?: number; pageSize?: number },
): Promise<{
  list: ProductPublishTaskDTO[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams(`/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/publish-tasks`, params ?? {});
}
