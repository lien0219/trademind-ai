import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { afterEach, describe, expect, it } from 'vitest';
import { markTenantContextChanged, resetTenantSwitchState } from '../tenantContext';
import { createTenantReadGuard } from '../tenantAsync';

describe('tenant-scoped finance read guard', () => {
  afterEach(() => {
    resetTenantSwitchState();
    localStorage.clear();
  });

  it('accepts only the most recent request for one tenant', () => {
    const guard = createTenantReadGuard();
    const older = guard.begin();
    expect(older()).toBe(true);
    const latest = guard.begin();
    expect(older()).toBe(false);
    expect(latest()).toBe(true);
  });

  it('rejects responses when switching tenant even if the auth token is unchanged', () => {
    const guard = createTenantReadGuard();
    localStorage.setItem(AUTH_TOKEN_KEY, 'tenant-a-session');
    const isCurrent = guard.begin();
    markTenantContextChanged();
    expect(isCurrent()).toBe(false);
  });

  it('rejects responses after a session token rotates', () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'session-a');
    const isCurrent = createTenantReadGuard().begin();
    localStorage.setItem(AUTH_TOKEN_KEY, 'session-b');
    expect(isCurrent()).toBe(false);
  });

  it('rejects requests invalidated by drawer close or unmount', () => {
    const guard = createTenantReadGuard();
    const isCurrent = guard.begin();
    guard.invalidate();
    expect(isCurrent()).toBe(false);
  });
});
