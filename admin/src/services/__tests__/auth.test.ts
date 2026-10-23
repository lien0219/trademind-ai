import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { getProfile } from '../auth';

describe('authoritative login profile', () => {
  it('preserves an explicit empty permission list for an administrator', async () => {
    vi.mocked(request).mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'e2e-admin', role: 'admin', permissions: [] } });
    await expect(getProfile()).resolves.toEqual({ id: 'e2e-admin', role: 'admin', permissions: [] });
  });

  it.each([null, {}, { id: 'e2e-admin', role: 'admin' }])('rejects an incomplete profile without guessing permissions: %j', async (data) => {
    vi.mocked(request).mockResolvedValueOnce({ code: 0, message: 'ok', data });
    await expect(getProfile()).rejects.toThrow('权限资料暂时无法加载');
  });
});
