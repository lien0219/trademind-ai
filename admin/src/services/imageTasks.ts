import { getJSON, getWithParams, postJSON } from '@/services/request';

export type ImageTaskListRow = {
  id: string;
  taskType: string;
  provider: string;
  status: string;
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  resultFileId?: string;
  resultUrl?: string;
  errorMessage?: string;
  createdBy?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type ImageTaskDetail = ImageTaskListRow & {
  input?: unknown;
  output?: unknown;
};

export type ImageTasksPagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

type ListResponse = {
  list: ImageTaskListRow[];
  pagination: ImageTasksPagination;
};

export async function queryImageTasks(params: {
  page?: number;
  pageSize?: number;
  taskType?: string;
  status?: string;
  provider?: string;
  productId?: string;
  start?: string;
  end?: string;
}): Promise<ListResponse> {
  return getWithParams<ListResponse>('/api/v1/image/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    taskType: params.taskType || undefined,
    status: params.status || undefined,
    provider: params.provider || undefined,
    productId: params.productId || undefined,
    start: params.start || undefined,
    end: params.end || undefined,
  });
}

export async function getImageTask(id: string): Promise<ImageTaskDetail> {
  return getJSON<ImageTaskDetail>(`/api/v1/image/tasks/${id}`);
}

export async function createImageTask(payload: {
  taskType: string;
  provider?: string;
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  input?: Record<string, unknown>;
}): Promise<ImageTaskDetail> {
  return postJSON<ImageTaskDetail>('/api/v1/image/tasks', {
    taskType: payload.taskType,
    provider: payload.provider ?? 'noop',
    productId: payload.productId ?? '',
    sourceImageId: payload.sourceImageId ?? '',
    sourceImageUrl: payload.sourceImageUrl ?? '',
    input: payload.input ?? {},
  });
}

export async function retryImageTask(id: string): Promise<ImageTaskDetail> {
  return postJSON<ImageTaskDetail>(`/api/v1/image/tasks/${id}/retry`, {});
}
