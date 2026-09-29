import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke warehouse-aware inventory center', () => {
  for (const viewport of viewports) {
    test(`renders warehouse facts at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/overview');
      const main = page.getByRole('main');
      await expect(main.getByText('库存中心', { exact: true }).first()).toBeVisible({ timeout: 30_000 });
      await expect(main.getByText('E2E 库存中心商品')).toBeVisible();
      await expect(main.getByText('投影一致')).toBeVisible();
      await expect(main.getByText('兼容投影只用于迁移期对账')).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('passes the selected warehouse to the inventory query', async ({ admin, page }) => {
    let selectedWarehouse = '';
    await page.route('**/api/v1/inventory**', async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname !== '/api/v1/inventory') {
        await route.fallback();
        return;
      }
      selectedWarehouse = url.searchParams.get('warehouseId') ?? '';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          list: [{
            productId: 'e2e-product-1', productTitle: 'E2E 分仓库存商品',
            productSkuId: 'e2e-product-sku-blue', skuCode: 'BLUE-01', skuName: '蓝色',
            stock: 8, projectionStock: 8, inventoryScope: selectedWarehouse ? 'warehouse' : 'global',
            warehouseId: selectedWarehouse || undefined,
            warehouseCode: selectedWarehouse ? 'MAIN' : undefined,
            warehouseName: selectedWarehouse ? 'E2E 华东主仓' : undefined,
            onHandStock: 10, reservedStock: 1, inTransitStock: 3, damagedStock: 2,
            sellableStock: 8, availableStock: 7, warehouseBalanceCount: 1,
            reconciliationStatus: 'matched', warningStock: 5, safetyStock: 2,
            stockStatus: 'normal', alertTypes: [], publicationCount: 0, platformStocks: [],
            skuBindStatus: 'none', platformSyncStatus: 'none', exceptionCount: 0,
          }],
          pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
        })),
      });
    });

    await admin.goto('/inventory/overview');
    await page.getByLabel('仓库范围').click();
    await page.getByText('E2E 华东主仓（MAIN） · 默认').click();
    await page.getByRole('button', { name: /查\s*询/ }).click();
    await expect.poll(() => selectedWarehouse).toBe('e2e-warehouse-main');
    await expect(page.getByText('E2E 华东主仓（MAIN）').first()).toBeVisible();
    await expect(page).toHaveURL(/warehouseId=e2e-warehouse-main/);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('shows a warehouse-filter error separately from inventory rows', async ({ admin, page }) => {
    await page.route('**/api/v1/warehouses', async (route) => {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ code: 50301, message: '仓库服务暂不可用', data: null }),
      });
    });
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503/);

    await admin.goto('/inventory/overview');
    const alert = page.getByRole('alert').filter({ hasText: '仓库筛选暂不可用' });
    await expect(alert).toContainText('仓库服务暂不可用');
    await expect(page.getByText('E2E 库存中心商品')).toBeVisible();
    await expectNoRootOverflow(page);
  });
});
