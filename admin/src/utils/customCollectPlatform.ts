export type CustomCollectPlatformBlocked = {
  kind: 'blocked';
  source: string;
  platformLabel: string;
  message: string;
  actionLabel: string;
};

export type CustomCollectPlatformPlanned = {
  kind: 'planned';
  platformLabel: string;
  message: string;
  actionLabel: '我知道了';
};

export type CustomCollectPlatformHint = CustomCollectPlatformBlocked | CustomCollectPlatformPlanned;

function hostnameFromUrl(urlStr: string): string {
  try {
    return new URL(urlStr.trim()).hostname.toLowerCase();
  } catch {
    return '';
  }
}

function hostMatches1688(host: string): boolean {
  return host === '1688.com' || host.endsWith('.1688.com');
}

function hostMatchesAliExpress(host: string): boolean {
  return host.includes('aliexpress');
}

function hostMatchesTaobaoTmall(host: string): boolean {
  if (host === 'taobao.com' || host.endsWith('.taobao.com')) return true;
  if (host === 'tmall.com' || host.endsWith('.tmall.com')) return true;
  return host === 'item.taobao.com' || host === 'detail.tmall.com';
}

function hostMatchesPdd(host: string): boolean {
  if (host === 'pinduoduo.com' || host === 'yangkeduo.com' || host === 'mobile.yangkeduo.com') {
    return true;
  }
  return host.endsWith('.pinduoduo.com') || host.endsWith('.yangkeduo.com');
}

function hostMatchesSheinTemu(host: string): boolean {
  if (host === 'shein.com' || host.endsWith('.shein.com')) return true;
  return host === 'temu.com' || host.endsWith('.temu.com');
}

/** Detect whether a URL belongs to a known dedicated platform (for custom collect UX). */
export function detectCustomCollectPlatform(urlStr: string): CustomCollectPlatformHint | null {
  const host = hostnameFromUrl(urlStr);
  if (!host) return null;

  if (hostMatches1688(host)) {
    return {
      kind: 'blocked',
      source: '1688',
      platformLabel: '1688',
      message:
        '该链接属于 1688 平台，请使用「1688 采集器」。1688 采集器已针对商品标题、主图、详情图、商品参数、商品规格做专门适配，识别更稳定。',
      actionLabel: '去使用 1688 采集器',
    };
  }

  if (hostMatchesAliExpress(host)) {
    return {
      kind: 'blocked',
      source: 'aliexpress',
      platformLabel: 'AliExpress',
      message: '该链接属于 AliExpress 平台，请使用「速卖通采集器」。专用采集器字段识别更稳定。',
      actionLabel: '去使用速卖通采集器',
    };
  }

  if (hostMatchesTaobaoTmall(host)) {
    return {
      kind: 'planned',
      platformLabel: '淘宝 / 天猫',
      message:
        '该链接属于淘宝 / 天猫平台，专用采集器暂未开放。当前不建议使用自定义链接采集器采集该平台，可能因为登录、风控或页面结构导致失败。',
      actionLabel: '我知道了',
    };
  }

  if (hostMatchesPdd(host)) {
    return {
      kind: 'planned',
      platformLabel: '拼多多',
      message:
        '该链接属于暂未开放专用采集器的平台。请先创建采集规则并测试采集效果，再开始采集。',
      actionLabel: '我知道了',
    };
  }

  if (hostMatchesSheinTemu(host)) {
    return {
      kind: 'planned',
      platformLabel: 'SHEIN / Temu',
      message:
        '该链接属于暂未开放专用采集器的平台。请先创建采集规则并测试采集效果，再开始采集。',
      actionLabel: '我知道了',
    };
  }

  return null;
}

export const CUSTOM_COLLECT_USAGE_LINES = [
  '用于采集没有专用采集器的网站商品页。请先创建采集规则，再开始采集。',
  '如果是 1688、速卖通等已支持的平台，请优先使用对应的专用采集器，识别更稳定。',
  '建议先测试采集效果，确认标题、价格、图片识别正确后再提交采集任务。',
] as const;

export const CUSTOM_COLLECT_CARD_DESCRIPTION =
  '用于采集没有专用采集器的网站商品页。请先创建采集规则，再开始采集。';

export const CUSTOM_COLLECT_CARD_NOTES =
  '已支持 1688、速卖通等平台请使用专用采集器；自定义链接批量采集暂未开放。';

export const CUSTOM_BATCH_DISABLED_TOOLTIP =
  '自定义链接批量采集暂未开放。请先使用单条采集验证规则是否稳定。';
