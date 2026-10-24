import type { Page } from '@playwright/test';
import type { AdminRoleRow, AdminUserRow } from '../../src/services/adminUsers';
import { PERMISSIONS } from '../../src/utils/permission';
import { e2eUser } from './auth';
import { ok, paged } from './envelope';

export function systemRoles(): AdminRoleRow[] {
  return [
    { code: 'admin', name: 'admin', description: '', permissions: Object.values(PERMISSIONS), enabled: true, system: true, memberCount: 1 },
    { code: 'operator', name: 'operator', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 1 },
    { code: 'readonly', name: 'readonly', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 0 },
  ];
}

export async function mockUsers(page: Page, users: AdminUserRow[] = [], roles = systemRoles()) {
  await page.route('**/api/v1/admin/users?**', (route) => route.fulfill({ json: ok(paged(users)) }));
  await page.route('**/api/v1/admin/roles', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ list: roles }) });
  });
  await page.route('**/api/v1/admin/permissions', (route) => route.fulfill({ json: ok({ permissions: [PERMISSIONS.PRODUCT_VIEW] }) }));
}

export function testUser(): AdminUserRow {
  return { ...e2eUser, id: 'e2e-operator', email: 'operator@example.test', displayName: 'E2E 运营', role: 'operator' };
}
