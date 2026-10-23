import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { fail, ok, paged } from '../mocks/envelope';
import { PERMISSIONS } from '../../src/utils/permission';
import { AUTH_SESSION_MODE_KEY, AUTH_TOKEN_KEY } from '../../src/constants/auth';
import type { WriteRecord } from '../utils/network-guard';
import type { Page } from '@playwright/test';
import type { AdminUserRow } from '../../src/services/adminUsers';

const restrictedProfiles = [
  { label: 'operator', role: 'operator', permissions: [PERMISSIONS.PRODUCT_WRITE] },
  { label: 'readonly', role: 'readonly', permissions: [PERMISSIONS.PRODUCT_VIEW] },
  { label: 'reviewer', role: 'reviewer', permissions: [PERMISSIONS.OPERATION_TASK_REVIEW] },
  { label: 'custom', role: 'r_0123456789abcdef01234567', permissions: [PERMISSIONS.PRODUCT_VIEW] },
  { label: 'custom without permissions', role: 'r_0123456789abcdef01234567', permissions: [] },
  { label: 'unknown with forged grant', role: 'unexpected', permissions: [PERMISSIONS.USER_MANAGE] },
];

for (const profile of restrictedProfiles) {
  test(`@smoke blocks ${profile.label} from direct user administration`, async ({ admin, page }) => {
    let userListReads = 0;
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({
        ...e2eUser,
        role: profile.role,
        permissions: profile.permissions,
      })) });
    });
    await page.route('**/api/v1/admin/users?**', async (route) => {
      userListReads += 1;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [], pagination: { total: 0 } })) });
    });

    await admin.goto('/settings/users');

    await expect(page.getByText('当前账号无权限访问此页面')).toBeVisible();
    expect(userListReads).toBe(0);
    await admin.writeGuard.expectNoUnexpectedWrites();
  });
}

async function mockUsers(page: Page, users: AdminUserRow[] = []) {
  await page.route('**/api/v1/admin/users?**', (route) => route.fulfill({ json: ok(paged(users)) }));
  await page.route('**/api/v1/admin/roles', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ list: [
      { code: 'admin', name: '管理员', permissions: Object.values(PERMISSIONS), enabled: true, system: true, memberCount: 1 },
      { code: 'operator', name: '运营', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 1 },
      { code: 'readonly', name: '只读', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 0 },
    ] }) });
  });
  await page.route('**/api/v1/admin/permissions', (route) => route.fulfill({ json: ok({ permissions: [PERMISSIONS.PRODUCT_VIEW] }) }));
}

function testUser(): AdminUserRow {
  return { ...e2eUser, id: 'e2e-operator', email: 'operator@example.test', displayName: 'E2E 运营', role: 'operator' };
}

test('@smoke keeps the selected grant recipient stable while shops load', async ({ admin, page }) => {
  const first = { ...testUser(), displayName: 'E2E 用户 A' };
  const second = { ...testUser(), id: 'e2e-other-user', displayName: 'E2E 用户 B' };
  await mockUsers(page, [first, second]);
  let release: (() => void) | undefined;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  let reads = 0;
  await page.route('**/api/v1/shops?**', async (route) => {
    reads += 1;
    await pending;
    await route.fulfill({ json: ok(paged([])) });
  });
  await admin.goto('/settings/users');
  await page.getByRole('row', { name: /E2E 用户 A/ }).getByRole('button', { name: '店铺权限' }).click();
  try {
    await expect.poll(() => reads).toBe(1);
    await expect(page.getByRole('row', { name: /E2E 用户 B/ }).getByRole('button', { name: '店铺权限' })).toBeDisabled();
  } finally { release?.(); }
  await expect(page.getByRole('dialog', { name: '分配店铺权限 — E2E 用户 A' })).toBeVisible();
  expect(reads).toBe(1);
});

test('@smoke retries role directory loading after the role list fails', async ({ admin, page }) => {
  await mockUsers(page);
  let rolesReads = 0;
  await page.route('**/api/v1/admin/roles', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    rolesReads += 1;
    await route.fulfill({ json: rolesReads === 1
      ? fail('角色列表暂时不可用', 50001)
      : ok({ list: [
        { code: 'operator', name: '运营', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 1 },
      ] }) });
  });

  await admin.goto('/settings/users');
  await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
  await expect(page.getByRole('alert')).toContainText('角色列表暂时不可用');
  await expect(page.getByRole('button', { name: '新建角色' })).toBeDisabled();

  await page.getByRole('button', { name: '重试角色目录' }).click();
  await expect(page.getByRole('row', { name: /运营/ })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByRole('button', { name: '新建角色' })).toBeEnabled();
  expect(rolesReads).toBe(2);
  await admin.writeGuard.expectNoUnexpectedWrites();
});

