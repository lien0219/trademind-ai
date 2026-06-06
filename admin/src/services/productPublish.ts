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
  skuMappingsSummary?: string[];
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
