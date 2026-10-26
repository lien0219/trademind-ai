import { describe, expect, it } from 'vitest';
import { mayLeaveTenant } from '../tenantLeave';

const tenant = { id: 31, status: 'active' as const, ownerUserId: 'owner' };
const normal: API.CurrentUser = {
  id: 'member',
  username: 'operator@example.test',
  displayName: '测试运营',
  role: 'operator',
  tenantId: 31,
  isRoot: false,
  permissions: ['product.view'],
};

describe('tenant self-service leave display policy', () => {
  it('only permits ordinary active members in the selected tenant', () => {
    expect(mayLeaveTenant(normal, tenant)).toBe(true);
    expect(mayLeaveTenant({ ...normal, role: 'readonly' }, tenant)).toBe(true);
    expect(mayLeaveTenant({ ...normal, role: 'reviewer' }, tenant)).toBe(true);
    expect(mayLeaveTenant({ ...normal, role: 'r_custom' }, tenant)).toBe(true);
  });
  it('hides leave for Root, enterprise admins, and tenant owners', () => {
    expect(mayLeaveTenant({ ...normal, isRoot: true }, tenant)).toBe(false);
    expect(mayLeaveTenant({ ...normal, role: 'admin' }, tenant)).toBe(false);
    expect(mayLeaveTenant({ ...normal, role: 'owner' }, tenant)).toBe(false);
    expect(mayLeaveTenant({ ...normal, id: 'owner' }, tenant)).toBe(false);
    expect(mayLeaveTenant({ ...normal, role: 'r_custom', permissions: ['user.manage'] }, tenant)).toBe(false);
  });
  it('fails closed for absent, mismatched, inactive and system contexts', () => {
    expect(mayLeaveTenant(undefined, tenant)).toBe(false);
    expect(mayLeaveTenant(normal, null)).toBe(false);
    expect(mayLeaveTenant({ ...normal, tenantId: 0 }, tenant)).toBe(false);
    expect(mayLeaveTenant({ ...normal, tenantId: 12 }, tenant)).toBe(false);
    expect(mayLeaveTenant(normal, { ...tenant, status: 'suspended' })).toBe(false);
    expect(mayLeaveTenant({ ...normal, role: '' }, tenant)).toBe(false);
  });
});