test('@smoke retries permission catalog loading before enabling role creation', async ({ admin, page }) => {
  await mockUsers(page);
  let permissionReads = 0;
  await page.route('**/api/v1/admin/permissions', async (route) => {
    permissionReads += 1;
    await route.fulfill({ json: permissionReads === 1
      ? fail('权限目录暂时不可用', 50002)
      : ok({ permissions: [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.PRODUCT_WRITE] }) });
  });

  await admin.goto('/settings/users');
  await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
  await expect(page.getByRole('alert')).toContainText('权限目录暂时不可用');
  await expect(page.getByRole('button', { name: '新建角色' })).toBeDisabled();

  await page.getByRole('button', { name: '重试角色目录' }).click();
  await expect(page.getByRole('button', { name: '新建角色' })).toBeEnabled();
  await page.getByRole('button', { name: '新建角色' }).click();
  const dialog = page.getByRole('dialog', { name: '新增自定义角色' });
  await expect(dialog.getByRole('checkbox', { name: PERMISSIONS.PRODUCT_WRITE })).toBeVisible();
  expect(permissionReads).toBe(2);
  await admin.writeGuard.expectRequestCount('create-role', 0);
  await admin.writeGuard.expectNoUnexpectedWrites();
});

test('@smoke prevents duplicate user creation and preserves input after rejection', async ({ admin, page }) => {
  const users: AdminUserRow[] = [];
  await mockUsers(page, users);
  let release: (() => void) | undefined;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  let attempts = 0;
  admin.writeGuard.allow({ operation: 'create-user', method: 'POST', path: /^\/api\/v1\/admin\/users$/, response: async (record) => {
    await pending;
    if (++attempts === 1) return fail('邮箱或手机号已被使用', 40001);
    const body = record.postDataJSON as { email: string; displayName: string; role: string };
    const user = { ...testUser(), ...body };
    users.push(user);
    return ok(user);
  } });
  await admin.goto('/settings/users');
  await page.getByRole('button', { name: '新建用户' }).click();
  const dialog = page.getByRole('dialog', { name: '新建用户' });
  await dialog.getByLabel('邮箱').fill('operator@example.test');
  await dialog.getByLabel('初始密码').fill('e2e-temporary-password');
  await dialog.getByRole('button', { name: /确\s*定|OK/ }).click();
  try {
    await admin.writeGuard.expectRequestCount('create-user', 1);
    await expect(dialog.getByRole('button', { name: /确\s*定|OK/ })).toBeDisabled();
  } finally { release?.(); }
  await expect(page.getByText('邮箱或手机号已被使用').first()).toBeVisible();
  await expect(dialog.getByLabel('邮箱')).toHaveValue('operator@example.test');
  await admin.writeGuard.expectRequestCount('create-user', 1);
  await dialog.getByLabel('邮箱').fill('new@example.test');
  await dialog.getByLabel('显示名').fill('E2E 新用户');
  await dialog.getByRole('button', { name: /确\s*定|OK/ }).click();
  await expect(page.getByRole('row', { name: /E2E 新用户/ })).toBeVisible();
  await admin.writeGuard.expectRequestCount('create-user', 2);
  expect(admin.writeGuard.calls('create-user')[1].postDataJSON).toMatchObject({ email: 'new@example.test', role: 'operator' });
});

