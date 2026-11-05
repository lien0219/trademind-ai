import { afterEach, describe, expect, it } from 'vitest';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { createTenantReadGuard } from '../tenantAsync';
import { markTenantContextChanged, resetTenantSwitchState } from '../tenantContext';
import { canSubmitWarehouseFeeRateMutation, type WarehouseFeeRateMutationScope } from '../warehouseFeeRateMutation';

describe('warehouse fee rate-card mutation scope', () => {
  afterEach(() => {
    resetTenantSwitchState();
    localStorage.clear();
  });

  const scope = (overrides: Partial<WarehouseFeeRateMutationScope> = {}): WarehouseFeeRateMutationScope => ({
    mode: 'revise',
    rateId: 'e2e-rate-a',
    revision: 2,
    isCurrent: () => true,
    ...overrides,
  });

  it('accepts a matching rate-card revision and a fresh create action', () => {
    expect(canSubmitWarehouseFeeRateMutation(scope(), {
      canManage: true, editing: { id: 'e2e-rate-a', revision: 2 },
    })).toBe(true);
    expect(canSubmitWarehouseFeeRateMutation(scope({ mode: 'create', rateId: undefined, revision: undefined }), {
      canManage: true,
    })).toBe(true);
  });

  it('rejects switching cards, changing revision, or changing mode mid-confirmation', () => {
    for (const editing of [{ id: 'e2e-rate-b', revision: 2 }, { id: 'e2e-rate-a', revision: 3 }]) {
      expect(canSubmitWarehouseFeeRateMutation(scope(), { canManage: true, editing })).toBe(false);
    }
    expect(canSubmitWarehouseFeeRateMutation(scope(), { canManage: true })).toBe(false);
    expect(canSubmitWarehouseFeeRateMutation(scope({ mode: 'create', rateId: undefined, revision: undefined }), {
      canManage: true, editing: { id: 'e2e-rate-a', revision: 2 },
    })).toBe(false);
  });

  it('rejects revoked permission and a cancelled modal', () => {
    expect(canSubmitWarehouseFeeRateMutation(scope(), {
      canManage: false, editing: { id: 'e2e-rate-a', revision: 2 },
    })).toBe(false);
    expect(canSubmitWarehouseFeeRateMutation(scope({ isCurrent: () => false }), {
      canManage: true, editing: { id: 'e2e-rate-a', revision: 2 },
    })).toBe(false);
    expect(canSubmitWarehouseFeeRateMutation(undefined, { canManage: true })).toBe(false);
  });

  it('rejects old tenant sessions and newer superseding modal sessions', () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'e2e-tenant-a-token');
    const guard = createTenantReadGuard();
    const original = scope({ isCurrent: guard.begin() });
    markTenantContextChanged();
    expect(canSubmitWarehouseFeeRateMutation(original, {
      canManage: true, editing: { id: 'e2e-rate-a', revision: 2 },
    })).toBe(false);
    const newer = scope({ isCurrent: guard.begin() });
    guard.invalidate();
    expect(canSubmitWarehouseFeeRateMutation(newer, {
      canManage: true, editing: { id: 'e2e-rate-a', revision: 2 },
    })).toBe(false);
  });
});
