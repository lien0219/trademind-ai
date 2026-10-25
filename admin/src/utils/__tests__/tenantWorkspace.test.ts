import { describe, expect, it } from 'vitest';
import { canEnterTenantWorkspace, workspaceLandingPath } from '../tenantWorkspace';

describe('workspace separation', () => {
  it('routes only cross-boundary switches to the new landing page', () => {
    expect(workspaceLandingPath(0, 12)).toBe('/dashboard/product-operations');
    expect(workspaceLandingPath(12, 0)).toBe('/platform/tenants');
    expect(workspaceLandingPath(12, 15)).toBeUndefined();
  });

  it('requires an active member record for explicit enterprise entry', () => {
    const tenant = { id: 12, status: 'active' as const };
    expect(canEnterTenantWorkspace(tenant, [])).toBe(false);
    expect(canEnterTenantWorkspace(tenant, [{ id: 0, status: 'active' }])).toBe(false);
    expect(canEnterTenantWorkspace(tenant, [{ id: 12, status: 'suspended' }])).toBe(false);
    expect(canEnterTenantWorkspace({ ...tenant, status: 'suspended' }, [{ id: 12, status: 'active' }])).toBe(false);
    expect(canEnterTenantWorkspace(tenant, [{ id: 12, status: 'active' }])).toBe(true);
  });
});
