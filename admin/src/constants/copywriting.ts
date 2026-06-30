/**
 * 管理端统一术语与页面文案字典。
 * 用户可见文案优先从此处引用，避免各页面叫法不一致。
 */

import { mapErrorCodeToUserMessage } from '@/constants/errorMessages';

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
    title: '运营总览',
    description: '查看采集、商品、AI、刊登、订单、库存、客服与配置的整体运营概况。',
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
  configStatus: {
    title: '配置状态中心',
    description: '聚合 AI / OCR / 存储 / 平台凭证 / 后台任务进程 等配置健康状态（不含密钥明文）。',
    snapshotAt: '快照时间',
    nextStep: '下一步',
    goConfigure: '前往配置',
  },
  usersSettings: {
    title: '用户与权限',
    description: '管理员可管理后台账号、角色与店铺授权。',
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

/** 列表页空状态文案（F7 EmptyState rollout） */
export type ListEmptyCopy = {
  title: string;
  description: string;
  action?: string;
  actionPath?: string;
  onAction?: () => void;
  permissionHint?: string;
};

export type ListEmptyKey = keyof typeof LIST_EMPTY_COPY;

const PERM_HINT =
  '若你使用的是运营或只读账号，也可能是店铺权限范围导致看不到数据。';

export const LIST_EMPTY_COPY = {
  dashboard: {
    title: '暂无最近动态',
    description: '完成采集、AI 优化或刊登操作后，这里会显示最近运营动态。',
    action: '前往运营总览',
    actionPath: '/dashboard/product-operations',
  },
  collectHub: {
    title: '暂未获取到采集器',
    description: '请检查采集服务是否已启动；也可先在采集设置中配置连接。',
    action: '前往采集设置',
    actionPath: '/settings/collector',
  },
  collectTasks: {
    title: '暂无采集任务',
    description: '输入商品链接创建采集任务，或运行 Demo 数据脚本生成样本。',
    action: '前往采集中心',
    actionPath: '/collect/hub',
    permissionHint: PERM_HINT,
  },
  productDrafts: {
    title: '暂无商品草稿',
    description: '可以先从采集中心采集商品，或手动创建商品草稿；演示环境也可运行 Demo 数据脚本。',
    action: '前往采集中心',
    actionPath: '/collect/hub',
    permissionHint: PERM_HINT,
  },
  aiOperationWorkbench: {
    title: '暂无待办商品',
    description: '当商品有待优化标题、描述、图片或刊登检查时，会出现在此工作台。',
    action: '查看商品草稿',
    actionPath: '/product/drafts',
  },
  aiTextBatches: {
    title: '暂无 AI 文案批次',
    description: '在商品草稿中选择商品发起 AI 标题/描述批次任务。',
    action: '前往商品草稿',
    actionPath: '/product/drafts',
  },
  aiImageBatches: {
    title: '暂无 AI 图片批次',
    description: '在商品草稿或 AI 图片任务页发起图片处理批次。',
    action: '前往商品草稿',
    actionPath: '/product/drafts',
  },
  publishBatches: {
    title: '暂无刊登批次',
    description: '商品通过发布检查后，可批量创建刊登草稿。',
    action: '前往商品草稿',
    actionPath: '/product/drafts',
  },
  orderList: {
    title: '暂无订单数据',
    description: '可以先配置店铺授权并手动同步订单；演示环境也可以运行 Demo 数据脚本生成样本。',
    action: '前往店铺管理',
    actionPath: '/shops/manage',
    permissionHint: PERM_HINT,
  },
  orderExceptions: {
    title: '暂无订单异常',
    description: '订单同步或 SKU 匹配出现问题时会在此展示；当前没有需要处理的异常。',
    action: '查看订单列表',
    actionPath: '/orders/list',
    permissionHint: PERM_HINT,
  },
  inventoryCenter: {
    title: '暂无库存数据',
    description: '商品 SKU 创建后库存会在此汇总；也可运行 Demo 数据脚本生成样本。',
    action: '前往商品草稿',
    actionPath: '/product/drafts',
    permissionHint: PERM_HINT,
  },
  inventoryAlerts: {
    title: '暂无库存预警',
    description: '当 SKU 库存低于预警线或缺货时，会在此显示预警记录。',
    action: '查看库存中心',
    actionPath: '/inventory',
    permissionHint: PERM_HINT,
  },
  inventoryDeductions: {
    title: '暂无扣减记录',
    description: '订单库存扣减成功或失败后，记录会出现在此列表。',
    action: '查看订单列表',
    actionPath: '/orders/list',
    permissionHint: PERM_HINT,
  },
  inventorySyncTasks: {
    title: '暂无库存同步任务',
    description: '开启库存同步并在商品详情发起同步后，任务会在此展示。',
    action: '查看配置状态',
    actionPath: '/settings/config-status',
    permissionHint: PERM_HINT,
  },
  customerHub: {
    title: '暂无客服概览数据',
    description: '创建客服会话或同步平台消息后，概览数据会在此展示。',
    action: '查看会话列表',
    actionPath: '/customer/conversations',
    permissionHint: PERM_HINT,
  },
  customerConversations: {
    title: '暂无客服会话',
    description: '买家咨询会话会在此展示；演示环境可运行 Demo 数据脚本生成样本。',
    action: '新建会话',
    actionPath: '/customer/conversations',
    permissionHint: PERM_HINT,
  },
  taskFailures: {
    title: '暂无失败任务',
    description: '系统运行正常，当前没有需要重试的失败任务。',
    action: '查看配置状态',
    actionPath: '/settings/config-status',
  },
  configStatus: {
    title: '暂无配置项',
    description: '配置状态中心会聚合 AI、存储、平台等配置健康状态；若为空请刷新或检查后台服务。',
    action: '前往系统设置',
    actionPath: '/settings/integrations',
  },
  usersSettings: {
    title: '暂无用户',
    description: '管理员可在此创建后台账号并分配角色与店铺权限。',
    action: '创建用户',
  },
  operationLogs: {
    title: '暂无操作日志',
    description: '关键配置变更与业务操作会记录在此，便于审计追溯。',
  },
  orderSync: {
    title: '暂无同步任务',
    description: '完成店铺授权后，可以手动同步订单。',
    action: '前往店铺管理',
    actionPath: '/shops/manage',
  },
  productDraft: {
    title: '暂无商品草稿',
    description: '可以先从采集中心采集商品，或手动创建商品草稿。',
    action: '前往采集中心',
    actionPath: '/collect/hub',
  },
} as const satisfies Record<string, ListEmptyCopy>;

/** @deprecated 使用 LIST_EMPTY_COPY */
export const EMPTY_COPY = LIST_EMPTY_COPY;

/** 发布检查项级别 */
export const READINESS_LEVEL_LABEL: Record<string, string> = {
  error: '错误',
  warning: '建议检查',
  failed: '错误',
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
  ready: '已准备好',
  draft: '草稿',
  draft_created: '平台草稿已创建',
  warning: '建议检查',
  blocked: '暂不能继续',
  passed: '检查通过',
  checking: '检查中',
  publishing: '刊登中',
  pending_review: '待复核',
  local_draft_only: '仅本地草稿',
  real_draft_create: '创建平台草稿',
  blocked_by_real_credentials: '缺少真实凭证',
  blocked_by_provider_config: '接入服务未配置',
  unsupported_by_provider: '当前服务不支持',
  permission_denied: '无权限',
  readonly_operation_forbidden: '只读账号不可操作',
  store_permission_denied: '无店铺权限',
  inventory_sync_enabled: '库存同步已开启',
  manual_bound: '人工绑定',
};

export function commonStatusLabel(status?: string | null): string {
  const raw = (status ?? '').trim();
  const k = raw.toLowerCase();
  if (!k) return '—';
  if (/^[A-Z][A-Z0-9_]+$/.test(raw)) {
    const mapped = mapErrorCodeToUserMessage(raw);
    if (mapped) return mapped.title;
  }
  return COMMON_STATUS_LABEL[k] ?? raw ?? '—';
}

/** 空值展示 */
export const EMPTY_CELL = '—';
