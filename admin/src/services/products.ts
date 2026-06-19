import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from './request';

export type ProductListRow = {
  id: string;
  tenantId?: number;
  createdBy?: string;
  source: string;
  sourceUrl?: string;
  title: string;
  status: string;
  currency?: string;
  createdAt: string;
  updatedAt?: string;
  coverUrl?: string;
  operationProgress?: ProductOperationProgressSummary;
};

export type Pagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export async function fetchProducts(params: {
  page?: number;
  pageSize?: number;
  status?: string;
  source?: string;
  keyword?: string;
  operationStep?: string;
  missingAiTitle?: boolean;
  missingAiDescription?: boolean;
  readinessBlocked?: boolean;
  publishable?: boolean;
}) {
  return getWithParams<{ list: ProductListRow[]; pagination: Pagination }>('/api/v1/products', {
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
    source: params.source,
    keyword: params.keyword,
    operationStep: params.operationStep,
    missingAiTitle: params.missingAiTitle ? '1' : undefined,
    missingAiDescription: params.missingAiDescription ? '1' : undefined,
    readiness: params.readinessBlocked ? 'blocked' : undefined,
    publishable: params.publishable ? '1' : undefined,
  });
}

export type ProductOperationProgressSummary = {
  completionPercent: number;
  currentStep: string;
  currentStepLabel: string;
  nextActionLabel: string;
  nextActionKey: string;
  nextActionUrl?: string;
  blockerCount: number;
  warningCount: number;
  publishReady: boolean;
};

export type ProductOperationIssue = {
  code: string;
  title: string;
  message: string;
  severity: 'warning' | 'failed' | string;
  actionLabel?: string;
  actionKey?: string;
  actionUrl?: string;
};

export type ProductOperationProgress = ProductOperationProgressSummary & {
  productId: string;
  nextActionUrl?: string;
  completedSteps: string[];
  pendingSteps: string[];
  blockers: ProductOperationIssue[];
  warnings: Array<{ code: string; title: string; message: string }>;
  updatedAt: string;
  stepStatus?: Record<string, string>;
};

export type ProductImageRow = {
  id: string;
  productId: string;
  imageType: string;
  source?: string;
  sourceTaskId?: string;
  originalImageId?: string;
  originUrl: string;
  objectKey?: string;
  storageKey?: string;
  publicUrl: string;
  score?: number;
  isBestMain?: boolean;
  sortOrder: number;
};

export type ProductSKURow = {
  id: string;
  productId: string;
  skuCode: string;
  skuName: string;
  attrs?: Record<string, unknown>;
  price?: number;
  costPrice?: number;
  compareAtPrice?: number;
  minPublishPrice?: number;
  stock?: number;
  warningStock?: number;
  safetyStock?: number;
  stockStatus?: string;
  imageUrl?: string;
};

export type ProductDetail = {
  id: string;
  tenantId: number;
  createdBy?: string;
  source: string;
  sourceUrl: string;
  originalTitle: string;
  title: string;
  aiTitle?: string;
  description: string;
  aiDescription?: string;
  currency: string;
  status: string;
  rawData?: unknown;
  raw?: unknown;
  mainImages?: string[];
  descriptionImages?: string[];
  attributes?: unknown;
  skuGroups?: unknown;
  costPrice?: number;
  salePrice?: number;
  stock?: number;
  collectWarnings?: string[];
  publishStatus?: string;
  createdAt: string;
  updatedAt: string;
  images: ProductImageRow[];
  skus: ProductSKURow[];
};

