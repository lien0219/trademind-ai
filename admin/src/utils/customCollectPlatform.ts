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
      kind: 'blocked',
      source: 'pinduoduo',
      platformLabel: '拼多多',
      message:
        '该链接属于拼多多平台，请使用「拼多多采集器」。专用采集器字段识别更稳定。',
      actionLabel: '去使用拼多多采集器',
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
  '自定义链接采集适合采基础信息。不同网站结构差异较大，采集结果请先预览确认后再使用。',
  '用于采集没有专用采集器的网站商品页。请先创建采集规则，再开始采集。',
  '如果是 1688、速卖通、拼多多等已支持的平台，请优先使用对应的专用采集器，识别更稳定。',
  '建议先测试采集效果，确认标题、价格、图片识别正确后再提交采集任务。',
] as const;

export const CUSTOM_COLLECT_CARD_DESCRIPTION =
  '适合采集没有专用采集器的网站商品页，可采集商品标题、价格、图片、参数等基础信息。';

export const CUSTOM_COLLECT_CARD_NOTES =
  '商品规格、库存、动态价格不保证完整。使用前建议先测试采集规则。已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放。';

export const DEDICATED_COLLECT_CARD_NOTES =
  '专用采集器适合采集完整商品数据，含商品规格与库存；已针对平台页面结构专门适配，识别更稳定。';

export const COLLECT_HUB_TYPE_HINT = {
  dedicated: {
    title: '专用采集器',
    summary: '适合采集完整商品数据（含规格、库存等），已针对平台专门适配。',
  },
  custom: {
    title: '自定义采集器',
    summary: '适合采集页面基础信息（标题、价格、图片、参数）；SKU / 库存 / 动态价格不保证完整。',
  },
} as const;

export const CUSTOM_BATCH_DISABLED_TOOLTIP =
  '已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放。';
