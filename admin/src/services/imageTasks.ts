import { deleteJSON, getJSON, getWithParams, postJSON } from '@/services/request';

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
  retryCount?: number;
  maxRetries?: number;
  nextRetryAt?: string;
  retryEnqueuedAt?: string;
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
  return getWithParams<ListResponse>('/api/v1/ai/image/tasks', {
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
  return getJSON<ImageTaskDetail>(`/api/v1/ai/image/tasks/${id}`);
}

export async function createImageTask(payload: {
  taskType: string;
  provider?: string;
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  input?: Record<string, unknown>;
}): Promise<ImageTaskDetail> {
  const body: Record<string, unknown> = {
    taskType: payload.taskType,
    productId: payload.productId ?? '',
    sourceImageId: payload.sourceImageId ?? '',
    sourceImageUrl: payload.sourceImageUrl ?? '',
    input: payload.input ?? {},
  };
  const p = payload.provider?.trim();
  if (p) {
    body.provider = p;
  }
  return postJSON<ImageTaskDetail>('/api/v1/ai/image/tasks', body);
}

export async function retryImageTask(id: string): Promise<ImageTaskDetail> {
  return postJSON<ImageTaskDetail>(`/api/v1/image/tasks/${id}/retry`, {});
}

export type ImageQueueMonitorQueue = {
  enabled: boolean;
  name: string;
  redisAvailable: boolean;
  depth: number;
  workerEnabled: boolean;
  workerRunning: boolean;
  concurrency: number;
};

export type ImageTaskMonitorSnapshot = {
  queue: ImageQueueMonitorQueue;
  worker: { enabled: boolean; concurrency: number; running: boolean };
  tasks: {
    pending: number;
    running: number;
    retrying: number;
    success: number;
    failed: number;
    cancelled: number;
  };
  retry: {
    enabled: boolean;
    maxRetries: number;
    baseDelaySeconds: number;
    maxDelaySeconds: number;
    nextRetryDueCount: number;
    oldestRetryingSeconds?: number;
  };
  recentRetrying: Array<{
    id: string;
    taskType: string;
    provider: string;
    productId?: string;
    retryCount: number;
    maxRetries: number;
    nextRetryAt?: string;
    errorMessage?: string;
    updatedAt: string;
  }>;
  recentFailures: Array<{
    id: string;
    taskType: string;
    provider: string;
    productId?: string;
    errorMessage: string;
    updatedAt: string;
  }>;
};

export async function applyImageTaskResult(
  taskId: string,
  payload: { productId: string; itemId?: string; applyMode?: string; setBest?: boolean },
) {
  return postJSON(`/api/v1/image/tasks/${taskId}/apply`, payload);
}

export async function saveImageTaskItemToProduct(
  itemId: string,
  payload: { productId: string; applyMode?: string; setBest?: boolean },
) {
  return postJSON(`/api/v1/ai/image/task-items/${itemId}/save-to-product`, payload);
}

export async function setImageTaskItemAsMain(itemId: string, payload: { productId: string }) {
  return postJSON(`/api/v1/ai/image/task-items/${itemId}/set-as-main`, payload);
}

export type ImageScoreResult = {
  overallScore: number;
  clarityScore: number;
  cleanlinessScore: number;
  compositionScore: number;
  mainSuitabilityScore: number;
  detailSuitabilityScore: number;
  issues: string[];
  suggestion: string;
  width?: number;
  height?: number;
  source?: string;
};

export async function scoreProductImage(payload: {
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  imageType?: string;
}) {
  return postJSON<ImageScoreResult>('/api/v1/ai/image/score', payload);
}

export type ImageTaskItemRow = {
  id: string;
  taskId: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  outputImageUrl?: string;
  outputStorageKey?: string;
  outputFileId?: string;
  scoreJson?: unknown;
  isSelectedBest?: boolean;
  status: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
};

export async function listImageTaskItems(taskId: string): Promise<{ list: ImageTaskItemRow[] }> {
  return getJSON(`/api/v1/image/tasks/${taskId}/items`);
}

export async function deleteImageTaskItem(taskId: string, itemId: string) {
  return deleteJSON(`/api/v1/image/tasks/${taskId}/items/${itemId}`);
}

export const IMAGE_TASK_TYPE_OPTIONS: { label: string; value: string; group?: string }[] = [
  { label: '去背景', value: 'remove_background', group: '基础' },
  { label: '换背景', value: 'replace_background', group: '基础' },
  { label: '场景图', value: 'generate_scene', group: '基础' },
  { label: '去水印', value: 'remove_watermark', group: '清理' },
  { label: '去 Logo', value: 'remove_logo', group: '清理' },
  { label: '去角标/贴纸', value: 'remove_badge', group: '清理' },
  { label: '去二维码', value: 'remove_qrcode', group: '清理' },
  { label: '综合清理', value: 'cleanup', group: '清理' },
  { label: '详情图增强', value: 'enhance_detail', group: '增强' },
  { label: '高清修复', value: 'upscale', group: '增强' },
  { label: '营销图生成', value: 'generate_marketing', group: '生成' },
  { label: '主图生成', value: 'generate_main_image', group: '生成' },
  { label: '批量主图生成', value: 'batch_generate_main', group: '生成' },
  { label: '商品图评分', value: 'score_image', group: '评分' },
  { label: '自动选最佳主图', value: 'select_best_main', group: '评分' },
  { label: '缩放', value: 'resize', group: '其他' },
  { label: '增强', value: 'enhance', group: '其他' },
];

export const IMAGE_TASK_TEMPLATES: { title: string; taskType: string; description: string }[] = [
  { title: '去水印', taskType: 'remove_watermark', description: '去除商品图水印，结果自动入库' },
  { title: '去 Logo', taskType: 'remove_logo', description: '去除品牌 Logo 与角标' },
  { title: '去角标/贴纸', taskType: 'remove_badge', description: '去除角标、贴纸等装饰元素' },
  { title: '去二维码', taskType: 'remove_qrcode', description: '去除二维码、条码等扫描元素' },
  { title: '综合清理', taskType: 'cleanup', description: '一次性清理水印/Logo/贴纸/二维码' },
  { title: '去背景', taskType: 'remove_background', description: '白底图 / 抠图（remove.bg）' },
  { title: '高清修复', taskType: 'upscale', description: '提升清晰度，适合模糊主图' },
  { title: '营销图生成', taskType: 'generate_marketing', description: '基于商品图生成营销图' },
  { title: '详情图增强', taskType: 'enhance_detail', description: '增强详情图清晰度并去杂' },
  { title: '批量主图生成', taskType: 'batch_generate_main', description: '为多商品批量生成主图候选' },
  { title: '商品图评分', taskType: 'score_image', description: '多维评分与优化建议' },
  { title: '自动选最佳主图', taskType: 'select_best_main', description: '评分并推荐/自动设主图' },
];

export function taskTypeLabel(taskType: string): string {
  const hit = IMAGE_TASK_TYPE_OPTIONS.find((t) => t.value === taskType);
  return hit?.label ?? taskType;
}

