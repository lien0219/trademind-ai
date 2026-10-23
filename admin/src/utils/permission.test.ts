import { describe, expect, it } from 'vitest';

import { hasPermission, isReadonly, permissionsForRole, PERMISSIONS, ROLES } from './permission';

describe('profile permissions', () => {
  it('rejects grants from an unknown role even when the profile includes them', () => {
    expect(
      hasPermission('unexpected', PERMISSIONS.USER_MANAGE, [PERMISSIONS.USER_MANAGE]),
    ).toBe(false);
    expect(permissionsForRole('unexpected', [PERMISSIONS.USER_MANAGE])).toEqual([]);
  });

  it('accepts server grants only for a valid custom role code', () => {
    expect(
      hasPermission('r_0123456789abcdef01234567', PERMISSIONS.PRODUCT_VIEW, [PERMISSIONS.PRODUCT_VIEW]),
    ).toBe(true);
    expect(
      hasPermission('r_not-a-role-code', PERMISSIONS.USER_MANAGE, [PERMISSIONS.USER_MANAGE]),
    ).toBe(false);
  });

  it('preserves permission keys returned by the backend catalog for custom roles', () => {
    const role = 'r_0123456789abcdef01234567';
    const backendOnlyKeys = [
      'audit.export', 'audit.read', 'config.read', 'export.create', 'export.read',
      'inventory_snapshot.read', 'inventory_sync.audit.read', 'inventory_sync.rerun',
      'observability.manage', 'pii.export', 'pii.read_masked', 'security.key.rotate',
      'security.session.manage', 'sku_binding.manage', 'sku_binding.read',
      'sku_binding.resolve_manual', 'slo.manage', 'slo.read',
    ];
    expect(permissionsForRole(role, backendOnlyKeys).map(String)).toEqual(backendOnlyKeys);
  });

  it('keeps inventory synchronization fallback grants aligned with the backend roles', () => {
    const operator = permissionsForRole(ROLES.OPERATOR).map(String);
    const reviewer = permissionsForRole(ROLES.REVIEWER).map(String);
    const readonly = permissionsForRole(ROLES.READONLY).map(String);

    expect(operator).toContain('security.session.manage');
    expect(operator).toContain('sku_binding.manage');
    expect(operator).not.toContain('inventory.approve');
    expect(reviewer).toContain('inventory.approve');
    expect(reviewer).toContain('sku_binding.resolve_manual');
    expect(readonly).toContain('audit.read');
    expect(readonly).toContain('sku_binding.read');
  });

  it('keeps a valid custom role writable while treating unknown roles as read-only', () => {
    expect(isReadonly('r_0123456789abcdef01234567')).toBe(false);
    expect(isReadonly('unexpected')).toBe(true);
  });
});

describe('ERP permission fallbacks', () => {
  it('separates operator execution from reviewer approval', () => {
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.PROCUREMENT_MANAGE)).toBe(
      true,
    );
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.PROCUREMENT_RECEIVE)).toBe(
      true,
    );
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.PROCUREMENT_APPROVE)).toBe(
      false,
    );

    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.PROCUREMENT_APPROVE)).toBe(
      true,
    );
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.PROCUREMENT_MANAGE)).toBe(
      false,
    );
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.PROCUREMENT_RECEIVE)).toBe(
      false,
    );
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.SALES_RETURN_MANAGE)).toBe(
      true,
    );
    expect(
      hasPermission(ROLES.OPERATOR, PERMISSIONS.SALES_RETURN_RECEIVE),
    ).toBe(true);
    expect(
      hasPermission(ROLES.OPERATOR, PERMISSIONS.SALES_RETURN_APPROVE),
    ).toBe(false);
    expect(
      hasPermission(ROLES.REVIEWER, PERMISSIONS.SALES_RETURN_APPROVE),
    ).toBe(true);
    expect(
      hasPermission(ROLES.REVIEWER, PERMISSIONS.SALES_RETURN_RECEIVE),
    ).toBe(false);
    expect(
      hasPermission(ROLES.OPERATOR, PERMISSIONS.SALES_RETURN_REFUND),
    ).toBe(true);
    expect(
      hasPermission(ROLES.OPERATOR, PERMISSIONS.ORDER_PROFIT_EXPORT),
    ).toBe(true);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.SETTLEMENT_IMPORT)).toBe(true);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.SETTLEMENT_EXPORT)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.SETTLEMENT_VIEW)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.SETTLEMENT_EXPORT)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.SETTLEMENT_IMPORT)).toBe(false);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.WAREHOUSE_FEE_VIEW)).toBe(true);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.WAREHOUSE_FEE_MANAGE)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.WAREHOUSE_FEE_VIEW)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.WAREHOUSE_FEE_MANAGE)).toBe(false);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.ADVERTISING_FEE_VIEW)).toBe(true);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.ADVERTISING_FEE_IMPORT)).toBe(true);
    expect(hasPermission(ROLES.OPERATOR, PERMISSIONS.ADVERTISING_FEE_MANAGE)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.ADVERTISING_FEE_VIEW)).toBe(true);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.ADVERTISING_FEE_IMPORT)).toBe(false);
    expect(hasPermission(ROLES.REVIEWER, PERMISSIONS.ADVERTISING_FEE_MANAGE)).toBe(false);
    expect(
      hasPermission(ROLES.REVIEWER, PERMISSIONS.SALES_RETURN_REFUND),
    ).toBe(false);
  });

  it('keeps readonly ERP access read-only', () => {
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.WAREHOUSE_VIEW)).toBe(
      true,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SUPPLIER_VIEW)).toBe(true);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.PROCUREMENT_VIEW)).toBe(
      true,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.WAREHOUSE_MANAGE)).toBe(
      false,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SUPPLIER_MANAGE)).toBe(
      false,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SALES_RETURN_VIEW)).toBe(
      true,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SALES_RETURN_MANAGE)).toBe(
      false,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SALES_RETURN_REFUND)).toBe(
      false,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.ORDER_PROFIT_VIEW)).toBe(
      true,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.ORDER_PROFIT_EXPORT)).toBe(
      false,
    );
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SETTLEMENT_VIEW)).toBe(true);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SETTLEMENT_IMPORT)).toBe(false);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.SETTLEMENT_EXPORT)).toBe(false);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.WAREHOUSE_FEE_VIEW)).toBe(true);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.WAREHOUSE_FEE_MANAGE)).toBe(false);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.ADVERTISING_FEE_VIEW)).toBe(true);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.ADVERTISING_FEE_IMPORT)).toBe(false);
    expect(hasPermission(ROLES.READONLY, PERMISSIONS.ADVERTISING_FEE_MANAGE)).toBe(false);
  });
});
