/** 管理端通用用户文案（避免直接展示英文技术词） */

/** 菜单、标题中完整写法 */
export const SKU_LABEL = 'SKU（商品规格）';

/** 正文、说明中的简称 */
export const SKU_SHORT = '商品规格';

export const AI_PROVIDER_LABEL = 'AI 服务商';

export const IMAGE_PROVIDER_LABEL = '图片处理服务';

export const PROMPT_TEMPLATE_LABEL = 'AI 技能模板';

export const WORKER_LABEL = '后台任务进程';

export const QUEUE_LABEL = '任务队列';

export const COLLECTOR_SERVICE_LABEL = '采集服务';

export const SETTINGS_COLLECTOR_PATH = '采集设置';

/** 商品图片字段（避免直接展示 API 字段名） */
export const PRODUCT_IMAGE_SORT_ORDER_LABEL = '排序';

export const PRODUCT_IMAGE_PUBLIC_URL_LABEL = '公开链接';

export const PRODUCT_IMAGE_ORIGIN_URL_LABEL = '原始链接';

export const PRODUCT_IMAGE_OBJECT_KEY_LABEL = '存储路径';

export const PRODUCT_IMAGE_URL_LABEL = '图片地址';

export const INVENTORY_SYNC_TASKS_LABEL = '库存同步任务';

export const INVENTORY_SYNC_BATCHES_LABEL = '库存同步批次';

/** 设置项「单次批量最多创建任务数」（对应 inventory_sync_batch_max_size） */
export const INVENTORY_SYNC_BATCH_MAX_SIZE_LABEL = '单次批量最多创建任务数';

/** 跨境平台显示名 */
export const PLATFORM_LABEL: Record<string, string> = {
  tiktok: 'TikTok',
  shopee: 'Shopee',
  lazada: 'Lazada',
  amazon: 'Amazon',
  mock: 'Mock',
  manual: '手动',
};

export function platformLabel(platform?: string): string {
  const k = (platform || '').trim().toLowerCase();
  if (!k) return '—';
  return PLATFORM_LABEL[k] || platform || '—';
}

/** 商品草稿来源（products.source）显示名 */
export const PRODUCT_SOURCE_LABEL: Record<string, string> = {
  '1688': '1688',
  pinduoduo: '拼多多',
  pdd: '拼多多',
  taobao_tmall: '淘宝/天猫',
  taobao: '淘宝',
  aliexpress: '速卖通',
  shein_temu: 'SHEIN/Temu',
  custom: '自定义链接',
  manual: '手动创建',
};

export function productSourceLabel(source?: string): string {
  const raw = (source || '').trim();
  if (!raw) return '—';
  const k = raw.toLowerCase();
  return PRODUCT_SOURCE_LABEL[k] ?? PRODUCT_SOURCE_LABEL[raw] ?? raw;
}

export const PLATFORM_OPTIONS = Object.entries(PLATFORM_LABEL).map(([value, label]) => ({
  value,
  label,
}));
