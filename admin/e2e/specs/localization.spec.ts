import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { supportedLanguages, translateMessage, type AppLanguage } from '../../src/i18n/dictionaries';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';
import type { Page } from '@playwright/test';
import { e2eTenant, mockTenantReads } from '../mocks/tenants';
import { E2E_PURCHASE_ORDER_ID, E2E_PURCHASE_RETURN_ID } from '../mocks/procurement';
import { E2E_OPERATION_TASK_ID, e2eOperationTaskDetail } from '../mocks/operation-tasks';
import { inventoryRun } from '../mocks/inventory-sync';

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

for (const viewport of [...viewports, { width: 1920, height: 1080 }, { width: 2560, height: 1307 }]) {
  test(`@responsive personal preferences keep a centered content rail at ${viewport.width}`, async ({ admin, page }) => {
    await mockLocalizedWorkspace(page, 'zh-CN');
    await page.setViewportSize(viewport);
    await admin.goto('/tenant/preferences');

    const form = page.getByRole('form', { name: '个人偏好', exact: true });
    for (const label of ['首选语言', '界面主题', '首选日期格式']) {
      await expect(form.getByLabel(label, { exact: true })).toBeVisible();
    }
    await expect(form.getByRole('button', { name: '保存个人偏好', exact: true })).toBeVisible();
    await expect(form.getByRole('button', { name: /^还\s*原$/ })).toBeVisible();

    const dimensions = await page.locator('.tm-page-container').evaluate((element) => {
      const page = element.getBoundingClientRect();
      const content = element.querySelector<HTMLElement>('.tm-page-container__content');
      const card = element.querySelector('.tm-section-card');
      if (!content || !card) return null;
      const contentBounds = content.getBoundingClientRect();
      const cardBounds = card.getBoundingClientRect();
      const style = getComputedStyle(content);
      return {
        contentWidth: contentBounds.width,
        centerDelta: Math.abs(cardBounds.x + cardBounds.width / 2 - page.x - page.width / 2),
        unusedInnerWidth: contentBounds.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight) - cardBounds.width,
        controls: Array.from(card.querySelectorAll('.ant-select')).map((control) => {
          const { x, y, width } = control.getBoundingClientRect();
          return { x, y, width };
        }),
      };
    });
    expect(dimensions, 'personal preferences layout metrics').not.toBeNull();
    if (!dimensions) throw new Error('Personal preferences content or card is missing.');
    expect(dimensions.contentWidth, 'readable page width on wide screens').toBeLessThanOrEqual(961);
    expect(dimensions.centerDelta, 'card centered within the available page').toBeLessThanOrEqual(2);
    expect(Math.abs(dimensions.unusedInnerWidth), 'no unused column beside the card').toBeLessThanOrEqual(2);
    expect(dimensions.controls).toHaveLength(3);
    expect(dimensions.controls.every(control => control.width >= 180), 'readable select widths').toBe(true);
    expect(Math.max(...dimensions.controls.map(control => control.x)) - Math.min(...dimensions.controls.map(control => control.x)), 'preference controls share one aligned column').toBeLessThanOrEqual(2);
    expect(dimensions.controls[1].y).toBeGreaterThan(dimensions.controls[0].y);
    expect(dimensions.controls[2].y).toBeGreaterThan(dimensions.controls[1].y);
    const [hint, actions] = await Promise.all([
      form.getByRole('alert').boundingBox(),
      form.locator('.tm-action-bar').boundingBox(),
    ]);
    expect(hint).not.toBeNull();
    expect(actions).not.toBeNull();
    expect(actions!.y - hint!.y - hint!.height, 'hint and save actions keep a clear separation').toBeGreaterThanOrEqual(16);
    await expectHeaderContentAligned(page);
    await expectNoRootOverflow(page);
    expect(admin.writeGuard.allCalls()).toHaveLength(0);
  });
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
  await expect(page.getByRole('menuitem', { name: /Organization Workspace/ })).toBeVisible();
  await expect(page.locator('.ant-breadcrumb')).toContainText(translateMessage('en-US', '企业工作台'));
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

