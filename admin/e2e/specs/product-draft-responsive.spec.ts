import { test, expect } from '../fixtures/admin.fixture';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';
import {
  expectAccountInTopNavbar,
  expectHeaderContentAligned,
  expectNoRootOverflow,
  expectPageChromeScrollbarsHidden,
  expectPageContentGuttersWithin,
  expectTopNavbarScrollBehavior,
} from '../utils/assertions';
import { layoutTokens } from '../../src/constants/layoutTokens';
import { THEME_MODE_STORAGE_KEY } from '../../src/theme/themeMode';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

const pages = [
  {
    path: `/product/drafts/${E2E_PRODUCT_ID}?tab=publish`,
    label: /刊登|商品详情/,
    verifyStickyNavigation: true,
  },
  { path: '/ops/task-center/alerts', label: /告警中心/ },
  { path: '/product/drafts', label: /商品草稿|E2E 商品草稿/ },
];

test.describe('@product-draft @responsive 五档响应式', () => {
  for (const viewport of viewports) {
    for (const target of pages) {
      test(`${target.path} has no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
        await page.setViewportSize(viewport);
        await admin.goto(target.path);
        await expect(page.getByText(target.label).first()).toBeVisible();
        await expectAccountInTopNavbar(page);
        await expectNoRootOverflow(page);
        await expectHeaderContentAligned(page);
        if (target.verifyStickyNavigation) {
          await expectTopNavbarScrollBehavior(page);
        }
        await expect(page.locator('#root')).toBeVisible();
      });
    }
  }

  for (const viewport of viewports) {
    test(`dark theme keeps the shared layout usable at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.addInitScript(
        ({ storageKey }) => window.localStorage.setItem(storageKey, 'dark'),
        { storageKey: THEME_MODE_STORAGE_KEY },
      );
      await admin.goto('/dashboard/product-operations');

      await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
      await expect(page.getByRole('button', { name: '切换到浅色模式' })).toBeVisible();
      await expectAccountInTopNavbar(page);
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
    });
  }

  test('global content track keeps wide-screen gutters compact', async ({ admin, page }) => {
    await page.setViewportSize({ width: 2048, height: 1024 });
    await admin.goto('/dashboard/product-operations');
    await expect(page.getByText(/运营总览|工作台/).first()).toBeVisible();
    await expectNoRootOverflow(page);
    await expectHeaderContentAligned(page);
    await expectPageChromeScrollbarsHidden(page);
    await expectPageContentGuttersWithin(
      page,
      layoutTokens.pageMaxOuterGap + layoutTokens.pagePaddingX,
    );
  });

  for (const viewport of [viewports[0], viewports[4]]) {
    test(`draft detail sections keep visible spacing at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=basic`);

      const progress = page.locator('.product-draft-progress');
      const tabsFrame = page.locator('.product-draft-tabs-frame');
      const tabs = page.locator('.product-draft-tabs');
      const firstSection = page.locator('.product-draft-basic__quality');
      const secondSection = page.locator('.product-draft-basic__source');
      await expect(progress).toBeVisible();
      await expect(tabsFrame).toBeVisible();
      await expect(firstSection).toBeVisible();
      await expect(secondSection).toBeVisible();

      const [progressBox, tabsFrameBox, tabsBox, firstSectionBox, secondSectionBox] = await Promise.all([
        progress.boundingBox(),
        tabsFrame.boundingBox(),
        tabs.boundingBox(),
        firstSection.boundingBox(),
        secondSection.boundingBox(),
      ]);
      if (!progressBox || !tabsFrameBox || !tabsBox || !firstSectionBox || !secondSectionBox) {
        throw new Error('商品详情主要区块不可见');
      }

      expect(tabsFrameBox.y - (progressBox.y + progressBox.height)).toBeGreaterThanOrEqual(12);
      expect(firstSectionBox.x - tabsBox.x).toBeGreaterThanOrEqual(viewport.width <= 480 ? 10 : 14);
      expect(tabsBox.x + tabsBox.width - (firstSectionBox.x + firstSectionBox.width)).toBeGreaterThanOrEqual(
        viewport.width <= 480 ? 10 : 14,
      );
      expect(secondSectionBox.y - (firstSectionBox.y + firstSectionBox.height)).toBeGreaterThanOrEqual(12);

      if (viewport.width <= 480) {
        const sourceFields = secondSection.locator('.ant-descriptions-item-container');
        await expect(sourceFields).toHaveCount(6);
        for (let index = 0; index < 6; index += 1) {
          const field = sourceFields.nth(index);
          const [labelBox, valueBox] = await Promise.all([
            field.locator('.ant-descriptions-item-label').boundingBox(),
            field.locator('.ant-descriptions-item-content').boundingBox(),
          ]);
          if (!labelBox || !valueBox) throw new Error('商品来源字段不可见');
          expect(valueBox.y).toBeGreaterThanOrEqual(labelBox.y + labelBox.height);
          expect(valueBox.width).toBeGreaterThanOrEqual(labelBox.width - 1);
        }

        const currency = sourceFields.nth(1).locator('.ant-descriptions-item-content');
        await expect(currency).toHaveText('CNY');
        const [currencyBox, lineHeight] = await Promise.all([
          currency.boundingBox(),
          currency.evaluate((element) => Number.parseFloat(getComputedStyle(element).lineHeight)),
        ]);
        if (!currencyBox) throw new Error('商品币种不可见');
        expect(currencyBox.height).toBeLessThanOrEqual(lineHeight + 1);
        await expect(sourceFields.nth(2).getByText(E2E_PRODUCT_ID, { exact: true })).toBeVisible();
        await expect(sourceFields.nth(2).locator('.ant-typography-copy')).toBeVisible();
      }
      await expectNoRootOverflow(page);
    });

    test(`inventory toolbars remain separated from tables at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=inventory`);

      const stockActions = page.locator('.product-draft-stock__section-actions');
      const syncToolbar = page.locator('.product-draft-inventory-sync__toolbar');
      const syncTable = page.locator('.product-draft-inventory-sync__table');
      await expect(stockActions).toBeVisible();
      await expect(syncToolbar).toBeVisible();
      await expect(syncTable).toBeVisible();

      const [toolbarBox, tableBox] = await Promise.all([syncToolbar.boundingBox(), syncTable.boundingBox()]);
      if (!toolbarBox || !tableBox) throw new Error('库存同步工具栏或表格不可见');
      expect(tableBox.y - (toolbarBox.y + toolbarBox.height)).toBeGreaterThanOrEqual(12);
      await expectNoRootOverflow(page);
    });
  }
});
