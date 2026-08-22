import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { downloadReplenishmentSuggestions, queryReplenishmentSuggestions } from '../procurement';

const requestMock = vi.mocked(request);

describe('replenishment suggestions API service', () => {
  it('uses a required warehouse and keeps filters on a GET request', async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: 'ok',
      data: { warehouseId: 'warehouse-1', list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 },
    });

    await queryReplenishmentSuggestions({
      warehouseId: 'warehouse-1',
      keyword: 'blue sku',
      status: 'actionable',
      page: 2,
      pageSize: 50,
    });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/procurement/replenishment-suggestions', {
      method: 'GET',
      params: {
        warehouseId: 'warehouse-1',
        keyword: 'blue sku',
        status: 'actionable',
        page: 2,
        pageSize: 50,
      },
    });
  });

  it('downloads CSV through the authenticated request client', async () => {
    requestMock.mockResolvedValue(new Blob(['仓库,商品']) as never);
    const createObjectURL = vi.fn(() => 'blob:replenishment');
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);

    await downloadReplenishmentSuggestions({ warehouseId: 'warehouse-1', keyword: 'blue sku', status: 'actionable' });

    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/procurement/replenishment-suggestions?warehouseId=warehouse-1&format=csv&keyword=blue+sku&status=actionable',
      { method: 'GET', responseType: 'blob' },
    );
    expect(createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(click).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:replenishment');
    click.mockRestore();
  });
});