for (const scene of [
  { path: '/ai/operation-workbench', emptyTitle: '暂无待办商品', action: '查看商品草稿' },
  { path: '/dashboard/product-operations', emptyTitle: '暂无最近动态', action: '前往运营总览' },
] as const) {
  test(`@smoke mounted fallback copy follows language changes on ${scene.path}`, async ({ admin, page }) => {
    const state = await mockLocalizedWorkspace(page, 'en-US');
    let expectedLanguage: AppLanguage = 'en-US';
    if (scene.path === '/dashboard/product-operations') {
      await page.route('**/api/v1/dashboard/product-operations*', route => {
        if (route.request().method() !== 'GET') return route.fallback();
        return route.fulfill({ json: ok({ todos: [], funnel: [], exceptions: [], recent: {} }) });
      });
    }
    admin.writeGuard.allow({
      operation: 'change-fallback-language', method: 'PUT', path: /^\/api\/v1\/tenant\/preferences$/,
      response: record => {
        const body = record.postDataJSON as { expectedVersion: number; values: Preferences };
        expect(body).toEqual({ expectedVersion: state.version, values: { ...state.values, language: expectedLanguage } });
        state.values = body.values;
        state.version++;
        return ok(state);
      },
    });
    await admin.goto(scene.path);
    const empty = page.locator('.tm-empty-state');
    await expect(empty.getByText(translateMessage('en-US', scene.emptyTitle), { exact: true })).toBeVisible();
    await page.evaluate(() => { document.documentElement.dataset.e2eLocaleMarker = 'preserved'; });
    let current: AppLanguage = 'en-US';
    const names = { 'ja-JP': '日本語', 'zh-TW': '繁體中文', 'ko-KR': '한국어', 'th-TH': 'ไทย', 'zh-CN': '简体中文' };
    for (const language of ['ja-JP', 'zh-TW', 'ko-KR', 'th-TH', 'zh-CN'] as const) {
      expectedLanguage = language;
      await page.getByRole('combobox', { name: translateMessage(current, '语言'), exact: true }).press('ArrowDown');
      await page.getByText(names[language], { exact: true }).last().click();
      await expect(empty.getByText(translateMessage(language, scene.emptyTitle), { exact: true })).toBeVisible();
      await expect(empty.getByRole('button', { name: translateMessage(language, scene.action), exact: true })).toBeVisible();
      if (scene.path === '/dashboard/product-operations') {
        await expect(page.getByText(translateMessage(language, '待补 AI 标题'), { exact: true })).toBeVisible();
        await expect(page.getByText(translateMessage(language, 'AI 文本任务失败'), { exact: true })).toBeVisible();
      }
      await expect(page.locator('html')).toHaveAttribute('data-e2e-locale-marker', 'preserved');
      await expectNoRootOverflow(page);
      current = language;
    }
    await admin.writeGuard.expectRequestCount('change-fallback-language', 5);
  });
}

