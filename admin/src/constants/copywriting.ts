/**
 * 管理端统一术语与页面文案字典。
 * 用户可见文案优先从此处引用，避免各页面叫法不一致。
 */

/** 页面标题与说明 */
export const PAGE_COPY = {
  platformSettings: {
    title: '平台接入设置',
    description: '配置需要连接的电商平台，完成应用信息填写后即可授权店铺。',
    heroTitle: '如何开始？',
    heroDescription:
      '先在平台开放中心创建应用，再将应用信息填写到这里。保存后，前往「店铺管理」完成店铺授权。',
  },
  shops: {
    title: '店铺管理',
    description: '授权并管理已连接的电商平台店铺，可在此同步订单与更新店铺信息。',
  },
  collectHub: {
    title: '采集中心',
    description: '从 1688、拼多多等平台采集商品信息，并保存为商品草稿。',
  },
  productDrafts: {
    title: '商品草稿',
    description: '管理采集或手动创建的商品草稿，可进行 AI 优化与刊登准备。',
  },
  productDraftDetail: {
    title: '商品详情',
    description: '编辑商品标题、描述、图片与规格，完成 AI 优化与刊登准备。',
  },
  dashboard: {
    title: '商品运营看板',
    description: '查看商品、采集、AI 与任务的整体运营概况。',
  },
  aiImageTasks: {
    title: 'AI 图片任务',
    description: '查看图片处理任务进度与结果，失败时可在此重试。',
  },
  orderList: {
    title: '订单管理',
    description: '查看与管理已同步的平台订单。',
  },
  orderSyncTasks: {
    title: '订单同步任务',
    description: '查看订单同步任务的执行状态与结果。',
  },
  inventorySyncTasks: {
    title: '库存同步任务',
    description: '查看库存同步任务的执行状态与结果。',
  },
  taskFailures: {
    title: '失败任务中心',
    description: '集中查看失败任务，按类型筛选并重试。',
  },
  operationLogs: {
    title: '操作日志',
    description: '查看系统操作记录，便于追溯配置变更与关键操作。',
  },
  systemSettings: {
    title: '系统设置',
    description: '配置站点基础信息与任务中心站内告警策略。',
  },
  storageSettings: {
    title: '存储设置',
    description: '配置商品图片与附件的存储方式，支持本地磁盘与主流对象存储。',
  },
  aiSettings: {
    title: 'AI 设置',
    description: '配置 AI 服务商与默认模型，用于标题优化与描述生成。',
  },
  collectorSettings: {
    title: '采集设置',
    description: '配置采集服务连接与各平台登录状态。',
  },
  integrations: {
    title: '第三方集成总览',
    description: '查看 AI、存储、采集与平台接入的整体配置状态。',
  },
} as const;

/** 商品相关 */
export const PRODUCT_COPY = {
  draft: '商品草稿',
  aiTitle: 'AI 优化标题',
  aiDescription: 'AI 优化描述',
  originalTitle: '原始标题',
  mainImages: '主图',
  descriptionImages: '详情图',
  attributes: '商品参数',
  sku: '商品规格',
  skuAttrs: '规格属性',
  mappedImages: '刊登图片',
  mapping: '刊登草稿',
  publishConfig: '刊登设置',
  publication: '已刊登商品',
  platformProduct: '平台商品',
} as const;

/** 店铺与平台相关 */
export const PLATFORM_COPY = {
  settings: '平台接入设置',
  appInfo: '平台应用信息',
  shopAuth: '授权店铺',
  oauth: '店铺授权',
  callbackUrl: '授权回调地址',
  testConnection: '测试连接',
  syncShopInfo: '更新店铺信息',
  revoke: '解除授权',
  appConfig: '应用配置',
  provider: '接入方式',
  appKey: '应用 Key',
  appSecret: '应用密钥',
  serviceId: '服务编号',
  authUrl: '授权地址',
  apiUrl: '接口地址',
  environment: '运行环境',
  realApi: '使用真实平台接口',
  orderSync: '开启订单同步',
  orderSyncMaxPages: '每次最多同步页数',
  inventorySync: '开启库存同步',
  productDraftCreate: '允许创建商品草稿',
  requestTimeout: '请求超时时间',
} as const;

