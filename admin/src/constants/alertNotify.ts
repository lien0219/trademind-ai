import { ALERT_SEVERITY_OPTIONS } from '@/constants/systemSettings';

export { ALERT_SEVERITY_OPTIONS as NOTIFICATION_SEVERITY_OPTIONS };

/** 通知通道（存库 JSON 数组元素） */
export const NOTIFICATION_CHANNEL_META: Record<
  string,
  { label: string; desc: string; planned?: boolean }
> = {
  mail: { label: '邮件', desc: '经邮件服务器发送；发信配置请在「邮箱设置」中完成' },
  webhook: { label: '回调通知', desc: '推送到自定义 HTTPS 地址' },
  feishu: { label: '飞书', desc: '后续版本支持，当前保存后发送结果为 skipped', planned: true },
  wecom: { label: '企业微信', desc: '后续版本支持，当前保存后发送结果为 skipped', planned: true },
};

export const NOTIFICATION_CHANNEL_OPTIONS = Object.entries(NOTIFICATION_CHANNEL_META).map(
  ([value, meta]) => ({
    value,
    label: meta.planned ? `${meta.label}（预留）` : meta.label,
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