test('@smoke edits a user, disables and enables access, and resets the password once', async ({ admin, page }) => {
  const user = testUser();
  await mockUsers(page, [user]);
  admin.writeGuard.allow({ operation: 'update-user', method: 'PATCH', path: /^\/api\/v1\/admin\/users\/e2e-operator$/, response: (record) => {
    Object.assign(user, record.postDataJSON);
    return ok(user);
  } });
  admin.writeGuard.allow({ operation: 'reset-user', method: 'POST', path: /^\/api\/v1\/admin\/users\/e2e-operator\/reset-password$/, response: ok({ temporaryPassword: 'e2e-one-time-password' }) });
  await admin.goto('/settings/users');
  let row = page.getByRole('row', { name: /E2E 运营/ });
  await row.getByRole('button', { name: '编辑用户' }).click();
  const edit = page.getByRole('dialog', { name: '编辑用户' });
  await edit.getByLabel('显示名').fill('E2E 新显示名');
  await edit.getByRole('button', { name: /保\s*存/ }).click();
  row = page.getByRole('row', { name: /E2E 新显示名/ });
  await expect(row).toBeVisible();
  expect(admin.writeGuard.calls('update-user')[0].postDataJSON).toEqual({ displayName: 'E2E 新显示名' });
  await row.getByRole('button', { name: /禁\s*用/ }).click();
  await page.getByRole('dialog', { name: '禁用用户' }).getByRole('button', { name: /确\s*认/ }).click();
  await expect(row.getByRole('button', { name: /启\s*用/ })).toBeVisible();
  await row.getByRole('button', { name: /启\s*用/ }).click();
  await expect(row.getByRole('button', { name: /禁\s*用/ })).toBeVisible();
  await row.getByRole('button', { name: '改角色' }).click();
  const roleDialog = page.getByRole('dialog', { name: '修改角色', exact: true });
  await roleDialog.getByRole('combobox').press('ArrowDown');
  await page.getByText('只读', { exact: true }).last().click();
  await roleDialog.getByRole('button', { name: '确认修改' }).click();
  await page.getByRole('dialog', { name: '修改用户角色' }).getByRole('button', { name: '确认执行' }).click();
  await expect(row).toContainText('只读');
  await row.getByRole('button', { name: '重置密码' }).click();
  await page.getByRole('dialog', { name: '重置用户密码' }).getByRole('button', { name: /取\s*消|Cancel/ }).click();
  await admin.writeGuard.expectRequestCount('reset-user', 0);
  await row.getByRole('button', { name: '重置密码' }).click();
  await page.getByRole('dialog', { name: '重置用户密码' }).getByRole('button', { name: /确\s*定|OK/ }).click();
  const password = page.getByRole('dialog', { name: '一次性临时密码' });
  await expect(password.locator('input')).toHaveValue('e2e-one-time-password');
  await password.getByRole('button', { name: '已安全保存' }).click();
  await expect(password).not.toBeVisible();
  await admin.writeGuard.expectRequestCount('reset-user', 1);
  await admin.writeGuard.expectRequestCount('update-user', 4);
});

test('@smoke grants one and multiple shops and revokes all grants', async ({ admin, page }) => {
  const user = testUser();
  await mockUsers(page, [user]);
  await page.route('**/api/v1/shops?**', (route) => route.fulfill({ json: ok(paged([
    { id: 'e2e-shop-a', shopName: 'E2E 店铺 A', platform: 'shopee' },
    { id: 'e2e-shop-b', shopName: 'E2E 店铺 B', platform: 'lazada' },
  ])) }));
  admin.writeGuard.allow({ operation: 'store-grants', method: 'PUT', path: /^\/api\/v1\/admin\/users\/e2e-operator\/store-permissions$/, response: (record) => {
    const { items } = record.postDataJSON as { items: NonNullable<AdminUserRow['storePermissions']> };
    user.storePermissions = items;
    return ok(user);
  } });
  await admin.goto('/settings/users');
  const openGrants = async () => {
    await page.getByRole('row', { name: /E2E 运营/ }).getByRole('button', { name: '店铺权限' }).click();
    return page.getByRole('dialog', { name: /分配店铺权限/ });
  };
  const saveGrants = async () => {
    await page.getByRole('dialog', { name: /分配店铺权限/ }).getByRole('button', { name: /确\s*定|OK/ }).click();
    await page.getByRole('dialog', { name: '保存店铺权限' }).getByRole('button', { name: '确认执行' }).click();
    await expect(page.getByRole('dialog', { name: /分配店铺权限/ })).not.toBeVisible();
  };
  let dialog = await openGrants();
  await dialog.getByRole('button', { name: '添加店铺' }).click();
  await dialog.getByRole('combobox', { name: '店铺授权 1' }).click();
  await page.getByText('E2E 店铺 A (shopee)', { exact: true }).click();
  await saveGrants();
  expect(admin.writeGuard.calls('store-grants')[0].postDataJSON).toEqual({ items: [{ storeId: 'e2e-shop-a', permissionScope: 'operate' }] });
  dialog = await openGrants();
  await dialog.getByRole('button', { name: '添加店铺' }).click();
  await dialog.getByRole('combobox', { name: '店铺授权 2' }).click();
  await page.getByText('E2E 店铺 B (lazada)', { exact: true }).click();
  await saveGrants();
  expect(admin.writeGuard.calls('store-grants')[1].postDataJSON).toEqual({ items: [
    { storeId: 'e2e-shop-a', permissionScope: 'operate' }, { storeId: 'e2e-shop-b', permissionScope: 'operate' },
  ] });
  dialog = await openGrants();
  await dialog.getByRole('button', { name: /移\s*除/ }).first().click();
  await dialog.getByRole('button', { name: /移\s*除/ }).first().click();
  await saveGrants();
  expect(admin.writeGuard.calls('store-grants')[2].postDataJSON).toEqual({ items: [] });
  await admin.writeGuard.expectRequestCount('store-grants', 3);
});

