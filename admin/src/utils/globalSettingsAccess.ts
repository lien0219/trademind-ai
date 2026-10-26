import { hasPermission, PERMISSIONS } from '@/utils/permission';

// UI-only precondition. Backend Root/tenant authorization is authoritative.
export function mayReadGlobalSettings(user?: API.CurrentUser): boolean {
  return Boolean(user && (user.isRoot === true || (user.tenantId === 0 && user.platformOperator === true)) && hasPermission(user.role, PERMISSIONS.SETTINGS_MANAGE, user.permissions));
}
