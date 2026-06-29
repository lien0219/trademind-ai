import { useModel } from '@umijs/max';
import {
  canManageSettings,
  canManageUsers,
  canRetryTasks,
  canWriteCustomer,
  canWriteInventory,
  canWriteOrders,
  hasPermission,
  isReadonly,
  normalizeRole,
  permissionsForRole,
  type PermissionKey,
} from '@/utils/permission';

export function usePermission() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const user = initialState?.currentUser;
  const role = user?.role;
  const perms = permissionsForRole(role, user?.permissions);

  return {
    user,
    role: normalizeRole(role),
    permissions: perms,
    readonly: isReadonly(role),
    can: (perm: PermissionKey) => hasPermission(role, perm, user?.permissions),
    canWriteOrders: canWriteOrders(role, user?.permissions),
    canWriteInventory: canWriteInventory(role, user?.permissions),
    canWriteCustomer: canWriteCustomer(role, user?.permissions),
    canManageSettings: canManageSettings(role, user?.permissions),
    canManageUsers: canManageUsers(role, user?.permissions),
    canRetryTasks: canRetryTasks(role, user?.permissions),
    storePermissions: user?.storePermissions || [],
  };
}
