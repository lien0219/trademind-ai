import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { E2E_RECONCILIATION_ORDER_ID } from '../mocks/order-fulfillment-reconciliation';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';

test.describe('@smoke order fulfillment reconciliation', () => {
  test('renders the read-only reconciliation workbench and opens detail', async ({ admin, page }) => {
    await admin.goto('/orders/fulfillment-reconciliation');
    await expect(page.getByText('履约库存对账', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('SO-E2E-RECON-0001')).toBeVisible();
    await expect(page.getByText('已匹配')).toBeVisible();
    await page.getByRole('button', { name: '查看' }).first().click();
    await expect(page.getByRole('dialog')).toContainText('SO-E2E-RECON-0001');
    await expect(page.getByRole('dialog')).toContainText('库存 effect');
    await expectNoRootOverflow(page);
    await expectHeaderContentAligned(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('keeps error state distinct on a readonly account', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['order.view'] })) });
    });
    await page.route('**/api/v1/orders/fulfillment-reconciliation**', async (route) => {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 50301, message: '履约对账服务暂不可用', data: null }) });
    });
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503/);
    await admin.goto('/orders/fulfillment-reconciliation');
    await expect(page.getByText('当前账号为只读模式')).toBeVisible();
    await expect(page.getByRole('alert').filter({ hasText: '履约对账服务暂不可用' })).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('supports a deep link to the reconciliation drawer', async ({ admin, page }) => {
    await admin.goto(`/orders/fulfillment-reconciliation?drawer=fulfillment-reconciliation&id=${E2E_RECONCILIATION_ORDER_ID}`);
    await expect(page.getByRole('dialog')).toContainText('SO-E2E-RECON-0001');
    await expect(page).toHaveURL(/drawer=fulfillment-reconciliation/);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
