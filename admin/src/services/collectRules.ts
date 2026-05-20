import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from './request';

export type CollectRuleRow = {
  id: string;
  name: string;
  source: string;
  domain: string;
  matchPattern?: string;
  status: string;
  priority: number;
  remark?: string;
  createdAt: string;
  updatedAt: string;
};

export type CollectRuleDetail = CollectRuleRow & {
  rule: unknown;
};

export type Pagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export async function queryCollectRules(params: {
  page?: number;
  pageSize?: number;
  name?: string;
  domain?: string;
  status?: string;
}) {
  return getWithParams<{ list: CollectRuleRow[]; pagination: Pagination }>('/api/v1/collect/rules', {
    page: params.page,
    pageSize: params.pageSize,
    name: params.name,
    domain: params.domain,
    status: params.status,
  });
}

export async function createCollectRule(payload: {
  name: string;
  domain: string;
  matchPattern?: string;
  priority?: number;
  status?: string;
  remark?: string;
  rule: unknown;
}) {
  return postJSON<CollectRuleDetail>('/api/v1/collect/rules', payload);
}

export async function getCollectRule(id: string) {
  return getJSON<CollectRuleDetail>(`/api/v1/collect/rules/${id}`);
}

export async function updateCollectRule(
  id: string,
  payload: {
    name?: string;
    domain?: string;
    matchPattern?: string;
    priority?: number;
    status?: string;
    remark?: string;
    rule?: unknown;
  },
) {
  return putJSON<CollectRuleDetail>(`/api/v1/collect/rules/${id}`, payload);
}

export async function deleteCollectRule(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/collect/rules/${id}`);
}

export async function enableCollectRule(id: string) {
  return postJSON<CollectRuleDetail>(`/api/v1/collect/rules/${id}/enable`, {});
}

export async function disableCollectRule(id: string) {
  return postJSON<CollectRuleDetail>(`/api/v1/collect/rules/${id}/disable`, {});
}

export type CollectRuleTestResult = {
  accessStatus: string;
  finalUrl: string;
  httpStatus?: number;
  extractedFields?: {
    title?: boolean;
    price?: boolean;
    mainImage?: boolean;
    detailImagesCount?: number;
    attributesCount?: number;
  };
  missingFields?: string[];
  warnings?: string[];
  errorCode?: string;
  suggestion?: string;
  product?: unknown;
};

export async function testCollectRule(
  id: string,
  payload: { url: string; profileId?: string; useBrowserProfile?: boolean },
) {
  return postJSON<CollectRuleTestResult>(`/api/v1/collect/rules/${id}/test`, payload);
}
