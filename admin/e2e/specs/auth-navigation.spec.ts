import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser, E2E_TOKEN } from '../mocks/auth';
import { fail, ok } from '../mocks/envelope';
import { AUTH_SESSION_MODE_KEY, AUTH_TOKEN_KEY } from '../../src/constants/auth';
import { PERMISSIONS } from '../../src/utils/permission';
import { THEME_MODE_STORAGE_KEY } from '../../src/theme';
import { expectNoRootOverflow } from '../utils/assertions';

const loginUser = {
  id: e2eUser.id,
  username: e2eUser.username,
  email: e2eUser.email,
  displayName: e2eUser.displayName,
};

// playwright.config.ts serves the production preview in CI. Internal inventory
// routes deliberately exist only in the development server.
const isProductionPreview = Boolean(process.env.CI);

test('@smoke disables global key rotation when the profile lacks its grant', async ({ admin, page }) => {
  await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: ok({
    ...e2eUser, tenantId: 101, isRoot: true, platformOperator: true, permissions: e2eUser.permissions.filter((permission) => permission !== PERMISSIONS.SECURITY_KEY_ROTATE),
  }) }));
  const keyReads: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/api/v1/security/keys/')) keyReads.push(request.url());
  });
  await admin.goto('/settings/security');
  await expect(page.getByText('仅全局系统管理员可管理密钥轮换')).toBeVisible();
  await expect(page.getByRole('button', { name: '预检（Dry Run）' })).toBeDisabled();
  await expect(page.getByRole('button', { name: '启动轮换', exact: true })).toBeDisabled();
  expect(keyReads).toEqual([]);
  await admin.writeGuard.expectNoUnexpectedWrites();
});

