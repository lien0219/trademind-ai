import { describe, expect, it } from 'vitest';

import { hasPermission, PERMISSIONS, ROLES } from './permission';

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
  });
});
