import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as request from '@/services/request';
import {
  confirmWarehouseFee,
  createWarehouseFeeAdjustment,
  listWarehouseFeeRateCards,
  previewWarehouseFee,
  reverseWarehouseFeeAdjustment,
  warehouseFeeIdempotencyKey,
} from './warehouseFees';

vi.mock('@/services/request', () => ({
  getJSON: vi.fn(),
  getWithParams: vi.fn(),
  postJSON: vi.fn(),
  putJSON: vi.fn(),
}));

describe('warehouse fee service contracts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps preview separate from explicit confirmation', async () => {
    vi.mocked(request.postJSON).mockResolvedValue({} as never);
    await previewWarehouseFee({ orderId: 'order-1', rateCardId: 'rate-1', rateCardRevision: 2 });
    await confirmWarehouseFee({
      orderId: 'order-1',
      rateCardId: 'rate-1',
      rateCardRevision: 2,
      waveRevision: 9,
      calculationHash: 'a'.repeat(64),
      idempotencyKey: 'warehouse-confirm-1',
    });
    expect(request.postJSON).toHaveBeenNthCalledWith(1, '/api/v1/warehouse-operation-fees/preview', {
      orderId: 'order-1',
      rateCardId: 'rate-1',
      rateCardRevision: 2,
    });
    expect(request.postJSON).toHaveBeenNthCalledWith(2, '/api/v1/warehouse-operation-fees', {
      orderId: 'order-1',
      rateCardId: 'rate-1',
      rateCardRevision: 2,
      waveRevision: 9,
      calculationHash: 'a'.repeat(64),
      idempotencyKey: 'warehouse-confirm-1',
    });
  });

  it('uses append-only adjustment and reversal endpoints', async () => {
    vi.mocked(request.postJSON).mockResolvedValue({} as never);
    await createWarehouseFeeAdjustment('snapshot-1', {
      amountMinor: 25,
      reason: '补收',
      idempotencyKey: 'adjustment-key',
    });
    await reverseWarehouseFeeAdjustment('snapshot-1', 'adjustment-1', {
      reason: '冲正',
      idempotencyKey: 'reversal-key',
    });
    expect(request.postJSON).toHaveBeenNthCalledWith(
      1,
      '/api/v1/warehouse-operation-fees/snapshot-1/adjustments',
      expect.any(Object),
    );
    expect(request.postJSON).toHaveBeenNthCalledWith(
      2,
      '/api/v1/warehouse-operation-fees/snapshot-1/adjustments/adjustment-1/reverse',
      expect.any(Object),
    );
  });

  it('normalizes rate card query flags and generates bounded idempotency keys', async () => {
    vi.mocked(request.getWithParams).mockResolvedValue({ list: [] } as never);
    await listWarehouseFeeRateCards({ warehouseId: 'warehouse-1', includeInactive: true });
    expect(request.getWithParams).toHaveBeenCalledWith('/api/v1/warehouse-fee-rate-cards', {
      warehouseId: 'warehouse-1',
      includeInactive: 'true',
    });
    const key = warehouseFeeIdempotencyKey('confirm');
    expect(key).toMatch(/^admin-warehouse-fee-confirm-/);
    expect(key.length).toBeLessThanOrEqual(128);
  });
});
