import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { mockUsers, testUser } from '../mocks/admin-users';
import { fail, ok, paged } from '../mocks/envelope';
import { PERMISSIONS } from '../../src/utils/permission';
import { AUTH_SESSION_MODE_KEY, AUTH_TOKEN_KEY } from '../../src/constants/auth';
import type { WriteRecord } from '../utils/network-guard';
import type { AdminRoleRow, AdminUserRow } from '../../src/services/adminUsers';

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
  await expect(page.getByRole('article', { name: '运营', exact: true })).toBeVisible();
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
  await expect(dialog.getByRole('checkbox', { name: '编辑商品', exact: true })).toBeVisible();
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
  await expect(row.getByRole('cell', { name: '运营', exact: true })).toBeVisible();
  await expect(row.getByRole('cell', { name: 'operator', exact: true })).toHaveCount(0);
  await row.getByRole('button', { name: '编辑用户' }).click();
  const edit = page.getByRole('dialog', { name: '编辑用户' });
  await edit.getByLabel('显示名').fill('E2E 新显示名');
  await edit.getByRole('button', { name: /保\s*存/ }).click();
  row = page.getByRole('row', { name: /E2E 新显示名/ });
  await expect(row).toBeVisible();
  expect(admin.writeGuard.calls('update-user')[0].postDataJSON).toEqual({ displayName: 'E2E 新显示名' });
  await row.getByRole('button', { name: /禁\s*用/ }).click();
  const disableDialog = page.getByRole('dialog', { name: '禁用用户' });
  await expect(disableDialog.getByRole('group', { name: '禁用目标账号' })).toContainText('E2E 新显示名');
  await expect(disableDialog.getByRole('list', { name: '禁用影响' })).toContainText('已登录会话将失效');
  await expect(disableDialog).toContainText('重新启用此账号');
  await expect(disableDialog).not.toContainText('任务中心');
  await expect(disableDialog.getByRole('button', { name: '确认禁用', exact: true })).toBeVisible();
  await disableDialog.getByRole('button', { name: /^取\s*消$/ }).click();
  await admin.writeGuard.expectRequestCount('update-user', 1);
  await expect(row.getByRole('button', { name: /禁\s*用/ })).toBeVisible();
  await row.getByRole('button', { name: /禁\s*用/ }).click();
  await disableDialog.getByRole('button', { name: '确认禁用', exact: true }).click();
  await expect(row.getByRole('button', { name: /启\s*用/ })).toBeVisible();
  expect(admin.writeGuard.calls('update-user')[1].postDataJSON).toEqual({ status: 'disabled' });
  await row.getByRole('button', { name: /启\s*用/ }).click();
  await expect(row.getByRole('button', { name: /禁\s*用/ })).toBeVisible();
  await row.getByRole('button', { name: '改角色' }).click();
  const roleDialog = page.getByRole('dialog', { name: '修改角色', exact: true });
  await expect(roleDialog.getByRole('button', { name: /^取\s*消$/ })).toBeVisible();
  await roleDialog.getByRole('combobox').press('ArrowDown');
  await page.getByText('只读', { exact: true }).last().click();
  await roleDialog.getByRole('button', { name: '确认修改' }).click();
  await page.getByRole('dialog', { name: '修改用户角色' }).getByRole('button', { name: '确认执行' }).click();
  await expect(row).toContainText('只读');
  expect(admin.writeGuard.calls('update-user')[3].postDataJSON).toEqual({ role: 'readonly' });
  await row.getByRole('button', { name: '重置密码' }).click();
  const resetDialog = page.getByRole('dialog', { name: '重置用户密码' });
  await expect(resetDialog.getByRole('button', { name: /^取\s*消$/ })).toBeVisible();
  await expect(resetDialog.getByRole('button', { name: '确认重置', exact: true })).toBeVisible();
  await resetDialog.getByRole('button', { name: /^取\s*消$/ }).click();
  await admin.writeGuard.expectRequestCount('reset-user', 0);
  await row.getByRole('button', { name: '重置密码' }).click();
  await page.getByRole('dialog', { name: '重置用户密码' }).getByRole('button', { name: '确认重置', exact: true }).click();
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

