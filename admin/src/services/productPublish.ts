import { request } from '@umijs/max';
import type { ApiResponse } from '@/services/request';
import { getWithParams, getJSON, postJSON } from '@/services/request';
import type { ProductReadinessResult } from '@/services/productReadiness';

export type PublishTargetShop = {
  shopId: string;
  shopName: string;
  authStatus: string;
  authStatusLabel?: string;
  enabled: boolean;
};

export type PublishTargetPlatform = {
  platform: string;
  platformLabel: string;
  capability: string;
  capabilityLabel: string;
  shops: PublishTargetShop[];
  settingsGroupKey?: string;
  settingsPath?: string;
};

export type PublishTargetsResponse = {
  productId: string;
  platforms: PublishTargetPlatform[];
};

export type PublishTargetRef = {
  platform: string;
  shopId?: string | null;
};

export type PublishTargetIssue = {
  code: string;
  title: string;
  message: string;
  severity: string;
  suggestion?: string;
  technicalDetails?: Record<string, unknown>;
};

export type PublishTargetCheckResult = {
  targetKey: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
  capability: string;
  status: string;
  statusLabel: string;
  canCreateDraft: boolean;
  issues: PublishTargetIssue[];
};

export type PublishTargetsCheckResponse = {
  summary: {
    targetCount: number;
    readyCount: number;
    warningCount: number;
    blockedCount: number;
  };
  targets: PublishTargetCheckResult[];
};

export type PublishTargetTaskResult = {
  targetKey: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
  taskId?: string;
  publicationId?: string;
  status: string;
  statusLabel: string;
  capability: string;
  localDraftOnly?: boolean;
  errorCode?: string;
  errorMessage?: string;
  platformProductId?: string;
};

export type PublishTargetsCreateDraftsResponse = {
  batchId: string;
  status: string;
  statusLabel: string;
  targetCount: number;
  successCount: number;
  failedCount: number;
  skippedCount: number;
  targets: PublishTargetTaskResult[];
};

export async function fetchPublishTargets(productId: string): Promise<PublishTargetsResponse> {
  return getJSON(`/api/v1/products/${encodeURIComponent(productId)}/publish-targets`);
}

export async function checkPublishTargets(
  productId: string,
  body: {
    targets: PublishTargetRef[];
    commonConfig?: Record<string, unknown>;
    targetConfigs?: Record<string, unknown>;
  },
): Promise<PublishTargetsCheckResponse> {
  return postJSON(`/api/v1/products/${encodeURIComponent(productId)}/publish-targets/check`, body);
}

export async function createPublishTargetDrafts(
  productId: string,
  body: {
    targets: PublishTargetRef[];
    commonConfig?: Record<string, unknown>;
    targetConfigs?: Record<string, unknown>;
    onlyReady?: boolean;
    retryFailedOnly?: boolean;
    batchId?: string;
    force?: boolean;
  },
): Promise<PublishTargetsCreateDraftsResponse> {
  return postJSON(`/api/v1/products/${encodeURIComponent(productId)}/publish-targets/create-drafts`, body);
}

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
