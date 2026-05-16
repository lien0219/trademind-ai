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

/**
 * Collector 统一错误码（与 Go 自动重试策略对齐）
 */
export type CollectTaskErrorCode =
  | 'INVALID_REQUEST'
  | 'PROVIDER_NOT_FOUND'
  | 'PROVIDER_NOT_AVAILABLE'
  | 'PROVIDER_NOT_IMPLEMENTED'
  | 'INVALID_URL'
  | 'COLLECT_FAILED'
  | 'PAGE_BLOCKED_OR_VERIFY_REQUIRED'
  | 'PAGE_TIMEOUT'
  | 'NAVIGATION_FAILED'
  | 'PAGE_LOAD_TIMEOUT'
  | 'NOT_FOUND'
  | 'INTERNAL';
