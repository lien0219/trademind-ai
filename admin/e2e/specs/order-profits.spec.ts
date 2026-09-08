import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import { E2E_ORDER_PROFIT_ORDER_ID } from '../mocks/order-profits';
import {
  expectDrawerWithinViewport,
  expectHeaderContentAligned,
  expectNoRootOverflow,
  expectTableFilterBarAlignedLeft,
} from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke order profit estimates', () => {
  for (const viewport of viewports) {
    test(`renders partial profit facts at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/finance/order-profits');
      await expect(page.getByText('订单预估利润', { exact: true }).first()).toBeVisible();
      await expect(page.getByText('SO-E2E-PROFIT-0001', { exact: true })).toBeVisible();
      await expect(page.getByText('本页为动态预估，不是会计利润')).toBeVisible();
      await expect(page.getByText('CNY 35.00', { exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expectTableFilterBarAlignedLeft(page);

      await page.getByRole('button', { name: '查看' }).first().click();
      const drawer = page.getByRole('dialog');
      await expect(drawer).toContainText('SO-E2E-PROFIT-0001');
      await expect(drawer).toContainText('平台结算账单');
      await expect(drawer).toContainText('履约成本快照');
      await expect(drawer).toContainText('E2E 蓝牙耳机');
      await expect(page.locator('.ant-drawer-content-wrapper:visible').first()).toHaveCSS(
        'transform',
        'none',
      );
      await expectDrawerWithinViewport(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('keeps CSV unavailable to readonly viewers', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({ ...e2eUser, role: 'readonly', permissions: ['order_profit.view'] }),
        ),
      });
    });
    await admin.goto('/finance/order-profits');
    await expect(page.getByRole('button', { name: '导出明细' })).toBeDisabled();
    await expect(page.getByText('SO-E2E-PROFIT-0001', { exact: true })).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('keeps service errors distinct from an empty result', async ({ admin, page }) => {
    await page.route('**/api/v1/order-profits?*', async (route) => {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ code: 50301, message: '预估利润服务暂不可用', data: null }),
      });
    });
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503/);
    await admin.goto('/finance/order-profits');
    await expect(page.getByRole('alert').filter({ hasText: '预估利润服务暂不可用' })).toBeVisible();
    await expect(page.getByText('预估利润数据暂不可用')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('supports a direct detail link without writes', async ({ admin, page }) => {
    await admin.goto(
      `/finance/order-profits?drawer=order-profit&id=${E2E_ORDER_PROFIT_ORDER_ID}`,
    );
    await expect(page.getByRole('dialog')).toContainText('SO-E2E-PROFIT-0001');
    await expect(page).toHaveURL(/drawer=order-profit/);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
