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
  DOUYIN_SKU_NOT_BOUND: {
    title: '该规格尚未绑定抖店规格',
    detail: '请先完成规格绑定后再同步库存。',
    action: '前往商品详情完成规格绑定',
  },
  DOUYIN_SKU_BINDING_AMBIGUOUS: {
    title: '规格绑定存在歧义',
    detail: '多个抖店规格与本地规格匹配，需要人工确认。',
    action: '在商品详情中校准规格绑定',
  },
  DOUYIN_PRODUCT_NOT_BOUND: {
    title: '商品尚未绑定抖店商品',
    detail: '请先完成商品绑定后再进行同步或刊登。',
  },
  DOUYIN_INVENTORY_SYNC_NOT_READY: {
    title: '库存同步尚未就绪',
    detail: '请确认已在平台接入设置中开启库存同步，并完成店铺授权。',
  },
  DOUYIN_INVENTORY_PERMISSION_DENIED: {
    title: '没有库存同步权限',
    detail: '请确认抖店应用已申请库存相关权限，并重新授权店铺。',
  },
  DOUYIN_PERMISSION_DENIED: {
    title: '平台权限不足',
    detail: '请确认应用权限已开通，并重新授权店铺。',
  },
  DOUYIN_STORE_NOT_AUTHORIZED: {
    title: '店铺尚未授权',
    detail: '请先在店铺管理中完成抖店授权。',
    action: '前往店铺管理',
  },
  DOUYIN_AUTH_EXPIRED: {
    title: '店铺授权已过期',
    detail: '请重新授权店铺后再试。',
    action: '前往店铺管理重新授权',
  },
  DOUYIN_IMAGE_UPLOAD_FAILED: {
    title: '图片上传到抖店失败',
    detail: '请检查图片是否可以正常访问，然后重试。',
  },
  DOUYIN_GRAY_RELEASE_NOT_ENABLED: {
    title: '抖店灰度发布未启用',
    detail: '当前环境尚未开启灰度发布，写操作已被拦截。',
    action: '前往平台接入设置检查灰度开关',
  },
  DOUYIN_SHOP_NOT_IN_GRAY_LIST: {
    title: '店铺不在灰度范围内',
    detail: '该店铺尚未加入抖店灰度店铺列表，暂时不能执行写操作。',
    action: '在平台接入设置中配置灰度店铺',
  },
  DOUYIN_WRITE_OPERATION_DISABLED: {
    title: '抖店写操作已关闭',
    detail: '写操作总开关已关闭，刊登、同步等写接口不会执行。',
    action: '前往平台接入设置开启写操作',
  },
  DOUYIN_TASK_STALE: {
    title: '任务执行时间过长',
    detail: '任务长时间无进展，可能 Worker 异常或平台响应缓慢。',
    action: '在失败任务中心查看详情并重试',
  },
  DOUYIN_TASK_RESULT_UNKNOWN: {
    title: '平台处理结果暂时无法确认',
    detail: '请求可能已到达平台，但本地未收到明确结果。商品草稿可先尝试回查恢复。',
    action: '在失败任务中心尝试恢复或人工确认',
  },
  DOUYIN_TASK_RECOVERY_REQUIRED: {
    title: '需要人工检查后才能继续',
    detail: '自动恢复未成功，请确认平台侧状态后再重试。',
    action: '在失败任务中心处理',
  },
  DOUYIN_TASK_RECOVERY_FAILED: {
    title: '自动恢复失败',
    detail: '平台回查未能确认结果，请人工核对后再操作。',
    action: '在失败任务中心查看详情',
  },
  DOUYIN_PLATFORM_PAUSED: {
    title: '抖店任务已暂停',
    detail: '管理员已暂停抖店写操作，任务不会继续执行。',
    action: '前往平台运行状态页恢复',
  },
  DOUYIN_PLATFORM_EMERGENCY_DISABLED: {
    title: '抖店已紧急停用',
    detail: '所有抖店写接口已停用，需管理员恢复后才能继续。',
    action: '前往平台运行状态页',
  },
  DOUYIN_TASK_BLOCKED_BY_RUNTIME_STATUS: {
    title: '任务被运行状态拦截',
    detail: '抖店当前处于暂停或紧急停用状态。',
    action: '前往平台运行状态页',
  },
  UNKNOWN_DOUYIN_AUTH_ERROR: {
    title: '抖店店铺授权失败',
    detail: '请检查应用信息是否正确，并确认授权回调地址与开放平台登记一致。',
  },
  platform_auth: {
    title: '平台授权失败',
    detail: '请检查店铺授权是否有效，必要时重新授权。',
  },
  platform_permission: {
    title: '平台权限不足',
    detail: '请在平台开放中心确认应用权限已开通，并重新授权店铺。',
  },
  platform_config_incomplete: {
    title: '平台配置不完整',
    detail: '请先在平台接入设置中补齐必填项。',
    action: '前往平台接入设置',
  },
  sku_mapping_missing: {
    title: '规格尚未绑定',
    detail: '请完成平台规格与本地规格的对应关系。',
  },
  inventory_mapping_missing: {
    title: '库存映射缺失',
    detail: '请确认商品与规格绑定关系完整后再同步库存。',
  },
};

/** 根据错误码获取用户可见提示 */
export function mapErrorCodeToUserMessage(code?: string | null): UserErrorMessage | undefined {
  const c = (code ?? '').trim().toUpperCase();
  if (!c) return undefined;
  if (ERROR_MAP[c]) return ERROR_MAP[c];
  // 前缀匹配
  for (const [key, msg] of Object.entries(ERROR_MAP)) {
    if (c.startsWith(key)) return msg;
  }
  return undefined;
}

/** 格式化用户可见错误（一行） */
export function formatUserErrorMessage(code?: string | null, fallback?: string): string {
  const mapped = mapErrorCodeToUserMessage(code);
  if (mapped) return mapped.detail ? `${mapped.title}：${mapped.detail}` : mapped.title;
  const fb = (fallback ?? '').trim();
  if (fb && !/^[A-Z][A-Z0-9_]+$/.test(fb)) return fb;
  return fb || '操作失败，请稍后重试';
}

/** 从英文异常信息中提取用户可读部分（隐藏 JSON / 堆栈） */
export function sanitizeErrorForDisplay(raw?: string | null): string {
  const s = (raw ?? '').trim();
  if (!s) return '—';
  if (s.startsWith('{') || s.startsWith('[')) return '操作失败，请查看技术详情';
  if (/^[A-Z][A-Z0-9_]+$/.test(s)) {
    return formatUserErrorMessage(s, s);
  }
  return s;
}
