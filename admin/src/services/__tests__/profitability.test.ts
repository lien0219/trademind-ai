import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  downloadOrderProfits,
  formatMarginBps,
  formatProfitAmount,
  getOrderProfit,
  queryOrderProfits,
} from '../profitability';

const requestMock = vi.mocked(request);

describe('order profitability API service', () => {
  it('keeps list filters on the read-only endpoint', async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: 'ok',
      data: { list: [], page: 2, pageSize: 50, total: 0, totalPages: 0 },
    });

    await queryOrderProfits({
      page: 2,
      pageSize: 50,
      orderNo: 'TM-1',
      platform: 'douyin_shop',
      shopId: 'shop-1',
      warehouseId: 'warehouse-1',
      currency: 'CNY',
      status: 'mismatch',
      start: '2026-09-01T00:00:00.000Z',
      end: '2026-09-02T23:59:59.999Z',
    });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/order-profits', {
      method: 'GET',
      params: {
        page: 2,
        pageSize: 50,
        orderNo: 'TM-1',
        platform: 'douyin_shop',
        shopId: 'shop-1',
        warehouseId: 'warehouse-1',
        currency: 'CNY',
        status: 'mismatch',
        start: '2026-09-01T00:00:00.000Z',
        end: '2026-09-02T23:59:59.999Z',
      },
    });
  });

  it('encodes the order id on detail reads', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { orderId: 'order/1' } });
    await getOrderProfit('order/1');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/order-profits/order%2F1', { method: 'GET' });
  });

  it('downloads bounded CSV through the authenticated request client', async () => {
    requestMock.mockResolvedValue(new Blob(['订单号']) as never);
    const createObjectURL = vi.fn(() => 'blob:profits');
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);

    await downloadOrderProfits({ orderNo: ' TM-1 ', currency: 'cny', status: 'pending' });

    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/order-profits?format=csv&orderNo=TM-1&currency=CNY&status=pending',
      { method: 'GET', responseType: 'blob' },
    );
    expect(createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(click).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:profits');
    click.mockRestore();
  });

  it('formats currency exponents and margins without lossy values', () => {
    expect(formatProfitAmount(1234, 'CNY')).toBe('CNY 12.34');
    expect(formatProfitAmount(1234, 'JPY')).toBe('JPY 1234');
    expect(formatProfitAmount(-1234, 'KWD')).toBe('KWD -1.234');
    expect(formatProfitAmount(Number.MAX_SAFE_INTEGER + 1, 'CNY')).toBe('—');
    expect(formatMarginBps(1250)).toBe('12.50%');
    expect(formatMarginBps(null)).toBe('—');
  });
});
