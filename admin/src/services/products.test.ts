import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  createProductSku,
  updateProductSku,
  type CreateProductSkuBody,
  type UpdateProductSkuBody,
} from './products';

const requestMock = vi.mocked(request);

describe('product SKU service inventory boundary', () => {
  it('does not send stock through the SKU create endpoint', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'sku-new' } });

    await createProductSku('product-1', {
      skuCode: 'NEW',
      skuName: 'New SKU',
      stock: 12,
    } as CreateProductSkuBody & { stock: number });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/product-1/skus', {
      method: 'POST',
      data: {
        skuCode: 'NEW',
        skuName: 'New SKU',
        imageUrl: '',
      },
    });
  });

  it('does not send stock through the SKU metadata update endpoint', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { id: 'sku-1' } });

    await updateProductSku('product-1', 'sku-1', {
      skuName: 'Updated SKU',
      stock: 9,
    } as UpdateProductSkuBody & { stock: number });

    expect(requestMock).toHaveBeenCalledWith('/api/v1/products/product-1/skus/sku-1', {
      method: 'PUT',
      data: { skuName: 'Updated SKU' },
    });
  });
});
