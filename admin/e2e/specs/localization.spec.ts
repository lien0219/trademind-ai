import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { supportedLanguages, translateMessage, type AppLanguage } from '../../src/i18n/dictionaries';
import { expectNoRootOverflow } from '../utils/assertions';
import type { Page } from '@playwright/test';
import { e2eTenant, mockTenantReads } from '../mocks/tenants';

type Preferences = { language: AppLanguage; theme: string; dateFormat: string };

const viewports = [
  { width: 1440, height: 900 }, { width: 1280, height: 800 },
  { width: 1024, height: 768 }, { width: 768, height: 900 }, { width: 375, height: 812 },
];

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
  for (const viewport of viewports) {
    test(`@smoke platform organizations preserve business data in ${language} at ${viewport.width}`, async ({ admin, page }) => {
      await mockTenantReads(page, { platformOperator: true });
      await mockLocalizedWorkspace(page, language, true);
      // A business name equal to a dictionary key must still remain untouched.
      await page.route('**/api/v1/platform/tenants/101', route => route.fulfill({ json: ok({ tenant: { ...e2eTenant, name: '设置', description: '订单' }, memberCount: 1, shopCount: 1 }) }));
      await page.setViewportSize(viewport);
      await admin.goto('/platform/tenants');
      await expect(page.getByRole('button', { name: `plus ${translateMessage(language, '创建企业')}`, exact: true })).toBeVisible();
      await expect(page.getByRole('columnheader', { name: translateMessage(language, '企业名称'), exact: true })).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.goto('/platform/tenants/101');
      await expect(page.locator('.tm-page-container').getByText('设置', { exact: true })).toBeVisible();
      await expect(page.locator('.tm-section-card').getByText('订单', { exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: `edit ${translateMessage(language, '编辑企业')}`, exact: true })).toBeVisible();
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
    const saveName = `save ${translateMessage(language, '保存企业配置')}`;
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
    const taskTypeRow = drawer.locator('.ant-descriptions-row').filter({ hasText: translateMessage(language, '任务类型') });
    await expect(taskTypeRow.getByText(translateMessage(language, '标题优化'), { exact: true })).toBeVisible();
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
  await expect(page.getByRole('button', { name: 'save 保存企业配置', exact: true })).toBeVisible();
  const currencySelect = page.getByLabel('报表显示货币', { exact: true });
  await currencySelect.press('ArrowDown');
  await page.getByText('美元 USD', { exact: true }).last().click();
  await page.evaluate(() => { document.documentElement.dataset.e2eLocaleMarker = 'preserved'; });
  await page.getByRole('combobox', { name: '语言', exact: true }).press('ArrowDown');
  await page.getByText('English', { exact: true }).last().click();
  await expect(page.getByRole('button', { name: 'save Save organization settings', exact: true })).toBeVisible();
  await expect(page.getByText(translateMessage('en-US', '美元 USD'), { exact: true }).last()).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-e2e-locale-marker', 'preserved');
  await admin.writeGuard.expectRequestCount('save-language', 1);
  await page.reload();
  await expect(page.getByRole('button', { name: 'save Save organization settings', exact: true })).toBeVisible();
});

test('@smoke configuration templates follow language changes after an English bootstrap', async ({ admin, page }) => {
  const state = await mockLocalizedWorkspace(page, 'en-US');
  await page.route('**/api/v1/tenant/settings/ai', route => route.fulfill({ json: ok({
    module: 'ai', version: 1, inherited: false, values: { defaultLanguage: 'en-US' },
  }) }));
  admin.writeGuard.allow({
    operation: 'change-template-language', method: 'PUT', path: /^\/api\/v1\/tenant\/preferences$/,
    response: record => {
      const body = record.postDataJSON as { expectedVersion: number; values: Preferences };
      expect(body.expectedVersion).toBe(state.version);
      expect(body.values).toMatchObject({ theme: 'light', dateFormat: 'DD/MM/YYYY' });
      state.values = body.values;
      state.version++;
      return ok(state);
    },
  });
  await admin.goto('/tenant/business-settings');
  await page.getByRole('button', { name: translateMessage('en-US', 'AI 与内容'), exact: true }).click();
  await expect(page.getByRole('button', { name: translateMessage('en-US', '中文内容创作'), exact: true })).toBeVisible();
  let current: AppLanguage = 'en-US';
  for (const language of ['ja-JP', 'zh-TW', 'ko-KR', 'th-TH', 'zh-CN'] as const) {
    await page.getByRole('combobox', { name: translateMessage(current, '语言'), exact: true }).press('ArrowDown');
    const names = { 'ja-JP': '日本語', 'zh-TW': '繁體中文', 'ko-KR': '한국어', 'th-TH': 'ไทย', 'zh-CN': '简体中文' };
    await page.getByText(names[language], { exact: true }).last().click();
    await expect(page.getByRole('button', { name: translateMessage(language, '中文内容创作'), exact: true })).toBeVisible();
    current = language;
  }
  await admin.writeGuard.expectRequestCount('change-template-language', 5);
});

for (const viewport of viewports) {
  test(`@responsive enterprise branding stays within the header at ${viewport.width}`, async ({ admin, page }) => {
    await mockLocalizedWorkspace(page, 'en-US');
    const displayName = 'E2E enterprise brand '.repeat(6).trim();
    const logoUrl = 'https://branding.example.test/e2e-logo.svg';
    await page.route(logoUrl, route => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="20" />' }));
    await page.route('**/api/v1/tenant/settings/branding', route => route.fulfill({ json: ok({
      module: 'branding', version: 1, inherited: false, values: { displayName, logoUrl },
    }) }));
    await page.setViewportSize(viewport);
    await admin.goto('/tenant/business-settings');
    const brand = page.getByRole('button', { name: 'Back to workspace', exact: true });
    await expect(brand.locator('img')).toHaveAttribute('src', logoUrl);
    await expect(brand).toContainText(displayName);
    await expectNoRootOverflow(page);
    const bounds = await brand.boundingBox();
    expect(bounds).not.toBeNull();
    if (!bounds) throw new Error('Visible workspace brand has no bounds');
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(viewport.width);
    await admin.writeGuard.expectNoUnexpectedWrites();
  });
}

test('@smoke saving and restoring branding refreshes the workspace header', async ({ admin, page }) => {
  await mockLocalizedWorkspace(page, 'en-US');
  const original = { displayName: 'E2E original brand', logoUrl: '', labels: ['e2e'] };
  let branding = { module: 'branding', version: 1, inherited: false, values: original };
  await page.route('**/api/v1/tenant/settings/branding', route => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok(branding) });
  });
  await page.route('**/api/v1/tenant/settings/branding/history', route => route.fulfill({ json: ok({ items: [
    { module: 'branding', version: 1, action: 'save', actorId: 'e2e-user', createdAt: '2026-10-27T01:00:00Z', values: original },
  ] }) }));
  admin.writeGuard.allow({
    operation: 'save-branding', method: 'PUT', path: /^\/api\/v1\/tenant\/settings\/branding$/,
    response: record => {
      expect(record.postDataJSON).toEqual({ expectedVersion: 1, values: { ...original, displayName: 'E2E updated brand' } });
      branding = { ...branding, version: 2, values: { ...original, displayName: 'E2E updated brand' } };
      return ok(branding);
    },
  });
  admin.writeGuard.allow({
    operation: 'restore-branding', method: 'POST', path: /^\/api\/v1\/tenant\/settings\/branding\/restore$/,
    response: record => {
      expect(record.postDataJSON).toEqual({ expectedVersion: 2, targetVersion: 1 });
      branding = { ...branding, version: 3, values: original };
      return ok(branding);
    },
  });
  await admin.goto('/tenant/business-settings');
  const brand = page.getByRole('button', { name: 'Back to workspace', exact: true });
  await expect(brand).toContainText(original.displayName);
  await page.getByRole('button', { name: translateMessage('en-US', '企业品牌与标签'), exact: true }).click();
  await page.getByLabel(translateMessage('en-US', '企业展示名称'), { exact: true }).fill('E2E updated brand');
  await page.getByRole('button', { name: 'save Save organization settings', exact: true }).click();
  await expect(brand).toContainText('E2E updated brand');
  await admin.writeGuard.expectRequestCount('save-branding', 1);
  await page.getByRole('button', { name: `history ${translateMessage('en-US', '版本历史')}`, exact: true }).click();
  await page.getByRole('button', { name: translateMessage('en-US', '恢复'), exact: true }).click();
  await page.getByRole('button', { name: 'OK', exact: true }).click();
  await expect(brand).toContainText(original.displayName);
  await admin.writeGuard.expectRequestCount('restore-branding', 1);
});
