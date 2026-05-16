import { getJSON, getWithParams } from '@/services/request';

export type AiTaskListRow = {
  id: string;
  taskType: string;
  provider?: string;
  model?: string;
  promptCode?: string;
  status: string;
  errorMessage?: string;
  tokenInput?: number;
  tokenOutput?: number;
  costAmount?: number;
  productId?: string;
  conversationId?: string;
  createdBy?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type AiTaskDetail = AiTaskListRow & {
  input?: unknown;
  output?: unknown;
  rawResponse?: unknown;
};

export type AiTasksPagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

type ListResponse = {
  list: AiTaskListRow[];
  pagination: AiTasksPagination;
};

export async function queryAiTasks(params: {
  page?: number;
  pageSize?: number;
  taskType?: string;
  status?: string;
  provider?: string;
  model?: string;
  promptCode?: string;
  productId?: string;
  conversationId?: string;
  start?: string;
  end?: string;
}): Promise<ListResponse> {
  return getWithParams<ListResponse>('/api/v1/ai/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    taskType: params.taskType || undefined,
    status: params.status || undefined,
    provider: params.provider || undefined,
    model: params.model || undefined,
    promptCode: params.promptCode || undefined,
    productId: params.productId || undefined,
    conversationId: params.conversationId || undefined,
    start: params.start || undefined,
    end: params.end || undefined,
  });
}

export async function getAiTask(id: string): Promise<AiTaskDetail> {
  return getJSON<AiTaskDetail>(`/api/v1/ai/tasks/${id}`);
}
