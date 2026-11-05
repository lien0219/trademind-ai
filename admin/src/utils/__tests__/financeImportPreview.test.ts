import { describe, expect, it } from 'vitest';
import { canConfirmFinanceImport, type FinanceImportPreviewScope } from '../financeImportPreview';
import { createTenantReadGuard } from '../tenantAsync';
import { markTenantContextChanged, resetTenantSwitchState } from '../tenantContext';

describe('finance import preview confirmation', () => {
  const file = new File(['a,b'], 'statement.csv', { type: 'text/csv' });

  function scope(overrides: Partial<FinanceImportPreviewScope> = {}): FinanceImportPreviewScope {
    return {
      file,
      shopId: 'shop-a',
      fileHash: 'hash-a',
      calculationHash: 'calculation-a',
      idempotencyKey: 'key-a',
      isCurrent: () => true,
      ...overrides,
    };
  }

  const input = (preview: FinanceImportPreviewScope) => ({
    file: preview.file,
    shopId: preview.shopId,
    fileHash: preview.fileHash,
    calculationHash: preview.calculationHash,
    idempotencyKey: preview.idempotencyKey,
  });

  it('accepts the same validated preview and retry key', () => {
    const current = scope();
    expect(canConfirmFinanceImport(current, current, input(current))).toBe(true);
  });

  it('rejects a delayed confirmation for an older preview after replacement', () => {
    const previous = scope();
    const replacement = scope({ idempotencyKey: 'key-b' });
    expect(canConfirmFinanceImport(replacement, previous, input(previous))).toBe(false);
  });

  it('rejects modified file, shop, hash, calculation or key', () => {
    const current = scope();
    const changes = [
      { file: new File(['c,d'], 'other.csv', { type: 'text/csv' }) },
      { shopId: 'shop-b' },
      { fileHash: 'hash-b' },
      { calculationHash: 'calculation-b' },
      { idempotencyKey: 'key-b' },
    ];
    for (const change of changes) {
      expect(canConfirmFinanceImport(current, current, { ...input(current), ...change })).toBe(false);
    }
  });

  it('rejects an invalidated preview on tenant switch or cancellation', () => {
    const guard = createTenantReadGuard();
    const current = scope({ isCurrent: guard.begin() });
    expect(canConfirmFinanceImport(current, current, input(current))).toBe(true);
    guard.invalidate();
    expect(canConfirmFinanceImport(current, current, input(current))).toBe(false);

    const next = scope({ isCurrent: guard.begin() });
    markTenantContextChanged();
    expect(canConfirmFinanceImport(next, next, input(next))).toBe(false);
    resetTenantSwitchState();
  });

  it('never accepts a missing confirmation scope', () => {
    const current = scope();
    expect(canConfirmFinanceImport(current, undefined, input(current))).toBe(false);
    expect(canConfirmFinanceImport(undefined, current, input(current))).toBe(false);
  });
});
