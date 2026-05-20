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
      return '图片缺失';
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
    case 'PRODUCT_NOT_FOUND':
      return '商品不存在或已下架';
    case 'INVALID_URL':
      return '链接无效';
    case 'UNSUPPORTED_PINDUODUO_URL':
      return '链接类型暂未支持';
    case 'WECHAT_AUTH_REQUIRED':
      return '需要微信授权';
    case 'APP_REDIRECT':
      return 'App 引导页';
    default:
      return '';
  }
}

/** 错误码 → 操作建议（主流程展示） */
export function mapCollectorErrorCodeDetail(code?: string | null, source?: string | null): string {
  const c = (code ?? '').trim().toUpperCase();
  const src = (source ?? '').trim().toLowerCase();
  const isPdd = src === 'pinduoduo' || src === 'pdd';

  switch (c) {
    case 'LOGIN_REQUIRED':
      return isPdd
        ? '该页面需要登录后才能采集。请打开拼多多采集浏览器完成登录或微信扫码授权后重试。'
        : '当前商品页跳转到了登录页面，请先使用采集浏览器登录后再测试。';
    case 'WECHAT_AUTH_REQUIRED':
      return '拼多多登录需要微信扫码授权，请在采集浏览器中完成扫码后再重试。';
    case 'PAGE_BLOCKED_OR_VERIFY_REQUIRED':
    case 'PAGE_BLOCKED':
    case 'VERIFY_REQUIRED':
    case 'CAPTCHA':
      return isPdd
        ? '拼多多页面可能触发验证或风控，请稍后重试，或在采集浏览器中手动完成验证。'
        : '目标网站可能出现验证码或安全验证，请稍后重试，或在采集浏览器中手动完成验证。';
    case 'CUSTOM_RULE_MISSING':
      return '请先创建采集规则，或使用「AI 帮我生成规则」。';
    case 'CUSTOM_RULE_INVALID':
      return '采集规则内容格式不正确，建议使用「AI 帮我生成规则」重新生成，或由熟悉网站结构的人员调整。';
    case 'PARSE_FAILED_TITLE_MISSING':
      return isPdd
        ? '页面已打开，但没有识别到商品标题，可能是页面结构变化或登录态不足。'
        : '请检查商品标题对应的页面位置，或重新使用 AI 生成规则。';
    case 'PARSE_FAILED_IMAGE_MISSING':
      return isPdd
        ? '系统未识别到商品主图。请重试采集，或进入商品草稿后手动添加主图。'
        : '请检查主图规则，或开启图片过滤后重新测试。';
    case 'PARSE_FAILED':
      return isPdd
        ? '部分商品信息未能识别，请采集后人工检查。'
        : '部分商品信息未能识别，请测试采集效果后调整规则或重新生成。';
    case 'NAVIGATION_FAILED':
      return '请检查商品链接是否有效、网络是否正常。';
    case 'TIMEOUT':
    case 'PAGE_TIMEOUT':
    case 'PAGE_LOAD_TIMEOUT':
      return '页面加载时间过长，请稍后重试。';
    case 'PRODUCT_NOT_FOUND':
      return '商品不存在、已下架或链接无效。';
    case 'INVALID_URL':
      return isPdd ? '请输入拼多多商品详情页链接。' : '请输入有效的商品详情页链接。';
    case 'UNSUPPORTED_PINDUODUO_URL':
      return isPdd
        ? '请使用拼多多批发商品详情链接（pifa.pinduoduo.com/goods/detail/?gid=）。移动端商品页、批发首页等链接暂不支持直接采集。'
        : '当前链接类型暂未支持。';
    case 'APP_REDIRECT':
      return '当前为 App 引导页，请换用拼多多批发商品详情链接。';
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
export function mapCollectErrorMessage(err: unknown, source?: string | null): string {
  const raw = err instanceof Error ? err.message : String(err ?? '');
  const upper = raw.toUpperCase();

  if (
    upper.includes('CUSTOM_COLLECT_PROVIDER_CONFLICT') ||
    raw.includes('请使用「1688 采集器」') ||
    raw.includes('请使用「速卖通采集器」') ||
    raw.includes('请使用「拼多多采集器」')
  ) {
    return raw.includes('请使用') ? raw : '该链接已有专用采集器，请使用对应专用采集器。';
  }
  if (upper.includes('NOT_A_PINDUODUO')) {
    return mapCollectorErrorCodeDetail('INVALID_URL', 'pinduoduo');
  }
  if (raw.includes('custom collect rule not found')) {
    return mapCollectorErrorCodeDetail('CUSTOM_RULE_MISSING', source);
  }
  if (upper.includes('CUSTOM_RULE_MISSING') || raw.includes('missing rule')) {
    return mapCollectorErrorCodeDetail('CUSTOM_RULE_MISSING', source);
  }
  if (upper.includes('CUSTOM_RULE_INVALID')) {
    return mapCollectorErrorCodeDetail('CUSTOM_RULE_INVALID', source);
  }
  if (upper.includes('AI_RULE_INVALID') || raw.includes('AI_RULE_INVALID')) {
    return mapCollectorErrorCodeDetail('AI_RULE_INVALID', source);
  }
  if (raw.includes('请先到「设置 → AI 设置」')) {
    return raw;
  }
  if (raw.includes('AI 生成采集规则已关闭')) {
    return 'AI 生成采集规则功能已关闭，请在「采集设置 → 自定义链接」中开启。';
  }
  if (upper.includes('PRODUCT_NOT_FOUND')) {
    return mapCollectorErrorCodeDetail('PRODUCT_NOT_FOUND', source);
  }
  if (upper.includes('WECHAT_AUTH') || raw.includes('open.weixin.qq.com')) {
    return mapCollectorErrorCodeDetail('WECHAT_AUTH_REQUIRED', source);
  }
  if (upper.includes('LOGIN_REQUIRED')) {
    return mapCollectorErrorCodeDetail('LOGIN_REQUIRED', source);
  }
  if (upper.includes('UNSUPPORTED_PINDUODUO_URL')) {
    return mapCollectorErrorCodeDetail('UNSUPPORTED_PINDUODUO_URL', source);
  }
  if (upper.includes('PARSE_FAILED_TITLE_MISSING')) {
    return mapCollectorErrorCodeDetail('PARSE_FAILED_TITLE_MISSING', source);
  }
  if (upper.includes('NO_MAIN_IMAGES') || upper.includes('NO_MAIN_IMAGES_WARNING')) {
    return '未识别到商品主图，请在图片管理中手动添加主图后再发布。';
  }
  if (upper.includes('PARSE_FAILED_IMAGE_MISSING') || raw.includes('no_main_images')) {
    return mapCollectorErrorCodeDetail('PARSE_FAILED_IMAGE_MISSING', source);
  }
  if (upper.includes('PARSE_FAILED')) {
    return mapCollectorErrorCodeDetail('PARSE_FAILED', source);
  }
  if (upper.includes('TIMEOUT')) {
    return mapCollectorErrorCodeDetail('TIMEOUT', source);
  }
  if (upper.includes('NAVIGATION_FAILED')) {
    return mapCollectorErrorCodeDetail('NAVIGATION_FAILED', source);
  }
  if (upper.includes('PAGE_BLOCKED_OR_VERIFY_REQUIRED')) {
    return mapCollectorErrorCodeDetail('PAGE_BLOCKED_OR_VERIFY_REQUIRED', source);
  }
  if (upper.includes('PROFILE_LOGIN_REQUIRED')) {
    return mapCollectorErrorCodeDetail('PROFILE_LOGIN_REQUIRED', source);
  }
  if (upper.includes('PROFILE_NOT_FOUND')) {
    return mapCollectorErrorCodeDetail('PROFILE_NOT_FOUND', source);
  }
  if (upper.includes('HEADED_BROWSER_REQUIRED')) {
    return mapCollectorErrorCodeDetail('HEADED_BROWSER_REQUIRED', source);
  }
  if (raw.includes('url does not match') || raw.includes('hostname does not match')) {
    return raw;
  }
  return raw || '采集失败';
}
