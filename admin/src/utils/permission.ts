export const ROLES = {
  ADMIN: 'admin',
  OPERATOR: 'operator',
  READONLY: 'readonly',
  REVIEWER: 'reviewer',
} as const;

export type AdminRole = (typeof ROLES)[keyof typeof ROLES];

export const PERMISSIONS = {
  PRODUCT_VIEW: 'product.view',
  PRODUCT_WRITE: 'product.write',
  AI_TEXT_APPLY: 'ai_text.apply',
  AI_IMAGE_APPLY: 'ai_image.apply',
  PUBLISH_CREATE_DRAFT: 'publish.create_draft',
  ORDER_VIEW: 'order.view',
  ORDER_OPERATE: 'order.operate',
  SKU_BIND: 'sku.bind',
  INVENTORY_VIEW: 'inventory.view',
  INVENTORY_OPERATE: 'inventory.operate',
  INVENTORY_APPROVE: 'inventory.approve',
  CUSTOMER_VIEW: 'customer.view',
  CUSTOMER_OPERATE: 'customer.operate',
  TASK_RETRY: 'task.retry',
  OPERATION_TASK_AUDIT_READ: 'operationtask.audit.read',
  OPERATION_TASK_EDIT: 'operationtask.edit',
  OPERATION_TASK_EXECUTE: 'operationtask.execute',
  OPERATION_TASK_REVIEW: 'operationtask.review',
  OPERATION_TASK_RETRY: 'operationtask.retry',
  SETTINGS_MANAGE: 'settings.manage',
  USER_MANAGE: 'user.manage',
  OPERATIONLOG_VIEW: 'operationlog.view',
  STORE_VIEW: 'store.view',
  STORE_OPERATE: 'store.operate',
  WAREHOUSE_VIEW: 'warehouse.view',
  WAREHOUSE_MANAGE: 'warehouse.manage',
  SUPPLIER_VIEW: 'supplier.view',
  SUPPLIER_MANAGE: 'supplier.manage',
  PROCUREMENT_VIEW: 'procurement.view',
  PROCUREMENT_MANAGE: 'procurement.manage',
  PROCUREMENT_APPROVE: 'procurement.approve',
  PROCUREMENT_RECEIVE: 'procurement.receive',
  PROCUREMENT_RETURN: 'procurement.return',
  PII_READ_FULL: 'pii.read_full',
  OBSERVABILITY_READ: 'observability.read',
  ALERTS_READ: 'alerts.read',
  ALERTS_ACK: 'alerts.ack',
  ALERTS_SILENCE: 'alerts.silence',
  CONFIG_MANAGE: 'config.manage',
  INVENTORY_SYNC_READ: 'inventory_sync.read',
  INVENTORY_SYNC_RUN: 'inventory_sync.run',
} as const;

export type PermissionKey = (typeof PERMISSIONS)[keyof typeof PERMISSIONS];

const ROLE_PERMISSIONS: Record<string, PermissionKey[]> = {
  admin: Object.values(PERMISSIONS),
  operator: [
    PERMISSIONS.PRODUCT_VIEW,
    PERMISSIONS.PRODUCT_WRITE,
    PERMISSIONS.AI_TEXT_APPLY,
    PERMISSIONS.AI_IMAGE_APPLY,
    PERMISSIONS.PUBLISH_CREATE_DRAFT,
    PERMISSIONS.ORDER_VIEW,
    PERMISSIONS.ORDER_OPERATE,
    PERMISSIONS.SKU_BIND,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.INVENTORY_OPERATE,
    PERMISSIONS.CUSTOMER_VIEW,
    PERMISSIONS.CUSTOMER_OPERATE,
    PERMISSIONS.TASK_RETRY,
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATION_TASK_EDIT,
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.STORE_VIEW,
    PERMISSIONS.STORE_OPERATE,
    PERMISSIONS.OBSERVABILITY_READ,
    PERMISSIONS.ALERTS_READ,
    PERMISSIONS.INVENTORY_SYNC_READ,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.INVENTORY_APPROVE,
    PERMISSIONS.INVENTORY_SYNC_RUN,
    PERMISSIONS.WAREHOUSE_VIEW,
    PERMISSIONS.WAREHOUSE_MANAGE,
    PERMISSIONS.SUPPLIER_VIEW,
    PERMISSIONS.SUPPLIER_MANAGE,
    PERMISSIONS.PROCUREMENT_VIEW,
    PERMISSIONS.PROCUREMENT_MANAGE,
    PERMISSIONS.PROCUREMENT_RECEIVE,
    PERMISSIONS.PROCUREMENT_RETURN,
  ],
  reviewer: [
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATION_TASK_REVIEW,
    PERMISSIONS.OPERATION_TASK_EXECUTE,
    PERMISSIONS.OPERATION_TASK_RETRY,
    PERMISSIONS.INVENTORY_SYNC_READ,
    PERMISSIONS.WAREHOUSE_VIEW,
    PERMISSIONS.SUPPLIER_VIEW,
    PERMISSIONS.PROCUREMENT_VIEW,
    PERMISSIONS.PROCUREMENT_APPROVE,
  ],
  readonly: [
    PERMISSIONS.PRODUCT_VIEW,
    PERMISSIONS.ORDER_VIEW,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.CUSTOMER_VIEW,
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.STORE_VIEW,
    PERMISSIONS.OBSERVABILITY_READ,
    PERMISSIONS.ALERTS_READ,
    PERMISSIONS.INVENTORY_SYNC_READ,
    PERMISSIONS.WAREHOUSE_VIEW,
    PERMISSIONS.SUPPLIER_VIEW,
    PERMISSIONS.PROCUREMENT_VIEW,
  ],
};

export function normalizeRole(role?: string | null): AdminRole {
  const r = (role || '').trim().toLowerCase();
  if (r === ROLES.OPERATOR || r === ROLES.READONLY || r === ROLES.REVIEWER) return r;
  return ROLES.ADMIN;
}

export function permissionsForRole(role?: string | null, fromProfile?: string[]): PermissionKey[] {
  if (fromProfile && fromProfile.length > 0) {
    return fromProfile as PermissionKey[];
  }
  return ROLE_PERMISSIONS[normalizeRole(role)] || ROLE_PERMISSIONS.admin;
}

export function hasPermission(
  role: string | undefined | null,
  perm: PermissionKey,
  fromProfile?: string[],
): boolean {
  return permissionsForRole(role, fromProfile).includes(perm);
}

export function isReadonly(role?: string | null): boolean {
  return normalizeRole(role) === ROLES.READONLY;
}

export function canWriteOrders(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.ORDER_OPERATE, perms);
}

export function canWriteInventory(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.INVENTORY_OPERATE, perms);
}

export function canWriteCustomer(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.CUSTOMER_OPERATE, perms);
}

export function canManageSettings(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.SETTINGS_MANAGE, perms);
}

export function canManageUsers(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.USER_MANAGE, perms);
}

export function canRetryTasks(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.TASK_RETRY, perms);
}

export function canReadOperationTasks(role?: string | null, perms?: string[]): boolean {
  return hasPermission(role, PERMISSIONS.OPERATION_TASK_AUDIT_READ, perms);
}

export const PERMISSION_DENIED_MESSAGE = '当前账号无权限访问此页面';
export const READONLY_DENIED_MESSAGE = '只读账号不可执行写操作';
