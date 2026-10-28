import { tr } from '@/i18n';

/**
 * 管理端统一术语与页面文案字典。
 * 用户可见文案优先从此处引用，避免各页面叫法不一致。
 */

import { mapErrorCodeToUserMessage } from '@/constants/errorMessages';

/** 页面标题与说明 */
export const PAGE_COPY = {
  platformSettings: {
    get title() { return tr('平台接入设置'); },
    get description() { return tr('配置需要连接的电商平台，完成应用信息填写后即可授权店铺。'); },
    get heroTitle() { return tr('如何开始？'); },
    get heroDescription() { return tr('先在平台开放中心创建应用，再将应用信息填写到这里。保存后，前往「店铺管理」完成店铺授权。'); },
  },
  shops: {
    get title() { return tr('店铺管理'); },
    get description() { return tr('授权并管理已连接的电商平台店铺，可在此同步订单与更新店铺信息。'); },
  },
  collectHub: {
    get title() { return tr('采集中心'); },
    get description() { return tr('从 1688、拼多多等平台采集商品信息，并保存为商品草稿。'); },
  },
  productDrafts: {
    get title() { return tr('商品草稿'); },
    get description() { return tr('管理采集或手动创建的商品草稿，可进行 AI 优化与刊登准备。'); },
  },
  productDraftDetail: {
    get title() { return tr('商品详情'); },
    get description() { return tr('编辑商品标题、描述、图片与规格，完成 AI 优化与刊登准备。'); },
  },
  dashboard: {
    get title() { return tr('运营总览'); },
    get description() { return tr('查看采集、商品、AI、刊登、订单、库存、客服与配置的整体运营概况。'); },
  },
  aiImageTasks: {
    get title() { return tr('AI 图片任务'); },
    get description() { return tr('查看图片处理任务进度与结果，失败时可在此重试。'); },
  },
  orderList: {
    get title() { return tr('订单管理'); },
    get description() { return tr('查看与管理已同步的平台订单。'); },
  },
  orderSyncTasks: {
    get title() { return tr('订单同步任务'); },
    get description() { return tr('查看订单同步任务的执行状态与结果。'); },
  },
  inventorySyncTasks: {
    get title() { return tr('库存同步任务'); },
    get description() { return tr('查看库存同步任务的执行状态与结果。'); },
  },
  taskFailures: {
    get title() { return tr('失败任务中心'); },
    get description() { return tr('集中查看失败任务，按类型筛选并重试。'); },
  },
  operationTasks: {
    get title() { return tr('运营任务中心'); },
    get description() { return tr('查看运营任务、草稿版本、人工审核、受控执行与审计时间线。'); },
  },
  operationLogs: {
    get title() { return tr('操作日志'); },
    get description() { return tr('查看系统操作记录，便于追溯配置变更与关键操作。'); },
  },
  systemSettings: {
    get title() { return tr('系统设置'); },
    get description() { return tr('配置站点基础信息与任务中心站内告警策略。'); },
  },
  storageSettings: {
    get title() { return tr('存储设置'); },
    get description() { return tr('配置商品图片与附件的存储方式，支持本地磁盘与主流对象存储。'); },
  },
  aiSettings: {
    get title() { return tr('AI 设置'); },
    get description() { return tr('配置 AI 服务商与默认模型，用于标题优化与描述生成。'); },
  },
  collectorSettings: {
    get title() { return tr('采集设置'); },
    get description() { return tr('配置采集服务连接与各平台登录状态。'); },
  },
  integrations: {
    get title() { return tr('第三方集成总览'); },
    get description() { return tr('查看 AI、存储、采集与平台接入的整体配置状态。'); },
  },
  configStatus: {
    get title() { return tr('配置状态中心'); },
    get description() { return tr('聚合 AI / OCR / 存储 / 平台凭证 / 后台任务进程 等配置健康状态（不含密钥明文）。'); },
    get snapshotAt() { return tr('快照时间'); },
    get nextStep() { return tr('下一步'); },
    get goConfigure() { return tr('前往配置'); },
  },
  usersSettings: {
    get title() { return tr('用户与权限'); },
    get description() { return tr('管理员可管理后台账号、角色与店铺授权。'); },
  },
} as const;

