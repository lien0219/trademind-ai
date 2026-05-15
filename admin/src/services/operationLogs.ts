import { getWithParams } from '@/services/request';

export type OperationLogRow = {
  id: string;
  adminUserId?: string;
  username: string;
  action: string;
  resource: string;
  resourceId?: string;
  method: string;
  path: string;
  ip: string;
  userAgent?: string;
  requestId: string;
  status: string;
  message?: string;
  createdAt: string;
};

export type OperationLogsPagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

type ListResponse = {
  list: OperationLogRow[];
  pagination: OperationLogsPagination;
};

export async function fetchOperationLogs(params: {
  page?: number;
  pageSize?: number;
  action?: string;
  username?: string;
  resource?: string;
  start?: string;
  end?: string;
}): Promise<ListResponse> {
  return getWithParams<ListResponse>('/api/v1/operation-logs', {
    page: params.page,
    pageSize: params.pageSize,
    action: params.action || undefined,
    username: params.username || undefined,
    resource: params.resource || undefined,
    start: params.start || undefined,
    end: params.end || undefined,
  });
}
