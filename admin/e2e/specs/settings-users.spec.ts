import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const managedUser = {
  id: 'e2e-managed-user',
  username: 'e2e-managed-user',
  email: 'managed-user@example.test',
  phone: '13000000000',
  displayName: 'E2E 运营用户',
  role: 'readonly',
  status: 'active',
  storePermissions: [],
  lastOperationAt: '2026-08-10T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

test.describe('@smoke Settings users', () => {
  test('changes a user role with themed modals and one write request', async ({ admin, page }) => {
    await page.route('**/api/v1/admin/users?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [managedUser], pagination: { total: 1 } })),
      });
    });
    await page.route('**/api/v1/admin/roles', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [
          { code: 'admin', name: '管理员', permissions: [], enabled: true, system: true, memberCount: 1 },
          { code: 'operator', name: '运营', permissions: [], enabled: true, system: true, memberCount: 1 },
          { code: 'readonly', name: '只读', permissions: [], enabled: true, system: true, memberCount: 1 },
          { code: 'reviewer', name: '审核', permissions: [], enabled: true, system: true, memberCount: 0 },
        ] })),
      });
    });
    await page.route('**/api/v1/admin/permissions', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ permissions: [] })) });
    });
    admin.writeGuard.allow({
      operation: 'change-user-role',
      method: 'PATCH',
      path: /^\/api\/v1\/admin\/users\/e2e-managed-user$/,
      response: ok({ ...managedUser, role: 'operator' }),
    });

    await admin.goto('/settings/users');
    await expect(page.getByText('E2E 运营用户')).toBeVisible();

    await page.getByRole('button', { name: '改角色' }).click();
    const roleDialog = page.getByRole('dialog', { name: '修改角色' });
    await expect(roleDialog).toBeVisible();
    await roleDialog.getByRole('button', { name: /^(取\s*消|Cancel)$/ }).click();
    await admin.writeGuard.expectRequestCount('change-user-role', 0);

    await page.getByRole('button', { name: '改角色' }).click();
    const reopenedRoleDialog = page.getByRole('dialog', { name: '修改角色' });
    await reopenedRoleDialog.getByText('只读', { exact: true }).click();
    await page.getByText('运营', { exact: true }).last().click();
    await reopenedRoleDialog.getByRole('button', { name: '确认修改' }).click();

    const sensitiveDialog = page.getByRole('dialog', { name: '修改用户角色' });
    await expect(sensitiveDialog).toContainText('E2E 运营用户');
    await sensitiveDialog.getByRole('button', { name: '确认执行' }).click();

    await admin.writeGuard.expectRequestCount('change-user-role', 1);
    expect(admin.writeGuard.calls('change-user-role')[0]).toMatchObject({
      method: 'PATCH',
      path: '/api/v1/admin/users/e2e-managed-user',
      postDataJSON: { role: 'operator' },
    });
    await expectNoRootOverflow(page);
  });
});
