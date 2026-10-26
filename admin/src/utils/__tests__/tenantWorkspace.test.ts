import { describe, expect, it } from 'vitest';
import { canEnterTenantWorkspace, visibleWorkspaceOptions, workspaceDisplayName, workspaceLandingPath } from '../tenantWorkspace';

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

describe('single Root workspace selector', () => {
  const options = [
    { id: 0, code: 'system', name: '系统控制台' },
    { id: 7, code: 'root-workspace', name: '系统工作台' },
    { id: 21, code: 'tenant-a', name: '企业 A' },
  ];

  it('hides tenant zero for Root without hiding tenant business access', () => {
    expect(visibleWorkspaceOptions(options, true).map((item) => item.id)).toEqual([7, 21]);
    expect(workspaceDisplayName(options[1])).toBe('Root 工作台');
  });

  it('preserves the system context for non-Root platform operators', () => {
    expect(visibleWorkspaceOptions(options, false)).toEqual(options);
  });
});
