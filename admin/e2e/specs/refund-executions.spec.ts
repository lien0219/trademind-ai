import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import {
  E2E_REFUND_EXECUTION_ID,
  E2E_SALES_RETURN_ID,
  e2eRefundExecution,
  e2eSalesReturn,
} from '../mocks/sales-returns';
import {
  expectHeaderActionsSpaced,
  expectHeaderContentAligned,
  expectModalWithinViewport,
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

test.describe('@smoke refund execution workspace', () => {
  for (const viewport of viewports) {
    test(`renders refund execution at ${viewport.width}x${viewport.height} without writes`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/orders/refund-executions');
      await expect(page.getByText('RF-E2E-0001').first()).toBeVisible({
        timeout: 30_000,
      });
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expectHeaderActionsSpaced(page);
      await expectTableFilterBarAlignedLeft(page);

      await admin.goto(`/orders/refund-executions/${E2E_REFUND_EXECUTION_ID}`);
      await expect(page.getByText('外部退款编号').first()).toBeVisible();
      await expect(page.getByText('本页面只登记外部结果或确认只读平台事实').first()).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels manual result without a write and records one exact result', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'record-refund-result',
      method: 'POST',
      path: new RegExp(
        `^/api/v1/refund-executions/${E2E_REFUND_EXECUTION_ID}/result$`,
      ),
      response: ok({
        ...e2eRefundExecution,
        status: 'succeeded',
        source: 'manual',
        revision: 2,
        externalRefundId: 'refund-e2e-external-1',
        executedAt: '2026-08-24T08:00:00Z',
      }),
    });

    await admin.goto(`/orders/refund-executions/${E2E_REFUND_EXECUTION_ID}`);
    await page.getByRole('button', { name: '人工登记结果' }).click();
    let dialog = page.getByRole('dialog', { name: '人工登记退款结果' });
    await expectModalWithinViewport(page);
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('record-refund-result', 0);

    await page.getByRole('button', { name: '人工登记结果' }).click();
    dialog = page.getByRole('dialog', { name: '人工登记退款结果' });
    await dialog.getByLabel('外部退款编号').fill('refund-e2e-external-1');
    await dialog.getByLabel('结果说明').fill('已在支付渠道完成退款');
    await dialog.getByRole('button', { name: '确认登记' }).click();

    await admin.writeGuard.expectRequestCount('record-refund-result', 1);
    const payload = admin.writeGuard.calls('record-refund-result')[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      expectedRevision: 1,
      result: 'succeeded',
      externalRefundId: 'refund-e2e-external-1',
      reason: '已在支付渠道完成退款',
    });
    expect(String(payload.idempotencyKey)).toMatch(/^admin-refund-result-/);
    expect(String(payload.executedAt)).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });

  test('cancels creation without a write and creates one pending execution', async ({
    admin,
    page,
  }) => {
    await page.route(
      `**/api/v1/sales-returns/${E2E_SALES_RETURN_ID}`,
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(
            ok({
              ...e2eSalesReturn,
              status: 'completed',
              revision: 4,
              completedAt: '2026-08-22T03:00:00Z',
            }),
          ),
        });
      },
    );
    await page.route('**/api/v1/refund-executions**', async (route) => {
      if (new URL(route.request().url()).pathname !== '/api/v1/refund-executions') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({ list: [], page: 1, pageSize: 1, total: 0, totalPages: 0 }),
        ),
      });
    });
    admin.writeGuard.allow({
      operation: 'create-refund-execution',
      method: 'POST',
      path: new RegExp(
        `^/api/v1/sales-returns/${E2E_SALES_RETURN_ID}/refund-execution$`,
      ),
      response: ok(e2eRefundExecution),
    });

    await admin.goto(`/orders/sales-returns/${E2E_SALES_RETURN_ID}`);
    await page.getByRole('button', { name: '创建退款执行单' }).click();
    let dialog = page.getByRole('dialog', { name: '创建退款执行单' });
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('create-refund-execution', 0);

    await page.getByRole('button', { name: '创建退款执行单' }).click();
    dialog = page.getByRole('dialog', { name: '创建退款执行单' });
    await dialog.getByRole('button', { name: '确认创建' }).click();

    await admin.writeGuard.expectRequestCount('create-refund-execution', 1);
    const payload = admin.writeGuard.calls('create-refund-execution')[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-refund-execution-create-/,
    );
    await expect(page).toHaveURL(
      new RegExp(`/orders/refund-executions/${E2E_REFUND_EXECUTION_ID}$`),
    );
  });

  test('cancels platform confirmation without a write and confirms exactly once', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'confirm-refund-platform',
      method: 'POST',
      path: new RegExp(
        `^/api/v1/refund-executions/${E2E_REFUND_EXECUTION_ID}/confirm-from-platform$`,
      ),
      response: ok({
        ...e2eRefundExecution,
        status: 'succeeded',
        source: 'platform_fact',
        revision: 2,
        externalRefundId: 'e2e-after-sale-1',
      }),
    });

    await admin.goto(`/orders/refund-executions/${E2E_REFUND_EXECUTION_ID}`);
    await page.getByRole('button', { name: '按平台事实确认' }).click();
    let dialog = page.getByRole('dialog', { name: '按平台事实确认' });
    await dialog.getByRole('button', { name: /返\s*回/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('confirm-refund-platform', 0);

    await page.getByRole('button', { name: '按平台事实确认' }).click();
    dialog = page.getByRole('dialog', { name: '按平台事实确认' });
    await dialog.locator('textarea').fill('平台退款终态已核对');
    await dialog.getByRole('button', { name: '确认平台结果' }).click();

    await admin.writeGuard.expectRequestCount('confirm-refund-platform', 1);
    const payload = admin.writeGuard.calls('confirm-refund-platform')[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      expectedRevision: 1,
      platformAfterSaleId: 'e2e-platform-after-sale-1',
      reason: '平台退款终态已核对',
    });
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-refund-confirm_platform-/,
    );
  });

  test('shows readonly state without refund actions', async ({ admin, page }) => {
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
    await admin.goto(`/orders/refund-executions/${E2E_REFUND_EXECUTION_ID}`);
    await expect(page.getByText(/只读模式/)).toBeVisible();
    await expect(page.getByRole('button', { name: '人工登记结果' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '按平台事实确认' })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