export type ProductPlatformPublishConfig = {
  productId: string;
  platform: string;
  shopId?: string;
  categoryId?: string;
  categoryPath?: string;
  platformAttributes?: Record<string, unknown>;
  mapping?: DouyinDraftMapping;
  lastMappedAt?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type DouyinMappingIssue = {
  code: string;
  level: 'warning' | 'error' | string;
  message: string;
  suggestion?: string;
  field?: string;
  relatedResourceType?: string;
  relatedResourceId?: string;
};

export type DouyinDraftImage = {
  localImageId?: string;
  sourceUrl?: string;
  storageUrl?: string;
  platformImageId?: string;
  platformImageUrl?: string;
  imageType: string;
  url: string;
  originUrl?: string;
  publicUrl?: string;
  objectKey?: string;
  storageKey?: string;
  source?: string;
  status: string;
  needSync: boolean;
  uploadStatus?: 'pending' | 'processing' | 'uploaded' | 'failed' | 'skipped' | string;
  errorCode?: string;
  errorMessage?: string;
  uploadedAt?: string;
  processed?: boolean;
  raw?: Record<string, unknown>;
};

export type DouyinDraftAttribute = {
  attrId: string;
  name: string;
  required?: boolean;
  valueType?: string;
  value?: unknown;
  options?: unknown[];
};

export type DouyinDraftSku = {
  localSkuId?: string;
  name: string;
  attrs?: Record<string, unknown>;
  price: number;
  stock?: number | null;
  imageUrl?: string;
  platformSkuDraft?: Record<string, unknown>;
};

export type DouyinDraftMapping = {
  platform: 'douyin_shop' | string;
  productId?: string;
  source?: string;
  shopId?: string;
  categoryId?: string;
  categoryPath?: string;
  title?: string;
  description?: string;
  mainImages?: DouyinDraftImage[];
  detailImages?: DouyinDraftImage[];
  attributes?: DouyinDraftAttribute[];
  skus?: DouyinDraftSku[];
  price?: {
    currency?: string;
    min?: number;
    max?: number;
    costMin?: number;
    source?: string;
  };
  stock?: {
    total?: number;
    min?: number;
    unconfirmed?: boolean;
  };
  warnings?: DouyinMappingIssue[];
  errors?: DouyinMappingIssue[];
  lastMappedAt?: string;
  platformDraftHint?: Record<string, unknown>;
};

export type DouyinMappingValidationResult = {
  productId?: string;
  platform: string;
  status: string;
  result: string;
  canPublish: boolean;
  errorCount: number;
  warningCount: number;
  checks: DouyinMappingIssue[];
};

export type DouyinImageUploadResult = {
  productId: string;
  platform: string;
  summary: {
    uploaded: number;
    skipped: number;
    failed: number;
    pending: number;
  };
  mapping: DouyinDraftMapping;
};

export async function fetchProductDetail(id: string) {
  return getJSON<ProductDetail>(`/api/v1/products/${id}`);
}

export async function fetchProductOperationProgress(id: string) {
  return getJSON<ProductOperationProgress>(`/api/v1/products/${encodeURIComponent(id)}/operation-progress`);
}

export async function getProductPlatformPublishConfig(productId: string, platform: string) {
  return getJSON<ProductPlatformPublishConfig>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/${encodeURIComponent(platform)}`,
  );
}

export async function putProductPlatformPublishConfig(
  productId: string,
  platform: string,
  body: {
    shopId?: string;
    categoryId?: string;
    categoryPath?: string;
    platformAttributes?: Record<string, unknown>;
  },
) {
  return putJSON<ProductPlatformPublishConfig, typeof body>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/${encodeURIComponent(platform)}`,
    body,
  );
}

export async function buildDouyinDraftMapping(productId: string, body: { shopId?: string } = {}) {
  return postJSON<DouyinDraftMapping>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/build-mapping`,
    body,
  );
}

export async function getDouyinDraftMapping(productId: string) {
  return getJSON<DouyinDraftMapping>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/mapping`,
  );
}

export async function saveDouyinDraftMapping(productId: string, body: DouyinDraftMapping) {
  return putJSON<DouyinDraftMapping, DouyinDraftMapping>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/mapping`,
    body,
  );
}

export async function validateDouyinDraftMapping(productId: string, body?: DouyinDraftMapping) {
  return postJSON<DouyinMappingValidationResult>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/validate`,
    body,
  );
}

export async function uploadDouyinImages(
  productId: string,
  body: { imageTypes?: string[]; retryFailed?: boolean; force?: boolean } = {},
) {
  return postJSON<DouyinImageUploadResult>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/images/upload`,
    body,
  );
}

export async function retryDouyinImage(productId: string, imageKey: string) {
  return postJSON<DouyinImageUploadResult>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/images/${encodeURIComponent(imageKey)}/retry`,
    {},
  );
}

