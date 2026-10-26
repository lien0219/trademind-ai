import { describe, expect, it } from 'vitest';
import {
  MAX_TENANT_SETTINGS_BYTES,
  formatTenantSettings,
  inspectTenantSettings,
  summarizeTenantSettings,
} from '../tenantSettings';

describe('tenant advanced settings', () => {
  it('accepts only bounded JSON objects and preserves legitimate extensions', () => {
    expect(inspectTenantSettings('{}')).toEqual({ ok: true, value: {} });
    expect(inspectTenantSettings('')).toEqual({ ok: true, value: {} });
    expect(inspectTenantSettings('{"locale":"zh-CN","features":{"view":"compact"}}')).toEqual({
      ok: true, value: { locale: 'zh-CN', features: { view: 'compact' } },
    });
    expect(inspectTenantSettings('[]').ok).toBe(false);
    expect(inspectTenantSettings('null').ok).toBe(false);
    expect(inspectTenantSettings('{ broken').ok).toBe(false);
  });

  it('rejects credential-like keys and values even when nested', () => {
    for (const raw of [
      '{"apiKey":"x"}',
      '{"nested":[{"client_secret":"x"}]}',
      '{"passWord":"x"}',
      '{"cookie":"x"}',
      '{"x":{"privateKey":"x"}}',
      '{"__proto__":{"isAdmin":true}}',
      '{"x":"Bearer abc123"}',
      '{"x":"-----BEGIN PRIVATE KEY-----"}',
    ]) {
      expect(inspectTenantSettings(raw).ok).toBe(false);
    }
  });

  it('applies a UTF-8 byte limit rather than a JS character limit', () => {
    expect(inspectTenantSettings(JSON.stringify({ title: '中'.repeat(MAX_TENANT_SETTINGS_BYTES / 3) })).ok).toBe(false);
  });

  it('summarizes only key counts, never the stored values', () => {
    expect(summarizeTenantSettings('{"first":"personal","second":true}')).toEqual({
      valid: true, label: '已保存 2 项扩展字段',
    });
    expect(summarizeTenantSettings('{}').label).toBe('未配置扩展项');
    expect(summarizeTenantSettings('oops').valid).toBe(false);
    expect(formatTenantSettings('{"locale":"zh-CN"}')).toBe('{\n  "locale": "zh-CN"\n}');
    expect(formatTenantSettings('broken')).toBe('broken');
  });
});
