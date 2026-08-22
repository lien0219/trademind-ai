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

test.describe('@smoke warehouse transfers', () => {
  test('keeps cancel side-effect free and dispatch confirmation idempotent', async ({ admin, page }) => {
    admin.writeGuard.allow({ operation: 'dispatch-transfer', method: 'POST', path: /^\/api\/v1\/inventory\/warehouse-transfers\/e2e-transfer-1\/dispatch$/, response: ok({ id: 'e2e-transfer-1', status: 'in_transit', revision: 4 }) });
    await page.route('**/api/v1/warehouses', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [
      { id: 'e2e-warehouse-main', code: 'MAIN', name: 'E2E 华东主仓', status: 'active', isDefault: true },
      { id: 'e2e-warehouse-east', code: 'EAST', name: 'E2E 华东备仓', status: 'active', isDefault: false },
    ] })) }));
    await page.route('**/api/v1/inventory', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [{ productId: 'e2e-product-1', productTitle: 'E2E 调拨商品', productSkuId: 'e2e-product-sku-blue', skuCode: 'BLUE-01', skuName: '蓝色', stock: 10 }], pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 } })) }));
    await admin.goto('/inventory/warehouse-transfers');
    await expect(page.getByText('TRF-E2E-0001')).toBeVisible();
    await expectNoRootOverflow(page);
    await page.getByRole('button', { name: '查看' }).click();
    await expect(page.getByText('调拨详情 · TRF-E2E-0001')).toBeVisible();
    await page.getByRole('button', { name: '关闭' }).click();
    await expect(page.getByText('调拨详情 · TRF-E2E-0001')).toBeHidden();
    await page.getByRole('button', { name: '发出' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('dispatch-transfer', 0);
    await page.getByRole('button', { name: '发出' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /发\s*出/ }).click();
    await admin.writeGuard.expectRequestCount('dispatch-transfer', 1);
    expect(admin.writeGuard.calls('dispatch-transfer')[0]?.postDataJSON).toMatchObject({ expectedRevision: 3 });
  });

  test('opens a responsive create form without form connection warnings', async ({ admin, page }) => {
    await page.route('**/api/v1/warehouses', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [
      { id: 'e2e-warehouse-main', code: 'MAIN', name: 'E2E 华东主仓', status: 'active', isDefault: true },
      { id: 'e2e-warehouse-east', code: 'EAST', name: 'E2E 华东备仓', status: 'active', isDefault: false },
    ] })) }));
    await page.route('**/api/v1/inventory', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [{ productId: 'e2e-product-1', productTitle: 'E2E 调拨商品', productSkuId: 'e2e-product-sku-blue', skuCode: 'BLUE-01', skuName: '蓝色', stock: 10 }], pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 } })) }));

    for (const viewport of viewports) {
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/warehouse-transfers');
      await page.getByRole('button', { name: '新建调拨' }).click();
      const dialog = page.getByRole('dialog', { name: '新建仓库调拨' });
      await expect(dialog).toBeVisible();
      await expectModalWithinViewport(page);
      await expectNoRootOverflow(page);

      const [sourceBox, targetBox] = await dialog.getByTestId(/^(source|target)-warehouse-field$/).evaluateAll((elements) => elements.map((element) => {
        const { x, y, width, height } = element.getBoundingClientRect();
        return { x, y, width, height };
      }));
      expect(sourceBox, `source warehouse field at ${viewport.width}x${viewport.height}`).not.toBeNull();
      expect(targetBox, `target warehouse field at ${viewport.width}x${viewport.height}`).not.toBeNull();
      if (sourceBox && targetBox) {
        expect(sourceBox.width, `source warehouse width at ${viewport.width}x${viewport.height}`).toBeGreaterThan(0);
        expect(targetBox.width, `target warehouse width at ${viewport.width}x${viewport.height}`).toBeGreaterThan(0);
        if (viewport.width < 576) {
          expect(Math.abs(sourceBox.x - targetBox.x), `stacked warehouse fields at ${viewport.width}x${viewport.height}`).toBeLessThanOrEqual(2);
          expect(targetBox.y, `stacked warehouse fields at ${viewport.width}x${viewport.height}`).toBeGreaterThan(sourceBox.y);
        } else {
          expect(Math.abs(sourceBox.width - targetBox.width), `balanced warehouse fields at ${viewport.width}x${viewport.height}`).toBeLessThanOrEqual(8);
        }
      }
      await dialog.getByRole('button', { name: /取\s*消/ }).click();
      await expect(dialog).toBeHidden();
    }
  });

  test('disables transfer actions for readonly users', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['inventory.view'] })) }));
    await admin.goto('/inventory/warehouse-transfers');
    await expect(page.getByText('TRF-E2E-0001')).toBeVisible();
    await expect(page.getByRole('button', { name: '新建调拨' })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
