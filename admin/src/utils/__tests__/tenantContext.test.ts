import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  captureTenantContext,
  isTenantContextCurrent,
  isTenantSwitchRequest,
  isTenantSwitching,
  markTenantContextChanged,
  resetTenantSwitchState,
  switchTenantAndReload,
} from '../tenantContext';

describe('tenant context switching', () => {
  afterEach(() => {
    resetTenantSwitchState();
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('fences snapshots when the tenant context changes', () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'token-a');
    const snapshot = captureTenantContext();
    expect(isTenantContextCurrent(snapshot)).toBe(true);

    markTenantContextChanged();

    expect(isTenantContextCurrent(snapshot)).toBe(false);
  });

  it('allows only the exact tenant switch endpoint during a switch', () => {
    expect(isTenantSwitchRequest('/api/v1/auth/tenants/switch')).toBe(true);
    expect(isTenantSwitchRequest('/api/v1/auth/tenants/switch?tenantId=7')).toBe(true);
    expect(isTenantSwitchRequest('/api/v1/auth/tenants/switch/other')).toBe(false);
  });

  it('uses the caller-provided switch request and clears switching state on failure', async () => {
    const switchRequest = vi.fn().mockRejectedValue(new Error('switch failed'));

    await expect(switchTenantAndReload(7, switchRequest)).rejects.toThrow('switch failed');

    expect(switchRequest).toHaveBeenCalledWith(7);
    expect(isTenantSwitching()).toBe(false);
  });
});
