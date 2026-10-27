import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { supportedLanguages, translateMessage, type AppLanguage } from '../../src/i18n/dictionaries';
import { expectNoRootOverflow } from '../utils/assertions';
import type { Page } from '@playwright/test';
import { e2eTenant, mockTenantReads } from '../mocks/tenants';

type Preferences = { language: AppLanguage; theme: string; dateFormat: string };

async function mockLocalizedWorkspace(page: Page, language: AppLanguage, platformOperator = false) {
  const state = { values: { language, theme: 'light', dateFormat: 'DD/MM/YYYY' } as Preferences, version: 1, inherited: false };
  await page.route('**/api/v1/auth/profile', route => route.fulfill({ json: ok({ ...e2eUser, tenantId: 101, tenantName: 'E2E organization', platformOperator }) }));
  await page.route('**/api/v1/tenant/preferences', route => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok(state) });
  });
  await page.route('**/api/v1/tenant/settings/general', route => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ module: 'general', version: 2, inherited: false, values: { language: 'zh-CN', timezone: 'Asia/Tokyo', currency: 'JPY' } }) });
  });
  return state;
}

for (const language of supportedLanguages) {
  for (const viewport of [
    { width: 1440, height: 900 }, { width: 1280, height: 800 },
    { width: 1024, height: 768 }, { width: 768, height: 900 }, { width: 375, height: 812 },
  ]) {
    test(`@smoke platform organizations preserve business data in ${language} at ${viewport.width}`, async ({ admin, page }) => {
      await mockTenantReads(page, { platformOperator: true });
      await mockLocalizedWorkspace(page, language, true);
      // A business name equal to a dictionary key must still remain untouched.
      await page.route('**/api/v1/platform/tenants/101', route => route.fulfill({ json: ok({ tenant: { ...e2eTenant, name: '设置', description: '订单' }, memberCount: 1, shopCount: 1 }) }));
      await page.setViewportSize(viewport);
      await admin.goto('/platform/tenants');
      await expect(page.getByRole('button', { name: translateMessage(language, '创建企业'), exact: true })).toBeVisible();
      await expect(page.getByRole('columnheader', { name: translateMessage(language, '企业名称'), exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.goto('/platform/tenants/101');
      await expect(page.locator('.tm-page-container').getByText('设置', { exact: true })).toBeVisible();
      await expect(page.locator('.tm-section-card').getByText('订单', { exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: translateMessage(language, '编辑企业'), exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectNoUnexpectedWrites();
    });
  }
}

for (const language of supportedLanguages) {
  test(`@smoke organization settings and refresh honor ${language}`, async ({ admin, page }) => {
    await mockLocalizedWorkspace(page, language);
    await page.setViewportSize({ width: 375, height: 812 });
    await admin.goto('/tenant/business-settings');
    const saveName = translateMessage(language, '保存企业配置');
    await expect(page.getByRole('button', { name: saveName, exact: true })).toBeVisible();
    await expect(page.getByLabel(translateMessage(language, '默认界面语言'), { exact: true })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('lang', language);
    await expectNoRootOverflow(page);
    await page.reload();
    await expect(page.getByRole('button', { name: saveName, exact: true })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('lang', language);
  });

  test(`@smoke AI task diagnostics retain raw identifiers in ${language}`, async ({ admin, page }) => {
    await mockLocalizedWorkspace(page, language);
    const task = {
      id: 'e2e-ai-task', taskType: 'title_optimize', promptCode: 'product_title_optimize',
      status: 'success', provider: 'e2e-provider', model: '设置', tokenInput: 123,
      tokenOutput: 456, costAmount: 0.25, productId: 'e2e-product',
      createdAt: '2026-10-27T01:00:00Z', updatedAt: '2026-10-27T01:00:00Z',
      input: { title: '订单', language: 'zh-CN' }, output: { title: '设置' },
    };
    await page.route('**/api/v1/ai/tasks*', route => route.fulfill({ json: ok({ list: [task], pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 } }) }));
    await page.route('**/api/v1/ai/tasks/e2e-ai-task', route => route.fulfill({ json: ok(task) }));
    await page.setViewportSize({ width: 375, height: 812 });
    await admin.goto('/ai/tasks');
    await page.getByRole('button', { name: translateMessage(language, '查看详情'), exact: true }).click();
    const drawer = page.getByRole('dialog', { name: translateMessage(language, 'AI 任务详情'), exact: true });
    await expect(drawer.getByText(translateMessage(language, '商品标题优化'), { exact: true })).toBeVisible();
    await expect(drawer.getByText('设置', { exact: true })).toBeVisible();
    await expect(drawer.getByText('e2e-provider', { exact: true })).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectNoUnexpectedWrites();
  });
}

test('@smoke switching language persists once and updates the view without reloading auth', async ({ admin, page }) => {
  const state = await mockLocalizedWorkspace(page, 'zh-CN');
  admin.writeGuard.allow({
    operation: 'save-language', method: 'PUT', path: /^\/api\/v1\/tenant\/preferences$/,
    response: record => {
      expect(record.postDataJSON).toEqual({ expectedVersion: 1, values: { language: 'en-US', theme: 'light', dateFormat: 'DD/MM/YYYY' } });
      state.values.language = 'en-US';
      state.version++;
      return ok(state);
    },
  });
  await admin.goto('/tenant/business-settings');
  await expect(page.getByRole('button', { name: '保存企业配置', exact: true })).toBeVisible();
  const currencySelect = page.getByLabel('报表显示货币', { exact: true });
  await currencySelect.click();
  await page.getByRole('option', { name: '美元 USD', exact: true }).click();
  await page.evaluate(() => { document.documentElement.dataset.e2eLocaleMarker = 'preserved'; });
  await page.getByRole('combobox', { name: '语言', exact: true }).click();
  await page.getByRole('option', { name: 'English', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save organization settings', exact: true })).toBeVisible();
  await expect(page.getByLabel('Preferred report currency', { exact: true }).locator('..')).toContainText('USD');
  await expect(page.locator('html')).toHaveAttribute('data-e2e-locale-marker', 'preserved');
  await admin.writeGuard.expectRequestCount('save-language', 1);
  await page.reload();
  await expect(page.getByRole('button', { name: 'Save organization settings', exact: true })).toBeVisible();
});
