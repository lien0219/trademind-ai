import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  createSalesReturn,
  getSalesReturn,
  listReturnableSalesItems,
  listSalesReturns,
  listPlatformAfterSaleReconciliation,
  getPlatformAfterSaleReconciliation,
  transitionSalesReturn,
} from '../salesReturns';

const requestMock = vi.mocked(request);

describe('sales return API service', () => {
  it('uses encoded detail and order-source URLs with exact filters', async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: 'ok',
      data: { list: [], total: 0 },
    });

    await listReturnableSalesItems('order/source');
    await listSalesReturns({
      page: 2,
      pageSize: 20,
      status: 'approved',
      type: 'return_refund',
      orderId: 'order/source',
    });
    await getSalesReturn('return/one');

    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/orders/order%2Fsource/sales-returnable-items',
      { method: 'GET' },
    );
    expect(requestMock).toHaveBeenCalledWith('/api/v1/sales-returns', {
      method: 'GET',
      params: {
        page: 2,
        pageSize: 20,
        status: 'approved',
        type: 'return_refund',
        orderId: 'order/source',
      },
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/sales-returns/return%2Fone',
      { method: 'GET' },
    );
  });

  it('keeps platform after-sale reconciliation endpoints read-only and encoded', async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: 'ok',
      data: { list: [], total: 0, page: 1, pageSize: 20, totalPages: 0 },
    });
    await listPlatformAfterSaleReconciliation({
      page: 2,
      pageSize: 20,
      platform: 'douyin_shop',
      reconciliationStatus: 'mismatch',
      orderNo: 'order/source',
    });
    await getPlatformAfterSaleReconciliation('after-sale/one');
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/sales-return-reconciliation',
      {
        method: 'GET',
        params: {
          page: 2,
          pageSize: 20,
          platform: 'douyin_shop',
          reconciliationStatus: 'mismatch',
          orderNo: 'order/source',
        },
      },
    );
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/sales-return-reconciliation/after-sale%2Fone',
      { method: 'GET' },
    );
  });

  it('sends exact creation and revision-checked action payloads', async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: 'ok',
      data: { id: 'return-1' },
    });
    const createPayload = {
      idempotencyKey: 'sales-return-create-key',
      orderId: 'order-1',
      type: 'return_refund' as const,
      reason: 'customer request',
      remark: 'opened packaging',
      items: [
        {
          orderItemId: 'item-1',
          quantity: 2,
          disposition: 'sellable' as const,
          refundAmountMinor: 2000,
        },
      ],
    };
    const actionPayload = {
      expectedRevision: 3,
      idempotencyKey: 'sales-return-complete-key',
      reason: 'received',
    };

    await createSalesReturn(createPayload);
    await transitionSalesReturn('return/one', 'complete', actionPayload);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/sales-returns', {
      method: 'POST',
      data: createPayload,
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/sales-returns/return%2Fone/complete',
      { method: 'POST', data: actionPayload },
    );
  });
});
