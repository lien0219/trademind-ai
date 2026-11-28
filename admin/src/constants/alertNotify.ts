import { tr } from '@/i18n';
import { ALERT_SEVERITY_OPTIONS } from '@/constants/systemSettings';

export { ALERT_SEVERITY_OPTIONS as NOTIFICATION_SEVERITY_OPTIONS };

/** 通知通道（存库 JSON 数组元素） */
export const NOTIFICATION_CHANNEL_META: Record<
  string,
  { label: string }
> = {
  mail: { get label() { return tr("邮件"); } },
  webhook: { get label() { return tr("回调通知"); } },
  feishu: { get label() { return tr("飞书"); } },
  wecom: { get label() { return tr("企业微信"); } },
};

export const NOTIFICATION_CHANNEL_OPTIONS = Object.entries(NOTIFICATION_CHANNEL_META).map(
  ([value, meta]) => ({
    value,
    label: meta.label,
  }),
);

export const WEBHOOK_METHOD_OPTIONS = [
  { label: 'POST', value: 'POST' },
  { label: 'PUT', value: 'PUT' },
];

const VALID_CHANNELS = new Set(Object.keys(NOTIFICATION_CHANNEL_META));

export function parseNotificationChannels(raw: string | undefined): string[] {
  const s = String(raw ?? '').trim();
  if (!s) return [];
  try {
    const arr = JSON.parse(s) as unknown;
    if (!Array.isArray(arr)) return [];
    return arr
      .map((x) => String(x).trim().toLowerCase())
      .filter((x) => VALID_CHANNELS.has(x));
  } catch {
    return [];
  }
}

export function stringifyNotificationChannels(channels: string[] | undefined): string {
  const list = (channels ?? [])
    .map((x) => String(x).trim().toLowerCase())
    .filter((x) => VALID_CHANNELS.has(x));
  return JSON.stringify(list);
}

export function notificationChannelLabel(channel: string): string {
  const k = channel.trim().toLowerCase();
  return NOTIFICATION_CHANNEL_META[k]?.label || channel;
}