/** 任务相关 */
export const TASK_COPY = {
  task: '任务',
  failed: '失败',
  partialSuccess: '部分成功',
  running: '处理中',
  pending: '等待处理',
  success: '成功',
  cancelled: '已取消',
  retry: '重试',
  platformError: '平台返回信息',
  requestId: '请求编号',
  traceId: '排查编号',
} as const;

/** 库存相关 */
export const INVENTORY_COPY = {
  sync: '库存同步',
  localStock: '本地库存',
  platformStock: '平台库存',
  stockSnapshot: '平台库存记录',
  safetyStock: '安全库存',
  reserved: '预留库存',
  available: '可用库存',
  bindStatus: '规格绑定状态',
  skuBinding: '规格绑定',
  platformSkuId: '平台规格编号',
  externalProductId: '平台商品编号',
} as const;

/** 通用按钮文案 */
export const ACTION_COPY = {
  saveSettings: '保存设置',
  saveConfig: '保存设置',
  testConnection: '测试连接',
  reload: '重新加载',
  authorizeShop: '授权店铺',
  syncOrders: '同步订单',
  uploadImage: '上传图片',
  createDraft: '创建商品草稿',
  calibrateSkuBinding: '校准规格绑定',
  retryFailed: '重试失败任务',
  cancel: '取消',
  confirm: '确认',
  delete: '删除',
  viewDetails: '查看详情',
  viewTechnicalDetails: '查看技术详情',
  goToShops: '前往店铺管理',
} as const;

/** 空状态默认文案 */
export const EMPTY_COPY = {
  defaultTitle: '暂无内容',
  defaultDescription: '当前没有可展示的数据。',
  orderSync: {
    title: '暂无同步任务',
    description: '完成店铺授权后，可以手动同步订单。',
    action: '前往店铺管理',
    actionPath: '/shops',
  },
  productDraft: {
    title: '暂无商品草稿',
    description: '可以先从采集中心采集商品，或手动创建商品草稿。',
    action: '前往采集中心',
    actionPath: '/collect/hub',
  },
} as const;

/** 发布检查项级别 */
export const READINESS_LEVEL_LABEL: Record<string, string> = {
  error: '错误',
  warning: '警告',
};

export function readinessLevelLabel(level?: string | null): string {
  const k = (level ?? '').trim().toLowerCase();
  if (!k) return '—';
  return READINESS_LEVEL_LABEL[k] ?? level ?? '—';
}

/** 刊登任务发布模式 */
export const PUBLISH_MODE_LABEL: Record<string, string> = {
  save_as_platform_draft: '保存为平台草稿',
  publish: '直接刊登',
  draft: '保存草稿',
};

export function publishModeLabel(mode?: string | null): string {
  const k = (mode ?? 'save_as_platform_draft').trim();
  return PUBLISH_MODE_LABEL[k] ?? k;
}

/** AI 文案字段（商品草稿内） */
export const AI_FIELD_COPY = {
  aiTitle: 'AI 优化标题',
  aiDescription: 'AI 优化描述',
} as const;

/** 将内部英文状态码转为用户可见文案（兜底） */
export const COMMON_STATUS_LABEL: Record<string, string> = {
  pending: '等待处理',
  running: '处理中',
  success: '成功',
  partial_success: '部分成功',
  failed: '失败',
  cancelled: '已取消',
  enabled: '已开启',
  disabled: '未开启',
  configured: '已配置',
  unconfigured: '未配置',
  authorized: '已授权',
  expired: '授权已过期',
  need_check: '需要检查',
  bound: '已绑定',
  unmatched: '未匹配',
  ambiguous: '需要人工确认',
  skipped: '已跳过',
  retryable: '可以重试',
};

export function commonStatusLabel(status?: string | null): string {
  const k = (status ?? '').trim().toLowerCase();
  if (!k) return '—';
  return COMMON_STATUS_LABEL[k] ?? status ?? '—';
}

/** 空值展示 */
export const EMPTY_CELL = '—';
