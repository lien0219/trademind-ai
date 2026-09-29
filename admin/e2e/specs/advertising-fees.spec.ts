import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import {
  E2E_ADVERTISING_FEE_ADJUSTMENT_ID,
  E2E_ADVERTISING_FEE_ALLOCATION_ID,
  e2eAdvertisingFeeAllocation,
  e2eAdvertisingFeeDetail,
  e2eAdvertisingFeePreview,
} from '../mocks/advertising-fees';
import { ok } from '../mocks/envelope';
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

const csv = [
  'spend_date,currency,spend_minor,settlement_coverage',
  '2026-09-05,CNY,600,excluded',
].join('\n');

test.describe('@smoke advertising fee attribution ledger', () => {
  for (const viewport of viewports) {
    test(`renders attributed advertising fees at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/finance/advertising-fees');
      await expect(page.getByText('广告费用归属', { exact: true }).first()).toBeVisible();
      await expect(page.getByText('订单归属是运营估算事实')).toBeVisible();
      await expect(page.getByText(e2eAdvertisingFeeAllocation.orderNo, { exact: true })).toBeVisible();
      await expect(page.getByText('CNY 3.00', { exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expectTableFilterBarAlignedLeft(page);

      await page.getByRole('button', { name: '查看' }).first().click();
      const drawer = page.getByRole('dialog');
      await expect(drawer).toContainText('补录站内推广消耗');
      await expect(drawer.getByRole('button', { name: '查看预估利润' })).toBeVisible();
      await expect(page.locator('.ant-drawer-content-wrapper:visible').first()).toHaveCSS('transform', 'none');
      await expectDrawerWithinViewport(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels import without any request', async ({ admin, page }) => {
    await admin.goto('/finance/advertising-fees');
    await page.getByRole('button', { name: '导入费用' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('advertising-fee-preview', 0);
    await admin.writeGuard.expectRequestCount('advertising-fee-confirm', 0);
  });

  test('previews once before one idempotent confirmation', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'advertising-fee-preview',
      method: 'POST',
      path: /^\/api\/v1\/advertising-fee-imports\/preview$/,
      response: ok(e2eAdvertisingFeePreview),
    });
    admin.writeGuard.allow({
      operation: 'advertising-fee-confirm',
      method: 'POST',
      path: /^\/api\/v1\/advertising-fee-imports$/,
      response: ok({
        import: {
          id: 'e2e-advertising-fee-import-1',
          importedSpends: 1,
          duplicateSpends: 0,
          allocationCount: 2,
        },
        replayed: false,
      }),
    });

    await admin.goto('/finance/advertising-fees');
    await page.getByRole('button', { name: '导入费用' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('combobox', { name: '费用店铺' }).click();
    await page.locator('.ant-select-dropdown:visible').getByText('E2E 抖店测试店铺 · 抖店', { exact: true }).click();
    await dialog.locator('input[type="file"]').setInputFiles({ name: 'advertising.csv', mimeType: 'text/csv', buffer: Buffer.from(csv) });
    await dialog.getByRole('button', { name: '校验预览' }).click();
    await expect(dialog.getByText('广告费用校验通过')).toBeVisible();
    await expect(dialog.getByText('SO-E2E-CANCELLED-0001')).toBeVisible();
    const confirm = dialog.getByRole('button', { name: '确认归属' });
    await confirm.evaluate((element) => {
      (element as HTMLButtonElement).click();
      (element as HTMLButtonElement).click();
    });
    await expect(page.getByText('已生成 2 条订单归属')).toBeVisible();
    await admin.writeGuard.expectRequestCount('advertising-fee-preview', 1);
    await admin.writeGuard.expectRequestCount('advertising-fee-confirm', 1);
  });

  test('appends one adjustment and one one-time reversal from a direct detail link', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'advertising-fee-adjust',
      method: 'POST',
      path: new RegExp(`^/api/v1/advertising-fees/${E2E_ADVERTISING_FEE_ALLOCATION_ID}/adjustments$`),
      response: ok({ adjustment: e2eAdvertisingFeeDetail.adjustments[0], replayed: false }),
    });
    admin.writeGuard.allow({
      operation: 'advertising-fee-reverse',
      method: 'POST',
      path: new RegExp(`^/api/v1/advertising-fees/${E2E_ADVERTISING_FEE_ALLOCATION_ID}/adjustments/${E2E_ADVERTISING_FEE_ADJUSTMENT_ID}/reverse$`),
      response: ok({
        adjustment: {
          ...e2eAdvertisingFeeDetail.adjustments[0],
          id: 'e2e-advertising-fee-reversal-1',
          factType: 'reversal',
          amountMinor: -20,
          reversesAdjustmentId: E2E_ADVERTISING_FEE_ADJUSTMENT_ID,
        },
        replayed: false,
      }),
    });

    await admin.goto(`/finance/advertising-fees?drawer=advertising-fee&id=${E2E_ADVERTISING_FEE_ALLOCATION_ID}`);
    await expect(page.getByRole('dialog')).toContainText('补录站内推广消耗');
    await page.getByRole('button', { name: '追加调整' }).click();
    const adjustmentDialog = page.getByRole('dialog').last();
    await adjustmentDialog.getByRole('spinbutton', { name: /调整金额/ }).fill('30');
    await adjustmentDialog.getByRole('textbox', { name: '原因' }).fill('E2E 人工补录广告费用');
    const adjust = adjustmentDialog.getByRole('button', { name: '确认追加' });
    await adjust.evaluate((element) => {
      (element as HTMLButtonElement).click();
      (element as HTMLButtonElement).click();
    });
    await expect(page.getByText('广告费用调整已追加')).toBeVisible();
    await admin.writeGuard.expectRequestCount('advertising-fee-adjust', 1);

    await page.getByRole('button', { name: /冲\s*正$/ }).click();
    const reversalDialog = page.getByRole('dialog').last();
    await reversalDialog.getByRole('textbox', { name: '原因' }).fill('E2E 撤销误录广告费用');
    const reverse = reversalDialog.getByRole('button', { name: '确认冲正' });
    await reverse.evaluate((element) => {
      (element as HTMLButtonElement).click();
      (element as HTMLButtonElement).click();
    });
    await expect(page.getByText('广告费用调整已冲正')).toBeVisible();
    await admin.writeGuard.expectRequestCount('advertising-fee-reverse', 1);
    await expect(page).toHaveURL(/drawer=advertising-fee/);
  });

  test('keeps import and correction unavailable to readonly viewers', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['advertising_fee.view'] })),
      });
    });
    await admin.goto(`/finance/advertising-fees?drawer=advertising-fee&id=${E2E_ADVERTISING_FEE_ALLOCATION_ID}`);
    await expect(page.getByRole('button', { name: '导入费用' })).toBeDisabled();
    await expect(page.getByRole('button', { name: '追加调整' })).toBeDisabled();
    await expect(page.getByRole('button', { name: /冲\s*正$/ })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('distinguishes loading, empty, and request error states without writes', async ({ admin, page }) => {
    let mode: 'empty' | 'error' = 'empty';
    await page.route('**/api/v1/advertising-fees?*', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 600));
      if (mode === 'error') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ code: 50301, message: 'E2E 广告费用归属不可用', data: null }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 })),
      });
    });

    await admin.goto('/finance/advertising-fees');
    await expect(page.locator('.ant-spin-spinning')).toBeVisible();
    await expect(page.getByText('暂无广告费用归属')).toBeVisible();

    mode = 'error';
    await page.reload();
    await expect(page.getByRole('alert').filter({ hasText: 'E2E 广告费用归属不可用' })).toBeVisible();
    await expect(page.getByText('广告费用归属数据暂不可用')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
