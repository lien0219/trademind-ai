import type { Page } from '@playwright/test';
import { e2eUser } from './auth';
import { ok } from './envelope';
import { PERMISSIONS } from '../../src/utils/permission';

export const E2E_TENANT_B_TOKEN = 'e2e-tenant-b-session-token';
export const e2eSecondTenant = { id: 102, code: 'e2e-beta', name: 'E2E 第二企业', status: 'active', role: 'operator' };

export async function mockTenantSwitchReads(page: Page) {
  await mockTenantReads(page);
  await page.route('**/api/v1/auth/profile', (route) => {
    const second = route.request().headers().authorization === `Bearer ${E2E_TENANT_B_TOKEN}`;
    return route.fulfill({ json: ok({
      ...e2eUser,
      tenantId: second ? e2eSecondTenant.id : e2eTenant.id,
      tenantName: second ? e2eSecondTenant.name : e2eTenant.name,
      role: second ? 'operator' : 'admin',
    }) });
  });
  await page.route('**/api/v1/auth/tenants', (route) => route.fulfill({ json: ok({
    currentTenantId: route.request().headers().authorization === `Bearer ${E2E_TENANT_B_TOKEN}` ? e2eSecondTenant.id : e2eTenant.id,
    items: [{ ...e2eTenant, role: 'admin' }, e2eSecondTenant],
  }) }));
}

export const e2eTenant = {
  id: 101,
  code: 'e2e-acme',
  name: 'E2E 示例企业',
  status: 'active' as const,
  description: '租户管理回归企业',
  legalName: 'E2E 示例企业有限公司',
  contactEmail: 'owner@example.test',
  contactPhone: '13800000000',
  ownerUserId: e2eUser.id,
  version: 1,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

export const e2eShop = {
  id: 'e2e-shop-1',
  platform: 'douyin_shop',
  shopName: 'E2E 抖店',
  shopCode: 'e2e-shop',
  status: 'active',
  authStatus: 'connected',
  updatedAt: '2026-01-02T00:00:00Z',
};

export const e2eTenantMember = {
  id: 'e2e-membership-1',
  userId: e2eUser.id,
  email: e2eUser.email,
  displayName: e2eUser.displayName,
  role: 'admin',
  status: 'active',
  storePermissions: [],
};

export const e2eTenantInvitation = {
  id: 'e2e-invitation-1',
  email: 'pending@example.test',
  role: 'operator',
  status: 'pending',
  expiresAt: '2026-01-08T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
};

export async function mockTenantReads(
  page: Page,
  profile: { permissions?: string[]; platformOperator?: boolean } = {},
) {
  await page.route('**/api/v1/auth/profile', (route) => route.fulfill({
    json: ok({
      ...e2eUser,
      permissions: profile.permissions || e2eUser.permissions,
      platformOperator: profile.platformOperator === true,
    }),
  }));
  await page.route('**/api/v1/platform/tenants**', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ items: [e2eTenant], total: 1 }) });
  });
  await page.route('**/api/v1/platform/tenants/101/audit', (route) => route.fulfill({
    json: ok({ items: [{
      id: 'e2e-audit-1',
      action: 'tenant.create',
      status: 'success',
      message: '企业已创建',
      createdAt: '2026-01-01T00:00:00Z',
    }] }),
  }));
  await page.route('**/api/v1/platform/tenants/101', (route) => route.fulfill({
    json: ok({ tenant: e2eTenant, memberCount: 1, shopCount: 1 }),
  }));
  await page.route('**/api/v1/tenant/current', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ tenant: e2eTenant, memberCount: 1, shopCount: 1 }) });
  });
  await page.route('**/api/v1/tenant/members', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ items: [e2eTenantMember] }) });
  });
  await page.route('**/api/v1/tenant/invitations', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ items: [e2eTenantInvitation] }) });
  });
  await page.route('**/api/v1/admin/roles', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ list: [
      { code: 'admin', name: '管理员', description: '', permissions: Object.values(PERMISSIONS), enabled: true, system: true, memberCount: 1 },
      { code: 'operator', name: '运营', description: '', permissions: [PERMISSIONS.PRODUCT_VIEW], enabled: true, system: true, memberCount: 0 },
    ] }) });
  });
  await page.route('**/api/v1/shops**', (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: ok({ list: [e2eShop], pagination: { page: 1, pageSize: 200, total: 1, totalPages: 1 } }) });
  });
}
