/** 商品运营进度、发布检查相关中文映射 */

export const READINESS_STATUS_LABEL: Record<string, string> = {
  ready: '已准备好',
  warning: '建议检查',
  blocked: '暂不能继续',
  passed: '检查通过',
  failed: '检查未通过',
};

export const READINESS_RESULT_LABEL: Record<string, string> = {
  passed: '检查通过',
  warning: '建议检查',
  failed: '检查未通过',
};

export const OPERATION_STEP_LABEL: Record<string, string> = {
  collect_review: '检查采集结果',
  title: '优化商品标题',
  description: '完善商品描述',
  images: '检查商品图片',
  pricing: '设置销售价格',
  attributes: '补充商品参数',
  publish_check: '完成发布检查',
  ready: '可以生成刊登草稿',
};

export const READINESS_GROUP_LABEL: Record<string, string> = {
  product: '商品信息',
  sku: '商品规格',
  image: '图片',
  inventory: '库存',
  collect: '采集提示',
  platform: '平台配置',
  pricing: '价格',
  attribute: '商品参数',
};

export function readinessStatusLabel(status?: string | null): string {
  const k = (status ?? '').trim().toLowerCase();
  return READINESS_STATUS_LABEL[k] ?? (k || '—');
}

export function readinessResultLabel(result?: string | null): string {
  const k = (result ?? '').trim().toLowerCase();
  return READINESS_RESULT_LABEL[k] ?? (k || '—');
}

export function readinessGroupLabel(group?: string | null): string {
  const k = (group ?? '').trim().toLowerCase();
  return READINESS_GROUP_LABEL[k] ?? group ?? '—';
}

/** 发布检查错误码 → 中文（前端兜底，后端已本地化时优先用 API 字段） */
export const PUBLISH_CHECK_CODE_LABEL: Record<string, { title: string; message: string }> = {
  DETAIL_IMAGES_INCOMPLETE: {
    title: '详情图不完整',
    message: '建议补充商品详情图，帮助买家了解商品细节。',
  },
  ATTRIBUTES_EMPTY: {
    title: '商品参数未完善',
    message: '未识别到商品参数，建议手动补充。',
  },
  STOCK_UNKNOWN: {
    title: '库存信息不明确',
    message: '库存状态未知，发布前请人工确认。',
  },
  PRICE_MISSING: { title: '销售价格未设置', message: '请为商品或规格填写有效销售价。' },
  PRICE_INVALID: { title: '销售价格不正确', message: '销售价格无效或低于保护线。' },
  PRICE_PROFIT_TOO_LOW: { title: '预计利润率低于保护线', message: '请调整售价后再发布。' },
  MAIN_IMAGES_EMPTY: { title: '商品主图缺失', message: '至少需要一张有效主图。' },
  MAIN_IMAGES_NOT_UPLOADED: { title: '主图还未同步到平台', message: '请先同步图片到平台存储。' },
  DESCRIPTION_EMPTY: { title: '商品描述待完善', message: '请填写商品描述或生成 AI 描述。' },
  TITLE_EMPTY: { title: '商品标题待完善', message: '请填写清晰的商品标题。' },
  TITLE_TOO_LONG: { title: '商品标题过长', message: '请缩短标题长度。' },
  SKU_INCOMPLETE: { title: '规格信息不完整', message: '请核对规格、价格与库存。' },
  SKU_PRICE_MISSING: { title: '规格价格未设置', message: '请为每个规格填写售价。' },
  SKU_STOCK_MISSING: { title: '规格库存未确认', message: '请确认各规格库存。' },
  CATEGORY_REQUIRED: { title: '平台类目未选择', message: '请在刊登配置中选择平台类目。' },
  PLATFORM_ATTRIBUTES_REQUIRED: { title: '平台必填属性未完善', message: '请补齐平台必填属性。' },
  SHOP_NOT_AUTHORIZED: { title: '店铺尚未授权', message: '请前往店铺管理完成授权。' },
  PLATFORM_NOT_SUPPORTED: {
    title: '当前平台暂未接入真实发布',
    message: '将仅生成本地刊登草稿，不会调用平台 API。',
  },
  PUBLISH_CONFIG_MISSING: { title: '刊登配置未完成', message: '请补齐平台刊登配置。' },
};

export function localizePublishCheckItem(item: {
  code?: string;
  title?: string;
  message?: string;
  level?: string;
}): { title: string; message: string; severity: string } {
  const code = (item.code ?? '').trim().toUpperCase();
  const mapped = PUBLISH_CHECK_CODE_LABEL[code];
  const title = (item.title ?? mapped?.title ?? '').trim();
  const message = (item.message ?? mapped?.message ?? title).trim();
  const severity = (item.level ?? 'warning').toLowerCase();
  if (title && title !== code && !/^[A-Z][A-Z0-9_]+$/.test(title)) {
    return { title, message, severity };
  }
  if (mapped) {
    return { title: mapped.title, message: mapped.message || mapped.title, severity };
  }
  return {
    title: title || '需要检查',
    message: message && message !== code ? message : '请查看详情并处理后重试。',
    severity,
  };
}

export function localizeCollectWarningCode(code: string): string {
  const mapped = PUBLISH_CHECK_CODE_LABEL[code.trim().toUpperCase()];
  if (mapped) {
    return mapped.message ? `${mapped.title}：${mapped.message}` : mapped.title;
  }
  if (/^[A-Z][A-Z0-9_]+$/.test(code.trim())) {
    return '采集提示需检查：请核对商品内容后再发布。';
  }
  return code;
}
