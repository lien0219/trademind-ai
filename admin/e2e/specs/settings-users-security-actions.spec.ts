import { test, expect } from '../fixtures/admin.fixture';
import { mockUsers, systemRoles, testUser } from '../mocks/admin-users';
import { e2eUser } from '../mocks/auth';
import { fail, ok } from '../mocks/envelope';
import type { AdminRoleRow } from '../../src/services/adminUsers';

function customRole(memberCount = 0): AdminRoleRow {
  return { code: 'r_0123456789abcdef01234567', name: 'E2E 自定义角色', description: '', permissions: [], enabled: true, system: false, memberCount };
}

test('@smoke validates passwords, generates and copies securely, and clears cancelled secrets', async ({ admin, page }) => {
  await mockUsers(page);
  await page.addInitScript(() => {
    const copied: string[] = [];
    Object.defineProperty(window, 'e2eCopiedPasswords', { value: copied });
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { copied.push(value); } } });
  });
  admin.writeGuard.allow({ operation: 'create-user', method: 'POST', path: /^\/api\/v1\/admin\/users$/, response: fail('邮箱或手机号已被使用', 40001) });
  await admin.goto('/settings/users');
  await page.getByRole('button', { name: '新建用户' }).click();
  const dialog = page.getByRole('dialog', { name: '新建用户' });
  await dialog.getByLabel('邮箱').fill('e2e-new@example.test');
  await expect(dialog.getByRole('button', { name: '复制密码' })).toBeDisabled();
  for (const [value, hint] of [
    ['中文密码Abcdef123456', '密码仅支持英文字母、数字和英文符号，不能包含中文或空格'],
    ['invalid pass123', '密码仅支持英文字母、数字和英文符号，不能包含中文或空格'],
    ['123456789012', '密码须同时包含英文字母和数字'],
  ]) {
    await dialog.getByLabel('初始密码').fill(value);
    await dialog.getByRole('button', { name: /确\s*定|OK/ }).click();
    await expect(dialog.getByText(hint, { exact: true })).toBeVisible();
    await admin.writeGuard.expectRequestCount('create-user', 0);
  }
  await dialog.getByRole('button', { name: '随机生成密码' }).click();
  const password = await dialog.getByLabel('初始密码').inputValue();
  expect(password).toMatch(/^[\x21-\x7e]{20}$/);
  expect(password).toMatch(/[A-Z]/);
  expect(password).toMatch(/[a-z]/);
  expect(password).toMatch(/[0-9]/);
  expect(password).toMatch(/[^A-Za-z0-9]/);
  await dialog.getByRole('button', { name: '复制密码' }).click();
  await expect(page.getByText('密码已复制，请妥善保管', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => (window as Window & { e2eCopiedPasswords: string[] }).e2eCopiedPasswords)).toEqual([password]);
  await admin.writeGuard.expectRequestCount('create-user', 0);
  await dialog.getByRole('button', { name: /确\s*定|OK/ }).click();
  await expect(page.getByText('邮箱或手机号已被使用').first()).toBeVisible();
  await expect(dialog.getByLabel('初始密码')).toHaveValue(password);
  await admin.writeGuard.expectRequestCount('create-user', 1);
  expect(admin.writeGuard.calls('create-user')[0].postDataJSON).toMatchObject({ password, email: 'e2e-new@example.test' });
  await dialog.getByRole('button', { name: /^取\s*消$/ }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole('button', { name: '新建用户' }).click();
  await expect(dialog.getByLabel('初始密码')).toHaveValue('');
  await expect(dialog.getByRole('button', { name: '复制密码' })).toBeDisabled();
});

test('@smoke deletes a user only after confirmation and prevents repeated deletion', async ({ admin, page }) => {
  const users = [e2eUser, testUser()];
  await mockUsers(page, users);
  let release: (() => void) | undefined;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  admin.writeGuard.allow({ operation: 'delete-user', method: 'DELETE', path: /^\/api\/v1\/admin\/users\/e2e-operator$/, response: async () => {
    await pending;
    users.splice(1, 1);
    return ok({ deleted: true });
  } });
  await admin.goto('/settings/users');
  await expect(page.getByRole('row', { name: /E2E 管理员/ }).getByRole('button', { name: '删除用户' })).toBeDisabled();
  const row = page.getByRole('row', { name: /E2E 运营/ });
  await row.getByRole('button', { name: '删除用户' }).click();
  const dialog = page.getByRole('dialog', { name: '删除用户', exact: true });
  await expect(dialog).toContainText('E2E 运营');
  await expect(dialog).toContainText('历史业务数据与操作记录会保留');
  await dialog.getByRole('button', { name: /^取\s*消$/ }).click();
  await admin.writeGuard.expectRequestCount('delete-user', 0);
  await row.getByRole('button', { name: '删除用户' }).click();
  try {
    await dialog.getByRole('button', { name: '确认删除用户', exact: true }).dblclick();
    await admin.writeGuard.expectRequestCount('delete-user', 1);
    await expect(dialog.getByRole('button', { name: '确认删除用户', exact: true })).toBeDisabled();
    await expect(dialog.getByRole('button', { name: /^取\s*消$/ })).toBeDisabled();
  } finally { release?.(); }
  await expect(dialog).not.toBeVisible();
  await expect(row).toHaveCount(0);
  await admin.writeGuard.expectRequestCount('delete-user', 1);
  expect(admin.writeGuard.calls('delete-user')[0].postDataJSON).toBeNull();
});

