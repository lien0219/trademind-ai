import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  createPurchaseReturn,
  getPurchaseReturn,
  listPurchaseReturns,
  listReturnableReceiptItems,
  transitionPurchaseReturn,
} from '../procurement';

const requestMock = vi.mocked(request);

describe('purchase return API service', () => {
  it('uses encoded detail and source-receipt URLs with exact list filters', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { list: [], total: 0 } });

    await listReturnableReceiptItems('purchase/order');
    await listPurchaseReturns({ page: 2, pageSize: 20, status: 'approved', purchaseOrderId: 'purchase/order' });
    await getPurchaseReturn('return/one');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/purchase-orders/purchase%2Forder/returnable-receipt-items', { method: 'GET' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/purchase-returns', {
      method: 'GET',
      params: { page: 2, pageSize: 20, status: 'approved', purchaseOrderId: 'purchase/order' },
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/purchase-returns/return%2Fone', { method: 'GET' });
  });

  it('sends exact creation and revision-checked action payloads', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { id: 'return-1' } });
    const createPayload = {
      idempotencyKey: 'return-create-key',
      purchaseOrderId: 'purchase-order-1',
      reason: 'quality issue',
      remark: 'return to supplier',
      items: [{ goodsReceiptItemId: 'receipt-item-1', quantity: 2 }],
    };
    const actionPayload = { expectedRevision: 3, idempotencyKey: 'return-complete-key', reason: 'shipped' };

    await createPurchaseReturn(createPayload);
    await transitionPurchaseReturn('return/one', 'complete', actionPayload);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/purchase-returns', { method: 'POST', data: createPayload });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/purchase-returns/return%2Fone/complete', { method: 'POST', data: actionPayload });
  });
});