export async function getDouyinImageStatus(productId: string) {
  return getJSON<DouyinImageUploadResult>(
    `/api/v1/products/${encodeURIComponent(productId)}/platform-configs/douyin_shop/images/status`,
  );
}

export type UpdateProductBody = {
  title?: string;
  originalTitle?: string;
  original_title?: string;
  aiTitle?: string;
  ai_title?: string;
  description?: string;
  aiDescription?: string;
  ai_description?: string;
  currency?: string;
  status?: string;
};

export type CreateProductSkuBody = {
  skuCode?: string;
  skuName: string;
  attrs?: Record<string, unknown> | string;
  price?: number;
  stock?: number;
  imageUrl?: string;
};

export type UpdateProductSkuBody = {
  skuCode?: string;
  skuName?: string;
  attrs?: Record<string, unknown> | string | null;
  price?: number | null;
  stock?: number | null;
  imageUrl?: string | null;
};

export type CreateProductImageBody = {
  fileId?: string;
  objectKey?: string;
  originUrl?: string;
  publicUrl?: string;
  imageType: string;
  sortOrder?: number;
};

export type UpdateProductImageBody = {
  imageType?: string;
  objectKey?: string;
  originUrl?: string;
  publicUrl?: string;
  sortOrder?: number;
};

export type ReorderProductImagesBody = {
  imageIds: string[];
};

export async function updateProduct(id: string, body: UpdateProductBody) {
  return putJSON<ProductDetail, UpdateProductBody>(`/api/v1/products/${id}`, body);
}

export type ProductCreateBody = {
  tenantId?: number;
  source?: string;
  sourceUrl?: string;
  originalTitle?: string;
  title: string;
  description?: string;
  currency?: string;
  status?: string;
  rawData?: unknown;
};

export async function createProduct(body: ProductCreateBody) {
  return postJSON<ProductDetail>('/api/v1/products', body);
}

export async function createProductSku(productId: string, body: CreateProductSkuBody) {
  const payload = normalizeSkuBody(body);
  return postJSON<ProductSKURow>(`/api/v1/products/${productId}/skus`, payload);
}

export async function updateProductSku(productId: string, skuId: string, body: UpdateProductSkuBody) {
  const payload = normalizeSkuUpdateBody(body);
  return putJSON<ProductSKURow, Record<string, unknown>>(
    `/api/v1/products/${productId}/skus/${skuId}`,
    payload,
  );
}

export async function updateProductSkuStockSettings(
  productId: string,
  skuId: string,
  body: { warningStock: number; safetyStock: number },
) {
  return putJSON<ProductSKURow, typeof body>(`/api/v1/products/${productId}/skus/${skuId}/stock-settings`, body);
}

export async function deleteProductSku(productId: string, skuId: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/products/${productId}/skus/${skuId}`);
}

export async function createProductImage(productId: string, body: CreateProductImageBody) {
  return postJSON<ProductImageRow>(`/api/v1/products/${productId}/images`, body);
}

export async function updateProductImage(productId: string, imageId: string, body: UpdateProductImageBody) {
  return putJSON<ProductImageRow, UpdateProductImageBody>(
    `/api/v1/products/${productId}/images/${imageId}`,
    body,
  );
}

export async function deleteProductImage(productId: string, imageId: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/products/${productId}/images/${imageId}`);
}

export async function reorderProductImages(productId: string, body: ReorderProductImagesBody) {
  return postJSON<{ ok: boolean }>(`/api/v1/products/${productId}/images/reorder`, body);
}

export type SyncProductImagesResult = {
  synced: number;
  skipped: number;
  failed: number;
  errors?: string[];
};

export async function syncProductImages(
  productId: string,
  body: { scope?: 'all' | 'main' | 'detail' } = {},
) {
  return postJSON<SyncProductImagesResult>(`/api/v1/products/${productId}/sync-images`, body);
}

function attrsToJSON(attrs?: Record<string, unknown> | string | null): object | string | undefined {
  if (attrs === undefined) return undefined;
  if (attrs === null) return null;
  if (typeof attrs === 'string') {
    const t = attrs.trim();
    if (!t) return {};
    try {
      return JSON.parse(t) as object;
    } catch {
      throw new Error('attrs 需为合法 JSON');
    }
  }
  return attrs;
}

