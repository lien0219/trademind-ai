import type { MenuDataItem } from '@umijs/route-utils';
import {
  hasPermission,
  normalizeRole,
  PERMISSIONS,
  type PermissionKey,
} from '@/utils/permission';

/** Route path → minimum view permission (undefined = all authenticated users). */
export const ROUTE_PERMISSIONS: Record<string, PermissionKey | PermissionKey[]> = {
  '/dashboard': PERMISSIONS.PRODUCT_VIEW,
  '/system/operation-logs': PERMISSIONS.OPERATIONLOG_VIEW,
  '/ops/workers/monitor': PERMISSIONS.TASK_RETRY,
  '/ops/task-center/failures': PERMISSIONS.TASK_RETRY,
  '/ops/task-center/alerts': PERMISSIONS.TASK_RETRY,
  '/ops/platform-runtime': PERMISSIONS.STORE_VIEW,
  '/files': PERMISSIONS.PRODUCT_VIEW,
  '/ai/prompts': PERMISSIONS.SETTINGS_MANAGE,
  '/ai/tasks': PERMISSIONS.PRODUCT_VIEW,
  '/ai/image-tasks': PERMISSIONS.PRODUCT_VIEW,
  '/ai/text-batches': PERMISSIONS.PRODUCT_VIEW,
  '/ai/image-batches': PERMISSIONS.PRODUCT_VIEW,
  '/ai/operation-workbench': PERMISSIONS.PRODUCT_VIEW,
  '/product/drafts': PERMISSIONS.PRODUCT_VIEW,
  '/product/publish-tasks': PERMISSIONS.PRODUCT_VIEW,
  '/collect/hub': PERMISSIONS.PRODUCT_VIEW,
  '/collect/tasks': PERMISSIONS.PRODUCT_VIEW,
  '/collect/batches': PERMISSIONS.PRODUCT_VIEW,
  '/collect/browser-profiles': PERMISSIONS.SETTINGS_MANAGE,
  '/collect/rules': PERMISSIONS.SETTINGS_MANAGE,
  '/collect/monitor': PERMISSIONS.SETTINGS_MANAGE,
  '/shops/manage': PERMISSIONS.STORE_VIEW,
  '/orders/list': PERMISSIONS.ORDER_VIEW,
  '/orders/sync-tasks': PERMISSIONS.ORDER_VIEW,
  '/orders/sku-matches': PERMISSIONS.ORDER_VIEW,
  '/orders/exceptions': PERMISSIONS.ORDER_VIEW,
  '/inventory': PERMISSIONS.INVENTORY_VIEW,
  '/inventory/alerts': PERMISSIONS.INVENTORY_VIEW,
  '/inventory/deductions': PERMISSIONS.INVENTORY_VIEW,
  '/inventory/sync-tasks': PERMISSIONS.INVENTORY_VIEW,
  '/inventory/sync-batches': PERMISSIONS.INVENTORY_VIEW,
  '/inventory/logs': PERMISSIONS.INVENTORY_VIEW,
  '/customer/hub': PERMISSIONS.CUSTOMER_VIEW,
  '/customer/conversations': PERMISSIONS.CUSTOMER_VIEW,
  '/customer/message-sync-tasks': PERMISSIONS.CUSTOMER_VIEW,
  '/settings/system': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/security': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/email': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/alert-notify': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/storage': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/ai': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/image': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/collector': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/inventory': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/pricing': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/platforms': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/platform-publish': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/integrations': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/config-status': PERMISSIONS.SETTINGS_MANAGE,
  '/settings/users': PERMISSIONS.USER_MANAGE,
};

function routePerm(path?: string): PermissionKey | PermissionKey[] | undefined {
  if (!path) return undefined;
  const exact = ROUTE_PERMISSIONS[path];
  if (exact) return exact;
  const sorted = Object.keys(ROUTE_PERMISSIONS).sort((a, b) => b.length - a.length);
  for (const key of sorted) {
    if (path.startsWith(key)) return ROUTE_PERMISSIONS[key];
  }
  return undefined;
}

function canAccessRoute(
  path: string | undefined,
  role?: string | null,
  profilePerms?: string[],
): boolean {
  const perm = routePerm(path);
  if (!perm) return true;
  const list = Array.isArray(perm) ? perm : [perm];
  return list.some((p) => hasPermission(role, p, profilePerms));
}

/** Filter sidebar menu tree by RBAC (F6 menu-level permission). */
export function filterMenuByPermission(
  menus: MenuDataItem[],
  role?: string | null,
  profilePerms?: string[],
): MenuDataItem[] {
  const r = normalizeRole(role);

  const walk = (items: MenuDataItem[]): MenuDataItem[] =>
    items
      .map((item) => {
        if (item.hideInMenu) return item;
        const path = item.path || item.redirect;
        if (path && !canAccessRoute(path, r, profilePerms)) {
          return null;
        }
        const children = item.children ? walk(item.children) : undefined;
        if (children && item.children?.length && children.length === 0 && !item.component) {
          return null;
        }
        return { ...item, children: children?.length ? children : item.children };
      })
      .filter(Boolean) as MenuDataItem[];

  return walk(menus);
}

export function canAccessPath(
  pathname: string,
  role?: string | null,
  profilePerms?: string[],
): boolean {
  return canAccessRoute(pathname, role, profilePerms);
}
