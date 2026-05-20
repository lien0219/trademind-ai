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
    missingAiTitle: params.missingAiTitle ? '1' : undefined,
    missingAiDescription: params.missingAiDescription ? '1' : undefined,
    readiness: params.readinessBlocked ? 'blocked' : undefined,
    publishable: params.publishable ? '1' : undefined,
  });
}

export type ProductImageRow = {
  id: string;
  productId: string;
  imageType: string;
  originUrl: string;
  objectKey?: string;
  publicUrl: string;
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
  createdAt: string;
  updatedAt: string;
  images: ProductImageRow[];
  skus: ProductSKURow[];
};

export async function fetchProductDetail(id: string) {
  return getJSON<ProductDetail>(`/api/v1/products/${id}`);
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

export async function applyProductAITitle(id: string, body: { aiTitle: string; taskId: string }) {
  return postJSON<ProductDetail>(`/api/v1/products/${id}/apply-ai-title`, body);
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

export async function applyAiDescription(id: string, body: { aiDescription: string; taskId: string }) {
  return postJSON<ProductDetail>(`/api/v1/products/${id}/apply-ai-description`, body);
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
