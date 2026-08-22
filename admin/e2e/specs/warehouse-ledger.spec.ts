import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke warehouse inventory ledger', () => {
  for (const viewport of viewports) {
    test(`renders reconciliation at ${viewport.width}x${viewport.height} without writes`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/warehouse-ledger');
      const main = page.getByRole('main');
      await expect(main.getByText('仓库库存账', { exact: true }).first()).toBeVisible({ timeout: 30_000 });
      await expect(main.getByText('E2E 库存账商品')).toBeVisible();
      await expect(page.getByText('不会自动补货')).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('migrates only after explicit confirmation with a bounded batch', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'migrate-legacy-stock',
      method: 'POST',
      path: /^\/api\/v1\/inventory\/warehouse-ledger\/migrate-legacy$/,
      response: ok({ warehouseId: 'e2e-warehouse-main', warehouseCode: 'MAIN', migratedCount: 1, remainingCount: 0 }),
    });

    await admin.goto('/inventory/warehouse-ledger');
    await page.getByRole('button', { name: '迁移一批历史库存' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('migrate-legacy-stock', 0);

    await page.getByRole('button', { name: '迁移一批历史库存' }).click();
    await page.getByRole('dialog').getByRole('button', { name: '确认迁移' }).click();
    await admin.writeGuard.expectRequestCount('migrate-legacy-stock', 1);
    expect(admin.writeGuard.calls('migrate-legacy-stock')[0]?.postDataJSON).toEqual({ limit: 100 });
  });

  test('requires a warehouse and stable idempotency key for manual adjustment', async ({ admin, page }) => {
    await page.route('**/api/v1/inventory/alerts**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          list: [{
            productId: 'e2e-product-1',
            productTitle: 'E2E 库存调整商品',
            productSkuId: 'e2e-product-sku-blue',
            skuCode: 'BLUE-01',
            skuName: '蓝色',
            stock: 10,
            warningStock: 5,
            safetyStock: 2,
            stockStatus: 'normal',
            alertTypes: [],
            publicationCount: 0,
            platformStocks: [],
          }],
          pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
        })),
      });
    });
    admin.writeGuard.allow({
      operation: 'manual-warehouse-adjustment',
      method: 'POST',
      path: /^\/api\/v1\/products\/e2e-product-1\/skus\/e2e-product-sku-blue\/adjust-stock$/,
      response: ok({ productSkuId: 'e2e-product-sku-blue', warehouseId: 'e2e-warehouse-main', warehouseOnHand: 8, aggregateStock: 8 }),
    });

    await admin.goto('/inventory/alerts');
    await page.getByRole('button', { name: '调整库存' }).click();
    let dialog = page.getByRole('dialog', { name: /调整库存/ });
    await expect(dialog.getByText('E2E 华东主仓（MAIN） · 默认', { exact: true })).toBeVisible();
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('manual-warehouse-adjustment', 0);

    await page.getByRole('button', { name: '调整库存' }).click();
    dialog = page.getByRole('dialog', { name: /调整库存/ });
    await dialog.getByLabel('库存（≥0）').fill('8');
    await dialog.getByRole('button', { name: /确\s*定/ }).click();
    let sensitiveDialog = page.getByRole('dialog', { name: '人工修正库存' });
    await sensitiveDialog.getByRole('button', { name: /取\s*消/ }).click();
    await expect(sensitiveDialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('manual-warehouse-adjustment', 0);

    await dialog.getByRole('button', { name: /确\s*定/ }).click();
    sensitiveDialog = page.getByRole('dialog', { name: '人工修正库存' });
    await sensitiveDialog.getByRole('button', { name: '确认执行' }).click();
    await admin.writeGuard.expectRequestCount('manual-warehouse-adjustment', 1);
    const payload = admin.writeGuard.calls('manual-warehouse-adjustment')[0]?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      warehouseId: 'e2e-warehouse-main',
      stock: 8,
      reason: 'manual_adjust',
      remark: 'from_inventory_alerts',
    });
    expect(payload.idempotencyKey).toMatch(/^admin-manual-adjust-/);
  });

  test('distinguishes an empty ledger and disables migration for readonly users', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['inventory.view'] })),
      });
    });
    await page.route('**/api/v1/inventory/warehouse-ledger/reconciliation**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          list: [], total: 0, page: 1, pageSize: 20, totalPages: 0,
          matched: 0, unmigrated: 0, mismatch: 0,
        })),
      });
    });

    await admin.goto('/inventory/warehouse-ledger');
    await expect(page.getByText('暂无库存账记录')).toBeVisible();
    await expect(page.getByRole('button', { name: '迁移一批历史库存' })).toBeDisabled();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('shows a request error instead of presenting it as an empty ledger', async ({ admin, page }) => {
    await page.route('**/api/v1/inventory/warehouse-ledger/reconciliation**', async (route) => {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ code: 50301, message: '库存账服务暂不可用', data: null }),
      });
    });
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503/);

    await admin.goto('/inventory/warehouse-ledger');
    const reconciliationError = page.getByRole('alert');
    await expect(reconciliationError).toContainText('库存账对账加载失败');
    await expect(reconciliationError).toContainText('库存账服务暂不可用');
    await expect(reconciliationError.getByRole('button', { name: /重\s*试/ })).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('disables warehouse adjustment on inventory alerts for readonly users', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['inventory.view'] })),
      });
    });
    await page.route('**/api/v1/inventory/alerts**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          list: [{
            productId: 'e2e-product-1', productTitle: 'E2E 只读库存商品',
            productSkuId: 'e2e-product-sku-blue', skuCode: 'BLUE-01', skuName: '蓝色',
            stock: 10, warningStock: 5, safetyStock: 2, stockStatus: 'normal',
            alertTypes: [], publicationCount: 0, platformStocks: [],
          }],
          pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
        })),
      });
    });

    await admin.goto('/inventory/alerts');
    await expect(page.getByText('E2E 只读库存商品')).toBeVisible();
    await expect(page.getByRole('button', { name: '调整库存' })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
