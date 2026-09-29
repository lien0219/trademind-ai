import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import {
  E2E_FULFILL_ORDER_ID,
  E2E_FULFILL_SHIPMENT_ID,
  e2eFulfillmentOrder,
  e2eFulfillmentShipment,
} from '../mocks/order-fulfillment';
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

  test('renders the shipment timeline and records one manual event', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'append-shipment-event',
      method: 'POST',
      path: new RegExp(
        `^/api/v1/orders/${E2E_FULFILL_ORDER_ID}/shipments/${E2E_FULFILL_SHIPMENT_ID}/events$`,
      ),
      response: ok({
        event: {
          id: 'e2e-shipment-event-manual',
          tenantId: 1,
          orderId: E2E_FULFILL_ORDER_ID,
          shipmentId: E2E_FULFILL_SHIPMENT_ID,
          eventKey: 'manual-e2e-event-1',
          status: 'in_transit',
          occurredAt: '2026-08-23T03:00:00Z',
          location: '广州中转站',
          description: '人工补录轨迹',
          source: 'manual',
        },
        shipment: { ...e2eFulfillmentShipment, status: 'in_transit' },
        replay: false,
      }),
    });

    await admin.goto(`/orders/${E2E_FULFILL_ORDER_ID}`);
    await page.getByRole('tab', { name: '物流轨迹' }).click();
    await expect(page.getByText('地点：深圳转运中心', { exact: true })).toBeVisible();
    await expect(page.getByText('物流轨迹仅记录本地履约事实')).toBeVisible();

    await page.getByRole('button', { name: '录入物流事件' }).click();
    const dialog = page.getByRole('dialog', { name: '录入物流事件' });
    await dialog.getByLabel('事件键').fill('manual-e2e-event-1');
    await dialog.getByLabel('地点').fill('广州中转站');
    await dialog.getByLabel('描述').fill('人工补录轨迹');
    await dialog.locator('.ant-modal-footer button').last().click();
    await admin.writeGuard.expectRequestCount('append-shipment-event', 1);

    const payload = admin.writeGuard.calls('append-shipment-event')[0]?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      eventKey: 'manual-e2e-event-1',
      status: 'in_transit',
      location: '广州中转站',
      description: '人工补录轨迹',
    });
    expect(String(payload.occurredAt)).toMatch(/T/);
  });

  test('keeps shipment event entry readonly without order.operate', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['order.view'] })),
      });
    });
    await admin.goto(`/orders/${E2E_FULFILL_ORDER_ID}`);
    await page.getByRole('tab', { name: '物流轨迹' }).click();
    await expect(page.getByRole('button', { name: '录入物流事件' })).toBeDisabled();
    await expect(page.getByText('当前账号为只读权限，只能查看物流轨迹。')).toBeVisible();
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
