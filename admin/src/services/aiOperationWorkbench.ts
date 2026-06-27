import { getWithParams } from './request';
import { request } from '@umijs/max';

export type WorkbenchSummary = {
  aiTextReviewCount: number;
  aiImageReviewCount: number;
  publishCheckIssueCount: number;
  publishTaskIssueCount: number;
  todayResolvedCount: number;
  highPriorityCount: number;
  aiTextReviewHighPriority?: number;
  aiImageReviewHighPriority?: number;
  publishCheckHighPriority?: number;
  publishTaskIssueHighPriority?: number;
  aiTextReviewTodayNew?: number;
  aiImageReviewTodayNew?: number;
  publishCheckTodayNew?: number;
  publishTaskIssueTodayNew?: number;
};

export type WorkbenchTodoItem = {
  id: string;
  type: string;
  typeLabel: string;
  priority: string;
  priorityLabel: string;
  productId?: string;
  productTitle?: string;
  platform?: string;
  platformLabel?: string;
  shopId?: string;
  shopName?: string;
  title: string;
  message: string;
  actionLabel: string;
  actionUrl: string;
  sourceType: string;
  sourceId: string;
  issueCode?: string;
  createdAt: string;
  updatedAt: string;
  technicalDetails?: Record<string, unknown>;
};

export type WorkbenchTodosResult = {
  items: WorkbenchTodoItem[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
};

export type WorkbenchRefreshResult = {
  refreshedAt: string;
  summary: WorkbenchSummary;
};

export async function queryWorkbenchSummary(params?: Record<string, string | number | undefined>) {
  const data = await getWithParams<{ summary: WorkbenchSummary }>(
    '/api/v1/ai/operation-workbench/summary',
    params,
  );
  return data.summary;
}

export async function queryWorkbenchTodos(params?: Record<string, string | number | undefined>) {
  return getWithParams<WorkbenchTodosResult>('/api/v1/ai/operation-workbench/todos', params);
}

export async function getWorkbenchTodo(id: string, params?: Record<string, string | number | undefined>) {
  return getWithParams<WorkbenchTodoItem>(`/api/v1/ai/operation-workbench/todos/${encodeURIComponent(id)}`, params);
}

export async function refreshWorkbenchTodos(params?: Record<string, string | number | undefined>) {
  const res = await request<import('./request').ApiResponse<WorkbenchRefreshResult>>(
    '/api/v1/ai/operation-workbench/todos/refresh',
    { method: 'POST', params },
  );
  if (res.code !== 0) {
    throw new Error(res.message || 'request_failed');
  }
  return res.data;
}
