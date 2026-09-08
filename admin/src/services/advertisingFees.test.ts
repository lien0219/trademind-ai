import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as request from '@/services/request';
import {
  advertisingFeeIdempotencyKey,
  confirmAdvertisingFeeImport,
  createAdvertisingFeeAdjustment,
  previewAdvertisingFeeImport,
  queryAdvertisingFees,
  reverseAdvertisingFeeAdjustment,
} from './advertisingFees';

vi.mock('@/services/request', () => ({
  getJSON: vi.fn(),
  getWithParams: vi.fn(),
  postFormData: vi.fn(),
  postJSON: vi.fn(),
}));

describe('advertising fee service contracts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps preview separate from explicit confirmation hashes', async () => {
    vi.mocked(request.postFormData).mockResolvedValue({} as never);
    const file = new File(['csv'], 'advertising.csv', { type: 'text/csv' });
    await previewAdvertisingFeeImport(file, 'shop-1');
    await confirmAdvertisingFeeImport(file, 'shop-1', 'a'.repeat(64), 'b'.repeat(64), 'advertising-confirm-1');
    expect(request.postFormData).toHaveBeenNthCalledWith(1, '/api/v1/advertising-fee-imports/preview', expect.any(FormData));
    expect(request.postFormData).toHaveBeenNthCalledWith(2, '/api/v1/advertising-fee-imports', expect.any(FormData));
    const confirmation = vi.mocked(request.postFormData).mock.calls[1][1] as FormData;
    expect(confirmation.get('expectedFileHash')).toBe('a'.repeat(64));
    expect(confirmation.get('expectedCalculationHash')).toBe('b'.repeat(64));
    expect(confirmation.get('idempotencyKey')).toBe('advertising-confirm-1');
  });

  it('uses scoped reads and append-only correction endpoints', async () => {
    vi.mocked(request.getWithParams).mockResolvedValue({ list: [] } as never);
    vi.mocked(request.postJSON).mockResolvedValue({} as never);
    await queryAdvertisingFees({ orderNo: 'AD-1', shopId: 'shop-1', settlementCoverage: 'excluded' });
    await createAdvertisingFeeAdjustment('allocation/1', { amountMinor: 25, reason: '补录', idempotencyKey: 'adjust-key' });
    await reverseAdvertisingFeeAdjustment('allocation/1', 'adjustment/1', { reason: '冲正', idempotencyKey: 'reverse-key' });
    expect(request.getWithParams).toHaveBeenCalledWith('/api/v1/advertising-fees', expect.objectContaining({ shopId: 'shop-1' }));
    expect(request.postJSON).toHaveBeenNthCalledWith(1, '/api/v1/advertising-fees/allocation%2F1/adjustments', expect.any(Object));
    expect(request.postJSON).toHaveBeenNthCalledWith(2, '/api/v1/advertising-fees/allocation%2F1/adjustments/adjustment%2F1/reverse', expect.any(Object));
    const key = advertisingFeeIdempotencyKey('confirm');
    expect(key).toMatch(/^admin-advertising-fee-confirm-/);
    expect(key.length).toBeLessThanOrEqual(128);
  });
});
