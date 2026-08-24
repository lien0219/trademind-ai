import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestMocks = vi.hoisted(() => ({
  getJSON: vi.fn(),
  getWithParams: vi.fn(),
  patchJSON: vi.fn(),
  postJSON: vi.fn(),
}));

vi.mock('./request', () => requestMocks);

import { createInventoryIdempotencyKey, queryInventoryCenter } from './inventory';

describe('inventory service helpers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('creates bounded action-scoped idempotency keys', () => {
    const first = createInventoryIdempotencyKey('manual-adjust');
    const second = createInventoryIdempotencyKey('manual-adjust');

    expect(first).toMatch(/^admin-manual-adjust-/);
    expect(first.length).toBeLessThanOrEqual(128);
    expect(second).not.toBe(first);
  });

  it('passes the warehouse scope and availability filters to the inventory center', async () => {
    requestMocks.getWithParams.mockResolvedValue({
      list: [],
      pagination: { page: 1, pageSize: 20, total: 0, totalPages: 0 },
    });

    await queryInventoryCenter({
      warehouseId: 'warehouse-main',
      stockStatus: 'low_stock',
      hasException: true,
      page: 2,
      pageSize: 50,
    });

    expect(requestMocks.getWithParams).toHaveBeenCalledWith('/api/v1/inventory', {
      keyword: undefined,
      productId: undefined,
      productSkuId: undefined,
      platform: undefined,
      shopId: undefined,
      warehouseId: 'warehouse-main',
      stockStatus: 'low_stock',
      alertStatus: undefined,
      skuBindStatus: undefined,
      syncStatus: undefined,
      page: 2,
      pageSize: 50,
      hasException: 'true',
    });
  });
});