test('@smoke keeps shop grants closed after a shop list failure and allows retry', async ({ admin, page }) => {
  await mockUsers(page, [testUser()]);
  let shopReads = 0;
  await page.route('**/api/v1/shops?**', async (route) => {
    shopReads += 1;
    if (shopReads === 1) {
      await route.fulfill({ json: fail('店铺列表暂时不可用', 50003) });
      return;
    }
    await route.fulfill({ json: ok(paged([
      { id: 'e2e-shop-retry', shopName: 'E2E 重试店铺', platform: 'shopee' },
    ])) });
  });

  await admin.goto('/settings/users');
  const grantButton = page.getByRole('row', { name: /E2E 运营/ }).getByRole('button', { name: '店铺权限' });
  await grantButton.click();
  await expect(page.getByText('店铺列表暂时不可用').first()).toBeVisible();
  await expect(page.getByRole('dialog', { name: /分配店铺权限/ })).not.toBeVisible();

  await grantButton.click();
  const dialog = page.getByRole('dialog', { name: /分配店铺权限/ });
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: '添加店铺' }).click();
  await dialog.getByRole('combobox', { name: '店铺授权 1' }).click();
  await expect(page.getByText('E2E 重试店铺 (shopee)', { exact: true })).toBeVisible();
  expect(shopReads).toBe(2);
  await admin.writeGuard.expectNoUnexpectedWrites();
});

test('@smoke handles server permission denial without retry or losing edits', async ({ admin, page }) => {
  await mockUsers(page, [testUser()]);
  admin.consoleGuard.allowError(/^Failed to load resource: the server responded with a status of 403 \(Forbidden\)$/);
  admin.writeGuard.allow({ operation: 'denied-edit', method: 'PATCH', path: /^\/api\/v1\/admin\/users\/e2e-operator$/, status: 403, response: fail('当前账号无权限执行此操作', 40306) });
  await admin.goto('/settings/users');
  await page.getByRole('row', { name: /E2E 运营/ }).getByRole('button', { name: '编辑用户' }).click();
  const dialog = page.getByRole('dialog', { name: '编辑用户' });
  await dialog.getByLabel('显示名').fill('保留输入');
  await dialog.getByRole('button', { name: /保\s*存/ }).click();
  await expect(page.getByText('当前账号无权限执行此操作').first()).toBeVisible();
  await expect(dialog.getByLabel('显示名')).toHaveValue('保留输入');
  await admin.writeGuard.expectRequestCount('denied-edit', 1);
});

