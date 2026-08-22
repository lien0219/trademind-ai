import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  actOnInventoryStocktake,
  createInventoryStocktake,
  getInventoryStocktake,
  queryInventoryStocktakes,
  updateInventoryStocktakeItem,
} from '../inventory';

const requestMock = vi.mocked(request);

describe('inventory stocktake API service', () => {
  it('keeps list and detail URLs tenant-safe and query-shaped', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { list: [], total: 0 } });

    await queryInventoryStocktakes({ page: 2, pageSize: 20, status: 'counting' });
    await getInventoryStocktake('stocktake/1');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/inventory/stocktakes', {
      method: 'GET',
      params: { page: 2, pageSize: 20, status: 'counting' },
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/inventory/stocktakes/stocktake%2F1', { method: 'GET' });
  });

  it('sends exact create, counted quantity, and lifecycle payloads', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: { id: 'stocktake-1' } });
    const createPayload = {
      idempotencyKey: 'key-create',
      warehouseId: 'warehouse-1',
      reason: 'cycle count',
      remark: 'monthly',
      items: [{ productSkuId: 'sku-1' }],
    };

    await createInventoryStocktake(createPayload);
    await updateInventoryStocktakeItem('stocktake/1', 'item/1', {
      expectedRevision: 2,
      idempotencyKey: 'key-count',
      countedOnHand: 9,
      remark: 'verified',
    });
    await actOnInventoryStocktake('stocktake/1', 'approve', {
      expectedRevision: 3,
      idempotencyKey: 'key-approve',
      reason: 'reviewed',
    });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/inventory/stocktakes', {
      method: 'POST',
      data: createPayload,
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/inventory/stocktakes/stocktake%2F1/items/item%2F1', {
      method: 'PATCH',
      data: {
        expectedRevision: 2,
        idempotencyKey: 'key-count',
        countedOnHand: 9,
        remark: 'verified',
      },
    });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/inventory/stocktakes/stocktake%2F1/approve', {
      method: 'POST',
      data: { expectedRevision: 3, idempotencyKey: 'key-approve', reason: 'reviewed' },
    });
  });
});
