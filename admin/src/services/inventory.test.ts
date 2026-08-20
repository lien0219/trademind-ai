import { describe, expect, it } from 'vitest';
import { createInventoryIdempotencyKey } from './inventory';

describe('inventory service helpers', () => {
  it('creates bounded action-scoped idempotency keys', () => {
    const first = createInventoryIdempotencyKey('manual-adjust');
    const second = createInventoryIdempotencyKey('manual-adjust');

    expect(first).toMatch(/^admin-manual-adjust-/);
    expect(first.length).toBeLessThanOrEqual(128);
    expect(second).not.toBe(first);
  });
});
