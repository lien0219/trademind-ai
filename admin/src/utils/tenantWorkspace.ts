import type { AuthTenant, Tenant } from '@/services/tenants';

export const PLATFORM_WORKSPACE_PATH = '/platform/tenants' as const;
export const ENTERPRISE_WORKSPACE_PATH = '/dashboard/product-operations' as const;

// The backend decides whether an identity can enter the requested tenant.
// Client-side lists only prevent misleading navigation and never grant access.
export function canEnterTenantWorkspace(
  tenant: Pick<Tenant, 'id' | 'status'>,
  authorized: readonly Pick<AuthTenant, 'id' | 'status'>[],
): boolean {
  return tenant.id > 0 && tenant.status === 'active' &&
    authorized.some((member) => member.id === tenant.id && member.status === 'active');
}

export function workspaceLandingPath(currentTenantId: number, targetTenantId: number) {
  if (targetTenantId === 0) return PLATFORM_WORKSPACE_PATH;
  if (currentTenantId === 0 && targetTenantId > 0) return ENTERPRISE_WORKSPACE_PATH;
  return undefined;
}
