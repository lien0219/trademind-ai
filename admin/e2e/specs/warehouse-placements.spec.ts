import { expect, test } from '../fixtures/admin.fixture';
import { expectNoRootOverflow, expectTableFilterBarAlignedLeft } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke warehouse placements', () => {
  for (const viewport of viewports) {
    test(`keeps filters left and connects modal forms at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/warehouse-placements');

      await expect(page.getByText('库位与条码', { exact: true }).first()).toBeVisible();
      await expectTableFilterBarAlignedLeft(page);
      await expectNoRootOverflow(page);

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
  }
});