test.describe('mounted control localization', () => {
  test.describe.configure({ mode: 'parallel' });
  const scenes = [
    { path: '/ops/workers/monitor', role: 'columnheader', label: '进程 ID' },
    { path: '/ops/task-center/operation-tasks', role: 'columnheader', label: '任务标题' },
    { path: '/inventory/deductions', role: 'columnheader', label: '扣减时间' },
    { path: '/inventory/logs', role: 'columnheader', label: '变更类型' },
    { path: '/inventory/sync-tasks', role: 'columnheader', label: '商品标题' },
    { path: '/inventory/alerts', role: 'columnheader', label: '库存状态' },
    { path: '/ops/task-center/failures', role: 'columnheader', label: '恢复状态' },
    { path: `/procurement/purchase-orders/${E2E_PURCHASE_ORDER_ID}`, role: 'button', label: '确认收货' },
    { path: `/procurement/purchase-returns/${E2E_PURCHASE_RETURN_ID}`, role: 'button', label: '执行退货' },
    ...(!process.env.CI ? [
      { path: '/ops/inventory-sync', role: 'columnheader', label: '运行 ID', select: { field: '运行状态', option: '等待执行' } },
      { path: `/ops/inventory-sync/runs/${inventoryRun.id}`, role: 'columnheader', label: '快照 ID' },
      { path: '/ops/inventory-sync/calibration', role: 'columnheader', label: '校准 ID' },
      { path: '/ops/inventory-sync/manual-bindings', role: 'columnheader', label: '请求 ID', select: { field: '请求状态', option: '待处理' } },
      { path: '/ops/inventory-sync/bindings', role: 'columnheader', label: '绑定 ID' },
    ] as const : []),
  ] as const;

  for (const scene of scenes) {
    test(`@smoke mounted control labels follow language changes on ${scene.path}`, async ({ admin, page }) => {
      const state = await mockLocalizedWorkspace(page, 'en-US');
      let expectedLanguage: AppLanguage = 'en-US';
      admin.writeGuard.allow({
        operation: 'change-control-language', method: 'PUT', path: /^\/api\/v1\/tenant\/preferences$/,
        response: record => {
          const body = record.postDataJSON as { expectedVersion: number; values: Preferences };
          expect(body).toEqual({ expectedVersion: state.version, values: { ...state.values, language: expectedLanguage } });
          state.values = body.values;
          state.version++;
          return ok(state);
        },
      });
      const label = (language: AppLanguage) => page.getByRole(scene.role).filter({ hasText: translateMessage(language, scene.label) }).first();
      const selectOption = scene.select
        ? (language: AppLanguage) => page.getByRole('combobox', { name: translateMessage(language, scene.select.field), exact: true })
        : undefined;
      await admin.goto(scene.path);
      await expect(label('en-US')).toBeVisible();
      if (selectOption && scene.select) {
        await selectOption('en-US').press('ArrowDown');
        await expect(page.getByText(translateMessage('en-US', scene.select.option), { exact: true }).last()).toBeVisible();
        await page.keyboard.press('Escape');
      }
      await page.evaluate(() => { document.documentElement.dataset.e2eLocaleMarker = 'preserved'; });
      let current: AppLanguage = 'en-US';
      const names = { 'ja-JP': '日本語', 'zh-TW': '繁體中文', 'ko-KR': '한국어', 'th-TH': 'ไทย', 'zh-CN': '简体中文' };
      for (const language of ['ja-JP', 'zh-TW', 'ko-KR', 'th-TH', 'zh-CN'] as const) {
        expectedLanguage = language;
        await page.getByRole('combobox', { name: translateMessage(current, '语言'), exact: true }).press('ArrowDown');
        await page.getByText(names[language], { exact: true }).last().click();
        await expect(label(language)).toBeVisible();
        if (selectOption && scene.select) {
          await selectOption(language).press('ArrowDown');
          await expect(page.getByText(translateMessage(language, scene.select.option), { exact: true }).last()).toBeVisible();
          await page.keyboard.press('Escape');
        }
        await expect(page.locator('html')).toHaveAttribute('data-e2e-locale-marker', 'preserved');
        await expectNoRootOverflow(page);
        current = language;
      }
      await admin.writeGuard.expectRequestCount('change-control-language', 5);
    });
  }

  test('@smoke mounted operation confirmation follows language changes', async ({ admin, page }) => {
    await mockLocalizedWorkspace(page, 'en-US');
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, route => {
      if (route.request().method() !== 'GET') return route.fallback();
      return route.fulfill({ json: ok({
        ...e2eOperationTaskDetail, status: 'suggested', revision: 1,
        latestDraftVersion: undefined, latestDraft: undefined, latestAttempt: undefined,
        allowedActions: { canEditDraft: true, canApprove: false, canReject: false, canExecute: false, canRetry: false, canCancel: true },
      }) });
    });
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/drafts?**`, route => {
      if (route.request().method() !== 'GET') return route.fallback();
      return route.fulfill({ json: ok({ items: [], limit: 50 }) });
    });
    await admin.goto(`/ops/task-center/operation-tasks/${E2E_OPERATION_TASK_ID}`);
    await page.getByRole('button', { name: translateMessage('en-US', '创建首版草稿'), exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toHaveAccessibleName(translateMessage('en-US', '创建首版草稿'));
    await page.getByLabel(translateMessage('en-US', '变更原因'), { exact: true }).fill('E2E locale-change reason');
    for (const language of supportedLanguages) {
      await page.evaluate((value) => {
        localStorage.setItem('trademind_admin_language', value);
        window.dispatchEvent(new CustomEvent('trademind:locale-change', { detail: value }));
      }, language);
      await expect(dialog).toHaveAccessibleName(translateMessage(language, '创建首版草稿'));
      await expect(dialog.getByRole('button', { name: translateMessage(language, '创建草稿'), exact: true })).toBeVisible();
      await expect(page.getByLabel(translateMessage(language, '变更原因'), { exact: true })).toHaveValue('E2E locale-change reason');
    }
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await admin.writeGuard.expectNoUnexpectedWrites();
  });
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