for (const scene of ['login'] as const) {
  test(`@smoke loads profile before opening the admin menus after ${scene}`, async ({ admin, page }) => {
    await page.addInitScript((key) => localStorage.removeItem(key), AUTH_TOKEN_KEY);
    admin.writeGuard.allow({
      operation: scene, method: 'POST', path: new RegExp(`^/api/v1/auth/${scene}$`),
      response: ok({ token: E2E_TOKEN, expiresAt: 4_000_000_000, user: loginUser, sessionMode: 'secure_session' }),
    });
    let release: (() => void) | undefined;
    const pending = new Promise<void>((resolve) => { release = resolve; });
    await page.route('**/api/v1/auth/profile', async (route) => {
      await pending;
      await route.fulfill({ json: ok(e2eUser) });
    });
    await page.goto(`/user/${scene}?redirect=%2Fops%2Fobservability`);
    if (scene === 'login') {
      await page.getByLabel('邮箱 / 手机号').fill('e2e-user@example.test');
      await page.getByLabel('密码', { exact: true }).fill('e2e-login-password');
      await page.getByRole('button', { name: '登录工作台' }).click();
    } else {
      await page.getByLabel('邮箱', { exact: true }).fill('e2e-user@example.test');
      await page.getByPlaceholder('6位验证码').fill('123456');
      await page.getByLabel('密码', { exact: true }).fill('e2e-login-password');
      await page.getByLabel('确认密码').fill('e2e-login-password');
      await page.getByRole('button', { name: /^注\s*册/ }).click();
    }
    try {
      await admin.writeGuard.expectRequestCount(scene, 1);
      await expect(page).toHaveURL(new RegExp(`/user/${scene}`));
    } finally { release?.(); }
    await expect(page).toHaveURL(/\/ops\/observability$/);
    await expect(page.getByText('总体运行状态')).toBeVisible();
    await expect(page.getByRole('menuitem', { name: /设置/ }).first()).toBeVisible();
    await page.getByRole('button', { name: /^当前用户 / }).click();
    await expect(page.getByRole('menuitem', { name: '用户与权限', exact: true }).last()).toBeVisible();
    await admin.writeGuard.expectRequestCount(scene, 1);
  });

  test(`@smoke clears the incomplete session when profile fails after ${scene}`, async ({ admin, page }) => {
    await page.addInitScript((key) => localStorage.removeItem(key), AUTH_TOKEN_KEY);
    admin.writeGuard.allow({
      operation: scene, method: 'POST', path: new RegExp(`^/api/v1/auth/${scene}$`),
      response: ok({ token: E2E_TOKEN, expiresAt: 4_000_000_000, user: loginUser, sessionMode: 'secure_session' }),
    });
    await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: fail('权限资料暂时无法加载，请重试', 50301) }));
    await page.goto(`/user/${scene}`);
    if (scene === 'login') {
      await page.getByLabel('邮箱 / 手机号').fill('e2e-user@example.test');
      await page.getByLabel('密码', { exact: true }).fill('e2e-login-password');
      await page.getByRole('button', { name: '登录工作台' }).click();
    } else {
      await page.getByLabel('邮箱', { exact: true }).fill('e2e-user@example.test');
      await page.getByPlaceholder('6位验证码').fill('123456');
      await page.getByLabel('密码', { exact: true }).fill('e2e-login-password');
      await page.getByLabel('确认密码').fill('e2e-login-password');
      await page.getByRole('button', { name: /^注\s*册/ }).click();
    }
    await expect(page.getByText(scene === 'login' ? '登录失败' : '注册失败', { exact: true }).first()).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/user/${scene}$`));
    expect(await page.evaluate((keys) => keys.map((key) => localStorage.getItem(key)), [AUTH_TOKEN_KEY, AUTH_SESSION_MODE_KEY])).toEqual([null, null]);
    await admin.writeGuard.expectRequestCount(scene, 1);
  });
}

for (const theme of ['light', 'dark'] as const) {
  test(`@smoke prevents public account provisioning in ${theme} mode`, async ({ admin, page }) => {
    await page.addInitScript(([authKey, themeKey, mode]) => {
      localStorage.removeItem(authKey);
      localStorage.setItem(themeKey, mode);
    }, [AUTH_TOKEN_KEY, THEME_MODE_STORAGE_KEY, theme]);
    await page.goto('/user/register');
    await expect(page.getByText('请联系管理员创建账号，当前不开放自助注册')).toBeVisible();
    await expect(page.getByRole('button', { name: '获取验证码' })).toBeDisabled();
    await expect(page.getByRole('button', { name: /^注\s*册/ })).toBeDisabled();
    await expect(page.getByLabel('邮箱', { exact: true })).toBeDisabled();
    expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBeNull();
    await expectNoRootOverflow(page);
    await page.getByRole('link', { name: '去登录' }).click();
    await expect(page).toHaveURL(/\/user\/login$/);
    await expect(page.getByRole('button', { name: '登录工作台' })).toBeEnabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
}

const opsProfiles = [
  { label: 'administrator', role: 'admin', permissions: Object.values(PERMISSIONS), destination: '/ops/workers/monitor' },
  { label: 'operator', role: 'operator', permissions: ['task.retry'], destination: '/ops/workers/monitor' },
  { label: 'readonly', role: 'readonly', permissions: ['alerts.read'], destination: '/ops/task-center/alerts' },
  { label: 'reviewer', role: 'reviewer', permissions: ['operationtask.audit.read'], destination: '/ops/task-center/operation-tasks' },
  { label: 'custom', role: 'r_0123456789abcdef01234567', permissions: ['observability.read'], destination: '/ops/observability' },
  { label: 'inventory-sync viewer', role: 'r_0123456789abcdef01234567', permissions: [PERMISSIONS.INVENTORY_SYNC_READ], destination: isProductionPreview ? null : '/ops/inventory-sync' },
  { label: 'unprivileged admin', role: 'admin', permissions: [], destination: null },
  { label: 'unknown', role: 'unexpected', permissions: ['task.retry'], destination: null },
];

for (const profile of opsProfiles) {
  test(`@smoke opens an authorized operations page for ${profile.label}`, async ({ admin, page }) => {
    let inventoryReads = 0;
    await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: ok({ ...e2eUser, role: profile.role, permissions: profile.permissions }) }));
    if (profile.label === 'inventory-sync viewer') {
      await page.route('**/api/v1/inventory-sync/**', (route) => {
        inventoryReads += 1;
        return route.fulfill({ json: ok({ items: [], hasMore: false, limit: 20 }) });
      });
    }
    await admin.goto('/ops');
    if (profile.destination) {
      await expect(page).toHaveURL((url) => url.pathname === profile.destination);
      await expect(page.locator('.tm-page-container')).toBeVisible();
      if (profile.label === 'administrator') {
        await expect(page.getByRole('menuitem', { name: /运维/ }).first()).toBeVisible();
        await expect(page.getByRole('menuitem', { name: '后台任务监控', exact: true }).first()).toBeVisible();
      }
      await page.reload();
      await expect(page).toHaveURL((url) => url.pathname === profile.destination);
      await expect(page.locator('.tm-page-container')).toBeVisible();
      if (profile.label === 'administrator') {
        await expect(page.getByRole('menuitem', { name: /运维/ }).first()).toBeVisible();
        await expect(page.getByRole('menuitem', { name: '后台任务监控', exact: true }).first()).toBeVisible();
      }
    } else {
      await expect(page.getByText('当前账号无权限访问此页面')).toBeVisible();
      await expect(page).toHaveURL(/\/ops$/);
      await expect(page.getByRole('menuitem', { name: /运维/ })).toHaveCount(0);
    }
    if (isProductionPreview && profile.label === 'inventory-sync viewer') {
      await admin.goto('/ops/inventory-sync');
      await expect(page.getByText('404', { exact: true })).toBeVisible();
      await expect(page.getByText('页面不存在', { exact: true })).toBeVisible();
      expect(inventoryReads).toBe(0);
    }
    await expectNoRootOverflow(page);
  });
}

for (const path of ['/ops/workers/monitor', '/ops/task-center/failures', '/ops/observability', '/ops/inventory-sync']) {
  test(`@smoke refuses an unprivileged operations deep link ${path}`, async ({ admin, page }) => {
    let sensitiveReads = 0;
    await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: ok({ ...e2eUser, permissions: [] }) }));
    await page.route(/\/api\/v1\/(workers|task-center|observability|inventory-sync)\//, (route) => {
      sensitiveReads += 1;
      return route.fulfill({ json: ok({}) });
    });
    await admin.goto(path);
    if (isProductionPreview && path === '/ops/inventory-sync') {
      await expect(page.getByText('404', { exact: true })).toBeVisible();
      await expect(page.getByText('页面不存在', { exact: true })).toBeVisible();
    } else {
      await expect(page.getByText('当前账号无权限访问此页面')).toBeVisible();
    }
    expect(sensitiveReads).toBe(0);
  });
}

test('@smoke clears an incomplete profile during a deep-link reload', async ({ admin, page }) => {
  await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: ok({ ...loginUser, role: 'admin' }) }));
  await admin.goto('/ops/workers/monitor');
  await expect(page).toHaveURL(/\/user\/login/);
  expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBeNull();
});

for (const theme of ['light', 'dark']) {
  for (const viewport of [
    { width: 1440, height: 900 }, { width: 1280, height: 800 },
    { width: 1024, height: 768 }, { width: 768, height: 900 }, { width: 375, height: 812 },
  ]) {
    test(`@smoke shows the operations refusal at ${viewport.width} in ${theme}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await page.addInitScript((mode) => localStorage.setItem('trademind_admin_theme_mode', mode), theme);
      await page.route('**/api/v1/auth/profile', (route) => route.fulfill({ json: ok({ ...e2eUser, permissions: [] }) }));
      await admin.goto('/ops');
      await expect(page.getByText('当前账号无权限访问此页面')).toBeVisible();
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
      await expectNoRootOverflow(page);
    });
  }
}
