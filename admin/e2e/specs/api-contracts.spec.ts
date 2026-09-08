import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { e2eProduct, E2E_PRODUCT_ID, e2eReadinessPassed, publication } from '../mocks/product.fixture';
import { imageProviderCapabilities } from '../mocks/image-providers';
import { e2eUser } from '../mocks/auth';
import { skuBindingsResponse } from '../mocks/publish';
import { e2ePurchaseOrder, e2ePurchaseReturn, e2eReturnableReceiptItem, e2eSupplier, e2eWarehouse, E2E_PURCHASE_ORDER_ID, E2E_PURCHASE_RETURN_ID, E2E_WAREHOUSE_ID } from '../mocks/procurement';
import {
  e2eReturnableSalesItem,
  e2eSalesReturn,
  E2E_SALES_ORDER_ID,
  E2E_SALES_RETURN_ID,
  e2ePlatformAfterSale,
  E2E_PLATFORM_AFTER_SALE_ID,
  E2E_REFUND_EXECUTION_ID,
  e2eRefundExecution,
} from '../mocks/sales-returns';
import { E2E_ALLOCATION_ORDER_ID, e2eWarehouseAllocation } from '../mocks/order-warehouse-allocation';
import {
  E2E_ORDER_PROFIT_ORDER_ID,
  e2eOrderProfit,
  e2eOrderProfitDetail,
} from '../mocks/order-profits';
import { E2E_SETTLEMENT_ID, e2eSettlementDetail, e2eSettlementReconciliation } from '../mocks/settlement';

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
    expect(await fetchApi(page, `/api/v1/orders/${E2E_SALES_ORDER_ID}/sales-returnable-items`)).toMatchObject(
      ok({ orderId: E2E_SALES_ORDER_ID, list: [e2eReturnableSalesItem] }),
    );
    expect(await fetchApi(page, '/api/v1/orders/warehouse-allocations')).toMatchObject(
      ok({
        list: [
          expect.objectContaining({
            id: E2E_ALLOCATION_ORDER_ID,
            allocationStatus: 'allocatable',
          }),
        ],
      }),
    );
    expect(await fetchApi(page, `/api/v1/orders/${E2E_ALLOCATION_ORDER_ID}/warehouse-allocation`)).toEqual(ok(e2eWarehouseAllocation));
    expect(await fetchApi(page, '/api/v1/sales-returns')).toEqual(
      ok({ list: [{ ...e2eSalesReturn, items: undefined }], page: 1, pageSize: 20, total: 1, totalPages: 1 }),
    );
    expect(await fetchApi(page, `/api/v1/sales-returns/${E2E_SALES_RETURN_ID}`)).toEqual(ok(e2eSalesReturn));
    expect(await fetchApi(page, '/api/v1/sales-return-reconciliation')).toEqual(
      ok({ list: [e2ePlatformAfterSale], page: 1, pageSize: 20, total: 1, totalPages: 1 }),
    );
    expect(await fetchApi(page, `/api/v1/sales-return-reconciliation/${E2E_PLATFORM_AFTER_SALE_ID}`)).toEqual(ok(e2ePlatformAfterSale));
    expect(await fetchApi(page, '/api/v1/refund-executions')).toEqual(
      ok({ list: [e2eRefundExecution], page: 1, pageSize: 20, total: 1, totalPages: 1 }),
    );
    expect(await fetchApi(page, `/api/v1/refund-executions/${E2E_REFUND_EXECUTION_ID}`)).toEqual(
      ok(e2eRefundExecution),
    );
    expect(await fetchApi(page, '/api/v1/order-profits')).toMatchObject(
      ok({
        list: [e2eOrderProfit],
        page: 1,
        pageSize: 20,
        total: 1,
        totalPages: 1,
        formula: { version: 'order_profit_estimate_v3' },
      }),
    );
    expect(await fetchApi(page, `/api/v1/order-profits/${E2E_ORDER_PROFIT_ORDER_ID}`)).toEqual(
      ok(e2eOrderProfitDetail),
    );
    expect(await fetchApi(page, '/api/v1/settlement-reconciliation')).toEqual(
      ok({ list: [e2eSettlementReconciliation], page: 1, pageSize: 20, total: 1, totalPages: 1 }),
    );
    expect(await fetchApi(page, `/api/v1/settlement-reconciliation/${E2E_SETTLEMENT_ID}`)).toEqual(
      ok(e2eSettlementDetail),
    );
  });
});
