import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { E2E_TENANT_B_TOKEN, e2eSecondTenant, e2eShop, e2eTenant, mockTenantReads, mockTenantSwitchReads } from '../mocks/tenants';
import { fail, ok } from '../mocks/envelope';
import { AUTH_SESSION_MODE_KEY, AUTH_TOKEN_KEY } from '../../src/constants/auth';
import { AdminPage } from '../pages/admin.page';
import { e2eUser } from '../mocks/auth';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';
import { layoutTokens } from '../../src/constants/layoutTokens';

const INVITE_SESSION_KEY = 'trademind_invitation_pending';
const tenantViewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];
const generalSettingsPath = '/api/v1/tenant/settings/general';

async function mockGeneralBusinessSettings(page: Page) {
  await mockTenantReads(page, { platformOperator: true });
  const state = {
    snapshot: {
      module: 'general' as const,
      version: 2,
      inherited: false,
      values: { language: 'zh-CN', timezone: 'Asia/Tokyo', currency: 'JPY' },
    },
  };
  await page.route('**/api/v1/auth/profile', (route) => route.fulfill({
    json: ok({ ...e2eUser, tenantId: e2eTenant.id, tenantName: e2eTenant.name, platformOperator: true }),
  }));
  await page.route('**/api/v1/tenant/preferences', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({
      values: { language: 'zh-CN', theme: 'light', dateFormat: 'YYYY-MM-DD' },
      version: 1,
      inherited: false,
    }) });
  });
  await page.route(`**${generalSettingsPath}`, (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok(state.snapshot) });
  });
  return state;
}

test('@smoke switches an authorized system administrator back to tenant zero', async ({ admin, page }) => {
  await mockTenantReads(page);
  const systemToken = 'e2e-system-context-token';
  await page.route('**/api/v1/auth/profile', (route) => {
    const system = route.request().headers().authorization === `Bearer ${systemToken}`;
    return route.fulfill({ json: ok({ ...e2eUser, tenantId: system ? 0 : e2eTenant.id, tenantName: system ? '系统控制台' : e2eTenant.name, platformOperator: true }) });
  });
  await page.route('**/api/v1/auth/tenants', (route) => route.fulfill({ json: ok({
    currentTenantId: route.request().headers().authorization === `Bearer ${systemToken}` ? 0 : e2eTenant.id,
    items: [{ ...e2eTenant, role: 'admin' }, { id: 0, code: 'system', name: '系统控制台', role: 'admin', status: 'active' }],
  }) }));
  admin.writeGuard.allow({
    operation: 'switch-system-context', method: 'POST', path: /^\/api\/v1\/auth\/tenants\/switch$/,
    response: (record) => {
      expect(record.postDataJSON).toEqual({ tenantId: 0 });
      return ok({ token: systemToken });
    },
  });
  await admin.goto('/tenant/members');
  await page.getByRole('button', { name: `当前企业 ${e2eTenant.name}`, exact: true }).click();
  await page.getByRole('option', { name: /系统控制台/ }).click();
  await admin.writeGuard.expectRequestCount('switch-system-context', 1);
  await expect(page.getByRole('button', { name: '当前工作空间 系统控制台', exact: true })).toBeVisible();
  await expect(page).toHaveURL(/\/platform\/tenants$/);
  await expect.poll(() => page.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBe(systemToken);
});

test('@smoke enters an authorized enterprise from the platform-only workspace', async ({ admin, page }) => {
  await mockTenantReads(page, { platformOperator: true });
  const enterpriseToken = 'e2e-enterprise-switch-token';
  await page.route('**/api/v1/auth/profile', (route) => {
    const business = route.request().headers().authorization === `Bearer ${enterpriseToken}`;
    return route.fulfill({ json: ok({
      ...e2eUser,
      platformOperator: true,
      tenantId: business ? e2eTenant.id : 0,
      tenantName: business ? e2eTenant.name : '系统控制台',
    }) });
  });
  await page.route('**/api/v1/auth/tenants', (route) => route.fulfill({ json: ok({
    currentTenantId: route.request().headers().authorization === `Bearer ${enterpriseToken}` ? e2eTenant.id : 0,
    items: [
      { id: 0, code: 'system', name: '系统控制台', role: 'admin', status: 'active' },
      { ...e2eTenant, role: 'admin' },
    ],
  }) }));
  admin.writeGuard.allow({
    operation: 'enter-enterprise', method: 'POST', path: /^\/api\/v1\/auth\/tenants\/switch$/,
    response: (record) => {
      expect(record.postDataJSON).toEqual({ tenantId: e2eTenant.id });
      return ok({ token: enterpriseToken });
    },
  });
  await admin.goto('/platform/tenants');
  await expect(page.getByRole('button', { name: '当前工作空间 系统控制台', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '进入企业工作台' }).click();
  await expect(page).toHaveURL(/\/dashboard\/product-operations$/);
  await expect(page.getByRole('button', { name: `当前企业 ${e2eTenant.name}`, exact: true })).toBeVisible();
  await admin.writeGuard.expectRequestCount('enter-enterprise', 1);
  expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBe(enterpriseToken);
});

test('@smoke blocks ungranted enterprise entry even for a platform operator', async ({ admin, page }) => {
  await mockTenantReads(page, { platformOperator: true });
  await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: ok({
    ...e2eUser, platformOperator: true, tenantId: 0, tenantName: '系统控制台',
  }) }));
  await page.route('**/api/v1/auth/tenants', (route) => route.fulfill({ json: ok({
    currentTenantId: 0,
    items: [{ id: 0, code: 'system', name: '系统控制台', role: 'admin', status: 'active' }],
  }) }));
  await admin.goto('/platform/tenants');
  await expect(page.getByText('需企业成员授权')).toBeVisible();
  await expect(page.getByRole('button', { name: '进入企业工作台' })).toHaveCount(0);
  await page.getByRole('button', { name: '当前工作空间 系统控制台', exact: true }).click();
  await expect(page.getByText('尚未加入企业，企业管理员可以邀请你加入。')).toBeVisible();
  await admin.writeGuard.expectNoUnexpectedWrites();
});

