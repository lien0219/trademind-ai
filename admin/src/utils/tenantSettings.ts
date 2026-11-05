import { tr, trFormat } from '@/i18n';
/**
 * Tenant extensions are opaque metadata, not feature flags or a secrets vault.
 * Never include their values in a summary or a toast.
 */
export const MAX_TENANT_SETTINGS_BYTES = 64 * 1024;

// Example of generic, non-sensitive metadata only. These keys are NOT
// registered feature flags and do not change business behavior or permissions.
export const TENANT_SETTINGS_EXAMPLE = JSON.stringify({
  custom: {
    department: '跨境运营一组',
    remark: '仅用于记录企业内部说明',
  },
  labels: ['华东团队', '测试示例'],
}, null, 2);

export type TenantSettingsValidation =
  | { ok: true; value: Record<string, unknown> }
  | { ok: false; error: string };

const forbiddenFieldNames = [
  'password', 'passwd', 'secret', 'token', 'credential', 'apikey',
  'privatekey', 'authorization', 'authheader', 'cookie', 'sessionid',
];

function sensitiveKey(key: string): boolean {
  if (key === '__proto__' || key === 'constructor' || key === 'prototype') return true;
  const normalized = key.toLowerCase().replace(/[^a-z0-9]/g, '');
  return forbiddenFieldNames.some((field) => normalized.includes(field));
}

function hasSuspiciousContent(value: string): boolean {
  const trimmed = value.trim();
  return /^Bearer\s+\S+/i.test(trimmed)
    || /-----BEGIN\s+(?:[A-Z ]*\s)?PRIVATE KEY-----/i.test(value)
    || /^sk-[A-Za-z0-9_-]{16,}/.test(trimmed);
}

function containsUnsafeSettings(value: unknown, depth = 0): boolean {
  if (depth > 32) return true;
  if (Array.isArray(value)) return value.some((item) => containsUnsafeSettings(item, depth + 1));
  if (value && typeof value === 'object') {
    return Object.entries(value).some(([key, child]) =>
      sensitiveKey(key) || containsUnsafeSettings(child, depth + 1));
  }
  return typeof value === 'string' && hasSuspiciousContent(value);
}

export function inspectTenantSettings(raw: string): TenantSettingsValidation {
  const source = raw.trim() || '{}';
  if (new TextEncoder().encode(source).length > MAX_TENANT_SETTINGS_BYTES) {
    return { ok: false, error: '扩展配置超过 64 KiB，请精简后再保存' };
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(source) as unknown;
  } catch {
    return { ok: false, error: 'JSON 格式不正确，请检查引号、逗号和括号' };
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    return { ok: false, error: '扩展配置必须是 JSON 对象，例如 {}' };
  }
  if (containsUnsafeSettings(parsed)) {
    return { ok: false, error: '检测到疑似凭据或不安全字段；请移除密码、Token、密钥或敏感值' };
  }
  return { ok: true, value: parsed as Record<string, unknown> };
}

export function formatTenantSettings(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw || '{}') as unknown, null, 2);
  } catch {
    return raw;
  }
}

export function summarizeTenantSettings(raw: string): { valid: boolean; label: string } {
  try {
    const value: unknown = JSON.parse(raw || '{}') as unknown;
    if (!value || Array.isArray(value) || typeof value !== 'object') {
      return { valid: false, label: tr("数据格式异常") };
    }
    const count = Object.keys(value).length;
    return { valid: true, label: count ? trFormat('已保存 {count} 项扩展字段', { count }) : tr('未配置扩展项') };
  } catch {
    return { valid: false, label: tr("数据格式异常") };
  }
}
