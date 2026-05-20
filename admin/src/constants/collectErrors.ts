/** 采集字段英文名 → 用户可见中文 */
export const COLLECT_FIELD_LABELS: Record<string, string> = {
  title: '商品标题',
  price: '商品价格',
  mainImage: '商品主图',
  mainImages: '商品主图',
  descriptionImages: '详情图片',
  detailImages: '详情图片',
  detailImagesCount: '详情图片',
  attributes: '商品参数',
  attributesCount: '商品参数',
  skus: '商品规格',
};

export function mapCollectFieldLabel(field: string): string {
  const key = field.trim();
  return COLLECT_FIELD_LABELS[key] ?? key;
}

/** 采集/规则测试错误码 → 用户可见短标题（不展示英文码） */
export function mapCollectorErrorCodeLabel(code?: string | null): string {
  const c = (code ?? '').trim().toUpperCase();
  switch (c) {
    case 'LOGIN_REQUIRED':
      return '页面需要登录';
    case 'PAGE_BLOCKED_OR_VERIFY_REQUIRED':
    case 'PAGE_BLOCKED':
    case 'VERIFY_REQUIRED':
    case 'CAPTCHA':
      return '页面需要验证';
    case 'CUSTOM_RULE_MISSING':
      return '没有找到可用采集规则';
    case 'CUSTOM_RULE_INVALID':
      return '采集规则内容有误';
    case 'PARSE_FAILED_TITLE_MISSING':
      return '没有识别到商品标题';
    case 'PARSE_FAILED_IMAGE_MISSING':
      return '没有识别到商品图片';
    case 'PARSE_FAILED':
      return '页面内容识别不完整';
    case 'NAVIGATION_FAILED':
      return '页面无法打开';
    case 'TIMEOUT':
    case 'PAGE_TIMEOUT':
    case 'PAGE_LOAD_TIMEOUT':
      return '页面加载超时';
    case 'PROFILE_NOT_FOUND':
      return '登录状态不存在或已停用';
    case 'PROFILE_LOGIN_REQUIRED':
      return '尚未完成登录';
    case 'HEADED_BROWSER_REQUIRED':
      return '无法打开登录窗口';
    case 'AI_RULE_INVALID':
      return 'AI 生成的规则未通过校验';
    default:
      return '';
  }
}

/** 错误码 → 操作建议（主流程展示） */
export function mapCollectorErrorCodeDetail(code?: string | null): string {
  const c = (code ?? '').trim().toUpperCase();
  switch (c) {
    case 'LOGIN_REQUIRED':
      return '当前商品页跳转到了登录页面，请先使用采集浏览器登录后再测试。';
    case 'PAGE_BLOCKED_OR_VERIFY_REQUIRED':
    case 'PAGE_BLOCKED':
    case 'VERIFY_REQUIRED':
    case 'CAPTCHA':
      return '目标网站可能出现验证码或安全验证，请稍后重试，或在采集浏览器中手动完成验证。';
    case 'CUSTOM_RULE_MISSING':
      return '请先创建采集规则，或使用「AI 帮我生成规则」。';
    case 'CUSTOM_RULE_INVALID':
      return '采集规则内容格式不正确，建议使用「AI 帮我生成规则」重新生成，或由熟悉网站结构的人员调整。';
    case 'PARSE_FAILED_TITLE_MISSING':
      return '请检查商品标题对应的页面位置，或重新使用 AI 生成规则。';
    case 'PARSE_FAILED_IMAGE_MISSING':
      return '请检查主图规则，或开启图片过滤后重新测试。';
    case 'PARSE_FAILED':
      return '部分商品信息未能识别，请测试采集效果后调整规则或重新生成。';
    case 'NAVIGATION_FAILED':
      return '请检查商品链接是否有效、网络是否正常。';
    case 'TIMEOUT':
    case 'PAGE_TIMEOUT':
    case 'PAGE_LOAD_TIMEOUT':
      return '页面加载时间过长，请稍后重试。';
    case 'PROFILE_NOT_FOUND':
      return '请重新选择登录状态，或新建一条适用于该网站的登录状态。';
    case 'PROFILE_LOGIN_REQUIRED':
      return '请先打开采集浏览器完成登录，再点击「重新检测登录状态」。';
    case 'HEADED_BROWSER_REQUIRED':
      return '当前采集服务未开启可视化浏览器，无法弹出登录窗口。请联系管理员在采集设置中开启「显示浏览器窗口」并重启采集服务。';
    case 'AI_RULE_INVALID':
      return '请调整「要采集的内容」后重新生成，或手动修改采集规则内容（高级）。';
    default:
      return '';
  }
}

/** 将后端 / 采集服务错误转为用户可读说明（不含英文错误码） */
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
    return mapCollectorErrorCodeDetail('CUSTOM_RULE_MISSING');
  }
  if (upper.includes('CUSTOM_RULE_MISSING') || raw.includes('missing rule')) {
    return mapCollectorErrorCodeDetail('CUSTOM_RULE_MISSING');
  }
  if (upper.includes('CUSTOM_RULE_INVALID')) {
    return mapCollectorErrorCodeDetail('CUSTOM_RULE_INVALID');
  }
  if (upper.includes('AI_RULE_INVALID') || raw.includes('AI_RULE_INVALID')) {
    return mapCollectorErrorCodeDetail('AI_RULE_INVALID');
  }
  if (raw.includes('请先到「设置 → AI 设置」')) {
    return raw;
  }
  if (raw.includes('AI 生成采集规则已关闭')) {
    return 'AI 生成采集规则功能已关闭，请在「采集设置 → 自定义链接」中开启。';
  }
  if (upper.includes('LOGIN_REQUIRED')) {
    return mapCollectorErrorCodeDetail('LOGIN_REQUIRED');
  }
  if (upper.includes('PARSE_FAILED_TITLE_MISSING')) {
    return mapCollectorErrorCodeDetail('PARSE_FAILED_TITLE_MISSING');
  }
  if (upper.includes('PARSE_FAILED_IMAGE_MISSING')) {
    return mapCollectorErrorCodeDetail('PARSE_FAILED_IMAGE_MISSING');
  }
  if (upper.includes('PARSE_FAILED')) {
    return mapCollectorErrorCodeDetail('PARSE_FAILED');
  }
  if (upper.includes('TIMEOUT')) {
    return mapCollectorErrorCodeDetail('TIMEOUT');
  }
  if (upper.includes('NAVIGATION_FAILED')) {
    return mapCollectorErrorCodeDetail('NAVIGATION_FAILED');
  }
  if (upper.includes('PAGE_BLOCKED_OR_VERIFY_REQUIRED')) {
    return mapCollectorErrorCodeDetail('PAGE_BLOCKED_OR_VERIFY_REQUIRED');
  }
  if (upper.includes('PROFILE_LOGIN_REQUIRED')) {
    return mapCollectorErrorCodeDetail('PROFILE_LOGIN_REQUIRED');
  }
  if (upper.includes('PROFILE_NOT_FOUND')) {
    return mapCollectorErrorCodeDetail('PROFILE_NOT_FOUND');
  }
  if (upper.includes('HEADED_BROWSER_REQUIRED')) {
    return mapCollectorErrorCodeDetail('HEADED_BROWSER_REQUIRED');
  }
  if (raw.includes('url does not match') || raw.includes('hostname does not match')) {
    return raw;
  }
  return raw || '采集失败';
}
