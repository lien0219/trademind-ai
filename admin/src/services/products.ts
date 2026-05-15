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
}) {
  return getWithParams<{ list: ProductListRow[]; pagination: Pagination }>('/api/v1/products', {
    page: params.page,
    pageSize: params.pageSize,
    status: params.status,
    source: params.source,
    keyword: params.keyword,
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
  stock?: number;
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

export async function updateProduct(id: string, body: Record<string, unknown>) {
  return putJSON<ProductDetail, Record<string, unknown>>(`/api/v1/products/${id}`, body);
}

export async function deleteProduct(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/products/${id}`);
}