/** 商品相关 */
export const PRODUCT_COPY = {
  get draft() { return tr('商品草稿'); },
  get aiTitle() { return tr('AI 优化标题'); },
  get aiDescription() { return tr('AI 优化描述'); },
  get originalTitle() { return tr('原始标题'); },
  get mainImages() { return tr('主图'); },
  get descriptionImages() { return tr('详情图'); },
  get attributes() { return tr('商品参数'); },
  get sku() { return tr('商品规格'); },
  get skuAttrs() { return tr('规格属性'); },
  get mappedImages() { return tr('刊登图片'); },
  get mapping() { return tr('刊登草稿'); },
  get publishConfig() { return tr('刊登设置'); },
  get publication() { return tr('已刊登商品'); },
  get platformProduct() { return tr('平台商品'); },
} as const;

/** 店铺与平台相关 */
export const PLATFORM_COPY = {
  get settings() { return tr('平台接入设置'); },
  get appInfo() { return tr('平台应用信息'); },
  get shopAuth() { return tr('授权店铺'); },
  get oauth() { return tr('店铺授权'); },
  get callbackUrl() { return tr('授权回调地址'); },
  get testConnection() { return tr('测试连接'); },
  get syncShopInfo() { return tr('更新店铺信息'); },
  get revoke() { return tr('解除授权'); },
  get appConfig() { return tr('应用配置'); },
  get provider() { return tr('接入方式'); },
  get appKey() { return tr('应用 Key'); },
  get appSecret() { return tr('应用密钥'); },
  get serviceId() { return tr('服务编号'); },
  get authUrl() { return tr('授权地址'); },
  get apiUrl() { return tr('接口地址'); },
  get environment() { return tr('运行环境'); },
  get realApi() { return tr('使用真实平台接口'); },
  get orderSync() { return tr('开启订单同步'); },
  get orderSyncMaxPages() { return tr('每次最多同步页数'); },
  get inventorySync() { return tr('开启库存同步'); },
  get productDraftCreate() { return tr('允许创建商品草稿'); },
  get requestTimeout() { return tr('请求超时时间'); },
} as const;

/** 任务相关 */
export const TASK_COPY = {
  get task() { return tr('任务'); },
  get failed() { return tr('失败'); },
  get partialSuccess() { return tr('部分成功'); },
  get running() { return tr('处理中'); },
  get pending() { return tr('等待处理'); },
  get success() { return tr('成功'); },
  get cancelled() { return tr('已取消'); },
  get retry() { return tr('重试'); },
  get platformError() { return tr('平台返回信息'); },
  get requestId() { return tr('请求编号'); },
  get traceId() { return tr('排查编号'); },
} as const;

/** 库存相关 */
export const INVENTORY_COPY = {
  get sync() { return tr('库存同步'); },
  get localStock() { return tr('本地库存'); },
  get platformStock() { return tr('平台库存'); },
  get stockSnapshot() { return tr('平台库存记录'); },
  get safetyStock() { return tr('安全库存'); },
  get reserved() { return tr('预留库存'); },
  get available() { return tr('可用库存'); },
  get bindStatus() { return tr('规格绑定状态'); },
  get skuBinding() { return tr('规格绑定'); },
  get platformSkuId() { return tr('平台规格编号'); },
  get externalProductId() { return tr('平台商品编号'); },
} as const;

