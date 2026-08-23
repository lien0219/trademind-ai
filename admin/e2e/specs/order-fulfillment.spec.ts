import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { E2E_FULFILL_ORDER_ID, e2eFulfillmentOrder } from '../mocks/order-fulfillment';
import { ok } from '../mocks/envelope';
import { expectHeaderContentAligned, expectModalWithinViewport, expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke order fulfillment V1', () => {
  for (const viewport of viewports) {
    test(`renders the real order detail fulfillment action at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto(`/orders/${E2E_FULFILL_ORDER_ID}`);
      await expect(page.getByText('SO-E2E-FULFILL-0001', { exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: '确认发货' })).toBeEnabled();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels without a write, then submits one idempotent fulfillment request', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'fulfill-order',
      method: 'POST',
      path: new RegExp(`^/api/v1/orders/${E2E_FULFILL_ORDER_ID}/fulfill$`),
      response: ok({
        order: { ...e2eFulfillmentOrder, status: 'shipped', fulfillmentStatus: 'fulfilled' },
        shipment: { id: 'e2e-shipment-1', carrier: '顺丰', trackingNo: 'SF-E2E-1', status: 'shipped' },
        inventoryDeduction: { action: 'deduct', linesSynced: 1 },
      }),
    });

    await admin.goto(`/orders/${E2E_FULFILL_ORDER_ID}`);
    await page.getByRole('button', { name: '确认发货' }).click();
    let dialog = page.getByRole('dialog', { name: '确认发货' });
    await expectModalWithinViewport(page);
    await dialog.locator('.ant-modal-footer button').first().click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('fulfill-order', 0);

    await page.getByRole('button', { name: '确认发货' }).click();
    dialog = page.getByRole('dialog', { name: '确认发货' });
    await dialog.locator('input').nth(0).fill('顺丰');
    await dialog.locator('input').nth(1).fill('SF-E2E-1');
    const submit = dialog.locator('.ant-modal-footer button').last();
    await submit.click();
    await admin.writeGuard.expectRequestCount('fulfill-order', 1);

    const payload = admin.writeGuard.calls('fulfill-order')[0]?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      warehouseId: 'e2e-warehouse-main',
      carrier: '顺丰',
      trackingNo: 'SF-E2E-1',
    });
    expect(String(payload.idempotencyKey)).toMatch(/^admin-order-fulfillment-/);
  });

  test('blocks fulfillment for readonly users', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['order.view'] })),
      });
    });
    await admin.goto(`/orders/${E2E_FULFILL_ORDER_ID}`);
    await expect(page.getByRole('button', { name: '确认发货' })).toBeDisabled();
    await expect(page.getByText('当前账号没有订单操作权限')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('blocks fulfillment when profile permissions omit order.operate', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'admin', permissions: ['order.view'] })),
      });
    });
    await admin.goto(`/orders/${E2E_FULFILL_ORDER_ID}`);
    await expect(page.getByRole('button', { name: '确认发货' })).toBeDisabled();
    await expect(page.getByText('当前账号没有订单操作权限')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