for (const viewport of [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
]) {
  test(`@smoke keeps forced password-change actions stacked without overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
    await page.setViewportSize(viewport);
    await page.addInitScript(([tokenKey, sessionModeKey]) => {
      localStorage.removeItem(tokenKey);
      localStorage.removeItem(sessionModeKey);
    }, [AUTH_TOKEN_KEY, AUTH_SESSION_MODE_KEY]);
    admin.writeGuard.allow({ operation: 'login', method: 'POST', path: /^\/api\/v1\/auth\/login$/, response: fail('AUTH_PASSWORD_CHANGE_REQUIRED', 40101) });
    await page.goto('/user/login');
    await page.getByLabel('邮箱 / 手机号').fill('new@example.test');
    await page.getByLabel('密码', { exact: true }).fill('e2e-temporary-password');
    await page.getByRole('button', { name: '登录工作台' }).click();
    await expect(page.getByText('你的账号使用初始或重置后的临时密码，请先设置新的密码。')).toBeVisible();

    const changePasswordButton = page.getByRole('button', { name: /修改密码/ });
    const returnToLoginButton = page.getByRole('button', { name: '返回登录' });
    const changePasswordBounds = await changePasswordButton.boundingBox();
    const returnToLoginBounds = await returnToLoginButton.boundingBox();
    expect(changePasswordBounds).not.toBeNull();
    expect(returnToLoginBounds).not.toBeNull();
    expect(returnToLoginBounds!.y).toBeGreaterThan(changePasswordBounds!.y + changePasswordBounds!.height);
    expect(changePasswordBounds!.width).toBeGreaterThan(returnToLoginBounds!.width * 2);
    const overflow = await page.evaluate(() => ({
      document: document.documentElement.scrollWidth - document.documentElement.clientWidth,
      body: document.body.scrollWidth - document.body.clientWidth,
    }));
    expect(overflow.document, `document horizontal overflow at ${viewport.width}px`).toBeLessThanOrEqual(0);
    expect(overflow.body, `body horizontal overflow at ${viewport.width}px`).toBeLessThanOrEqual(0);
    await admin.writeGuard.expectRequestCount('login', 1);
    await admin.writeGuard.expectRequestCount('required-password', 0);
  });
}

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
  await page.getByLabel('新密码', { exact: true }).fill('中文密码Abcdef123456');
  await page.getByLabel('确认新密码').fill('中文密码Abcdef123456');
  await page.getByRole('button', { name: /修改密码/ }).click();
  await expect(page.getByText('密码仅支持英文字母、数字和英文符号，不能包含中文或空格', { exact: true })).toBeVisible();
  await admin.writeGuard.expectRequestCount('required-password', 0);
  await page.getByLabel('新密码', { exact: true }).fill('e2e-new-password-123');
  await page.getByLabel('确认新密码').fill('e2e-new-password-123');
  const requiredPasswordButton = page.getByRole('button', { name: /修改密码/ });
  const returnToLoginButton = page.getByRole('button', { name: '返回登录' });
  const changePasswordBounds = await requiredPasswordButton.boundingBox();
  const returnToLoginBounds = await returnToLoginButton.boundingBox();
  expect(changePasswordBounds).not.toBeNull();
  expect(returnToLoginBounds).not.toBeNull();
  expect(returnToLoginBounds!.y).toBeGreaterThan(changePasswordBounds!.y + changePasswordBounds!.height);
  expect(changePasswordBounds!.width).toBeGreaterThan(returnToLoginBounds!.width * 2);
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

test('@smoke shows current-account password and status actions as disabled', async ({ admin, page }) => {
  await mockUsers(page, [e2eUser]);
  await admin.goto('/settings/users');

  const currentAccountRow = page.getByRole('row', { name: /E2E 管理员/ });
  const shopPermissionButton = currentAccountRow.getByRole('button', { name: '店铺权限' });
  const resetPasswordButton = currentAccountRow.getByRole('button', { name: '重置密码' });
  const disableButton = currentAccountRow.getByRole('button', { name: '禁用' });
  await expect(shopPermissionButton).toBeVisible();
  await expect(shopPermissionButton).toBeDisabled();
  await expect(resetPasswordButton).toBeVisible();
  await expect(resetPasswordButton).toBeDisabled();
  await expect(disableButton).toBeVisible();
  await expect(disableButton).toBeDisabled();
  await admin.writeGuard.expectNoUnexpectedWrites();
});

const userRbacViewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
] as const;

for (const theme of ['light', 'dark']) {
  for (const viewport of userRbacViewports) {
    test(`@smoke renders user and role dialogs without overflow at ${viewport.width}x${viewport.height} in ${theme}`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await page.addInitScript((mode) => localStorage.setItem('trademind_admin_theme_mode', mode), theme);
      const targetUser = { ...testUser(), displayName: `E2E 运营 ${'超长账号名称'.repeat(8)}` };
      await mockUsers(page, [e2eUser, targetUser]);
      await page.route('**/api/v1/admin/permissions', (route) => route.fulfill({ json: ok({
        permissions: Object.values(PERMISSIONS).filter((key) => key !== PERMISSIONS.SECURITY_KEY_ROTATE),
      }) }));
      await admin.goto('/settings/users');
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
      await expect(page.getByRole('row', { name: /E2E 运营/ })).toBeVisible();
      const currentAccountRow = page.getByRole('row', { name: /E2E 管理员/ });
      const shopPermissionButton = currentAccountRow.getByRole('button', { name: '店铺权限' });
      const resetPasswordButton = currentAccountRow.getByRole('button', { name: '重置密码' });
      const disableButton = currentAccountRow.getByRole('button', { name: '禁用' });
      await expect(shopPermissionButton).toBeVisible();
      await expect(shopPermissionButton).toBeDisabled();
      await expect(resetPasswordButton).toBeVisible();
      await expect(resetPasswordButton).toBeDisabled();
      await expect(disableButton).toBeVisible();
      await expect(disableButton).toBeDisabled();
      await expect(currentAccountRow.getByRole('button', { name: '删除用户' })).toBeDisabled();
      const expectNoOverflow = async () => {
        expect(await page.evaluate(() => Math.max(document.documentElement.scrollWidth - document.documentElement.clientWidth, document.body.scrollWidth - document.body.clientWidth))).toBeLessThanOrEqual(0);
      };
      await expectNoOverflow();
      await page.getByRole('row', { name: /E2E 运营/ }).getByRole('button', { name: /禁\s*用/ }).click();
      const disableDialog = page.getByRole('dialog', { name: '禁用用户' });
      await expect(disableDialog.getByRole('group', { name: '禁用目标账号' })).toContainText(targetUser.displayName);
      await expect(disableDialog.getByRole('button', { name: '确认禁用', exact: true })).toBeVisible();
      await expectNoOverflow();
      const bounds = await disableDialog.boundingBox();
      expect(bounds).not.toBeNull();
      if (!bounds) throw new Error('disable dialog has no visible bounds');
      expect(bounds.x).toBeGreaterThanOrEqual(0);
      expect(bounds.y).toBeGreaterThanOrEqual(0);
      expect(bounds.x + bounds.width).toBeLessThanOrEqual(viewport.width);
      expect(bounds.y + bounds.height).toBeLessThanOrEqual(viewport.height);
      expect(await disableDialog.evaluate((node) => node.scrollWidth - node.clientWidth)).toBeLessThanOrEqual(0);
      await disableDialog.getByRole('button', { name: /^取\s*消$/ }).click();
      await expect(disableDialog).not.toBeVisible();
      await admin.writeGuard.expectRequestCount('update-user', 0);
      await page.getByRole('button', { name: '新建用户' }).click();
      const createUser = page.getByRole('dialog', { name: '新建用户' });
      await expect(createUser.getByRole('button', { name: '随机生成密码' })).toBeVisible();
      await createUser.getByRole('button', { name: '随机生成密码' }).click();
      await expect(createUser.getByRole('button', { name: '复制密码' })).toBeEnabled();
      await expectNoOverflow();
      await createUser.getByRole('button', { name: /^取\s*消$/ }).click();
      await page.getByRole('row', { name: /E2E 运营/ }).getByRole('button', { name: '删除用户' }).click();
      const deleteUser = page.getByRole('dialog', { name: '删除用户', exact: true });
      await expect(deleteUser).toContainText(targetUser.displayName);
      await expectNoOverflow();
      expect(await deleteUser.evaluate((node) => node.scrollWidth - node.clientWidth)).toBeLessThanOrEqual(0);
      await deleteUser.getByRole('button', { name: /^取\s*消$/ }).click();
      await admin.writeGuard.expectRequestCount('delete-user', 0);
      await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
      const adminRole = page.getByRole('article', { name: '管理员', exact: true });
      await expect(adminRole.getByRole('button', { name: '编辑权限', exact: true })).toBeDisabled();
      await expect(adminRole).not.toContainText(PERMISSIONS.PRODUCT_VIEW);
      await expectNoOverflow();
      await adminRole.getByRole('button', { name: '权限预览', exact: true }).click();
      const preview = page.getByRole('dialog', { name: '角色权限预览', exact: true });
      await expect(preview).toContainText('查看商品');
      await expect(preview).toContainText('轮换系统密钥');
      await expect(preview).not.toContainText(PERMISSIONS.SECURITY_KEY_ROTATE);
      await expectNoOverflow();
      await preview.getByRole('button', { name: /^关\s*闭$/ }).click();
      await page.getByRole('button', { name: '新建角色' }).click();
      const roleDialog = page.getByRole('dialog', { name: '新增自定义角色' });
      await expect(roleDialog).toBeVisible();
      await expect(roleDialog).not.toContainText(PERMISSIONS.PRODUCT_VIEW);
      await expect(roleDialog).not.toContainText('轮换系统密钥');
      await roleDialog.getByRole('textbox', { name: '搜索权限' }).fill('查看商品');
      await roleDialog.getByRole('checkbox', { name: '查看商品', exact: true }).check();
      await expect(roleDialog).toContainText('已选择 1 项');
      await expectNoOverflow();
      const roleBounds = await roleDialog.boundingBox();
      expect(roleBounds).not.toBeNull();
      if (!roleBounds) throw new Error('role dialog has no visible bounds');
      expect(roleBounds.x).toBeGreaterThanOrEqual(0);
      expect(roleBounds.y).toBeGreaterThanOrEqual(0);
      expect(roleBounds.x + roleBounds.width).toBeLessThanOrEqual(viewport.width);
      expect(roleBounds.y + roleBounds.height).toBeLessThanOrEqual(viewport.height);
      expect(await roleDialog.evaluate((node) => node.scrollWidth - node.clientWidth)).toBeLessThanOrEqual(0);
      await roleDialog.getByRole('button', { name: /^取\s*消$/ }).click();
      await page.getByRole('button', { name: '新建角色' }).click();
      await expect(roleDialog.getByLabel('角色名称')).toHaveValue('');
      await expect(roleDialog).toContainText('已选择 0 项');
      await roleDialog.getByRole('button', { name: /^取\s*消$/ }).click();
      await admin.writeGuard.expectRequestCount('create-role', 0);
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
    { code: 'admin', name: 'admin', description: '', permissions: Object.values(PERMISSIONS), enabled: true, system: true, memberCount: 1 },
    { code: 'operator', name: 'operator', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.PRODUCT_WRITE], enabled: true, system: true, memberCount: 1 },
    { code: 'readonly', name: 'readonly', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 0 },
    { code: 'reviewer', name: 'reviewer', description: '', permissions: [PERMISSIONS.OPERATION_TASK_REVIEW], enabled: true, system: true, memberCount: 0 },
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
  for (const [code, label] of [['admin', '管理员'], ['operator', '运营'], ['readonly', '只读'], ['reviewer', '审核']]) {
    const builtInRole = page.getByRole('article', { name: label, exact: true });
    await expect(builtInRole.getByText(label, { exact: true })).toBeVisible();
    await expect(builtInRole.getByText(code, { exact: true })).toHaveCount(0);
  }
  await page.getByRole('button', { name: '新建角色' }).click();
  const createDialog = page.getByRole('dialog', { name: '新增自定义角色' });
  await expect(createDialog.getByLabel('角色名称')).toHaveValue('');
  await createDialog.getByLabel('角色名称').fill('E2E 自定义运营');
  await expect(createDialog.getByRole('region', { name: '商品管理', exact: true })).toBeVisible();
  await expect(createDialog).not.toContainText(PERMISSIONS.PRODUCT_VIEW);
  await createDialog.getByRole('checkbox', { name: '查看商品', exact: true }).check();
  await expect(createDialog.getByRole('checkbox', { name: '查看商品', exact: true })).toBeChecked();
  await expect(createDialog).toContainText('已选择 1 项');
  await createDialog.getByRole('button', { name: '创建角色', exact: true }).click();

  const roleRow = page.getByRole('article', { name: 'E2E 自定义运营', exact: true });
  await expect(roleRow).toContainText('成员：2 人');
  await expect(roleRow).toContainText('商品管理');
  await expect(roleRow).not.toContainText(PERMISSIONS.PRODUCT_VIEW);
  await admin.writeGuard.expectRequestCount('create-role', 1);
  expect(admin.writeGuard.calls('create-role')[0].postDataJSON).toMatchObject({
    name: 'E2E 自定义运营', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true,
  });
  await roleRow.getByRole('button', { name: '权限预览', exact: true }).click();
  const previewDialog = page.getByRole('dialog', { name: '角色权限预览', exact: true });
  await expect(previewDialog).toContainText('查看商品');
  await expect(previewDialog).not.toContainText(PERMISSIONS.PRODUCT_VIEW);
  await previewDialog.getByRole('button', { name: /^关\s*闭$/ }).click();

  await page.getByRole('combobox', { name: '选择对比角色' }).click();
  await page.getByText('只读', { exact: true }).last().click();
  await expect(roleRow).toContainText(`相对于「只读」的权限差异`);
  await expect(roleRow).toContainText('权限一致');

  await roleRow.getByRole('button', { name: '编辑权限' }).click();
  const editDialog = page.getByRole('dialog', { name: '编辑角色与权限' });
  await expect(editDialog.getByLabel('角色名称')).toHaveValue('E2E 自定义运营');
  await editDialog.getByRole('checkbox', { name: '编辑商品', exact: true }).check();
  await expect(editDialog.getByRole('checkbox', { name: '编辑商品', exact: true })).toBeChecked();
  await editDialog.getByRole('button', { name: '保存修改', exact: true }).click();
  await admin.writeGuard.expectRequestCount('update-role', 1);
  expect(admin.writeGuard.calls('update-role')[0].postDataJSON).toMatchObject({
    permissions: expect.arrayContaining([PERMISSIONS.PRODUCT_WRITE]),
  });
  await expect(roleRow).toContainText('多出：编辑商品');
  await expect(roleRow).not.toContainText(PERMISSIONS.PRODUCT_WRITE);

  await roleRow.getByRole('button', { name: /停\s*用/ }).click();
  const disableDialog = page.getByRole('dialog', { name: '停用角色并立即撤权？' });
  await expect(disableDialog).toContainText('2 位成员');
  await expect(disableDialog.getByRole('button', { name: /^取\s*消$/ })).toBeVisible();
  await expect(disableDialog.getByRole('button', { name: '确认停用', exact: true })).toBeVisible();
  await disableDialog.getByRole('button', { name: '确认停用', exact: true }).click();
  await expect(roleRow).toContainText('已停用');
  await expect.poll(() => admin.writeGuard.calls('update-role').length).toBe(2);

  await roleRow.getByRole('button', { name: /启\s*用/ }).click();
  const enableDialog = page.getByRole('dialog', { name: '重新启用角色？' });
  await expect(enableDialog.getByRole('button', { name: /^取\s*消$/ })).toBeVisible();
  await expect(enableDialog.getByRole('button', { name: '确认启用', exact: true })).toBeVisible();
  await enableDialog.getByRole('button', { name: '确认启用', exact: true }).click();
  await expect(roleRow).toContainText('启用');
  await admin.writeGuard.expectRequestCount('update-role', 3);
  await admin.writeGuard.expectNoUnexpectedWrites();
});

test('@smoke preserves role permissions after a rejected save and prevents duplicate submission', async ({ admin, page }) => {
  await mockUsers(page);
  const roles: AdminRoleRow[] = [];
  await page.route('**/api/v1/admin/roles', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ list: roles }) });
  });
  await page.route('**/api/v1/admin/permissions', (route) => route.fulfill({ json: ok({
    permissions: [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.ORDER_VIEW],
  }) }));
  let release: (() => void) | undefined;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  let attempts = 0;
  admin.writeGuard.allow({ operation: 'create-role', method: 'POST', path: /^\/api\/v1\/admin\/roles$/, response: async (record) => {
    await pending;
    if (++attempts === 1) return fail('角色保存失败，请稍后重试', 50001);
    const body = record.postDataJSON as { name: string; permissions: string[]; enabled: boolean };
    const role: AdminRoleRow = { ...body, code: 'r_0123456789abcdef01234567', description: '', memberCount: 0, system: false };
    roles.push(role);
    return ok(role);
  } });
  await admin.goto('/settings/users');
  await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
  await page.getByRole('button', { name: '新建角色' }).click();
  const dialog = page.getByRole('dialog', { name: '新增自定义角色' });
  await dialog.getByRole('button', { name: '创建角色', exact: true }).click();
  await expect(dialog.getByText('请输入角色名称', { exact: true })).toBeVisible();
  await admin.writeGuard.expectRequestCount('create-role', 0);
  await dialog.getByLabel('角色名称').fill('E2E 商品与订单运营');
  await dialog.getByRole('checkbox', { name: '查看商品', exact: true }).check();
  await dialog.getByRole('textbox', { name: '搜索权限' }).fill('订单');
  await dialog.getByRole('checkbox', { name: '查看订单', exact: true }).check();
  await expect(dialog).toContainText('已选择 2 项');
  await dialog.getByRole('button', { name: '创建角色', exact: true }).dblclick();
  try {
    await admin.writeGuard.expectRequestCount('create-role', 1);
    await expect(dialog.getByRole('button', { name: '创建角色', exact: true })).toBeDisabled();
  } finally { release?.(); }
  await expect(page.getByText('角色保存失败，请稍后重试').first()).toBeVisible();
  await expect(dialog.getByLabel('角色名称')).toHaveValue('E2E 商品与订单运营');
  await expect(dialog).toContainText('已选择 2 项');
  await admin.writeGuard.expectRequestCount('create-role', 1);
  await dialog.getByRole('textbox', { name: '搜索权限' }).fill('');
  await expect(dialog.getByRole('checkbox', { name: '查看商品', exact: true })).toBeChecked();
  await dialog.getByRole('button', { name: '创建角色', exact: true }).click();
  await expect(page.getByRole('article', { name: 'E2E 商品与订单运营', exact: true })).toBeVisible();
  await admin.writeGuard.expectRequestCount('create-role', 2);
  expect(admin.writeGuard.calls('create-role')[1].postDataJSON).toMatchObject({
    name: 'E2E 商品与订单运营', enabled: true, permissions: [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.ORDER_VIEW],
  });
});
