import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestMocks = vi.hoisted(() => ({
  getJSON: vi.fn(),
  getWithParams: vi.fn(),
  patchJSON: vi.fn(),
  postJSON: vi.fn(),
  putJSON: vi.fn(),
}));

vi.mock('./request', () => requestMocks);

import {
  createInventoryIdempotencyKey,
  createWarehouseSKUPlacement,
  listWarehouseLocations,
  listWarehouseSKUPlacements,
  queryInventoryCenter,
  updateWarehouseSKUPlacement,
} from './inventory';

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

  it('keeps placement and location requests tenant-scoped by warehouse', async () => {
    requestMocks.getJSON.mockResolvedValue({ list: [] });
    requestMocks.getWithParams.mockResolvedValue({ list: [] });

    await listWarehouseLocations('warehouse-main');
    await listWarehouseSKUPlacements({ warehouseId: 'warehouse-main', includeInactive: true });
    await createWarehouseSKUPlacement({ warehouseId: 'warehouse-main', productSkuId: 'sku-1', status: 'active' });
    await updateWarehouseSKUPlacement('placement-1', { status: 'inactive' });

    expect(requestMocks.getJSON).toHaveBeenCalledWith(
      '/api/v1/warehouses/warehouse-main/locations?includeInactive=false',
    );
    expect(requestMocks.getWithParams).toHaveBeenCalledWith('/api/v1/inventory/warehouse-placements', {
      warehouseId: 'warehouse-main',
      productSkuId: undefined,
      includeInactive: 'true',
    });
    expect(requestMocks.postJSON).toHaveBeenCalledWith('/api/v1/inventory/warehouse-placements', {
      warehouseId: 'warehouse-main',
      productSkuId: 'sku-1',
      locationId: undefined,
      barcode: undefined,
      status: 'active',
    });
    expect(requestMocks.putJSON).toHaveBeenCalledWith('/api/v1/inventory/warehouse-placements/placement-1', {
      status: 'inactive',
    });
  });
});
