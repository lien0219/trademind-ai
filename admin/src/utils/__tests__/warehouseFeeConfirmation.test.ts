import { afterEach, describe, expect, it } from 'vitest';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { createTenantReadGuard } from '../tenantAsync';
import { markTenantContextChanged, resetTenantSwitchState } from '../tenantContext';
import {
  canConfirmWarehouseFeePreview,
  createWarehouseFeeConfirmationScope,
} from '../warehouseFeeConfirmation';

describe('warehouse fee preview confirmation', () => {
  const preview = () => ({
    orderId: 'e2e-order-1',
    rateCardId: 'e2e-rate-card-1',
    rateCardRevision: 2,
    waveRevision: 4,
    calculationHash: 'sha256-e2e-a',
  });

  afterEach(() => {
    resetTenantSwitchState();
    localStorage.clear();
  });

  it('allows confirming the exact preview in the active tenant', () => {
    const current = preview();
    const scope = createWarehouseFeeConfirmationScope(current, () => true);
    expect(canConfirmWarehouseFeePreview(scope, current)).toBe(true);
  });

  it('rejects replacement and discarded previews even when their values match', () => {
    const current = preview();
    const scope = createWarehouseFeeConfirmationScope(current, () => true);
    expect(canConfirmWarehouseFeePreview(scope, { ...current })).toBe(false);
    expect(canConfirmWarehouseFeePreview(scope, undefined)).toBe(false);
    expect(canConfirmWarehouseFeePreview(undefined, current)).toBe(false);
  });

  it('rejects an edited order, rate, revision or calculation hash', () => {
    const current = preview();
    const scope = createWarehouseFeeConfirmationScope(current, () => true);
    for (const [key, value] of [
      ['orderId', 'e2e-order-2'],
      ['rateCardId', 'e2e-rate-card-2'],
      ['rateCardRevision', 3],
      ['waveRevision', 5],
      ['calculationHash', 'sha256-e2e-b'],
    ] as const) {
      const changed = { ...current, [key]: value };
      expect(canConfirmWarehouseFeePreview(scope, changed)).toBe(false);
    }
  });

  it('rejects old tenant and logged-out session previews', () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'e2e-tenant-a-token');
    const guard = createTenantReadGuard();
    const current = preview();
    const scope = createWarehouseFeeConfirmationScope(current, guard.begin());
    markTenantContextChanged();
    expect(canConfirmWarehouseFeePreview(scope, current)).toBe(false);

    const next = createWarehouseFeeConfirmationScope(current, guard.begin());
    localStorage.removeItem(AUTH_TOKEN_KEY);
    expect(canConfirmWarehouseFeePreview(next, current)).toBe(false);
  });

  it('rejects a preview canceled or superseded by another request', () => {
    const guard = createTenantReadGuard();
    const current = preview();
    const scope = createWarehouseFeeConfirmationScope(current, guard.begin());
    guard.begin();
    expect(canConfirmWarehouseFeePreview(scope, current)).toBe(false);
    const latest = createWarehouseFeeConfirmationScope(current, guard.begin());
    guard.invalidate();
    expect(canConfirmWarehouseFeePreview(latest, current)).toBe(false);
  });
});