test('@smoke requires a new password before issuing a login session', async ({ admin, page }) => {
  await page.addInitScript(([tokenKey, sessionModeKey]) => {
    localStorage.removeItem(tokenKey);
    localStorage.removeItem(sessionModeKey);
  }, [AUTH_TOKEN_KEY, AUTH_SESSION_MODE_KEY]);
  admin.writeGuard.allow({ operation: 'login', method: 'POST', path: /^\/api\/v1\/auth\/login$/, response: fail('AUTH_PASSWORD_CHANGE_REQUIRED', 40101) });
  let passwordAttempts = 0;
  admin.writeGuard.allow({ operation: 'required-password', method: 'POST', path: /^\/api\/v1\/auth\/change-required-password$/, response: () => {
    passwordAttempts += 1;
    return passwordAttempts === 1 ? fail('密码服务暂时不可用', 50004) : ok({ ok: true });
  } });
  await page.goto('/user/login');
  await page.getByLabel('邮箱 / 手机号').fill('new@example.test');
  await page.getByLabel('密码', { exact: true }).fill('e2e-temporary-password');
  await page.getByRole('button', { name: '登录工作台' }).click();
  await expect(page.getByText('你的账号使用初始或重置后的临时密码，请先设置新的密码。')).toBeVisible();
  await expect(page.getByLabel('临时密码')).toHaveValue('e2e-temporary-password');
  await page.getByLabel('新密码', { exact: true }).fill('e2e-new-password-123');
  await page.getByLabel('确认新密码').fill('e2e-new-password-123');
  const requiredPasswordButton = page.getByRole('button', { name: /修改密码/ });
  await requiredPasswordButton.click();
  await expect(requiredPasswordButton).toBeEnabled();
  await expect(page.getByText('密码修改失败', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('你的账号使用初始或重置后的临时密码，请先设置新的密码。')).toBeVisible();
  await expect(page.getByLabel('临时密码')).toHaveValue('e2e-temporary-password');
  await expect(page.getByLabel('新密码', { exact: true })).toHaveValue('e2e-new-password-123');
  expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBeNull();
  expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_SESSION_MODE_KEY)).toBeNull();

  await requiredPasswordButton.click();
  await expect(page.getByRole('button', { name: '登录工作台' })).toBeVisible();
  await admin.writeGuard.expectRequestCount('required-password', 2);
  expect(admin.writeGuard.calls('required-password')[0].postDataJSON).toEqual({ account: 'new@example.test', currentPassword: 'e2e-temporary-password', newPassword: 'e2e-new-password-123' });
  expect(admin.writeGuard.calls('required-password')[1].postDataJSON).toEqual({ account: 'new@example.test', currentPassword: 'e2e-temporary-password', newPassword: 'e2e-new-password-123' });
  expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_TOKEN_KEY)).toBeNull();
  expect(await page.evaluate((key) => localStorage.getItem(key), AUTH_SESSION_MODE_KEY)).toBeNull();
});

