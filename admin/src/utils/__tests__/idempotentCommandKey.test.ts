import { describe, expect, it, vi } from 'vitest';
import { idempotentCommandKey } from '../idempotentCommandKey';

describe('procurement command idempotency', () => {
  const purchase = () => ({
    supplierId: 'e2e-supplier-a',
    warehouseId: 'e2e-warehouse-a',
    currency: 'CNY',
    remark: '',
    items: [{ productSkuId: 'e2e-sku-a', quantity: 2, unitCostMinor: 2599 }],
  });

  it('uses the key created when the purchase modal opens', () => {
    const generate = vi.fn(() => 'admin-purchase-order-generated');
    const key = idempotentCommandKey(undefined, 'purchase-order', purchase(), generate, 'admin-purchase-order-initial');
    expect(key.key).toBe('admin-purchase-order-initial');
    expect(generate).not.toHaveBeenCalled();
  });

  it('reuses the original key for an identical network-error retry', () => {
    const generate = vi.fn(() => 'admin-purchase-order-generated');
    const first = idempotentCommandKey(undefined, 'purchase-order', purchase(), generate, 'admin-purchase-order-initial');
    const retried = idempotentCommandKey(first, 'purchase-order', purchase(), generate, 'ignored');
    expect(retried).toBe(first);
    expect(generate).not.toHaveBeenCalled();
  });

  it('rotates keys when a purchase order supplier, price, quantity or remarks change', () => {
    let seq = 0;
    const generate = () => `admin-purchase-order-${++seq}`;
    const first = idempotentCommandKey(undefined, 'purchase-order', purchase(), generate, 'initial');
    const changedSupplier = idempotentCommandKey(first, 'purchase-order', { ...purchase(), supplierId: 'e2e-supplier-b' }, generate);
    const changedPrice = idempotentCommandKey(changedSupplier, 'purchase-order', { ...purchase(), items: [{ ...purchase().items[0], unitCostMinor: 2600 }] }, generate);
    const changedQuantity = idempotentCommandKey(changedPrice, 'purchase-order', { ...purchase(), items: [{ ...purchase().items[0], quantity: 3 }] }, generate);
    const changedRemark = idempotentCommandKey(changedQuantity, 'purchase-order', { ...purchase(), remark: 'e2e-note' }, generate);
    expect([first.key, changedSupplier.key, changedPrice.key, changedQuantity.key, changedRemark.key]).toEqual([
      'initial', 'admin-purchase-order-1', 'admin-purchase-order-2',
      'admin-purchase-order-3', 'admin-purchase-order-4',
    ]);
  });

  it('distinguishes changed replenishment supplier, warehouse, SKU and suggestion hash', () => {
    let seq = 0;
    const generate = () => `admin-replenishment-draft-${++seq}`;
    const body = { warehouseId: 'e2e-wh-a', supplierId: 'e2e-supplier-a', remark: '',
      items: [{ productSkuId: 'e2e-sku-a', supplierSkuId: 'e2e-binding-a', quantity: 4, suggestionHash: 'e2e-hash-a' }] };
    const first = idempotentCommandKey(undefined, 'replenishment-draft', body, generate, 'initial');
    const second = idempotentCommandKey(first, 'replenishment-draft', { ...body, warehouseId: 'e2e-wh-b' }, generate);
    const third = idempotentCommandKey(second, 'replenishment-draft', { ...body, supplierId: 'e2e-supplier-b' }, generate);
    const fourth = idempotentCommandKey(third, 'replenishment-draft', { ...body,
      items: [{ ...body.items[0], supplierSkuId: 'e2e-binding-b', suggestionHash: 'e2e-hash-b' }] }, generate);
    expect(new Set([first.key, second.key, third.key, fourth.key]).size).toBe(4);
  });

  it('does not reuse the key for another action even with an identical payload', () => {
    const generate = vi.fn((action: string) => `admin-${action}-generated`);
    const first = idempotentCommandKey(undefined, 'purchase-order', purchase(), generate, 'initial');
    const second = idempotentCommandKey(first, 'replenishment-draft', purchase(), generate);
    expect(second.key).not.toBe(first.key);
  });
});
