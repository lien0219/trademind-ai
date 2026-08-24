import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  createSalesReturn,
  createRefundExecution,
  cancelRefundExecution,
  confirmRefundFromPlatform,
  getRefundExecution,
  getSalesReturn,
  listReturnableSalesItems,
  listSalesReturns,
  listRefundExecutions,
  listPlatformAfterSaleReconciliation,
  getPlatformAfterSaleReconciliation,
  transitionSalesReturn,
  recordRefundResult,
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

  it('uses encoded refund execution endpoints and exact write payloads', async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: 'ok',
      data: { id: 'refund-execution-1', list: [], total: 0 },
    });
    const createPayload = {
      idempotencyKey: 'refund-execution-create-key',
      platformAfterSaleId: 'platform/fact',
    };
    const resultPayload = {
      expectedRevision: 1,
      idempotencyKey: 'refund-execution-result-key',
      result: 'succeeded' as const,
      externalRefundId: 'external-refund-1',
      executedAt: '2026-08-24T08:00:00.000Z',
      reason: 'provider console completed',
    };
    const confirmPayload = {
      expectedRevision: 2,
      idempotencyKey: 'refund-execution-confirm-key',
      platformAfterSaleId: 'platform/fact',
      reason: 'provider fact verified',
    };
    const cancelPayload = {
      expectedRevision: 1,
      idempotencyKey: 'refund-execution-cancel-key',
      reason: 'external refund was not started',
    };

    await listRefundExecutions({
      page: 2,
      pageSize: 20,
      status: 'pending',
      salesReturnId: 'return/one',
    });
    await getRefundExecution('execution/one');
    await createRefundExecution('return/one', createPayload);
    await recordRefundResult('execution/one', resultPayload);
    await confirmRefundFromPlatform('execution/one', confirmPayload);
    await cancelRefundExecution('execution/one', cancelPayload);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/refund-executions', {
      method: 'GET',
      params: {
        page: 2,
        pageSize: 20,
        status: 'pending',
        salesReturnId: 'return/one',
      },
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/refund-executions/execution%2Fone',
      { method: 'GET' },
    );
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/sales-returns/return%2Fone/refund-execution',
      { method: 'POST', data: createPayload },
    );
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/refund-executions/execution%2Fone/result',
      { method: 'POST', data: resultPayload },
    );
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/refund-executions/execution%2Fone/confirm-from-platform',
      { method: 'POST', data: confirmPayload },
    );
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/refund-executions/execution%2Fone/cancel',
      { method: 'POST', data: cancelPayload },
    );
  });
});
