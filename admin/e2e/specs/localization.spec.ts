import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { supportedLanguages, translateMessage, type AppLanguage } from '../../src/i18n/dictionaries';
import { expectNoRootOverflow } from '../utils/assertions';
import type { Page } from '@playwright/test';

type Preferences = { language: AppLanguage; theme: string; dateFormat: string };

async function mockLocalizedWorkspace(page: Page, language: AppLanguage) {
  const state = { values: { language, theme: 'light', dateFormat: 'DD/MM/YYYY' } as Preferences, version: 1, inherited: false };
  await page.route('**/api/v1/auth/profile', route => route.fulfill({ json: ok({ ...e2eUser, tenantId: 101, tenantName: 'E2E organization' }) }));
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
  await page.evaluate(() => { document.documentElement.dataset.e2eLocaleMarker = 'preserved'; });
  await page.getByRole('combobox', { name: '语言', exact: true }).click();
  await page.getByRole('option', { name: 'English', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save organization settings', exact: true })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-e2e-locale-marker', 'preserved');
  await admin.writeGuard.expectRequestCount('save-language', 1);
  await page.reload();
  await expect(page.getByRole('button', { name: 'Save organization settings', exact: true })).toBeVisible();
});
