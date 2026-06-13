/** 运维「平台运行状态」统一入口。 */
export const PLATFORM_RUNTIME_ROUTE = '/ops/platform-runtime';

export const DEFAULT_PLATFORM_RUNTIME = 'douyin_shop';

/** 已接入完整 runtime 运维面板的平台（健康检查、指标、运行控制等）。 */
export const PLATFORM_RUNTIME_PANELS: Record<string, { sort?: number }> = {
  douyin_shop: { sort: 0 },
};

export function isPlatformRuntimeSupported(platform?: string | null): boolean {
  const key = (platform || '').trim().toLowerCase();
  return key in PLATFORM_RUNTIME_PANELS;
}

export function platformRuntimeHref(platform: string = DEFAULT_PLATFORM_RUNTIME): string {
  const key = (platform || DEFAULT_PLATFORM_RUNTIME).trim().toLowerCase();
  return `${PLATFORM_RUNTIME_ROUTE}?platform=${encodeURIComponent(key)}`;
}

/** 解析 Tab 选中项；platform 不在列表时回退到默认或首个平台。 */
export function resolvePlatformRuntimeTab(platform?: string | null, allPlatforms: string[] = []): string {
  const key = (platform || '').trim().toLowerCase();
  if (key && allPlatforms.includes(key)) {
    return key;
  }
  if (allPlatforms.includes(DEFAULT_PLATFORM_RUNTIME)) {
    return DEFAULT_PLATFORM_RUNTIME;
  }
  return allPlatforms[0] || DEFAULT_PLATFORM_RUNTIME;
}