test('@smoke deletes only empty custom roles and protects system roles', async ({ admin, page }) => {
  const empty = customRole();
  const occupied = { ...customRole(1), code: 'r_abcdef0123456789abcdef01', name: 'E2E 有成员角色' };
  const roles = [...systemRoles(), empty, occupied];
  await mockUsers(page, [], roles);
  let release: (() => void) | undefined;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  admin.writeGuard.allow({ operation: 'delete-role', method: 'DELETE', path: new RegExp(`^/api/v1/admin/roles/${empty.code}$`), response: async () => {
    await pending;
    roles.splice(roles.indexOf(empty), 1);
    return ok({ deleted: true });
  } });
  await admin.goto('/settings/users');
  await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
  await expect(page.getByRole('article', { name: '管理员', exact: true }).getByRole('button', { name: '删除角色' })).toHaveCount(0);
  await expect(page.getByRole('article', { name: occupied.name, exact: true }).getByRole('button', { name: '删除角色' })).toBeDisabled();
  const role = page.getByRole('article', { name: empty.name, exact: true });
  await role.getByRole('button', { name: '删除角色' }).click();
  const dialog = page.getByRole('dialog', { name: '删除角色', exact: true });
  await dialog.getByRole('button', { name: /^取\s*消$/ }).click();
  await admin.writeGuard.expectRequestCount('delete-role', 0);
  await role.getByRole('button', { name: '删除角色' }).click();
  try {
    await dialog.getByRole('button', { name: '确认删除角色', exact: true }).dblclick();
    await admin.writeGuard.expectRequestCount('delete-role', 1);
    await expect(dialog.getByRole('button', { name: '确认删除角色', exact: true })).toBeDisabled();
  } finally { release?.(); }
  await expect(role).toHaveCount(0);
  await expect(dialog).not.toBeVisible();
  await admin.writeGuard.expectRequestCount('delete-role', 1);
});

for (const scenario of [
  { kind: 'user', status: 403, hint: '当前账号无权限执行此操作', error: 'Forbidden' },
  { kind: 'user', status: 500, hint: '删除失败，请稍后重试', error: 'Internal Server Error' },
  { kind: 'role', status: 409, hint: '此角色仍有关联用户，请先调整成员角色后再删除', error: 'Conflict' },
  { kind: 'role', status: 500, hint: '删除失败，请稍后重试', error: 'Internal Server Error' },
]) {
  test(`@smoke preserves the ${scenario.kind} after deletion returns ${scenario.status}`, async ({ admin, page }) => {
    const role = customRole();
    await mockUsers(page, [testUser()], [...systemRoles(), role]);
    const label = scenario.kind === 'user' ? '用户' : '角色';
    admin.consoleGuard.allowError(new RegExp(`^Failed to load resource: the server responded with a status of ${scenario.status} \\(${scenario.error}\\)$`));
    admin.writeGuard.allow({ operation: 'delete-denied', method: 'DELETE', path: scenario.kind === 'user'
      ? /^\/api\/v1\/admin\/users\/e2e-operator$/ : new RegExp(`^/api/v1/admin/roles/${role.code}$`),
    status: scenario.status, response: fail(scenario.hint, scenario.status === 500 ? 50001 : scenario.status === 403 ? 40306 : 40001) });
    await admin.goto('/settings/users');
    if (scenario.kind === 'role') await page.getByRole('tab', { name: '角色与权限矩阵' }).click();
    await page.getByRole('button', { name: `删除${label}`, exact: true }).click();
    const dialog = page.getByRole('dialog', { name: `删除${label}`, exact: true });
    await dialog.getByRole('button', { name: `确认删除${label}`, exact: true }).click();
    await expect(page.getByText(scenario.hint).first()).toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button', { name: `确认删除${label}`, exact: true })).toBeEnabled();
    await expect(scenario.kind === 'user' ? page.getByRole('row', { name: /E2E 运营/ }) : page.getByRole('article', { name: role.name, exact: true })).toBeVisible();
    await admin.writeGuard.expectRequestCount('delete-denied', 1);
    await dialog.getByRole('button', { name: /^取\s*消$/ }).click();
    await admin.writeGuard.expectRequestCount('delete-denied', 1);
  });
}
