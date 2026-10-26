import type { Tenant } from '@/services/tenants';

/** Display-only check. The backend is authoritative for leave authorization. */
export function mayLeaveTenant(
  user: API.CurrentUser | undefined,
  tenant: Pick<Tenant, 'id' | 'status' | 'ownerUserId'> | null,
): boolean {
  if (!user?.id || user.tenantId == null || user.tenantId <= 0 || !tenant) return false;
  if (tenant.id !== user.tenantId || tenant.status !== 'active') return false;
  if (user.isRoot === true || tenant.ownerUserId === user.id) return false;
  const role = user.role?.trim().toLowerCase();
  if (!role || role === 'admin' || role === 'owner') return false;
  if (user.permissions?.includes('user.manage')) return false;
  return true;
}
