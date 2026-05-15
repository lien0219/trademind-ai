/** 与后端 / Redis 任务状态对齐 */
export const CollectTaskStatus = [
  'pending',
  'running',
  'success',
  'failed',
  'cancelled',
  'retrying',
] as const;

export type CollectTaskStatus = (typeof CollectTaskStatus)[number];

export type CollectTaskErrorCode =
  | 'INVALID_REQUEST'
  | 'PROVIDER_NOT_FOUND'
  | 'INVALID_URL'
  | 'PAGE_TIMEOUT'
  | 'PROVIDER_ERROR'
  | 'NAVIGATION_FAILED'
  | 'PAGE_LOAD_TIMEOUT'
  | 'NOT_IMPLEMENTED'
  | 'NOT_FOUND'
  | 'INTERNAL';
