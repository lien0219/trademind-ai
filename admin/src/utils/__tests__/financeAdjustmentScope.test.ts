import { afterEach, describe, expect, it } from 'vitest';
import { canSubmitFinanceAdjustment, type FinanceAdjustmentScope } from '../financeAdjustmentScope';
import { createTenantReadGuard } from '../tenantAsync';
import { markTenantContextChanged, resetTenantSwitchState } from '../tenantContext';

describe('finance adjustment target isolation', () => {
  afterEach(() => resetTenantSwitchState());

  const scope = (overrides: Partial<FinanceAdjustmentScope> = {}): FinanceAdjustmentScope => ({
    recordId: 'allocation-a',
    action: 'adjust',
    isCurrent: () => true,
    ...overrides,
  });
  const input = (overrides: Partial<Parameters<typeof canSubmitFinanceAdjustment>[1]> = {}) => ({
    recordId: 'allocation-a',
    action: 'adjust' as const,
    canManage: true,
    ...overrides,
  });

  it('accepts an unchanged adjustment for the selected ledger entry', () => {
    expect(canSubmitFinanceAdjustment(scope(), input())).toBe(true);
  });

  it('rejects a closed, replaced or switched ledger detail', () => {
    expect(canSubmitFinanceAdjustment(undefined, input())).toBe(false);
    expect(canSubmitFinanceAdjustment(scope(), input({ recordId: 'allocation-b' }))).toBe(false);
    expect(canSubmitFinanceAdjustment(scope({ isCurrent: () => false }), input())).toBe(false);
  });

  it('rejects changed action or changed reversal target', () => {
    const reversal = scope({ action: 'reverse', targetId: 'adjustment-a' });
    expect(canSubmitFinanceAdjustment(reversal, input({ action: 'reverse', targetId: 'adjustment-a' }))).toBe(true);
    expect(canSubmitFinanceAdjustment(reversal, input({ action: 'reverse', targetId: 'adjustment-b' }))).toBe(false);
    expect(canSubmitFinanceAdjustment(reversal, input({ action: 'reverse' }))).toBe(false);
    expect(canSubmitFinanceAdjustment(reversal, input())).toBe(false);
    expect(canSubmitFinanceAdjustment(scope(), input({ action: 'reverse', targetId: 'adjustment-a' }))).toBe(false);
  });

  it('blocks revoked manage permissions and malformed reversal scope', () => {
    expect(canSubmitFinanceAdjustment(scope(), input({ canManage: false }))).toBe(false);
    expect(canSubmitFinanceAdjustment(scope({ action: 'reverse' }), input({ action: 'reverse' }))).toBe(false);
  });

  it('rejects the original scope after tenant switch or explicit cancellation', () => {
    const guard = createTenantReadGuard();
    const original = scope({ isCurrent: guard.begin() });
    expect(canSubmitFinanceAdjustment(original, input())).toBe(true);
    markTenantContextChanged();
    expect(canSubmitFinanceAdjustment(original, input())).toBe(false);

    const newer = scope({ isCurrent: guard.begin() });
    guard.invalidate();
    expect(canSubmitFinanceAdjustment(newer, input())).toBe(false);
  });
});
