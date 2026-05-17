import { getWithParams, getJSON, postJSON } from '@/services/request';

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
  shopName?: string;
  productTitle?: string;
  platform: string;
  taskType: string;
  status: string;
  mode: string;
  startedAt?: string;
  finishedAt?: string;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function publishProduct(
  productId: string,
  body: { shopId: string; options?: Record<string, unknown> },
): Promise<ProductPublishTaskDTO> {
  return postJSON(`/api/v1/products/${productId}/publish`, body);
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
