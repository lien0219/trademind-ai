import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import {
  E2E_FREIGHT_FEE_ADJUSTMENT_ID,
  E2E_FREIGHT_FEE_CHARGE_ID,
  e2eFreightFeeCharge,
  e2eFreightFeeDetail,
  e2eFreightFeePreview,
} from '../mocks/freight-fees';
import {
  expectDrawerWithinViewport,
  expectHeaderContentAligned,
  expectNoRootOverflow,
} from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

const csv = [
  'external_line_id,carrier,tracking_no,amount_minor,currency,billed_at',
  'FREIGHT-E2E-0001,Carrier A,TRACK-E2E-FREIGHT-1,500,CNY,2026-09-28T01:00:00Z',
].join('\n');

test.describe('@smoke carrier freight fee ledger', () => {
  for (const viewport of viewports) {
    test(`renders freight facts and a bounded detail drawer at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto(`/finance/freight-fees?drawer=freight-fee&id=${E2E_FREIGHT_FEE_CHARGE_ID}`);
      await expect(page.getByText('承运商运费账单', { exact: true }).first()).toBeVisible();
      await expect(page.getByText(e2eFreightFeeCharge.orderNo, { exact: true }).first()).toBeVisible();
      await expect(page.getByRole('table').first().getByRole('cell', { name: 'CNY 5.25', exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expect(page.getByRole('dialog')).toContainText('TRACK-E2E-FREIGHT-1');
      await expectDrawerWithinViewport(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels import without sending a request', async ({ admin, page }) => {
    await admin.goto('/finance/freight-fees');
    await page.getByRole('button', { name: '导入账单' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('freight-fee-preview', 0);
    await admin.writeGuard.expectRequestCount('freight-fee-confirm', 0);
  });

  test('previews before one guarded import confirmation', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'freight-fee-preview',
      method: 'POST',
      path: /^\/api\/v1\/freight-fee-imports\/preview$/,
      response: ok(e2eFreightFeePreview),
    });
    admin.writeGuard.allow({
      operation: 'freight-fee-confirm',
      method: 'POST',
      path: /^\/api\/v1\/freight-fee-imports$/,
      response: ok({ import: { id: 'e2e-freight-fee-import-confirmed', importedRows: 1, duplicateRows: 0 }, replayed: false }),
    });

    await admin.goto('/finance/freight-fees');
    await page.getByRole('button', { name: '导入账单' }).click();
    const dialog = page.getByRole('dialog').first();
    const shopSelect = dialog.getByRole('combobox');
    await shopSelect.click();
    await shopSelect.press('ArrowDown');
    await shopSelect.press('Enter');
    await dialog.locator('input[type="file"]').setInputFiles({ name: 'freight.csv', mimeType: 'text/csv', buffer: Buffer.from(csv) });
    await dialog.getByRole('button', { name: '校验预览' }).click();
    await expect(dialog.getByText(/校验通过/)).toBeVisible();
    await expect(dialog.getByText('TRACK-E2E-FREIGHT-1')).toBeVisible();
    await dialog.getByRole('button', { name: '确认导入' }).click();
    const confirmation = page.getByRole('dialog').last();
    await expect(confirmation).toContainText('确认导入这批运费账单');
    await confirmation.getByRole('button').last().click();
    await expect(page.getByText('运费账单已确认')).toBeVisible();
    await admin.writeGuard.expectRequestCount('freight-fee-preview', 1);
    await admin.writeGuard.expectRequestCount('freight-fee-confirm', 1);
  });

  test('appends a correction and one-time reversal from a deep-linked detail', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'freight-fee-adjust',
      method: 'POST',
      path: new RegExp(`^/api/v1/freight-fees/${E2E_FREIGHT_FEE_CHARGE_ID}/adjustments$`),
      response: ok({ adjustment: e2eFreightFeeDetail.adjustments[0], replayed: false }),
    });
    admin.writeGuard.allow({
      operation: 'freight-fee-reverse',
      method: 'POST',
      path: new RegExp(`^/api/v1/freight-fees/${E2E_FREIGHT_FEE_CHARGE_ID}/adjustments/${E2E_FREIGHT_FEE_ADJUSTMENT_ID}/reverse$`),
      response: ok({
        adjustment: {
          ...e2eFreightFeeDetail.adjustments[0],
          id: 'e2e-freight-fee-reversal-1',
          factType: 'reversal',
          amountMinor: -25,
          reversesAdjustmentId: E2E_FREIGHT_FEE_ADJUSTMENT_ID,
        },
        replayed: false,
      }),
    });

    await admin.goto(`/finance/freight-fees?drawer=freight-fee&id=${E2E_FREIGHT_FEE_CHARGE_ID}`);
    await page.getByRole('button', { name: '追加调整' }).click();
    const adjustmentDialog = page.getByRole('dialog', { name: '追加运费调整' });
    await adjustmentDialog.getByRole('spinbutton', { name: /调整金额/ }).fill('25');
    await adjustmentDialog.getByRole('textbox', { name: '更正原因' }).fill('E2E 修正包裹附加费');
    await adjustmentDialog.getByRole('button').last().click();
    await expect(page.getByText('调整已追加')).toBeVisible();
    await admin.writeGuard.expectRequestCount('freight-fee-adjust', 1);
    expect(admin.writeGuard.calls('freight-fee-adjust')[0].postDataJSON).toMatchObject({
      amountMinor: 25,
      reason: 'E2E 修正包裹附加费',
    });

    await page.getByRole('button', { name: /冲\s*正$/ }).click();
    const reversalDialog = page.getByRole('dialog', { name: '冲正调整' });
    await reversalDialog.getByRole('textbox', { name: '更正原因' }).fill('E2E 撤销错误调整');
    await reversalDialog.getByRole('button').last().click();
    await expect(page.getByText('冲正已追加')).toBeVisible();
    await admin.writeGuard.expectRequestCount('freight-fee-reverse', 1);
    expect(admin.writeGuard.calls('freight-fee-reverse')[0].postDataJSON).toMatchObject({
      reason: 'E2E 撤销错误调整',
    });
    await expect(page).toHaveURL(/drawer=freight-fee/);
  });

  test('keeps import and correction controls unavailable to readonly viewers', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['freight_fee.view'] })),
      });
    });
    await admin.goto(`/finance/freight-fees?drawer=freight-fee&id=${E2E_FREIGHT_FEE_CHARGE_ID}`);
    await expect(page.getByRole('button', { name: '导入账单' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '追加调整' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /冲\s*正$/ })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('distinguishes empty results from a failed list request', async ({ admin, page }) => {
    let mode: 'empty' | 'error' = 'empty';
    await page.route('**/api/v1/freight-fees?*', async (route) => {
      if (mode === 'error') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ code: 50301, message: 'E2E 运费账单不可用', data: null }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 })),
      });
    });

    await admin.goto('/finance/freight-fees');
    await expect(page.getByText('暂无运费账单')).toBeVisible();
    mode = 'error';
    await page.reload();
    await expect(page.getByRole('alert').filter({ hasText: 'E2E 运费账单不可用' })).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
