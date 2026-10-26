import { hasPermission, isReadonly, PERMISSIONS } from '@/utils/permission';
// UI preflight only: backend checks tenant membership and store grants.
export function canAdministerShops(user?: API.CurrentUser): boolean {
  return Boolean(user && (user.tenantId ?? 0) > 0 && user.role?.trim().toLowerCase() === 'admin');
}
export function canOperateShop(user: API.CurrentUser | undefined, shopId: string): boolean {
  if (!user || (user.tenantId ?? 0) <= 0 || isReadonly(user.role) || !hasPermission(user.role, PERMISSIONS.STORE_OPERATE, user.permissions)) return false;
  if (user.role?.trim().toLowerCase() === 'admin') return true;
  return (user.storePermissions ?? []).some((g) => g.storeId === shopId && ['operate', 'manage'].includes(g.permissionScope.trim().toLowerCase()));
}
