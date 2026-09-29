import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as request from '@/services/request';
import {
  confirmFreightFeeImport,
  createFreightFeeAdjustment,
  freightFeeIdempotencyKey,
  getFreightFee,
  previewFreightFeeImport,
  queryFreightFees,
  reverseFreightFeeAdjustment,
} from '@/services/freightFees';

vi.mock('@/services/request', () => ({
  getJSON: vi.fn(),
  getWithParams: vi.fn(),
  postFormData: vi.fn(),
  postJSON: vi.fn(),
}));

describe('freight fee service contracts', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('keeps preview separate from multipart import confirmation', async () => {
    vi.mocked(request.postFormData).mockResolvedValue({} as never);
    const file = new File(['csv'], 'freight.csv', { type: 'text/csv' });

    await previewFreightFeeImport(file, 'shop-1');
    expect(request.postFormData).toHaveBeenNthCalledWith(
      1,
      '/api/v1/freight-fee-imports/preview',
      expect.any(FormData),
    );
    const previewForm = vi.mocked(request.postFormData).mock.calls[0][1] as FormData;
    expect(previewForm.get('file')).toBe(file);
    expect(previewForm.get('shopId')).toBe('shop-1');

    await confirmFreightFeeImport(file, 'shop-1', 'a'.repeat(64), 'b'.repeat(64), 'freight-confirm-key');
    const confirmForm = vi.mocked(request.postFormData).mock.calls[1][1] as FormData;
    expect(request.postFormData).toHaveBeenNthCalledWith(
      2,
      '/api/v1/freight-fee-imports',
      expect.any(FormData),
    );
    expect(confirmForm.get('expectedFileHash')).toBe('a'.repeat(64));
    expect(confirmForm.get('expectedCalculationHash')).toBe('b'.repeat(64));
    expect(confirmForm.get('idempotencyKey')).toBe('freight-confirm-key');
  });

  it('preserves list, detail, adjustment, and reversal API contracts', async () => {
    vi.mocked(request.getWithParams).mockResolvedValue({ list: [] } as never);
    vi.mocked(request.getJSON).mockResolvedValue({} as never);
    vi.mocked(request.postJSON).mockResolvedValue({} as never);

    await queryFreightFees({ page: 2, pageSize: 50, orderNo: 'SO-1', shopId: 'shop-1', status: 'mismatch' });
    expect(request.getWithParams).toHaveBeenCalledWith('/api/v1/freight-fees', {
      page: 2, pageSize: 50, orderNo: 'SO-1', shopId: 'shop-1', status: 'mismatch',
    });
    await getFreightFee('charge/1');
    expect(request.getJSON).toHaveBeenCalledWith('/api/v1/freight-fees/charge%2F1');

    const adjustment = { amountMinor: -25, reason: 'E2E correction', idempotencyKey: 'adjust-key' };
    await createFreightFeeAdjustment('charge-1', adjustment);
    expect(request.postJSON).toHaveBeenNthCalledWith(1, '/api/v1/freight-fees/charge-1/adjustments', adjustment);
    const reversal = { reason: 'E2E reversal', idempotencyKey: 'reverse-key' };
    await reverseFreightFeeAdjustment('charge-1', 'adjustment-1', reversal);
    expect(request.postJSON).toHaveBeenNthCalledWith(
      2,
      '/api/v1/freight-fees/charge-1/adjustments/adjustment-1/reverse',
      reversal,
    );
  });

  it('creates bounded idempotency keys for user actions', () => {
    const key = freightFeeIdempotencyKey('confirm');
    expect(key).toMatch(/^admin-freight-fee-confirm-/);
    expect(key.length).toBeLessThanOrEqual(128);
  });
});
