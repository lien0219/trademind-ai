import { describe, expect, it } from 'vitest';
import { canAccessPath, filterMenuByPermission } from '../menuAccess';

const opsMenu = [{
  path: '/ops',
  name: '运维',
  component: 'OpsGroupLayout',
  children: [
    { path: '/ops/workers/monitor', name: '后台任务监控' },
    { path: '/ops/observability', name: '可观测性中心' },
  ],
}];

describe('permission filtered menus', () => {
  it('keeps custom role links that its profile explicitly grants', () => {
    const role = 'r_0123456789abcdef01234567';
    expect(canAccessPath('/ops/observability', role, ['observability.read'])).toBe(true);
    expect(filterMenuByPermission(opsMenu, role, ['observability.read'])).toEqual([{
      ...opsMenu[0],
      children: [{ path: '/ops/observability', name: '可观测性中心' }],
    }]);
  });

  it('removes a group when every visible child is forbidden', () => {
    expect(filterMenuByPermission(opsMenu, 'admin', [])).toEqual([]);
  });

  it('does not expose protected children for an unknown role with forged grants', () => {
    expect(filterMenuByPermission(opsMenu, 'unknown', ['task.retry', 'observability.read'])).toEqual([]);
  });

  it('does not retain an empty navigation group because it has an unnamed index redirect', () => {
    const menus = [{
      path: '/shops', name: '店铺', component: 'ShopsGroupLayout',
      children: [
        { path: '/shops', redirect: '/shops/manage' },
        { path: '/shops/manage', name: '店铺管理' },
        { path: '/shops/manage/:id', name: '店铺详情', hideInMenu: true },
      ],
    }];
    expect(filterMenuByPermission(menus, 'readonly', [])).toEqual([]);
  });

  it('requires explicit inventory-sync permissions for hidden operations routes', () => {
    const role = 'r_0123456789abcdef01234567';
    expect(canAccessPath('/ops/inventory-sync/runs/run-1', role, [])).toBe(false);
    expect(canAccessPath('/ops/inventory-sync/runs/run-1', role, ['inventory_sync.read'])).toBe(true);
    expect(canAccessPath('/ops/inventory-sync/calibration', role, ['inventory_sync.read'])).toBe(false);
    expect(canAccessPath('/ops/inventory-sync/calibration', role, ['sku_binding.read'])).toBe(true);
  });
});
