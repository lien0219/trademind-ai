import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import {
  E2E_SALES_ORDER_ID,
  E2E_SALES_ORDER_ITEM_ID,
  E2E_SALES_RETURN_ID,
  e2eSalesReturn,
} from '../mocks/sales-returns';
import {
  expectHeaderContentAligned,
  expectModalWithinViewport,
  expectNoRootOverflow,
} from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke sales returns workspace', () => {
  for (const viewport of viewports) {
    test(`renders sales returns at ${viewport.width}x${viewport.height} without writes`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      for (const route of [
        { path: '/orders/sales-returns', text: 'SR-E2E-0001' },
        {
          path: `/orders/sales-returns/${E2E_SALES_RETURN_ID}`,
          text: '客户七天无理由退货',
        },
        { path: `/orders/${E2E_SALES_ORDER_ID}`, text: 'SO-E2E-0001' },
      ]) {
        await admin.goto(route.path);
        await expect(page.getByText(route.text).first()).toBeVisible({
          timeout: 30_000,
        });
        await expectNoRootOverflow(page);
        await expectHeaderContentAligned(page);
      }
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels creation without a write and creates one item-level return draft', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'create-sales-return',
      method: 'POST',
      path: /^\/api\/v1\/sales-returns$/,
      response: ok({ ...e2eSalesReturn, status: 'draft', revision: 1 }),
    });

    await admin.goto(`/orders/${E2E_SALES_ORDER_ID}`);
    await page.getByRole('button', { name: '发起售后' }).click();
    let dialog = page.getByRole('dialog', { name: '发起退货退款' });
    await expectModalWithinViewport(page);
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('create-sales-return', 0);

    await page.getByRole('button', { name: '发起售后' }).click();
    dialog = page.getByRole('dialog', { name: '发起退货退款' });
    await expect(dialog.getByText('E2E 售后蓝牙耳机')).toBeVisible();
    await dialog.getByLabel('售后原因').fill('客户七天无理由退货');
    await dialog.getByLabel('备注').fill('外包装已拆');
    const itemRow = dialog.getByRole('row').nth(1);
    await itemRow.getByRole('spinbutton').nth(0).fill('1');
    await itemRow.getByRole('spinbutton').nth(1).fill('99.99');
    await dialog.getByRole('button', { name: '创建草稿' }).click();

    await admin.writeGuard.expectRequestCount('create-sales-return', 1);
    const payload = admin.writeGuard.calls('create-sales-return')[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      orderId: E2E_SALES_ORDER_ID,
      type: 'return_refund',
      reason: '客户七天无理由退货',
      remark: '外包装已拆',
      items: [
        {
          orderItemId: E2E_SALES_ORDER_ITEM_ID,
          quantity: 1,
          disposition: 'sellable',
          refundAmountMinor: 9999,
        },
      ],
    });
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-sales-return-create-/,
    );
  });

  test('cancels receipt confirmation and completes exactly once', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'complete-sales-return',
      method: 'POST',
      path: new RegExp(
        `^/api/v1/sales-returns/${E2E_SALES_RETURN_ID}/complete$`,
      ),
      response: ok({
        ...e2eSalesReturn,
        status: 'completed',
        revision: 4,
        completedAt: '2026-08-22T03:00:00Z',
      }),
    });

    await admin.goto(`/orders/sales-returns/${E2E_SALES_RETURN_ID}`);
    await page.getByRole('button', { name: '确认退货收货' }).click();
    let dialog = page.getByRole('dialog', { name: '确认退货收货' });
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('complete-sales-return', 0);

    await page.getByRole('button', { name: '确认退货收货' }).click();
    dialog = page.getByRole('dialog', { name: '确认退货收货' });
    await dialog.getByLabel('操作说明').fill('已核对实物');
    await dialog.getByRole('button', { name: '确认退货收货' }).click();

    await admin.writeGuard.expectRequestCount('complete-sales-return', 1);
    const payload = admin.writeGuard.calls('complete-sales-return')[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      expectedRevision: 3,
      reason: '已核对实物',
    });
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-sales-return-complete-/,
    );
  });

  test('distinguishes empty, error, and readonly states', async ({
    admin,
    page,
  }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            ...e2eUser,
            role: 'readonly',
            permissions: ['sales_return.view', 'order.view'],
          }),
        ),
      });
    });
    await page.route('**/api/v1/sales-returns', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({ list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 }),
        ),
      });
    });
    await admin.goto('/orders/sales-returns');
    await expect(page.getByText('暂无售后记录。')).toBeVisible();
    await expect(page.getByRole('button', { name: '选择订单' })).toBeDisabled();
    await expect(page.getByText(/只读模式/)).toBeVisible();

    await page.unroute('**/api/v1/sales-returns');
    await page.route(
      `**/api/v1/sales-returns/${E2E_SALES_RETURN_ID}`,
      async (route) => {
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({
            code: 50000,
            message: 'sales return unavailable',
            data: null,
          }),
        });
      },
    );
    admin.consoleGuard.allowError(
      /Failed to load resource: the server responded with a status of 503/,
    );
    await admin.goto(`/orders/sales-returns/${E2E_SALES_RETURN_ID}`);
    await expect(
      page.getByText('售后详情加载失败，请稍后重试。'),
    ).toBeVisible();
    await expect(page.getByText(/只读模式/)).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
