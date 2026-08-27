import { expect, test } from '../fixtures/admin.fixture';

test.describe('@smoke warehouse placements', () => {
  test('connects each modal form before initializing it', async ({ admin, page }) => {
    await admin.goto('/inventory/warehouse-placements');

    await expect(page.getByText('库位与条码', { exact: true }).first()).toBeVisible();

    await page.getByRole('button', { name: '新增库位' }).click();
    const locationDialog = page.getByRole('dialog', { name: '新增库位' });
    await expect(locationDialog).toBeVisible();
    await locationDialog.getByRole('button', { name: /取\s*消/ }).click();

    await page.getByRole('button', { name: '新增绑定' }).click();
    const placementDialog = page.getByRole('dialog', { name: '新增 SKU 绑定' });
    await expect(placementDialog).toBeVisible();
    await placementDialog.getByRole('button', { name: /取\s*消/ }).click();

    await admin.consoleGuard.expectNoFatalErrors();
  });
});
