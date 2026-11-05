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
  LOGISTICS_VIEW: 'logistics.view',
  LOGISTICS_MANAGE: 'logistics.manage',
  SUPPLIER_VIEW: 'supplier.view',
  SUPPLIER_MANAGE: 'supplier.manage',
  PROCUREMENT_VIEW: 'procurement.view',
  PROCUREMENT_MANAGE: 'procurement.manage',
  PROCUREMENT_APPROVE: 'procurement.approve',
  PROCUREMENT_RECEIVE: 'procurement.receive',
  PROCUREMENT_RETURN: 'procurement.return',
  SALES_RETURN_VIEW: 'sales_return.view',
  SALES_RETURN_MANAGE: 'sales_return.manage',
  SALES_RETURN_APPROVE: 'sales_return.approve',
  SALES_RETURN_RECEIVE: 'sales_return.receive',
  SALES_RETURN_REFUND: 'sales_return.refund',
  AUDIT_READ: 'audit.read',
  AUDIT_EXPORT: 'audit.export',
  CONFIG_READ: 'config.read',
  EXPORT_CREATE: 'export.create',
  EXPORT_READ: 'export.read',
  ORDER_PROFIT_VIEW: 'order_profit.view',
  ORDER_PROFIT_EXPORT: 'order_profit.export',
  SETTLEMENT_VIEW: 'settlement.view',
  SETTLEMENT_IMPORT: 'settlement.import',
  SETTLEMENT_EXPORT: 'settlement.export',
  WAREHOUSE_FEE_VIEW: 'warehouse_fee.view',
  WAREHOUSE_FEE_MANAGE: 'warehouse_fee.manage',
  ADVERTISING_FEE_VIEW: 'advertising_fee.view',
  ADVERTISING_FEE_IMPORT: 'advertising_fee.import',
  ADVERTISING_FEE_MANAGE: 'advertising_fee.manage',
  FREIGHT_FEE_VIEW: 'freight_fee.view',
  FREIGHT_FEE_IMPORT: 'freight_fee.import',
  FREIGHT_FEE_MANAGE: 'freight_fee.manage',
  PII_READ_FULL: 'pii.read_full',
  PII_READ_MASKED: 'pii.read_masked',
  PII_EXPORT: 'pii.export',
  SECURITY_SESSION_MANAGE: 'security.session.manage',
  SECURITY_KEY_ROTATE: 'security.key.rotate',
  OBSERVABILITY_READ: 'observability.read',
  OBSERVABILITY_MANAGE: 'observability.manage',
  ALERTS_READ: 'alerts.read',
  ALERTS_ACK: 'alerts.ack',
  ALERTS_SILENCE: 'alerts.silence',
  SLO_READ: 'slo.read',
  SLO_MANAGE: 'slo.manage',
  CONFIG_MANAGE: 'config.manage',
  INVENTORY_SYNC_READ: 'inventory_sync.read',
  INVENTORY_SYNC_RUN: 'inventory_sync.run',
  INVENTORY_SYNC_RERUN: 'inventory_sync.rerun',
  INVENTORY_SYNC_AUDIT_READ: 'inventory_sync.audit.read',
  INVENTORY_SNAPSHOT_READ: 'inventory_snapshot.read',
  SKU_BINDING_READ: 'sku_binding.read',
  SKU_BINDING_MANAGE: 'sku_binding.manage',
  SKU_BINDING_RESOLVE_MANUAL: 'sku_binding.resolve_manual',
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
    PERMISSIONS.SECURITY_SESSION_MANAGE,
    PERMISSIONS.PII_READ_MASKED,
    PERMISSIONS.AUDIT_READ,
    PERMISSIONS.CONFIG_READ,
    PERMISSIONS.OBSERVABILITY_READ,
    PERMISSIONS.ALERTS_READ,
    PERMISSIONS.SLO_READ,
    PERMISSIONS.INVENTORY_SYNC_READ,
    PERMISSIONS.INVENTORY_SYNC_RUN,
    PERMISSIONS.INVENTORY_SYNC_RERUN,
    PERMISSIONS.INVENTORY_SNAPSHOT_READ,
    PERMISSIONS.SKU_BINDING_READ,
    PERMISSIONS.SKU_BINDING_MANAGE,
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.STORE_VIEW,
    PERMISSIONS.STORE_OPERATE,
    PERMISSIONS.WAREHOUSE_VIEW,
    PERMISSIONS.WAREHOUSE_MANAGE,
    PERMISSIONS.LOGISTICS_VIEW,
    PERMISSIONS.LOGISTICS_MANAGE,
    PERMISSIONS.SUPPLIER_VIEW,
    PERMISSIONS.SUPPLIER_MANAGE,
    PERMISSIONS.PROCUREMENT_VIEW,
    PERMISSIONS.PROCUREMENT_MANAGE,
    PERMISSIONS.PROCUREMENT_RECEIVE,
    PERMISSIONS.PROCUREMENT_RETURN,
    PERMISSIONS.SALES_RETURN_VIEW,
    PERMISSIONS.SALES_RETURN_MANAGE,
    PERMISSIONS.SALES_RETURN_RECEIVE,
    PERMISSIONS.SALES_RETURN_REFUND,
    PERMISSIONS.ORDER_PROFIT_VIEW,
    PERMISSIONS.ORDER_PROFIT_EXPORT,
    PERMISSIONS.SETTLEMENT_VIEW,
    PERMISSIONS.SETTLEMENT_IMPORT,
    PERMISSIONS.SETTLEMENT_EXPORT,
    PERMISSIONS.WAREHOUSE_FEE_VIEW,
    PERMISSIONS.WAREHOUSE_FEE_MANAGE,
    PERMISSIONS.ADVERTISING_FEE_VIEW,
    PERMISSIONS.ADVERTISING_FEE_IMPORT,
    PERMISSIONS.ADVERTISING_FEE_MANAGE,
    PERMISSIONS.FREIGHT_FEE_VIEW,
    PERMISSIONS.FREIGHT_FEE_IMPORT,
    PERMISSIONS.FREIGHT_FEE_MANAGE,
  ],
  reviewer: [
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.AUDIT_READ,
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATION_TASK_REVIEW,
    PERMISSIONS.OPERATION_TASK_EXECUTE,
    PERMISSIONS.OPERATION_TASK_RETRY,
    PERMISSIONS.INVENTORY_SYNC_READ,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.INVENTORY_APPROVE,
    PERMISSIONS.INVENTORY_SNAPSHOT_READ,
    PERMISSIONS.SKU_BINDING_READ,
    PERMISSIONS.SKU_BINDING_RESOLVE_MANUAL,
    PERMISSIONS.INVENTORY_SYNC_AUDIT_READ,
    PERMISSIONS.WAREHOUSE_VIEW,
    PERMISSIONS.LOGISTICS_VIEW,
    PERMISSIONS.SUPPLIER_VIEW,
    PERMISSIONS.PROCUREMENT_VIEW,
    PERMISSIONS.PROCUREMENT_APPROVE,
    PERMISSIONS.SALES_RETURN_VIEW,
    PERMISSIONS.SALES_RETURN_APPROVE,
    PERMISSIONS.SETTLEMENT_VIEW,
    PERMISSIONS.SETTLEMENT_EXPORT,
    PERMISSIONS.WAREHOUSE_FEE_VIEW,
    PERMISSIONS.ADVERTISING_FEE_VIEW,
    PERMISSIONS.FREIGHT_FEE_VIEW,
  ],
  readonly: [
    PERMISSIONS.PRODUCT_VIEW,
    PERMISSIONS.ORDER_VIEW,
    PERMISSIONS.INVENTORY_VIEW,
    PERMISSIONS.CUSTOMER_VIEW,
    PERMISSIONS.OPERATION_TASK_AUDIT_READ,
    PERMISSIONS.OPERATIONLOG_VIEW,
    PERMISSIONS.STORE_VIEW,
    PERMISSIONS.PII_READ_MASKED,
    PERMISSIONS.AUDIT_READ,
    PERMISSIONS.CONFIG_READ,
    PERMISSIONS.OBSERVABILITY_READ,
    PERMISSIONS.ALERTS_READ,
    PERMISSIONS.SLO_READ,
    PERMISSIONS.INVENTORY_SYNC_READ,
    PERMISSIONS.INVENTORY_SNAPSHOT_READ,
    PERMISSIONS.SKU_BINDING_READ,
    PERMISSIONS.WAREHOUSE_VIEW,
    PERMISSIONS.LOGISTICS_VIEW,
    PERMISSIONS.SUPPLIER_VIEW,
    PERMISSIONS.PROCUREMENT_VIEW,
    PERMISSIONS.SALES_RETURN_VIEW,
    PERMISSIONS.ORDER_PROFIT_VIEW,
    PERMISSIONS.SETTLEMENT_VIEW,
    PERMISSIONS.WAREHOUSE_FEE_VIEW,
    PERMISSIONS.ADVERTISING_FEE_VIEW,
    PERMISSIONS.FREIGHT_FEE_VIEW,
  ],
};

