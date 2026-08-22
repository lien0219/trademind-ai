import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectModalWithinViewport, expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke inventory stocktakes', () => {
  test('keeps cancel side-effect free and saves one counted quantity', async ({ admin, page }) => {
    admin.writeGuard.allow({ operation: 'stocktake-count', method: 'PATCH', path: /^\/api\/v1\/inventory\/stocktakes\/e2e-stocktake-1\/items\/e2e-stocktake-item-1$/, response: ok({ id: 'e2e-stocktake-1', stocktakeNo: 'STK-E2E-0001', warehouseId: 'e2e-warehouse-main', warehouseCode: 'MAIN', warehouseName: 'E2E 华东主仓', status: 'counting', revision: 2, idempotencyKey: 'e2e-stocktake-key', items: [{ id: 'e2e-stocktake-item-1', productId: 'e2e-product-1', productSkuId: 'e2e-product-sku-blue', snapshotOnHand: 10, snapshotReserved: 1, snapshotInTransit: 0, snapshotDamaged: 0, snapshotVersion: 1, countedOnHand: 9, skuCode: 'BLUE-01' }], createdAt: '2026-08-22T02:00:00Z', updatedAt: '2026-08-22T02:00:00Z' }) });
    await page.route('**/api/v1/warehouses', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [{ id: 'e2e-warehouse-main', code: 'MAIN', name: 'E2E 华东主仓', status: 'active', isDefault: true }] })) }));
    await page.route('**/api/v1/inventory', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [{ productId: 'e2e-product-1', productTitle: 'E2E 盘点商品', productSkuId: 'e2e-product-sku-blue', skuCode: 'BLUE-01', skuName: '蓝色', stock: 10 }], pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 } })) }));
    await admin.goto('/inventory/stocktakes');
    await expect(page.getByText('STK-E2E-0001')).toBeVisible();
    await page.getByRole('button', { name: '查看' }).click();
    await expect(page.getByText('盘点详情 · STK-E2E-0001')).toBeVisible();
    const detailDrawer = page.getByRole('dialog', { name: '盘点详情 · STK-E2E-0001' });
    const countInput = page.getByRole('spinbutton', { name: 'BLUE-01实盘数量', exact: true });
    await countInput.fill('9');
    await admin.writeGuard.expectRequestCount('stocktake-count', 0);
    await page.getByRole('button', { name: '保存BLUE-01实盘数量' }).click();
    await expect(page.getByText('盘点数量已保存')).toBeVisible();
    await admin.writeGuard.expectRequestCount('stocktake-count', 1);
    await detailDrawer.getByRole('button', { name: '提交审核' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('stocktake-count', 1);
  });

  test('keeps the create form usable at five viewports and disables readonly writes', async ({ admin, page }) => {
    await page.route('**/api/v1/warehouses', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [{ id: 'e2e-warehouse-main', code: 'MAIN', name: 'E2E 华东主仓', status: 'active', isDefault: true }] })) }));
    await page.route('**/api/v1/inventory', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [{ productId: 'e2e-product-1', productTitle: 'E2E 盘点商品', productSkuId: 'e2e-product-sku-blue', skuCode: 'BLUE-01', stock: 10 }], pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 } })) }));
    for (const viewport of viewports) {
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/stocktakes');
      await page.getByRole('button', { name: '新建盘点' }).click();
      const dialog = page.getByRole('dialog', { name: '新建库存盘点' });
      await expect(dialog).toBeVisible();
      await expectModalWithinViewport(page);
      await expectNoRootOverflow(page);
      await dialog.getByRole('button', { name: /取\s*消/ }).click();
      await expect(dialog).toBeHidden();
    }
    await page.route('**/api/v1/auth/profile', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['inventory.view'] })) }));
    await admin.goto('/inventory/stocktakes');
    await expect(page.getByRole('button', { name: '新建盘点' })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