for (const viewport of tenantViewports) {
  for (const entry of [
    { path: '/platform/tenants', title: '租户管理' },
    { path: '/platform/tenants/101', title: e2eTenant.name },
    { path: '/tenant/members', title: '企业成员与邀请' },
    { path: '/tenant/settings', title: '企业信息与设置' },
  ]) {
    test(`@responsive tenant page ${entry.path} at ${viewport.width}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await mockTenantReads(page, { platformOperator: true });
      await admin.goto(entry.path);
      await expect(page.locator('.tm-page-container').getByText(entry.title, { exact: true }).first()).toBeVisible();
      await expect(page.locator('.ant-spin-spinning')).toHaveCount(0);
      await expectHeaderContentAligned(page);
      await expectNoRootOverflow(page);
    });
  }

  test(`@smoke @responsive enterprise business settings keep readable form spacing at ${viewport.width}`, async ({ admin, page }) => {
    await page.setViewportSize(viewport);
    await mockGeneralBusinessSettings(page);
    await admin.goto('/tenant/business-settings');

    const form = page.locator('.tm-tenant-business-settings__form');
    await expect(form.getByLabel('默认界面语言', { exact: true })).toBeVisible();
    await expect(form.getByText('日元 JPY', { exact: true })).toBeVisible();
    await expect(form).toHaveCSS('max-width', '720px');
    const dimensions = await form.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      const parent = element.parentElement!;
      const style = getComputedStyle(parent);
      return {
        width: bounds.width,
        availableWidth: parent.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
      };
    });
    expect(
      Math.abs(dimensions.width - Math.min(720, dimensions.availableWidth)),
      `short form width at ${viewport.width}: ${JSON.stringify(dimensions)}`,
    ).toBeLessThanOrEqual(1);

    const guidance = form.getByRole('alert').filter({
      hasText: '个人语言优先于企业默认语言；货币偏好不执行换汇，也不改写订单金额。',
    });
    const actions = form.locator('.tm-action-bar');
    await expect(guidance).toBeVisible();
    await expect(actions.getByRole('button', { name: /保存企业配置$/ })).toBeVisible();
    await expect(actions.getByRole('button', { name: '放弃更改', exact: true })).toBeVisible();
    const [guidanceBounds, actionBounds] = await Promise.all([
      guidance.boundingBox(),
      actions.boundingBox(),
    ]);
    expect(guidanceBounds, 'business preferences guidance bounds').not.toBeNull();
    expect(actionBounds, 'business preferences action bounds').not.toBeNull();
    expect(
      actionBounds!.y - (guidanceBounds!.y + guidanceBounds!.height),
      `guidance to actions gap at ${viewport.width}`,
    ).toBeGreaterThanOrEqual(layoutTokens.fieldGroupGap);
    await expectHeaderContentAligned(page);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectNoUnexpectedWrites();
  });
}

test('@smoke business preferences stop loading after a failed read and allow retry', async ({ admin, page }) => {
  await mockGeneralBusinessSettings(page);
  let unavailable = true;
  await page.route(`**${generalSettingsPath}`, (route) => {
    if (route.request().method() !== 'GET' || !unavailable) return route.fallback();
    return route.fulfill({ status: 200, json: fail('E2E settings unavailable', 503, null) });
  });
  await admin.goto('/tenant/business-settings');
  const error = page.getByRole('alert').filter({ hasText: 'E2E settings unavailable' });
  await expect(error).toBeVisible();
  await expect(page.locator('.ant-spin-spinning')).toHaveCount(0);
  await expect(page.locator('.tm-tenant-business-settings__form')).toHaveCount(0);

  unavailable = false;
  await error.getByRole('button', { name: /^重\s*试$/ }).click();
  await expect(page.getByLabel('默认界面语言', { exact: true })).toBeVisible();
  await expect(error).toHaveCount(0);
  expect(admin.writeGuard.allCalls(), 'retry must only read business settings').toHaveLength(0);
});

test('@smoke business preferences discard local edits and save once on mobile', async ({ admin, page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const state = await mockGeneralBusinessSettings(page);
  let finishSave!: () => void;
  const savePending = new Promise<void>((resolve) => { finishSave = resolve; });
  admin.writeGuard.allow({
    operation: 'save-business-preferences',
    method: 'PUT',
    path: /^\/api\/v1\/tenant\/settings\/general$/,
    response: async (record) => {
      expect(record).toMatchObject({ method: 'PUT', path: generalSettingsPath, query: {} });
      expect(record.postDataJSON).toEqual({
        expectedVersion: 2,
        values: { language: 'zh-CN', timezone: 'Asia/Tokyo', currency: 'USD' },
      });
      await savePending;
      state.snapshot = {
        ...state.snapshot,
        version: 3,
        values: { ...state.snapshot.values, currency: 'USD' },
      };
      return ok(state.snapshot);
    },
  });

  try {
    await admin.goto('/tenant/business-settings');
    const form = page.locator('.tm-tenant-business-settings__form');
    const currency = form.getByLabel('报表显示货币', { exact: true });
    const chooseDollars = async () => {
      await currency.press('ArrowDown');
      await page.locator('.ant-select-dropdown:visible').getByText('美元 USD', { exact: true }).click();
      await expect(form.getByTitle('美元 USD', { exact: true })).toBeVisible();
    };
    await expect(form.getByTitle('日元 JPY', { exact: true })).toBeVisible();
    await chooseDollars();
    await Promise.all([
      page.waitForResponse((response) =>
        new URL(response.url()).pathname === generalSettingsPath && response.request().method() === 'GET',
      ),
      form.getByRole('button', { name: '放弃更改', exact: true }).click(),
    ]);
    await expect(form.getByTitle('日元 JPY', { exact: true })).toBeVisible();
    expect(admin.writeGuard.allCalls(), 'discard must not mutate business settings').toHaveLength(0);

    await chooseDollars();
    await form.getByRole('button', { name: /保存企业配置$/ }).dblclick();
    await admin.writeGuard.expectRequestCount('save-business-preferences', 1);
    await expect(currency).toBeDisabled();
    finishSave();
    await expect(currency).toBeEnabled();
    await expect(form.getByTitle('美元 USD', { exact: true })).toBeVisible();
    await expect(page.getByText('版本 3', { exact: true })).toBeVisible();
    await admin.writeGuard.expectRequestCount('save-business-preferences', 1);
    expect(admin.writeGuard.allCalls()).toHaveLength(1);
    await expectNoRootOverflow(page);
  } finally {
    finishSave();
  }
});

test('@smoke switches credentials and refreshes every tab while clearing tenant drafts', async ({ admin, page, context }) => {
  await mockTenantSwitchReads(page);
  let finishSwitch!: () => void;
  const switchPending = new Promise<void>((resolve) => { finishSwitch = resolve; });
  admin.writeGuard.allow({
    operation: 'switch-tenant', method: 'POST', path: /^\/api\/v1\/auth\/tenants\/switch$/,
    response: async () => { await switchPending; return ok({ token: E2E_TENANT_B_TOKEN }); },
  });
  await admin.goto('/tenant/members');
  const siblingPage = await context.newPage();
  const sibling = new AdminPage(siblingPage);
  try {
    await sibling.setup();
    await mockTenantSwitchReads(siblingPage);
    await sibling.goto('/tenant/members');
    await expect(siblingPage.getByRole('button', { name: `当前企业 ${e2eTenant.name}`, exact: true })).toBeVisible();
    await page.evaluate(() => {
      localStorage.setItem('trademind_tenant_cache:products', 'e2e-old-cache');
      localStorage.setItem('publish-batch-wizard:e2e-draft', 'e2e-old-draft');
    });
    await page.getByRole('button', { name: `当前企业 ${e2eTenant.name}`, exact: true }).click();
    await page.getByRole('option', { name: new RegExp(e2eSecondTenant.name) }).click();
    await admin.writeGuard.expectRequestCount('switch-tenant', 1);
    await expect(page.getByRole('status').filter({ hasText: '正在切换企业，正在刷新工作区' })).toBeVisible();
    await expect(siblingPage.getByRole('status').filter({ hasText: '正在切换企业，正在刷新工作区' })).toBeVisible();
    const profileAfterSwitch = page.waitForRequest((request) => request.url().endsWith('/api/v1/auth/profile') && request.headers().authorization === `Bearer ${E2E_TENANT_B_TOKEN}`);
    const siblingProfileAfterSwitch = siblingPage.waitForRequest((request) => request.url().endsWith('/api/v1/auth/profile') && request.headers().authorization === `Bearer ${E2E_TENANT_B_TOKEN}`);
    finishSwitch();
    await Promise.all([profileAfterSwitch, siblingProfileAfterSwitch]);
    for (const tab of [page, siblingPage]) {
      await expect(tab.getByRole('button', { name: `当前企业 ${e2eSecondTenant.name}`, exact: true })).toBeVisible();
      expect(await tab.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBe(E2E_TENANT_B_TOKEN);
    }
    expect(admin.writeGuard.calls('switch-tenant')[0]?.postDataJSON).toEqual({ tenantId: e2eSecondTenant.id });
    expect(await page.evaluate(() => [localStorage.getItem('trademind_tenant_cache:products'), localStorage.getItem('publish-batch-wizard:e2e-draft')])).toEqual([null, null]);
    await sibling.writeGuard.expectNoUnexpectedWrites();
    await sibling.consoleGuard.expectNoFatalErrors();
  } finally {
    finishSwitch();
    await siblingPage.close();
  }
});

test('@smoke discards an old tenant response even when switching fails', async ({ admin, page }) => {
  await mockTenantSwitchReads(page);
  let finishMembers!: () => void;
  const membersPending = new Promise<void>((resolve) => { finishMembers = resolve; });
  let memberRequestStarted!: () => void;
  const membersStarted = new Promise<void>((resolve) => { memberRequestStarted = resolve; });
  await page.route('**/api/v1/tenant/members', async (route) => {
    memberRequestStarted();
    await membersPending;
    await route.fulfill({ json: ok({ items: [{ id: 'e2e-stale-member', userId: 'e2e-stale-user', displayName: 'E2E 已过期成员响应', role: 'operator', status: 'active', storePermissions: [] }] }) });
  });
  let finishSwitch!: () => void;
  const switchPending = new Promise<void>((resolve) => { finishSwitch = resolve; });
  admin.writeGuard.allow({
    operation: 'switch-tenant', method: 'POST', path: /^\/api\/v1\/auth\/tenants\/switch$/,
    response: async () => { await switchPending; return fail('E2E 切换失败', 40301); },
  });
  try {
    await admin.goto('/tenant/members');
    await membersStarted;
    await page.getByRole('button', { name: `当前企业 ${e2eTenant.name}`, exact: true }).click();
    await page.getByRole('option', { name: new RegExp(e2eSecondTenant.name) }).click();
    await admin.writeGuard.expectRequestCount('switch-tenant', 1);
    const oldResponse = page.waitForResponse('**/api/v1/tenant/members');
    finishMembers();
    await oldResponse;
    finishSwitch();
    await expect(page.getByText('E2E 切换失败', { exact: true })).toBeVisible();
    await expect(page.getByText('E2E 已过期成员响应', { exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: `当前企业 ${e2eTenant.name}`, exact: true })).toBeVisible();
  } finally {
    finishMembers();
    finishSwitch();
  }
});

test('@smoke retries platform tenant creation with the same idempotency key', async ({ admin, page }) => {
  await mockTenantReads(page, { platformOperator: true });
  const requests: Record<string, unknown>[] = [];
  admin.writeGuard.allow({
    operation: 'create-tenant',
    method: 'POST',
    path: /^\/api\/v1\/platform\/tenants$/,
    response: (record) => {
      requests.push(record.postDataJSON as Record<string, unknown>);
      return requests.length === 1 ? fail('创建暂时失败', 50301) : ok(e2eTenant);
    },
  });

  await admin.goto('/platform/tenants');
  await expect(page.getByRole('link', { name: 'e2e-acme' })).toBeVisible();
  await page.getByRole('button', { name: /创建企业/ }).first().click();
  const dialog = page.getByRole('dialog', { name: '创建企业' });
  await dialog.getByLabel('企业标识').fill('e2e-retry');
  await dialog.getByLabel('企业名称').fill('E2E 重试企业');
  await dialog.getByLabel('企业管理员邮箱').fill('retry-owner@example.test');
  await dialog.getByRole('button', { name: '创建企业', exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  await expect(dialog).toBeVisible();
  const retryButton = dialog.getByRole('button', { name: /创建企业/ });
  await expect(retryButton).toBeEnabled();
  await retryButton.click();
  await expect.poll(() => requests.length).toBe(2);
  expect(requests[1].idempotencyKey).toBe(requests[0].idempotencyKey);
  expect(requests[1]).toMatchObject({ code: 'e2e-retry', name: 'E2E 重试企业', ownerEmail: 'retry-owner@example.test' });
  await expect(dialog).toBeHidden();
});

test('@smoke refuses tenant member reads without user.manage', async ({ admin, page }) => {
  await mockTenantReads(page, { permissions: [] });
  let sensitiveReads = 0;
  for (const path of ['/api/v1/tenant/members', '/api/v1/tenant/invitations', '/api/v1/admin/roles', '/api/v1/shops']) {
    await page.route(`**${path}**`, async (route) => {
      sensitiveReads += 1;
      await route.fulfill({ json: ok({ items: [], list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } }) });
    });
  }
  await admin.goto('/tenant/members');
  await expect(page.getByText('企业成员和邀请管理仅对拥有 user.manage 权限的成员开放。')).toBeVisible();
  expect(sensitiveReads).toBe(0);
});

test('@smoke maps built-in role codes to localized labels in member list and invitation form', async ({ admin, page }) => {
  await mockTenantReads(page);
  await page.route('**/api/v1/admin/roles', (route) => route.fulfill({ json: ok({ list: [
    { code: 'admin', name: 'admin', enabled: true, system: true, memberCount: 1 },
    { code: 'operator', name: 'operator', enabled: true, system: true, memberCount: 1 },
    { code: 'reviewer', name: 'reviewer', enabled: true, system: true, memberCount: 0 },
    { code: 'readonly', name: 'readonly', enabled: true, system: true, memberCount: 0 },
  ] }) }));
  await admin.goto('/tenant/members');
  await expect(page.getByRole('cell', { name: '管理员' })).toBeVisible();
  await page.getByRole('button', { name: '邀请成员' }).first().click();
  const dialog = page.getByRole('dialog', { name: '邀请成员' });
  await dialog.getByLabel('企业角色').click();
  for (const label of ['管理员', '运营', '审核', '只读']) {
    await expect(page.getByRole('option', { name: label })).toBeVisible();
  }
});

test('@smoke submits per-store permissions and selected invitation expiry', async ({ admin, page }) => {
  await mockTenantReads(page);
  let invitePayload: Record<string, unknown> | undefined;
  admin.writeGuard.allow({
    operation: 'create-invitation',
    method: 'POST',
    path: /^\/api\/v1\/tenant\/invitations$/,
    response: (record) => {
      invitePayload = record.postDataJSON as Record<string, unknown>;
      return ok({ invitation: { ...e2eTenant, id: 'e2e-invitation-2' }, token: 'e2e-one-time-token' });
    },
  });

  await admin.goto('/tenant/members');
  const inviteButton = page.getByRole('button', { name: /邀请成员/ }).first();
  await expect(inviteButton).toBeEnabled();
  await inviteButton.click();
  const dialog = page.getByRole('dialog', { name: '邀请成员' });
  await dialog.getByLabel('成员邮箱').fill('new-member@example.test');
  const expiry = dialog.getByLabel('邀请有效期');
  await expiry.press('ArrowDown');
  await page.getByText('3 天', { exact: true }).click();
  await expect(dialog.getByTitle('3 天', { exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: '添加店铺权限' }).click();
  const shop = dialog.getByRole('combobox', { name: '店铺授权 1', exact: true });
  await shop.press('ArrowDown');
  await shop.press('Enter');
  const scope = dialog.getByRole('combobox', { name: '店铺权限级别 1', exact: true });
  await scope.press('ArrowDown');
  await page.locator('.ant-select-dropdown:visible').getByText('管理', { exact: true }).click();
  await dialog.getByRole('button', { name: '创建邀请', exact: true }).click();

  await expect.poll(() => invitePayload).toBeDefined();
  expect(invitePayload).toMatchObject({
    email: 'new-member@example.test',
    role: 'operator',
    expiresInHours: 72,
    storePermissions: [{ storeId: e2eShop.id, platform: e2eShop.platform, permissionScope: 'manage' }],
  });
  await expect(page.getByRole('dialog', { name: '邀请已创建' })).toBeVisible();
});

test('@smoke accepts an invitation with the one-time token and clears the session', async ({ admin, page }) => {
  await page.addInitScript(([authKey, authModeKey, inviteKey]) => {
    localStorage.setItem(authKey, 'e2e-session-token');
    localStorage.setItem(authModeKey, 'secure_session');
    sessionStorage.setItem(inviteKey, JSON.stringify({ token: 'e2e-staged-invitation-token', expiresAt: Date.now() + 60 * 60 * 1000 }));
  }, [AUTH_TOKEN_KEY, AUTH_SESSION_MODE_KEY, INVITE_SESSION_KEY]);
  admin.writeGuard.allow({
    operation: 'accept-invitation',
    method: 'POST',
    path: /^\/api\/v1\/tenant\/invitations\/accept$/,
    response: ok({ tenantId: e2eTenant.id }),
  });

  await admin.goto('/tenant/invitations/accept#token=e2e-invitation-token');
  await expect(page).toHaveURL(/\/tenant\/invitations\/accept$/);
  await expect(page.getByLabel('一次性邀请令牌')).toHaveValue('e2e-invitation-token');
  await page.getByRole('button', { name: /接受邀请/ }).click();

  await admin.writeGuard.expectRequestCount('accept-invitation', 1);
  expect(admin.writeGuard.calls('accept-invitation')[0]?.postDataJSON).toEqual({ token: 'e2e-invitation-token' });
  await expect(page.getByText('当前会话已失效，请重新登录后手动切换到受邀企业。')).toBeVisible();
  expect(await page.evaluate(([authKey, authModeKey]) => [
    localStorage.getItem(authKey),
    localStorage.getItem(authModeKey),
  ], [AUTH_TOKEN_KEY, AUTH_SESSION_MODE_KEY])).toEqual([null, null]);
  expect(await page.evaluate((inviteKey) => sessionStorage.getItem(inviteKey), INVITE_SESSION_KEY)).toBeNull();

  await page.getByRole('button', { name: '重新登录' }).click();
  await expect(page).toHaveURL(/\/user\/login$/);
});
