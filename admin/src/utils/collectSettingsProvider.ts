import type { CollectProviderRow, CollectProviderStatus } from '@/services/collectProviders';

/** URL query `provider` values for /settings/collector */
export type CollectSettingsProviderKey =
  | '1688'
  | 'aliexpress'
  | 'pinduoduo'
  | 'taobao_tmall'
  | 'shein_temu'
  | 'custom';

export type CollectSettingsProviderOption = {
  key: CollectSettingsProviderKey;
  label: string;
  /** Collect hub / task `source` id */
  source: string;
  planned?: boolean;
};

export const COLLECT_SETTINGS_PROVIDER_OPTIONS: CollectSettingsProviderOption[] = [
  { key: '1688', label: '1688 采集器', source: '1688' },
  { key: 'aliexpress', label: '速卖通采集器', source: 'aliexpress' },
  { key: 'pinduoduo', label: '拼多多采集器', source: 'pinduoduo' },
  { key: 'taobao_tmall', label: '淘宝/天猫采集器', source: 'taobao_tmall' },
  { key: 'shein_temu', label: 'SHEIN/Temu 采集器', source: 'shein_temu', planned: true },
  { key: 'custom', label: '自定义链接采集器', source: 'custom' },
];

const PROVIDER_KEY_SET = new Set<string>(COLLECT_SETTINGS_PROVIDER_OPTIONS.map((o) => o.key));

/** Map collect hub card `source` → settings URL `provider`. */
export function collectSourceToSettingsProvider(source: string): CollectSettingsProviderKey {
  const s = source.trim().toLowerCase();
  switch (s) {
    case '1688':
      return '1688';
    case 'aliexpress':
      return 'aliexpress';
    case 'pdd':
    case 'pinduoduo':
      return 'pinduoduo';
    case 'taobao_tmall':
    case 'taobao':
      return 'taobao_tmall';
    case 'shein_temu':
      return 'shein_temu';
    case 'custom':
      return 'custom';
    default:
      return '1688';
  }
}

export function resolveCollectSettingsProvider(raw: string | null | undefined): CollectSettingsProviderKey {
  const key = (raw ?? '').trim();
  if (PROVIDER_KEY_SET.has(key)) {
    return key as CollectSettingsProviderKey;
  }
  return '1688';
}

export function collectSettingsPath(source: string): string {
  const provider = collectSourceToSettingsProvider(source);
  return `/settings/collector?provider=${encodeURIComponent(provider)}`;
}

export function findCollectSettingsOption(key: CollectSettingsProviderKey): CollectSettingsProviderOption {
  return COLLECT_SETTINGS_PROVIDER_OPTIONS.find((o) => o.key === key) ?? COLLECT_SETTINGS_PROVIDER_OPTIONS[0];
}

export function providerStatusFromRows(
  rows: CollectProviderRow[],
  source: string,
): CollectProviderStatus | undefined {
  const key = source.trim().toLowerCase();
  return rows.find((r) => r.source.trim().toLowerCase() === key)?.status;
}

export function isPlannedCollectProvider(
  rows: CollectProviderRow[],
  option: CollectSettingsProviderOption,
): boolean {
  const status = providerStatusFromRows(rows, option.source);
  if (status) return status === 'planned';
  return !!option.planned;
}

export function collectSettingsConfigButtonLabel(status?: CollectProviderStatus): string {
  return status === 'planned' ? '查看配置' : '采集设置';
}