export function normalizeRole(role?: string | null): AdminRole {
  const r = (role || '').trim().toLowerCase();
  if (r === ROLES.ADMIN || r === ROLES.OPERATOR || r === ROLES.READONLY || r === ROLES.REVIEWER) return r;
  return '' as AdminRole;
}

function isCustomRoleCode(role?: string | null): boolean {
  return /^r_[a-f0-9]{24}$/.test((role || '').trim().toLowerCase());
}

export function permissionsForRole(role?: string | null, fromProfile?: string[]): PermissionKey[] {
  if (fromProfile !== undefined) {
    const normalizedRole = normalizeRole(role);
    if (!normalizedRole && !isCustomRoleCode(role)) return [];
    return fromProfile.filter((key): key is PermissionKey =>
      (Object.values(PERMISSIONS) as string[]).includes(key),
    );
  }
  return ROLE_PERMISSIONS[normalizeRole(role)] || [];
}

export function hasPermission(
  role: string | undefined | null,
  perm: PermissionKey,
  fromProfile?: string[],
): boolean {
  return permissionsForRole(role, fromProfile).includes(perm);
}

export function isReadonly(role?: string | null): boolean {
  const normalizedRole = normalizeRole(role);
  return normalizedRole === ROLES.READONLY || (!normalizedRole && !isCustomRoleCode(role));
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
