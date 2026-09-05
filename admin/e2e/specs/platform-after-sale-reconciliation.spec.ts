import { test, expect } from '../fixtures/admin.fixture';
import { E2E_PLATFORM_AFTER_SALE_ID } from '../mocks/sales-returns';
import { expectHeaderContentAligned, expectNoRootOverflow, expectTableFilterBarAlignedLeft } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke platform after-sale reconciliation', () => {
  for (const viewport of viewports) {
    test(`renders read-only reconciliation at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/orders/sales-return-reconciliation');
      await expect(page.getByText('e2e-after-sale-1')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByText('已匹配')).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expectTableFilterBarAlignedLeft(page);

      await page.getByText('e2e-after-sale-1').click();
      await expect(page.getByText('平台事实')).toBeVisible();
      await expect(
        page
          .locator('.ant-descriptions')
          .getByText('e2e-after-sale-event-1', { exact: true }),
      ).toBeVisible();
      await expect(page).toHaveURL(new RegExp(`/orders/sales-return-reconciliation/${E2E_PLATFORM_AFTER_SALE_ID}$`));
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('distinguishes empty and API error states', async ({ admin, page }) => {
    const listRoute = '**/api/v1/sales-return-reconciliation**';
    await page.route(listRoute, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, message: 'ok', data: { list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 } }),
      });
    });
    await admin.goto('/orders/sales-return-reconciliation');
    await expect(page.getByText('暂无平台售后事实。')).toBeVisible();
    await page.unroute(listRoute);
    await page.route(listRoute, async (route) => {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 50000, message: 'unavailable', data: null }) });
    });
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503/);
    await admin.goto('/orders/sales-return-reconciliation');
    await expect(page.getByText('平台售后对账加载失败，请稍后重试。')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
