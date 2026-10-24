import { test, expect } from '../fixtures/admin.fixture';
import type { Page, Request } from '@playwright/test';
import { ok } from '../mocks/envelope';
import { inventoryRun } from '../mocks/inventory-sync';
import type { AdminPage } from '../pages/admin.page';
import { expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 375, height: 812 },
  { width: 768, height: 1024 },
  { width: 1024, height: 768 },
  { width: 1280, height: 900 },
  { width: 1440, height: 960 },
];

// Inventory Sync is an internal fixture-only route. Production preview deliberately
// omits it, so both builds assert their expected route behavior instead of skipping.
const isProductionPreview = Boolean(process.env.CI);

async function expectProductionRouteExcluded(admin: AdminPage, page: Page, path: string) {
  let sensitiveReads = 0;
  const countSensitiveReads = (request: Request) => {
    if (request.url().includes('/api/v1/inventory-sync/')) sensitiveReads += 1;
  };
  page.on('request', countSensitiveReads);
  try {
    await admin.goto(path);
    await expect(page.getByText('404', { exact: true })).toBeVisible();
    await expect(page.getByText('页面不存在', { exact: true })).toBeVisible();
    expect(sensitiveReads).toBe(0);
  } finally {
    page.off('request', countSensitiveReads);
  }
}

test.describe('@inventory-sync Admin fixture binding center', () => {
  for (const viewport of viewports) {
    test(`dashboard renders without overflow at ${viewport.width}px`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      if (isProductionPreview) {
        await expectProductionRouteExcluded(admin, page, '/ops/inventory-sync');
        await expectNoRootOverflow(page);
        await admin.writeGuard.expectNoUnexpectedWrites();
        return;
      }
      await admin.goto('/ops/inventory-sync');
      await expect(page.getByText('非真实库存读取')).toBeVisible({ timeout: 30000 });
      await expect(page.getByText(/inventory-run-e2/).first()).toBeVisible();
      await expect(page.getByText('success_single_page').first()).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }

  test('create run sends only fixture write body through guarded endpoint', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-run',
      method: 'POST',
      path: /^\/api\/v1\/inventory-sync\/runs$/,
      response: ok(inventoryRun),
    });

    if (isProductionPreview) {
      await expectProductionRouteExcluded(admin, page, '/ops/inventory-sync');
      await admin.writeGuard.expectRequestCount('create-run', 0);
      await admin.writeGuard.expectNoUnexpectedWrites();
      return;
    }
    await admin.goto('/ops/inventory-sync');
    await expect(page.getByText('非真实库存读取')).toBeVisible({ timeout: 30000 });
    await page.getByTestId('inventory-sync-create-run').click();
    await page.getByTestId('inventory-sync-create-run-shop').fill('inventory-shop-e2e');
    await page.getByTestId('inventory-sync-create-run-submit').click();

    await admin.writeGuard.expectRequestCount('create-run', 1);
    expect(admin.writeGuard.calls('create-run')[0].postDataJSON).toEqual({
      shopConnectionId: 'inventory-shop-e2e',
      platform: 'douyin',
      providerMode: 'mock',
      fixtureScenario: 'success_single_page',
    });
    await expect(page).toHaveURL(/\/ops\/inventory-sync\/runs\/inventory-run-e2e-1/);
  });

  test('calibration, manual binding, and binding history routes use fixture copy', async ({ admin, page }) => {
    if (isProductionPreview) {
      for (const path of [
        '/ops/inventory-sync/calibration?snapshotId=inventory-snapshot-e2e-1',
        '/ops/inventory-sync/manual-bindings?requestId=inventory-manual-e2e-1',
        '/ops/inventory-sync/bindings/inventory-binding-e2e-1',
      ]) {
        await expectProductionRouteExcluded(admin, page, path);
      }
      await admin.writeGuard.expectNoUnexpectedWrites();
      return;
    }
    await admin.goto('/ops/inventory-sync/calibration?snapshotId=inventory-snapshot-e2e-1');
    await expect(page.getByText('来自 fixture 快照')).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/inventory-cali/).first()).toBeVisible();
    await expect(page.getByText('85%').first()).toBeVisible();
    await expectNoRootOverflow(page);

    await admin.goto('/ops/inventory-sync/manual-bindings?requestId=inventory-manual-e2e-1');
    await expect(page.getByText('不触发真实平台库存写入')).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/inventory-manu/).first()).toBeVisible();
    await expect(page.getByText('manual_review_required').first()).toBeVisible();
    await expectNoRootOverflow(page);

    await admin.goto('/ops/inventory-sync/bindings/inventory-binding-e2e-1');
    await expect(page.getByText('不会写入真实平台库存')).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/inventory-bind/).first()).toBeVisible();
    await expect(page.getByText('token')).toHaveCount(0);
    await expectNoRootOverflow(page);
  });
});