for (const theme of ['light', 'dark']) {
  for (const width of [1440, 1280, 1024, 768, 375]) {
    test(`@smoke renders user and role dialogs without overflow at ${width} in ${theme}`, async ({ admin, page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript((mode) => localStorage.setItem('trademind_admin_theme_mode', mode), theme);
      await mockUsers(page, [testUser()]);
      await admin.goto('/settings/users');
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
      await expect(page.getByRole('row', { name: /E2E 运营/ })).toBeVisible();
      const expectNoOverflow = async () => {
        expect(await page.evaluate(() => Math.max(document.documentElement.scrollWidth - document.documentElement.clientWidth, document.body.scrollWidth - document.body.clientWidth))).toBeLessThanOrEqual(0);
      };
      await expectNoOverflow();
      await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
      await page.getByRole('button', { name: '新建角色' }).click();
      await expect(page.getByRole('dialog', { name: '新增自定义角色' })).toBeVisible();
      await expectNoOverflow();
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
}

test('@smoke creates, edits, disables, and re-enables a tenant custom role', async ({ admin, page }) => {
  const roleCode = 'r_0123456789abcdef01234567';
  type Role = {
    code: string;
    name: string;
    description: string;
    permissions: string[];
    enabled: boolean;
    system: boolean;
    memberCount: number;
  };
  const roles: Role[] = [
    { code: 'admin', name: '管理员', description: '', permissions: Object.values(PERMISSIONS), enabled: true, system: true, memberCount: 1 },
    { code: 'operator', name: '运营', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.PRODUCT_WRITE], enabled: true, system: true, memberCount: 1 },
    { code: 'readonly', name: '只读', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 0 },
    { code: 'reviewer', name: '审核', description: '', permissions: [PERMISSIONS.OPERATION_TASK_REVIEW], enabled: true, system: true, memberCount: 0 },
  ];
  const roleWrites = {
    create: (record: WriteRecord) => {
      const body = record.postDataJSON as { name: string; description?: string; permissions?: string[]; enabled?: boolean };
      const role: Role = {
        code: roleCode, name: body.name, description: body.description || '',
        permissions: body.permissions || [], enabled: body.enabled ?? true, system: false, memberCount: 2,
      };
      roles.push(role);
      return ok(role);
    },
    update: (record: WriteRecord) => {
      const body = record.postDataJSON as Partial<Role>;
      const role = roles.find((item) => item.code === roleCode);
      if (!role) throw new Error('custom role missing from mock state');
      Object.assign(role, body);
      return ok(role);
    },
  };

  await page.route('**/api/v1/admin/users?**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [], pagination: { total: 0 } })) });
  });
  await page.route('**/api/v1/admin/roles', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: roles })) });
  });
  await page.route('**/api/v1/admin/permissions', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ permissions: [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.PRODUCT_WRITE] })) });
  });

  admin.writeGuard.allow({ operation: 'create-role', method: 'POST', path: /^\/api\/v1\/admin\/roles$/, response: roleWrites.create });
  admin.writeGuard.allow({ operation: 'update-role', method: 'PATCH', path: new RegExp(`^/api/v1/admin/roles/${roleCode}$`), response: roleWrites.update });

  await admin.goto('/settings/users');
  await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
  await page.getByRole('button', { name: '新建角色' }).click();
  const createDialog = page.getByRole('dialog', { name: '新增自定义角色' });
  await expect(createDialog.getByLabel('角色名称')).toHaveValue('');
  await createDialog.getByLabel('角色名称').fill('E2E 自定义运营');
  await createDialog.getByText(PERMISSIONS.PRODUCT_VIEW, { exact: true }).click();
  await expect(createDialog.getByRole('checkbox', { name: PERMISSIONS.PRODUCT_VIEW })).toBeChecked();
  await createDialog.getByRole('button', { name: /确\s*定|OK/ }).click();

  const roleRow = page.getByRole('row', { name: /E2E 自定义运营/ });
  await expect(roleRow).toContainText('成员：2 人');
  await expect(roleRow).toContainText(PERMISSIONS.PRODUCT_VIEW);
  await admin.writeGuard.expectRequestCount('create-role', 1);

  await page.getByRole('combobox').click();
  await page.getByText('只读', { exact: true }).last().click();
  await expect(roleRow).toContainText(`相对于「只读」的权限差异`);
  await expect(roleRow).toContainText('权限一致');

  await roleRow.getByRole('button', { name: '编辑权限' }).click();
  const editDialog = page.getByRole('dialog', { name: '编辑角色与权限' });
  await expect(editDialog.getByLabel('角色名称')).toHaveValue('E2E 自定义运营');
  await editDialog.getByText(PERMISSIONS.PRODUCT_WRITE, { exact: true }).click();
  await expect(editDialog.getByRole('checkbox', { name: PERMISSIONS.PRODUCT_WRITE })).toBeChecked();
  await editDialog.getByRole('button', { name: /确\s*定|OK/ }).click();
  await admin.writeGuard.expectRequestCount('update-role', 1);
  expect(admin.writeGuard.calls('update-role')[0].postDataJSON).toMatchObject({
    permissions: expect.arrayContaining([PERMISSIONS.PRODUCT_WRITE]),
  });
  await expect(roleRow).toContainText(PERMISSIONS.PRODUCT_WRITE);
  await expect(roleRow).toContainText(`+ ${PERMISSIONS.PRODUCT_WRITE}`);

  await roleRow.getByRole('button', { name: /停\s*用/ }).click();
  const disableDialog = page.getByRole('dialog', { name: '停用角色并立即撤权？' });
  await expect(disableDialog).toContainText('2 位成员');
  await disableDialog.getByRole('button', { name: /确\s*定|OK/ }).click();
  await expect(roleRow).toContainText('已停用');
  await expect.poll(() => admin.writeGuard.calls('update-role').length).toBe(2);

  await roleRow.getByRole('button', { name: /启\s*用/ }).click();
  const enableDialog = page.getByRole('dialog', { name: '重新启用角色？' });
  await enableDialog.getByRole('button', { name: /确\s*定|OK/ }).click();
  await expect(roleRow).toContainText('启用');
  await admin.writeGuard.expectRequestCount('update-role', 3);
  await admin.writeGuard.expectNoUnexpectedWrites();
});
