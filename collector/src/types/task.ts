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
  | 'UNSUPPORTED_URL'
  | 'UNSUPPORTED_PINDUODUO_URL'
  | 'WECHAT_AUTH_REQUIRED'
  | 'APP_REDIRECT'
  | 'PRODUCT_NOT_FOUND'
  | 'COLLECT_FAILED'
  | 'LOGIN_REQUIRED'
  | 'CUSTOM_RULE_MISSING'
  | 'CUSTOM_RULE_INVALID'
  | 'PARSE_FAILED_TITLE_MISSING'
  | 'PARSE_FAILED_IMAGE_MISSING'
  | 'PAGE_BLOCKED_OR_VERIFY_REQUIRED'
  | 'TIMEOUT'
  | 'NAVIGATION_FAILED'
  | 'PARSE_FAILED'
  | 'PAGE_LOAD_TIMEOUT'
  | 'PAGE_TIMEOUT'
  | 'NOT_FOUND'
  | 'INTERNAL'
  | 'VERIFY_REQUIRED'
  | 'ITEM_NOT_FOUND'
  | 'MAIN_IMAGES_EMPTY'
  | 'ACCESS_DENIED'
  | 'UNKNOWN_COLLECT_ERROR'
  | 'PRICE_NOT_FOUND'
  | 'SKU_INCOMPLETE'
  | 'DETAIL_IMAGES_INCOMPLETE';
