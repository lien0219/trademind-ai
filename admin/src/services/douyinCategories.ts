import { getJSON, getWithParams, postJSON } from '@/services/request';

export type DouyinCategoryNode = {
  id: string;
  categoryId: string;
  parentId?: string;
  name: string;
  level: number;
  isLeaf: boolean;
  status?: string;
  path: string;
  syncedAt?: string;
  children?: DouyinCategoryNode[];
};

export type DouyinCategoryAttribute = {
  id: string;
  categoryId: string;
  attrId: string;
  name: string;
  required: boolean;
  valueType?: string;
  options?: { id?: string; name: string }[];
  unitOptions?: { id?: string; name: string }[];
  syncedAt?: string;
};

export async function queryDouyinCategories(params?: {
  keyword?: string;
  parentId?: string;
  onlyLeaf?: boolean;
  refresh?: boolean;
  shopId?: string;
}): Promise<{
  list: DouyinCategoryNode[];
  flat: DouyinCategoryNode[];
  total: number;
  leafCount: number;
  lastSyncedAt?: string;
}> {
  return getWithParams('/api/v1/platform/douyin/categories', {
    keyword: params?.keyword || undefined,
    parentId: params?.parentId || undefined,
    onlyLeaf: params?.onlyLeaf ? '1' : undefined,
    refresh: params?.refresh ? '1' : undefined,
    shopId: params?.shopId || undefined,
  });
}

export async function syncDouyinCategories(shopId: string): Promise<{
  count: number;
  leafCount: number;
  lastSyncedAt?: string;
}> {
  return postJSON('/api/v1/platform/douyin/categories/sync', { shopId });
}

export async function getDouyinCategoryStats(): Promise<{
  count: number;
  leafCount: number;
  lastSyncedAt?: string;
}> {
  return getJSON('/api/v1/platform/douyin/categories/stats');
}

export async function queryDouyinCategoryAttributes(categoryId: string): Promise<{
  list: DouyinCategoryAttribute[];
}> {
  return getJSON(`/api/v1/platform/douyin/categories/${encodeURIComponent(categoryId)}/attributes`);
}

export async function syncDouyinCategoryAttributes(
  categoryId: string,
  shopId: string,
): Promise<{ list: DouyinCategoryAttribute[] }> {
  return postJSON(`/api/v1/platform/douyin/categories/${encodeURIComponent(categoryId)}/attributes/sync`, { shopId });
}
