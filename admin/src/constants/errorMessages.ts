import { tr } from '@/i18n';
/**
 * 错误码 → 用户可见提示（含操作建议）。
 * 技术错误码仅在「技术详情」中展示。
 */

export type UserErrorMessage = {
  title: string;
  detail?: string;
  action?: string;
};

const ERROR_MAP: Record<string, UserErrorMessage> = {
  AUTH_INVALID_CREDENTIALS: {
    get title() { return tr("账号或密码不正确"); },
    get detail() { return tr('请检查邮箱、手机号和密码后再试。'); },
  },
  AUTH_ACCOUNT_TEMPORARILY_LOCKED: {
    get title() { return tr("账号暂时被锁定"); },
    get detail() { return tr('登录失败次数过多，请稍后再试。'); },
  },
  AUTH_TOO_MANY_ATTEMPTS: {
    get title() { return tr("登录尝试过于频繁"); },
    get detail() { return tr('请稍后再试，或联系管理员确认账号状态。'); },
  },
  AUTH_USER_DISABLED: {
    get title() { return tr("账号已被停用"); },
    get detail() { return tr('请联系管理员启用账号后再登录。'); },
  },
  DOUYIN_SKU_NOT_BOUND: {
    get title() { return tr("该规格尚未绑定抖店规格"); },
    get detail() { return tr('请先完成规格绑定后再同步库存。'); },
    get action() { return tr('前往商品详情完成规格绑定'); },
  },
  DOUYIN_SKU_BINDING_AMBIGUOUS: {
    get title() { return tr("规格绑定存在歧义"); },
    get detail() { return tr('多个抖店规格与本地规格匹配，需要人工确认。'); },
    get action() { return tr('在商品详情中校准规格绑定'); },
  },
  DOUYIN_PRODUCT_NOT_BOUND: {
    get title() { return tr("商品尚未绑定抖店商品"); },
    get detail() { return tr('请先完成商品绑定后再进行同步或刊登。'); },
  },
  DOUYIN_INVENTORY_SYNC_NOT_READY: {
    get title() { return tr("库存同步尚未就绪"); },
    get detail() { return tr('请确认已在平台接入设置中开启库存同步，并完成店铺授权。'); },
  },
  DOUYIN_INVENTORY_PERMISSION_DENIED: {
    get title() { return tr("没有库存同步权限"); },
    get detail() { return tr('请确认抖店应用已申请库存相关权限，并重新授权店铺。'); },
  },
  DOUYIN_PERMISSION_DENIED: {
    get title() { return tr("平台权限不足"); },
    get detail() { return tr('请确认应用权限已开通，并重新授权店铺。'); },
  },
  DOUYIN_STORE_NOT_AUTHORIZED: {
    get title() { return tr("店铺尚未授权"); },
    get detail() { return tr('请先在店铺管理中完成抖店授权。'); },
    get action() { return tr('前往店铺管理'); },
  },
  DOUYIN_AUTH_EXPIRED: {
    get title() { return tr("店铺授权已过期"); },
    get detail() { return tr('请重新授权店铺后再试。'); },
    get action() { return tr('前往店铺管理重新授权'); },
  },
  DOUYIN_IMAGE_UPLOAD_FAILED: {
    get title() { return tr("图片上传到抖店失败"); },
    get detail() { return tr('请检查图片是否可以正常访问，然后重试。'); },
  },
  DOUYIN_GRAY_RELEASE_NOT_ENABLED: {
    get title() { return tr("抖店灰度发布未启用"); },
    get detail() { return tr('当前环境尚未开启灰度发布，写操作已被拦截。'); },
    get action() { return tr('前往平台接入设置检查灰度开关'); },
  },
  DOUYIN_SHOP_NOT_IN_GRAY_LIST: {
    get title() { return tr("店铺不在灰度范围内"); },
    get detail() { return tr('该店铺尚未加入抖店灰度店铺列表，暂时不能执行写操作。'); },
    get action() { return tr('在平台接入设置中配置灰度店铺'); },
  },
  DOUYIN_WRITE_OPERATION_DISABLED: {
    get title() { return tr("抖店写操作已关闭"); },
    get detail() { return tr('写操作总开关已关闭，刊登、同步等写接口不会执行。'); },
    get action() { return tr('前往平台接入设置开启写操作'); },
  },
  DOUYIN_TASK_STALE: {
    get title() { return tr("任务执行时间过长"); },
    get detail() { return tr('任务长时间无进展，可能后台任务进程异常或平台响应缓慢。'); },
    get action() { return tr('在失败任务中心查看详情并重试'); },
  },
  DOUYIN_TASK_RESULT_UNKNOWN: {
    get title() { return tr("平台处理结果暂时无法确认"); },
    get detail() { return tr('请求可能已到达平台，但本地未收到明确结果。商品草稿可先尝试回查恢复。'); },
    get action() { return tr('在失败任务中心尝试恢复或人工确认'); },
  },
  DOUYIN_TASK_RECOVERY_REQUIRED: {
    get title() { return tr("需要人工检查后才能继续"); },
    get detail() { return tr('自动恢复未成功，请确认平台侧状态后再重试。'); },
    get action() { return tr('在失败任务中心处理'); },
  },
  DOUYIN_TASK_RECOVERY_FAILED: {
    get title() { return tr("自动恢复失败"); },
    get detail() { return tr('平台回查未能确认结果，请人工核对后再操作。'); },
    get action() { return tr('在失败任务中心查看详情'); },
  },
  DOUYIN_PLATFORM_PAUSED: {
    get title() { return tr("抖店任务已暂停"); },
    get detail() { return tr('管理员已暂停抖店写操作，任务不会继续执行。'); },
    get action() { return tr('前往平台运行状态页恢复'); },
  },
  DOUYIN_PLATFORM_EMERGENCY_DISABLED: {
    get title() { return tr("抖店已紧急停用"); },
    get detail() { return tr('所有抖店写接口已停用，需管理员恢复后才能继续。'); },
    get action() { return tr('前往平台运行状态页'); },
  },
  DOUYIN_TASK_BLOCKED_BY_RUNTIME_STATUS: {
    get title() { return tr("任务被运行状态拦截"); },
    get detail() { return tr('抖店当前处于暂停或紧急停用状态。'); },
    get action() { return tr('前往平台运行状态页'); },
  },
  UNKNOWN_DOUYIN_AUTH_ERROR: {
    get title() { return tr("抖店店铺授权失败"); },
    get detail() { return tr('请检查应用信息是否正确，并确认授权回调地址与开放平台登记一致。'); },
  },
  platform_auth: {
    get title() { return tr("平台授权失败"); },
    get detail() { return tr('请检查店铺授权是否有效，必要时重新授权。'); },
  },
  platform_permission: {
    get title() { return tr("平台权限不足"); },
    get detail() { return tr('请在平台开放中心确认应用权限已开通，并重新授权店铺。'); },
  },
  platform_config_incomplete: {
    get title() { return tr("平台配置不完整"); },
    get detail() { return tr('请先在平台接入设置中补齐必填项。'); },
    get action() { return tr('前往平台接入设置'); },
  },
  sku_mapping_missing: {
    get title() { return tr("规格尚未绑定"); },
    get detail() { return tr('请完成平台规格与本地规格的对应关系。'); },
  },
  inventory_mapping_missing: {
    get title() { return tr("库存映射缺失"); },
    get detail() { return tr('请确认商品与规格绑定关系完整后再同步库存。'); },
  },
  DETAIL_IMAGES_INCOMPLETE: {
    get title() { return tr("详情图不完整"); },
    get detail() { return tr('建议补充商品详情图，帮助买家了解商品细节。'); },
  },
  ATTRIBUTES_EMPTY: {
    get title() { return tr("商品参数未完善"); },
    get detail() { return tr('未识别到商品参数，建议手动补充品牌、材质等信息。'); },
  },
  STOCK_UNKNOWN: {
    get title() { return tr("库存信息不明确"); },
    get detail() { return tr('库存状态未知，发布前请人工确认各规格库存。'); },
  },
  PRICE_MISSING: { get title() { return tr("销售价格未设置"); }, get detail() { return tr('请为商品或规格填写有效销售价。'); } },
  MAIN_IMAGES_EMPTY: { get title() { return tr("商品主图缺失"); }, get detail() { return tr('至少需要一张有效主图。'); } },
  SKU_INCOMPLETE: { get title() { return tr("规格信息不完整"); }, get detail() { return tr('请核对规格、价格与库存。'); } },
  PLATFORM_NOT_SUPPORTED: {
    get title() { return tr("当前平台暂未接入真实发布"); },
    get detail() { return tr('将仅生成本地刊登草稿，不会调用平台接口。'); },
  },
  PUBLISH_CONFIG_MISSING: {
    get title() { return tr("刊登配置未完成"); },
    get detail() { return tr('请先在平台接入设置或商品刊登配置中补齐必填项。'); },
    get action() { return tr('前往平台接入设置'); },
  },
  PUBLISH_CONFIG_INVALID: {
    get title() { return tr("刊登配置不正确"); },
    get detail() { return tr('请检查统一配置与单独覆盖中的数值和策略选项。'); },
  },
};