/** 通用按钮文案 */
export const ACTION_COPY = {
  get saveSettings() { return tr('保存设置'); },
  get saveConfig() { return tr('保存设置'); },
  get testConnection() { return tr('测试连接'); },
  get reload() { return tr('重新加载'); },
  get authorizeShop() { return tr('授权店铺'); },
  get syncOrders() { return tr('同步订单'); },
  get uploadImage() { return tr('上传图片'); },
  get createDraft() { return tr('创建商品草稿'); },
  get calibrateSkuBinding() { return tr('校准规格绑定'); },
  get retryFailed() { return tr('重试失败任务'); },
  get cancel() { return tr('取消'); },
  get confirm() { return tr('确认'); },
  get delete() { return tr('删除'); },
  get viewDetails() { return tr('查看详情'); },
  get viewTechnicalDetails() { return tr('查看技术详情'); },
  get goToShops() { return tr('前往店铺管理'); },
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

/** 采集 / 商品草稿店铺归属提示（F8）。不改变权限规则，仅引导运营选择目标店铺。 */
export const COLLECT_TARGET_SHOP_HINT =
  '运营账号只能查看已授权店铺的数据。采集后请先在商品详情的「刊登」Tab 选择目标店铺，或由管理员分配店铺归属，否则该草稿可能仅管理员可见。建议在采集前明确目标店铺，方便后续商品、刊登、订单、库存权限一致。';

export const COLLECT_SUCCESS_SHOP_HINT =
  '采集任务已提交。若你使用运营账号，采集完成后请在商品详情「刊登」Tab 选择目标店铺，或由管理员分配店铺归属，否则草稿可能仅管理员可见。';

export const LIST_EMPTY_COPY = {
  dashboard: {
    get title() { return tr('暂无最近动态'); },
    get description() { return tr('完成采集、AI 优化或刊登操作后，这里会显示最近运营动态。'); },
    get action() { return tr('前往运营总览'); },
    actionPath: '/dashboard/product-operations',
  },
  collectHub: {
    get title() { return tr('暂未获取到采集器'); },
    get description() { return tr('请检查采集服务是否已启动；也可先在采集设置中配置连接。'); },
    get action() { return tr('前往采集设置'); },
    actionPath: '/settings/collector',
  },
  collectTasks: {
    get title() { return tr('暂无采集任务'); },
    get description() { return tr('输入商品链接后即可创建采集任务。'); },
    get action() { return tr('前往采集中心'); },
    actionPath: '/collect/hub',
    get permissionHint() { return tr(PERM_HINT); },
  },
  productDrafts: {
    get title() { return tr('暂无商品草稿'); },
    get description() { return tr('可以先从采集中心采集商品，或手动创建商品草稿。'); },
    get action() { return tr('前往采集中心'); },
    actionPath: '/collect/hub',
    get permissionHint() { return [tr(PERM_HINT), tr(COLLECT_TARGET_SHOP_HINT)].join(' '); },
  },
  aiOperationWorkbench: {
    get title() { return tr('暂无待办商品'); },
    get description() { return tr('当商品有待优化标题、描述、图片或刊登检查时，会出现在此工作台。'); },
    get action() { return tr('查看商品草稿'); },
    actionPath: '/product/drafts',
  },
  aiTextBatches: {
    get title() { return tr('暂无 AI 文案批次'); },
    get description() { return tr('在商品草稿中选择商品发起 AI 标题/描述批次任务。'); },
    get action() { return tr('前往商品草稿'); },
    actionPath: '/product/drafts',
  },
  aiImageBatches: {
    get title() { return tr('暂无 AI 图片批次'); },
    get description() { return tr('在商品草稿或 AI 图片任务页发起图片处理批次。'); },
    get action() { return tr('前往商品草稿'); },
    actionPath: '/product/drafts',
  },
  publishBatches: {
    get title() { return tr('暂无刊登批次'); },
    get description() { return tr('商品通过发布检查后，可批量创建刊登草稿。'); },
    get action() { return tr('前往商品草稿'); },
    actionPath: '/product/drafts',
  },
  orderList: {
    get title() { return tr('暂无订单数据'); },
    get description() { return tr('请先配置店铺授权，再手动发起订单同步。'); },
    get action() { return tr('前往店铺管理'); },
    actionPath: '/shops/manage',
    get permissionHint() { return tr(PERM_HINT); },
  },
  orderExceptions: {
    get title() { return tr('暂无订单异常'); },
    get description() { return tr('订单同步或 SKU 匹配出现问题时会在此展示；当前没有需要处理的异常。'); },
    get action() { return tr('查看订单列表'); },
    actionPath: '/orders/list',
    get permissionHint() { return tr(PERM_HINT); },
  },
  inventoryCenter: {
    get title() { return tr('暂无库存数据'); },
    get description() { return tr('商品 SKU 创建并产生库存记录后会在此汇总。'); },
    get action() { return tr('前往商品草稿'); },
    actionPath: '/product/drafts',
    get permissionHint() { return tr(PERM_HINT); },
  },
  inventoryAlerts: {
    get title() { return tr('暂无库存预警'); },
    get description() { return tr('当 SKU 库存低于预警线或缺货时，会在此显示预警记录。'); },
    get action() { return tr('查看库存中心'); },
    actionPath: '/inventory/overview',
    get permissionHint() { return tr(PERM_HINT); },
  },
  inventoryDeductions: {
    get title() { return tr('暂无扣减记录'); },
    get description() { return tr('订单库存扣减成功或失败后，记录会出现在此列表。'); },
    get action() { return tr('查看订单列表'); },
    actionPath: '/orders/list',
    get permissionHint() { return tr(PERM_HINT); },
  },
  inventorySyncTasks: {
    get title() { return tr('暂无库存同步任务'); },
    get description() { return tr('开启库存同步并在商品详情发起同步后，任务会在此展示。'); },
    get action() { return tr('查看配置状态'); },
    actionPath: '/settings/config-status',
    get permissionHint() { return tr(PERM_HINT); },
  },
  customerHub: {
    get title() { return tr('暂无客服概览数据'); },
    get description() { return tr('创建客服会话或同步平台消息后，概览数据会在此展示。'); },
    get action() { return tr('查看会话列表'); },
    actionPath: '/customer/conversations',
    get permissionHint() { return tr(PERM_HINT); },
  },
  customerConversations: {
    get title() { return tr('暂无客服会话'); },
    get description() { return tr('创建客服会话或同步平台消息后，会话会在此展示。'); },
    get action() { return tr('新建会话'); },
    actionPath: '/customer/conversations',
    get permissionHint() { return tr(PERM_HINT); },
  },
  taskFailures: {
    get title() { return tr('暂无失败任务'); },
    get description() { return tr('系统运行正常，当前没有需要重试的失败任务。'); },
    get action() { return tr('查看配置状态'); },
    actionPath: '/settings/config-status',
  },
  operationTasks: {
    get title() { return tr('暂无运营任务'); },
    get description() { return tr('当前没有需要审核或执行的运营任务；可稍后刷新查看。'); },
  },
  configStatus: {
    get title() { return tr('暂无配置项'); },
    get description() { return tr('配置状态中心会聚合 AI、存储、平台等配置健康状态；若为空请刷新或检查后台服务。'); },
    get action() { return tr('前往系统设置'); },
    actionPath: '/settings/integrations',
  },
  usersSettings: {
    get title() { return tr('暂无用户'); },
    get description() { return tr('管理员可在此创建后台账号并分配角色与店铺权限。'); },
    get action() { return tr('创建用户'); },
  },
  operationLogs: {
    get title() { return tr('暂无操作日志'); },
    get description() { return tr('关键配置变更与业务操作会记录在此，便于审计追溯。'); },
  },
  orderSync: {
    get title() { return tr('暂无同步任务'); },
    get description() { return tr('完成店铺授权后，可以手动同步订单。'); },
    get action() { return tr('前往店铺管理'); },
    actionPath: '/shops/manage',
  },
  productDraft: {
    get title() { return tr('暂无商品草稿'); },
    get description() { return tr('可以先从采集中心采集商品，或手动创建商品草稿。'); },
    get action() { return tr('前往采集中心'); },
    actionPath: '/collect/hub',
  },
} as const satisfies Record<string, ListEmptyCopy>;

/** @deprecated 使用 LIST_EMPTY_COPY */
export const EMPTY_COPY = LIST_EMPTY_COPY;

/** 发布检查项级别 */
export const READINESS_LEVEL_LABEL: Record<string, string> = {
  get error() { return tr('错误'); },
  get warning() { return tr('建议检查'); },
  get failed() { return tr('错误'); },
};

export function readinessLevelLabel(level?: string | null): string {
  const k = (level ?? '').trim().toLowerCase();
  if (!k) return '—';
  return READINESS_LEVEL_LABEL[k] ?? level ?? '—';
}

/** 刊登任务发布模式 */
export const PUBLISH_MODE_LABEL: Record<string, string> = {
  get save_as_platform_draft() { return tr('保存为平台草稿'); },
  get publish() { return tr('直接刊登'); },
  get draft() { return tr('保存草稿'); },
};

export function publishModeLabel(mode?: string | null): string {
  const k = (mode ?? 'save_as_platform_draft').trim();
  return PUBLISH_MODE_LABEL[k] ?? k;
}

/** AI 文案字段（商品草稿内） */
export const AI_FIELD_COPY = {
  get aiTitle() { return tr('AI 优化标题'); },
  get aiDescription() { return tr('AI 优化描述'); },
} as const;

/** 将内部英文状态码转为用户可见文案（兜底） */
export const COMMON_STATUS_LABEL: Record<string, string> = {
  get pending() { return tr('等待处理'); },
  get running() { return tr('处理中'); },
  get success() { return tr('成功'); },
  get partial_success() { return tr('部分成功'); },
  get failed() { return tr('失败'); },
  get cancelled() { return tr('已取消'); },
  get enabled() { return tr('已开启'); },
  get disabled() { return tr('未开启'); },
  get configured() { return tr('已配置'); },
  get unconfigured() { return tr('未配置'); },
  get authorized() { return tr('已授权'); },
  get expired() { return tr('授权已过期'); },
  get need_check() { return tr('需要检查'); },
  get bound() { return tr('已绑定'); },
  get unmatched() { return tr('未匹配'); },
  get ambiguous() { return tr('需要人工确认'); },
  get skipped() { return tr('已跳过'); },
  get retryable() { return tr('可以重试'); },
  get ready() { return tr('已准备好'); },
  get draft() { return tr('草稿'); },
  get draft_created() { return tr('平台草稿已创建'); },
  get warning() { return tr('建议检查'); },
  get blocked() { return tr('暂不能继续'); },
  get passed() { return tr('检查通过'); },
  get checking() { return tr('检查中'); },
  get publishing() { return tr('刊登中'); },
  get suggested() { return tr('待处理建议'); },
  get draft_preparing() { return tr('草稿准备中'); },
  get pending_review() { return tr('待复核'); },
  get approved() { return tr('已批准'); },
  get rejected() { return tr('已拒绝'); },
  get execution_queued() { return tr('等待执行'); },
  get executing() { return tr('执行中'); },
  get draft_written() { return tr('草稿已生成'); },
  get execution_failed() { return tr('执行失败'); },
  get editable() { return tr('可编辑'); },
  get superseded() { return tr('已被新版本取代'); },
  get written() { return tr('已写入草稿'); },
  get queued() { return tr('排队中'); },
  get succeeded() { return tr('已完成'); },
  get mock() { return tr('模拟模式'); },
  get sandbox() { return tr('沙箱模式'); },
  get local_draft_only() { return tr('仅本地草稿'); },
  get real_draft_create() { return tr('创建平台草稿'); },
  get blocked_by_real_credentials() { return tr('缺少真实凭证'); },
  get blocked_by_provider_config() { return tr('接入服务未配置'); },
  get unsupported_by_provider() { return tr('当前服务不支持'); },
  get permission_denied() { return tr('无权限'); },
  get readonly_operation_forbidden() { return tr('只读账号不可操作'); },
  get store_permission_denied() { return tr('无店铺权限'); },
  get inventory_sync_enabled() { return tr('库存同步已开启'); },
  get manual_bound() { return tr('人工绑定'); },
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
