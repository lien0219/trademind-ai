import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { ApiRequestError, getJSON, getWithParams, postJSON } from '../request';

const requestMock = vi.mocked(request);

describe('request helpers', () => {
  it('unwraps successful GET envelopes', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'p1' } });

    await expect(getJSON('/api/v1/products/p1')).resolves.toEqual({ id: 'p1' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1', { method: 'GET' });
  });

  it('sends POST data through the backend envelope', async () => {
    const body = { title: '测试商品' };
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'created' } });

    await expect(postJSON('/api/v1/products', body)).resolves.toEqual({ id: 'created' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/products', { method: 'POST', data: body });
  });

  it('passes query params without dropping undefined boundary keys', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [] } });

    await getWithParams('/api/v1/products/p1/readiness', { platform: 'douyin_shop', shopId: undefined, mode: 'draft' });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/p1/readiness', {
      method: 'GET',
      params: { platform: 'douyin_shop', shopId: undefined, mode: 'draft' },
    });
  });

  it('throws backend business errors with message fallback', async () => {
    requestMock.mockResolvedValueOnce({ code: 40001, message: '商品不存在', data: null });

    await expect(getJSON('/api/v1/products/missing')).rejects.toThrow('商品不存在');
  });

  it('preserves an error envelope returned with a non-2xx status', async () => {
    const requestError = Object.assign(new Error('Request failed with status code 503'), {
      response: {
        status: 503,
        data: { code: 50301, message: '库存账服务暂不可用', data: null },
      },
    });
    requestMock.mockRejectedValueOnce(requestError);
    const promise = getJSON('/api/v1/inventory/warehouse-ledger/reconciliation');

    await expect(promise).rejects.toBeInstanceOf(ApiRequestError);
    await expect(promise).rejects.toEqual(
      expect.objectContaining({
        name: 'ApiRequestError',
        code: 50301,
        message: '库存账服务暂不可用',
      }),
    );
  });
});