/** 根据错误码获取用户可见提示 */
export function mapErrorCodeToUserMessage(code?: string | null): UserErrorMessage | undefined {
  const c = (code ?? '').trim().toUpperCase();
  if (!c) return undefined;
  if (ERROR_MAP[c]) return ERROR_MAP[c];
  // 前缀匹配
  for (const [key, msg] of Object.entries(ERROR_MAP)) {
    if (c.startsWith(key.toUpperCase())) return msg;
  }
  return undefined;
}

/** 格式化用户可见错误（一行） */
export function formatUserErrorMessage(code?: string | null, fallback?: string): string {
  const mapped = mapErrorCodeToUserMessage(code);
  if (mapped) return mapped.detail ? `${mapped.title}：${mapped.detail}` : mapped.title;
  const fb = (fallback ?? '').trim();
  if (fb && !/^[A-Z][A-Z0-9_]+$/.test(fb)) return fb;
  return fb || tr('操作失败，请稍后重试');
}

/** 从英文异常信息中提取用户可读部分（隐藏 JSON / 堆栈） */
export function sanitizeErrorForDisplay(raw?: string | null): string {
  const s = (raw ?? '').trim();
  if (!s) return '—';
  if (s.startsWith('{') || s.startsWith('[')) return tr('操作失败，请查看技术详情');
  if (/^[A-Z][A-Z0-9_]+$/.test(s)) {
    return formatUserErrorMessage(s, s);
  }
  return s;
}
