import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { e2eProduct, E2E_PRODUCT_ID, e2eReadinessPassed, publication } from '../mocks/product.fixture';
import { imageProviderCapabilities } from '../mocks/image-providers';
import { e2eUser } from '../mocks/auth';
import { skuBindingsResponse } from '../mocks/publish';
import { e2ePurchaseOrder, e2ePurchaseReturn, e2eReturnableReceiptItem, e2eSupplier, e2eWarehouse, E2E_PURCHASE_ORDER_ID, E2E_PURCHASE_RETURN_ID, E2E_WAREHOUSE_ID } from '../mocks/procurement';

async function fetchApi(page: import('@playwright/test').Page, path: string) {
  if (page.url() === 'about:blank') {
    await page.goto('/dashboard/product-operations');
  }
  return page.evaluate(async (apiPath) => {
    const res = await fetch(apiPath);
    return res.json();
  }, path);
}

test.describe('@contract API envelope contracts', () => {
  test('image/providers accepts empty array envelope', async ({ page }) => {
    await page.route('**/api/v1/image/providers', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok([])) }));
    await page.goto('/settings/image');
    await expect(page.locator('#root')).toBeVisible();
    expect(await fetchApi(page, '/api/v1/image/providers')).toEqual(ok([]));
  });

  test('image/providers accepts ImageProviderCapability array envelope', async ({ page }) => {
    const json = await fetchApi(page, '/api/v1/image/providers');
    expect(json).toEqual(ok(imageProviderCapabilities));
    expect(Array.isArray(json.data)).toBe(true);
    expect(json.data[0]).toMatchObject({ provider: 'mock-image-provider', supportedTasks: expect.any(Array) });
  });

  test('image/providers business error envelope does not create fatal page error', async ({ page }) => {
    await page.route('**/api/v1/image/providers', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fail('provider disabled', 10001, [])) }));
    await page.goto('/settings/image');
    await expect(page.locator('#root')).toBeVisible();
    expect(await fetchApi(page, '/api/v1/image/providers')).toEqual(fail('provider disabled', 10001, []));
  });

  test('image/providers invalid data structure keeps page mounted without runtime patching', async ({ page }) => {
    await page.route('**/api/v1/image/providers', async (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [] })) }));
    await page.goto('/settings/image');
    await expect(page.locator('#root')).toBeVisible();
  });

  test('core admin envelopes match request helper shape', async ({ page }) => {
    expect(await fetchApi(page, '/api/v1/auth/profile')).toEqual(ok(e2eUser));
    expect(await fetchApi(page, `/api/v1/products/${E2E_PRODUCT_ID}`)).toEqual(ok(e2eProduct));
    expect(await fetchApi(page, `/api/v1/products/${E2E_PRODUCT_ID}/readiness`)).toEqual(ok(e2eReadinessPassed));
    expect(await fetchApi(page, `/api/v1/products/${E2E_PRODUCT_ID}/publications`)).toEqual(ok({ list: [publication()] }));
    expect(await fetchApi(page, '/api/v1/product-publications/e2e-publication-old/douyin/sku-bindings')).toEqual(skuBindingsResponse('e2e-publication-old'));
    expect(await fetchApi(page, '/api/v1/warehouses')).toEqual(ok({ list: [e2eWarehouse] }));
    expect(await fetchApi(page, '/api/v1/suppliers')).toEqual(ok({ list: [e2eSupplier] }));
    expect(await fetchApi(page, `/api/v1/procurement/replenishment-suggestions?warehouseId=${E2E_WAREHOUSE_ID}`)).toMatchObject(ok({ warehouseId: E2E_WAREHOUSE_ID }));
    expect(await fetchApi(page, `/api/v1/purchase-orders/${E2E_PURCHASE_ORDER_ID}`)).toEqual(ok(e2ePurchaseOrder));
    expect(await fetchApi(page, `/api/v1/purchase-orders/${E2E_PURCHASE_ORDER_ID}/returnable-receipt-items`)).toEqual(ok({ list: [e2eReturnableReceiptItem] }));
    expect(await fetchApi(page, '/api/v1/purchase-returns')).toEqual(ok({ list: [{ ...e2ePurchaseReturn, items: undefined }], page: 1, pageSize: 20, total: 1, totalPages: 1 }));
    expect(await fetchApi(page, `/api/v1/purchase-returns/${E2E_PURCHASE_RETURN_ID}`)).toEqual(ok(e2ePurchaseReturn));
  });
});
