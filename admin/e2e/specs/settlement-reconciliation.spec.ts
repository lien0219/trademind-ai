import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import { E2E_SETTLEMENT_ID } from '../mocks/settlement';
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
  'external_transaction_id,order_no,currency,order_gross_minor,platform_fee_minor,settlement_amount_minor,settled_at',
  'SETTLEMENT-E2E-0001,SO-E2E-PROFIT-0001,CNY,10000,1000,9000,2026-09-05T05:00:00Z',
].join('\n');

test.describe('@smoke settlement reconciliation', () => {
  for (const viewport of viewports) {
    test(`renders matched settlement facts at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/finance/settlement-reconciliation');
      await expect(page.getByText('平台结算对账', { exact: true }).first()).toBeVisible();
      await expect(page.getByText('SO-E2E-PROFIT-0001', { exact: true })).toBeVisible();
      await expect(page.getByText('CNY 10.00', { exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expectTableFilterBarAlignedLeft(page);

      await page.getByRole('button', { name: '查看' }).first().click();
      const drawer = page.getByRole('dialog');
      await expect(drawer).toContainText('SETTLEMENT-E2E-0001');
      await expect(drawer).toContainText('CNY 90.00');
      await expectDrawerWithinViewport(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels import without any POST request', async ({ admin, page }) => {
    await admin.goto('/finance/settlement-reconciliation');
    await page.getByRole('button', { name: '导入账单' }).click();
    await page.getByRole('button', { name: '取消' }).click();
    await admin.writeGuard.expectRequestCount('settlement-preview', 0);
    await admin.writeGuard.expectRequestCount('settlement-confirm', 0);
  });

  test('previews before one guarded confirmation request', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'settlement-preview',
      method: 'POST',
      path: /^\/api\/v1\/settlement-imports\/preview$/,
      response: ok({
        fileName: 'bill.csv',
        fileHash: 'a'.repeat(64),
        shopId: 'e2e-shop-douyin',
        shopName: 'E2E 抖店旗舰店',
        platform: 'douyin_shop',
        valid: true,
        sourceRows: 1,
        newRows: 1,
        duplicateRows: 0,
        issues: [],
        rows: [{
          line: 2,
          externalTransactionId: 'SETTLEMENT-E2E-0001',
          orderNo: 'SO-E2E-PROFIT-0001',
          currency: 'CNY',
          orderGrossMinor: 10000,
          platformFeeMinor: 1000,
          settlementAmountMinor: 9000,
          settledAt: '2026-09-05T05:00:00Z',
          disposition: 'new',
        }],
      }),
    });
    admin.writeGuard.allow({
      operation: 'settlement-confirm',
      method: 'POST',
      path: /^\/api\/v1\/settlement-imports$/,
      response: ok({ import: { id: 'e2e-import', importedRows: 1, duplicateRows: 0 }, replayed: false }),
    });
    await admin.goto('/finance/settlement-reconciliation');
    await page.getByRole('button', { name: '导入账单' }).click();
    await page.getByLabel('账单店铺').click();
    await page.getByRole('option').first().click();
    await page.locator('input[type="file"]').setInputFiles({ name: 'bill.csv', mimeType: 'text/csv', buffer: Buffer.from(csv) });
    await page.getByRole('button', { name: '校验预览' }).click();
    await expect(page.getByText('账单校验通过')).toBeVisible();
    const confirm = page.getByRole('button', { name: '确认导入' });
    await confirm.evaluate((element) => { (element as HTMLButtonElement).click(); (element as HTMLButtonElement).click(); });
    await expect(page.getByText('已导入 1 条结算交易')).toBeVisible();
    await admin.writeGuard.expectRequestCount('settlement-preview', 1);
    await admin.writeGuard.expectRequestCount('settlement-confirm', 1);
  });

  test('keeps import and export unavailable to readonly viewers', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['settlement.view'] })) });
    });
    await admin.goto('/finance/settlement-reconciliation');
    await expect(page.getByRole('button', { name: '导入账单' })).toBeDisabled();
    await expect(page.getByRole('button', { name: '导出明细' })).toBeDisabled();
    await expect(page.getByText('SO-E2E-PROFIT-0001', { exact: true })).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('restores a direct detail link without writes', async ({ admin, page }) => {
    await admin.goto(`/finance/settlement-reconciliation?drawer=settlement-reconciliation&id=${E2E_SETTLEMENT_ID}`);
    await expect(page.getByRole('dialog')).toContainText('SETTLEMENT-E2E-0001');
    await expect(page).toHaveURL(/drawer=settlement-reconciliation/);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