function normalizeSkuBody(body: CreateProductSkuBody): Record<string, unknown> {
  const attrs = attrsToJSON(body.attrs);
  return {
    skuCode: body.skuCode ?? '',
    skuName: body.skuName,
    ...(attrs !== undefined ? { attrs } : {}),
    price: body.price,
    stock: body.stock,
    imageUrl: body.imageUrl ?? '',
  };
}

function normalizeSkuUpdateBody(body: UpdateProductSkuBody): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (body.skuCode !== undefined) out.skuCode = body.skuCode;
  if (body.skuName !== undefined) out.skuName = body.skuName;
  if (body.price !== undefined) out.price = body.price;
  if (body.stock !== undefined) out.stock = body.stock;
  if (body.imageUrl !== undefined) out.imageUrl = body.imageUrl;
  if (body.attrs !== undefined) {
    if (body.attrs === null) out.attrs = null;
    else out.attrs = attrsToJSON(body.attrs);
  }
  return out;
}

export async function deleteProduct(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/products/${id}`);
}

export type OptimizeTitleResult = {
  optimizedTitle: string;
  keywords: string[];
  reason: string;
  taskId: string;
};

export async function optimizeProductTitle(
  id: string,
  body: { language?: string; platform?: string; maxLength?: number },
) {
  return postJSON<OptimizeTitleResult>(`/api/v1/products/${id}/ai/optimize-title`, body);
}

export async function applyProductAITitle(
  id: string,
  body: { aiTitle: string; taskId: string; expectedUpdatedAt?: string; sourceSnapshotHash?: string },
) {
  return postJSON<ProductDetail>(`/api/v1/products/${id}/apply-ai-title`, body);
}

export async function undoProductAITitle(
  id: string,
  body: { applicationId?: string; expectedUpdatedAt?: string } = {},
) {
  return postJSON<ProductDetail>(`/api/v1/products/${id}/undo-ai-title`, body);
}

export type GenerateDescriptionResult = {
  description: string;
  highlights: string[];
  specifications: string[];
  packageIncludes: string[];
  notes: string;
  reason: string;
  taskId: string;
};

export async function generateDescription(
  id: string,
  body: { language?: string; platform?: string; tone?: string },
) {
  return postJSON<GenerateDescriptionResult>(`/api/v1/products/${id}/ai/generate-description`, body);
}

export async function applyAiDescription(
  id: string,
  body: { aiDescription: string; taskId: string; expectedUpdatedAt?: string; sourceSnapshotHash?: string },
) {
  return postJSON<ProductDetail>(`/api/v1/products/${id}/apply-ai-description`, body);
}

export async function undoAiDescription(
  id: string,
  body: { applicationId?: string; expectedUpdatedAt?: string } = {},
) {
  return postJSON<ProductDetail>(`/api/v1/products/${id}/undo-ai-description`, body);
}

export type AITaskRow = {
  id: string;
  taskType: string;
  provider: string;
  model: string;
  promptCode: string;
  status: string;
  errorMessage?: string;
  tokenInput: number;
  tokenOutput: number;
  costAmount: number;
  productId?: string;
  createdBy?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export async function fetchProductAITasks(id: string) {
  return getJSON<{ list: AITaskRow[] }>(`/api/v1/products/${id}/ai/tasks`);
}

export async function selectBestMainProductImages(
  productId: string,
  payload?: { mode?: 'score_only' | 'recommend' | 'auto_set' },
) {
  return postJSON(`/api/v1/products/${productId}/images/select-best-main`, payload ?? {});
}

export type ProductSkuSearchHit = {
  productId: string;
  productTitle: string;
  productSkuId: string;
  skuCode: string;
  skuName?: string;
  stock?: number;
  attrs?: Record<string, unknown> | unknown;
};

export async function searchProductSkus(params: { keyword?: string; productId?: string; limit?: number }) {
  return getWithParams<{ list: ProductSkuSearchHit[] }>('/api/v1/product-skus/search', {
    keyword: params.keyword,
    productId: params.productId,
    limit: params.limit,
  });
}
