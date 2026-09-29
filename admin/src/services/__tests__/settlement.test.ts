import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  confirmSettlementImport,
  downloadSettlementReconciliation,
  getSettlementReconciliation,
  previewSettlementImport,
  querySettlementReconciliation,
} from '../settlement';

const requestMock = vi.mocked(request);

describe('settlement API service', () => {
  it('sends preview and confirmation as multipart with stable fields', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { valid: true } });
    const file = new File(['header\n'], 'bill.csv', { type: 'text/csv' });

    await previewSettlementImport(file, 'shop-1');
    const previewCall = requestMock.mock.calls.at(-1);
    expect(previewCall?.[0]).toBe('/api/v1/settlement-imports/preview');
    expect(previewCall?.[1]?.method).toBe('POST');
    const previewForm = previewCall?.[1]?.data as FormData;
    expect(previewForm.get('file')).toBe(file);
    expect(previewForm.get('shopId')).toBe('shop-1');

    await confirmSettlementImport(file, 'shop-1', 'abc123', 'settlement-import-fixed');
    const confirmCall = requestMock.mock.calls.at(-1);
    expect(confirmCall?.[0]).toBe('/api/v1/settlement-imports');
    const confirmForm = confirmCall?.[1]?.data as FormData;
    expect(confirmForm.get('expectedFileHash')).toBe('abc123');
    expect(confirmForm.get('idempotencyKey')).toBe('settlement-import-fixed');
  });

  it('keeps list filters and encodes detail ids', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { list: [] } });
    await querySettlementReconciliation({
      page: 2,
      pageSize: 50,
      orderNo: 'SO-1',
      shopId: 'shop-1',
      status: 'mismatch',
      currency: 'CNY',
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/settlement-reconciliation', {
      method: 'GET',
      params: {
        page: 2,
        pageSize: 50,
        orderNo: 'SO-1',
        shopId: 'shop-1',
        status: 'mismatch',
        currency: 'CNY',
      },
    });

    await getSettlementReconciliation('group/1');
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/settlement-reconciliation/group%2F1',
      { method: 'GET' },
    );
  });

  it('downloads bounded reconciliation CSV through the authenticated client', async () => {
    requestMock.mockResolvedValue(new Blob(['订单号']) as never);
    const createObjectURL = vi.fn(() => 'blob:settlement');
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);

    await downloadSettlementReconciliation({ orderNo: ' SO-1 ', currency: 'cny', status: 'matched' });

    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/settlement-reconciliation?format=csv&orderNo=SO-1&currency=CNY&status=matched',
      { method: 'GET', responseType: 'blob' },
    );
    expect(createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:settlement');
    click.mockRestore();
  });
});
