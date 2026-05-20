/** Chinese explanation for collector error_code on task detail. */
export function mapCollectorErrorCodeLabel(code?: string | null): string {
  const c = (code ?? '').trim().toUpperCase();
  switch (c) {
    case 'LOGIN_REQUIRED':
      return '页面疑似需要登录，自定义采集器无法自动登录。请确认商品页是否公开可访问。';
    case 'PAGE_BLOCKED_OR_VERIFY_REQUIRED':
      return '页面疑似触发验证码或风控，请稍后重试或降低采集频率。';
    case 'CUSTOM_RULE_MISSING':
      return '未找到匹配的自定义采集规则，请先创建并启用规则。';
    case 'CUSTOM_RULE_INVALID':
      return '采集规则 JSON 无效，请检查 selector 与 type。';
    case 'PARSE_FAILED_TITLE_MISSING':
      return '页面已打开但未提取到标题，请检查 title 选择器。';
    case 'PARSE_FAILED_IMAGE_MISSING':
      return '页面已打开但未提取到主图，请检查 mainImage 选择器。';
    case 'NAVIGATION_FAILED':
      return '页面导航失败，请检查链接是否可访问。';
    case 'TIMEOUT':
      return '页面加载超时，请稍后重试。';
    case 'PROFILE_NOT_FOUND':
      return '采集浏览器 Profile 不存在或已停用。';
    case 'PROFILE_LOGIN_REQUIRED':
      return '当前 Profile 仍未登录，请打开采集浏览器完成登录后重新检测。';
    case 'HEADED_BROWSER_REQUIRED':
      return 'Collector 为无头模式，无法打开登录窗口。请设置 COLLECTOR_HEADLESS=0 并重启采集服务。';
    default:
      return '';
  }
}

/** Map backend / collector error text to operator-friendly hints. */
export function mapCollectErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err ?? '');
  const upper = raw.toUpperCase();

  if (
    upper.includes('CUSTOM_COLLECT_PROVIDER_CONFLICT') ||
    raw.includes('请使用「1688 采集器」') ||
    raw.includes('请使用「速卖通采集器」')
  ) {
    return raw.includes('请使用') ? raw : '该链接已有专用采集器，请使用对应专用采集器。';
  }
  if (raw.includes('custom collect rule not found')) {
    return '未找到匹配的自定义采集规则，请先在「采集规则」创建并启用规则。';
  }
  if (upper.includes('CUSTOM_RULE_MISSING') || raw.includes('missing rule')) {
    return '未找到采集规则配置（CUSTOM_RULE_MISSING），请重新选择规则后提交。';
  }
  if (upper.includes('CUSTOM_RULE_INVALID')) {
    return '采集规则格式错误（CUSTOM_RULE_INVALID），请检查 selector 与 type。';
  }
  if (upper.includes('LOGIN_REQUIRED')) {
    return mapCollectorErrorCodeLabel('LOGIN_REQUIRED');
  }
  if (upper.includes('PARSE_FAILED_TITLE_MISSING')) {
    return '页面已打开，但未提取到商品标题（PARSE_FAILED_TITLE_MISSING），请检查标题选择器。';
  }
  if (upper.includes('TIMEOUT')) {
    return mapCollectorErrorCodeLabel('TIMEOUT');
  }
  if (upper.includes('PARSE_FAILED_IMAGE_MISSING')) {
    return '页面已打开，但未提取到商品图片（PARSE_FAILED_IMAGE_MISSING），请检查图片选择器。';
  }
  if (upper.includes('NAVIGATION_FAILED')) {
    return '页面打开失败（NAVIGATION_FAILED），请检查链接是否可访问。';
  }
  if (upper.includes('PAGE_BLOCKED_OR_VERIFY_REQUIRED')) {
    return '目标网站触发验证或登录（PAGE_BLOCKED_OR_VERIFY_REQUIRED），请稍后重试或完成站点登录。';
  }
  if (raw.includes('url does not match') || raw.includes('hostname does not match')) {
    return raw;
  }
  return raw || '采集失败';
}
