import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import {
  E2E_WAREHOUSE_FEE_RATE_ID,
  E2E_WAREHOUSE_FEE_SNAPSHOT_ID,
  e2eWarehouseFeeCandidate,
  e2eWarehouseFeeDetail,
  e2eWarehouseFeePreview,
  e2eWarehouseFeeRate,
  e2eWarehouseFeeSnapshot,
} from '../mocks/warehouse-fees';
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

test.describe('@smoke warehouse operation fee ledger', () => {
  for (const viewport of viewports) {
    test(`renders fee candidates, ledger, and rates at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/finance/warehouse-fees');
      await expect(page.getByText('仓库操作费台账', { exact: true }).first()).toBeVisible();
      await expect(page.getByText('SO-E2E-PROFIT-0001', { exact: true }).first()).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await expectTableFilterBarAlignedLeft(page);

      await page.getByRole('tab', { name: '费用台账' }).click();
      await expect(page.getByText('CNY 1.90', { exact: true })).toBeVisible();
      await page.getByRole('button', { name: '查看' }).first().click();
      await expect(page.getByRole('dialog')).toContainText('加固包装材料补收');
      await expect(page.locator('.ant-drawer-content-wrapper:visible').first()).toHaveCSS(
        'transform',
        'none',
      );
      await expectDrawerWithinViewport(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('cancels preview without any request', async ({ admin, page }) => {
    await admin.goto('/finance/warehouse-fees');
    await page.getByRole('button', { name: '预览' }).click();
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('warehouse-fee-preview', 0);
    await admin.writeGuard.expectRequestCount('warehouse-fee-confirm', 0);
  });

  test('previews before one idempotent confirmation', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'warehouse-fee-preview',
      method: 'POST',
      path: /^\/api\/v1\/warehouse-operation-fees\/preview$/,
      response: ok(e2eWarehouseFeePreview),
    });
    admin.writeGuard.allow({
      operation: 'warehouse-fee-confirm',
      method: 'POST',
      path: /^\/api\/v1\/warehouse-operation-fees$/,
      response: ok({ snapshot: e2eWarehouseFeeSnapshot, replayed: false }),
    });
    await admin.goto('/finance/warehouse-fees');
    await page.getByRole('button', { name: '预览' }).click();
    await expect(page.getByText('WH-EAST-STD · 华东仓标准操作费 · CNY', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '生成预览' }).click();
    await expect(page.getByText('CNY 1.70', { exact: true })).toBeVisible();
    const confirm = page.getByRole('button', { name: '确认登记' });
    await confirm.evaluate((element) => { (element as HTMLButtonElement).click(); (element as HTMLButtonElement).click(); });
    await expect(page.getByText('仓库操作费已确认')).toBeVisible();
    await admin.writeGuard.expectRequestCount('warehouse-fee-preview', 1);
    await admin.writeGuard.expectRequestCount('warehouse-fee-confirm', 1);
    const payload = admin.writeGuard.calls('warehouse-fee-confirm')[0].postDataJSON as Record<string, unknown>;
    expect(payload.rateCardId).toBe(E2E_WAREHOUSE_FEE_RATE_ID);
    expect(payload.calculationHash).toBe('a'.repeat(64));
    expect(String(payload.idempotencyKey)).toMatch(/^admin-warehouse-fee-confirm-/);
  });

  test('keeps all fee mutations unavailable to readonly viewers', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['warehouse_fee.view'] })),
      });
    });
    await admin.goto('/finance/warehouse-fees');
    await expect(page.getByRole('button', { name: '预览' })).toBeDisabled();
    await page.getByRole('tab', { name: '费率卡' }).click();
    await expect(page.getByRole('button', { name: '新增费率卡' })).toBeDisabled();
    await expect(page.getByRole('button', { name: '修订' })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('creates and revises rate cards with one guarded request per action', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'warehouse-fee-rate-create',
      method: 'POST',
      path: /^\/api\/v1\/warehouse-fee-rate-cards$/,
      response: ok(e2eWarehouseFeeRate),
    });
    admin.writeGuard.allow({
      operation: 'warehouse-fee-rate-update',
      method: 'PUT',
      path: new RegExp(`^/api/v1/warehouse-fee-rate-cards/${E2E_WAREHOUSE_FEE_RATE_ID}$`),
      response: ok({ ...e2eWarehouseFeeRate, name: '华东仓标准操作费修订', revision: 3 }),
    });
    await admin.goto('/finance/warehouse-fees');
    await page.getByRole('tab', { name: '费率卡' }).click();
    await page.getByRole('button', { name: '新增费率卡' }).click();
    const createDialog = page.getByRole('dialog');
    await createDialog.getByRole('combobox', { name: '适用仓库' }).click();
    await page.locator('.ant-select-dropdown:visible').getByText('MAIN · E2E 华东主仓', { exact: true }).click();
    await createDialog.getByRole('textbox', { name: '费率编码' }).fill('WH-EAST-NEW');
    await createDialog.getByRole('textbox', { name: '费率名称' }).fill('华东仓新费率');
    await createDialog.getByRole('spinbutton', { name: /每单出库基础费/ }).fill('120');
    await createDialog.getByRole('spinbutton', { name: /每件拣货费/ }).fill('25');
    await createDialog.getByRole('spinbutton', { name: /每包裹打包费/ }).fill('35');
    const create = createDialog.getByRole('button', { name: /创\s*建$/ });
    await create.evaluate((element) => { (element as HTMLButtonElement).click(); (element as HTMLButtonElement).click(); });
    await expect(page.getByText('费率卡已创建')).toBeVisible();
    await admin.writeGuard.expectRequestCount('warehouse-fee-rate-create', 1);
    expect(admin.writeGuard.calls('warehouse-fee-rate-create')[0].postDataJSON).toMatchObject({
      warehouseId: 'e2e-warehouse-main',
      code: 'WH-EAST-NEW',
      outboundBaseFeeMinor: 120,
      pickingFeePerItemMinor: 25,
      packingFeePerPackageMinor: 35,
    });

    await page.getByRole('button', { name: '修订' }).click();
    const updateDialog = page.getByRole('dialog');
    await updateDialog.getByRole('textbox', { name: '费率名称' }).fill('华东仓标准操作费修订');
    const update = updateDialog.getByRole('button', { name: '生成新修订' });
    await update.evaluate((element) => { (element as HTMLButtonElement).click(); (element as HTMLButtonElement).click(); });
    await expect(page.getByText('费率卡已生成新修订')).toBeVisible();
    await admin.writeGuard.expectRequestCount('warehouse-fee-rate-update', 1);
    expect(admin.writeGuard.calls('warehouse-fee-rate-update')[0].postDataJSON).toMatchObject({
      expectedRevision: 2,
      name: '华东仓标准操作费修订',
      status: 'active',
    });
  });

  test('appends one adjustment and one one-time reversal from a direct detail link', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'warehouse-fee-adjust',
      method: 'POST',
      path: new RegExp(`^/api/v1/warehouse-operation-fees/${E2E_WAREHOUSE_FEE_SNAPSHOT_ID}/adjustments$`),
      response: ok({ adjustment: e2eWarehouseFeeDetail.adjustments[0], replayed: false }),
    });
    admin.writeGuard.allow({
      operation: 'warehouse-fee-reverse',
      method: 'POST',
      path: new RegExp(`^/api/v1/warehouse-operation-fees/${E2E_WAREHOUSE_FEE_SNAPSHOT_ID}/adjustments/e2e-warehouse-fee-adjustment-1/reverse$`),
      response: ok({ adjustment: { ...e2eWarehouseFeeDetail.adjustments[0], id: 'e2e-warehouse-fee-reversal-1', factType: 'reversal', amountMinor: -20, reversesAdjustmentId: 'e2e-warehouse-fee-adjustment-1' }, replayed: false }),
    });
    await admin.goto(`/finance/warehouse-fees?drawer=warehouse-fee&id=${E2E_WAREHOUSE_FEE_SNAPSHOT_ID}`);
    await expect(page.getByRole('dialog')).toContainText('加固包装材料补收');
    await page.getByRole('button', { name: '追加调整' }).click();
    const adjustmentDialog = page.getByRole('dialog').last();
    await adjustmentDialog.getByRole('spinbutton', { name: /调整金额/ }).fill('30');
    await adjustmentDialog.getByRole('textbox', { name: '原因' }).fill('E2E 人工补收');
    const adjust = adjustmentDialog.getByRole('button', { name: '确认追加' });
    await adjust.evaluate((element) => { (element as HTMLButtonElement).click(); (element as HTMLButtonElement).click(); });
    await expect(page.getByText('费用调整已追加')).toBeVisible();
    await admin.writeGuard.expectRequestCount('warehouse-fee-adjust', 1);
    expect(admin.writeGuard.calls('warehouse-fee-adjust')[0].postDataJSON).toMatchObject({ amountMinor: 30, reason: 'E2E 人工补收' });

    await page.getByRole('button', { name: /冲\s*正$/ }).click();
    const reversalDialog = page.getByRole('dialog').last();
    await reversalDialog.getByRole('textbox', { name: '原因' }).fill('E2E 撤销误收');
    const reverse = reversalDialog.getByRole('button', { name: '确认冲正' });
    await reverse.evaluate((element) => { (element as HTMLButtonElement).click(); (element as HTMLButtonElement).click(); });
    await expect(page.getByText('调整事实已冲正')).toBeVisible();
    await admin.writeGuard.expectRequestCount('warehouse-fee-reverse', 1);
    expect(admin.writeGuard.calls('warehouse-fee-reverse')[0].postDataJSON).toMatchObject({ reason: 'E2E 撤销误收' });
    await expect(page).toHaveURL(/drawer=warehouse-fee/);
  });

  test('reuses the adjustment idempotency key after a recoverable failure', async ({ admin, page }) => {
    let attempts = 0;
    admin.writeGuard.allow({
      operation: 'warehouse-fee-adjust-retry',
      method: 'POST',
      path: new RegExp(`^/api/v1/warehouse-operation-fees/${E2E_WAREHOUSE_FEE_SNAPSHOT_ID}/adjustments$`),
      response: () => {
        attempts += 1;
        if (attempts === 1) return { code: 50301, message: 'E2E 调整暂未确认', data: null };
        return ok({ adjustment: e2eWarehouseFeeDetail.adjustments[0], replayed: true });
      },
    });
    await admin.goto(`/finance/warehouse-fees?drawer=warehouse-fee&id=${E2E_WAREHOUSE_FEE_SNAPSHOT_ID}`);
    await page.getByRole('button', { name: '追加调整' }).click();
    const dialog = page.getByRole('dialog').last();
    await dialog.getByRole('spinbutton', { name: /调整金额/ }).fill('30');
    await dialog.getByRole('textbox', { name: '原因' }).fill('E2E 网络结果待确认');
    await dialog.getByRole('button', { name: '确认追加' }).click();
    await expect(page.getByText('E2E 调整暂未确认')).toBeVisible();
    await dialog.getByRole('button', { name: '确认追加' }).click();
    await expect(page.getByText('费用调整已追加')).toBeVisible();

    await admin.writeGuard.expectRequestCount('warehouse-fee-adjust-retry', 2);
    const calls = admin.writeGuard.calls('warehouse-fee-adjust-retry');
    const first = calls[0].postDataJSON as Record<string, unknown>;
    const second = calls[1].postDataJSON as Record<string, unknown>;
    expect(first.idempotencyKey).toBe(second.idempotencyKey);
    expect(String(first.idempotencyKey)).toMatch(/^admin-warehouse-fee-adjust-/);
  });

  test('distinguishes loading, empty, and request error states without writes', async ({ admin, page }) => {
    let candidateMode: 'loading' | 'normal' | 'error' = 'loading';
    let failRateLoad = false;
    await page.route('**/api/v1/warehouse-operation-fees/candidates*', async (route) => {
      if (candidateMode === 'loading') {
        await new Promise((resolve) => setTimeout(resolve, 600));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(ok({ list: [], page: 1, pageSize: 20, total: 0, totalPages: 0 })),
        });
        return;
      }
      if (candidateMode === 'error') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ code: 50301, message: 'E2E 待登记费用不可用', data: null }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [e2eWarehouseFeeCandidate], page: 1, pageSize: 20, total: 1, totalPages: 1 })),
      });
    });
    await page.route('**/api/v1/warehouse-fee-rate-cards*', async (route) => {
      if (!failRateLoad) {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 50301, message: 'E2E 费率卡不可用', data: null }),
      });
    });

    await admin.goto('/finance/warehouse-fees');
    await expect(page.locator('.ant-spin-spinning')).toBeVisible();
    await expect(page.getByText('暂无可登记的履约订单')).toBeVisible();

    candidateMode = 'normal';
    await page.getByRole('button', { name: /刷新/ }).filter({ hasText: '刷新' }).click();
    await expect(page.getByText(e2eWarehouseFeeCandidate.orderNo, { exact: true }).first()).toBeVisible();
    failRateLoad = true;
    await page.getByRole('button', { name: '预览' }).click();
    await expect(page.getByRole('alert').filter({ hasText: '适用费率卡加载失败' })).toContainText('E2E 费率卡不可用');
    await page.getByRole('dialog').getByRole('button', { name: /取\s*消/ }).click();

    candidateMode = 'error';
    await page.getByRole('button', { name: /刷新/ }).filter({ hasText: '刷新' }).click();
    await expect(page.getByRole('alert').filter({ hasText: 'E2E 待登记费用不可用' })).toBeVisible();
    await expect(page.getByText(e2eWarehouseFeeCandidate.orderNo, { exact: true }).first()).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
