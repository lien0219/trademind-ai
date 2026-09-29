import { describe, expect, it } from 'vitest';
import { createProcurementIdempotencyKey, procurementErrorMessage } from './procurement';

describe('procurement service helpers', () => {
  it('creates bounded action-scoped idempotency keys', () => {
    const first = createProcurementIdempotencyKey('goods-receipt');
    const second = createProcurementIdempotencyKey('goods-receipt');

    expect(first).toMatch(/^admin-goods-receipt-/);
    expect(first.length).toBeLessThanOrEqual(128);
    expect(second).not.toBe(first);
  });

  it('maps backend business codes and concurrency messages to actionable copy', () => {
    expect(procurementErrorMessage({ code: 40301, message: 'supplier permission denied' })).toBe(
      '当前账号没有执行此操作的权限。',
    );
    expect(procurementErrorMessage({ code: 40901, message: 'purchase order revision conflict' })).toContain(
      '已被其他人更新',
    );
    expect(procurementErrorMessage({ code: 40901, message: 'purchase return revision conflict' })).toContain(
      '采购退货单',
    );
    expect(procurementErrorMessage({ code: 40901, message: 'purchase return approver cannot complete the return' })).toBe(
      '审批人与退货执行人必须为不同账号。',
    );
  });
});
