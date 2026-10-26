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

describe('tenant workbench navigation', () => {
  const enterprise = [{
    path: '/tenant',
    name: '企业工作台',
    children: [
      { path: '/tenant/settings', name: '企业信息与设置' },
      { path: '/tenant/members', name: '企业成员与邀请' },
    ],
  }];

  it('lets an operator view enterprise details without showing member management', () => {
    expect(canAccessPath('/tenant/settings', 'operator', ['product.view'])).toBe(true);
    expect(canAccessPath('/tenant/members', 'operator', ['product.view'])).toBe(false);
    expect(filterMenuByPermission(enterprise, 'operator', ['product.view'])).toEqual([{
      ...enterprise[0],
      children: [enterprise[0].children[0]],
    }]);
  });

  it('allows member management only when the effective role grants user.manage', () => {
    expect(canAccessPath('/tenant/members', 'admin', ['user.manage'])).toBe(true);
    expect(canAccessPath('/tenant/members', 'r_0123456789abcdef01234567', ['user.manage'])).toBe(true);
  });
});

describe('platform scoped navigation', () => {
  it('blocks tenant reader and tenant admin from global runtime/settings', () => {
    expect(canAccessPath('/ops/platform-runtime', 'readonly', ['store.view'], false, 2)).toBe(false);
    expect(canAccessPath('/ops/platform-runtime', 'admin', ['store.view'], true, 2)).toBe(false);
    expect(canAccessPath('/settings/system', 'admin', ['settings.manage'], false, 2)).toBe(false);
    expect(canAccessPath('/settings/users', 'admin', ['user.manage'], false, 2)).toBe(true);
    expect(canAccessPath('/orders/list', 'readonly', ['order.view'], false, 2)).toBe(true);
    expect(canAccessPath('/ops/platform-runtime', 'admin', ['store.view'], true, 2, true)).toBe(true);
    expect(canAccessPath('/ai/prompts', 'admin', ['settings.manage'], false, 2)).toBe(false);
    expect(canAccessPath('/collect/rules', 'admin', ['settings.manage'], false, 2)).toBe(false);
    expect(canAccessPath('/ai/prompts', 'admin', ['settings.manage'], true, 2, true)).toBe(true);
    expect(canAccessPath('/collect/rules', 'admin', ['settings.manage'], true, 2, true)).toBe(true);
    expect(canAccessPath('/ai/prompts', 'admin', ['settings.manage'], true, 0)).toBe(true);
    expect(canAccessPath('/ai/tasks', 'admin', ['product.view'], true, 0)).toBe(false);
    expect(canAccessPath('/collect/rules', 'admin', ['settings.manage'], true, 0)).toBe(true);
    expect(canAccessPath('/collect/rules', 'admin', ['settings.manage'], false, 0)).toBe(false);
    expect(canAccessPath('/collect/browser-profiles', 'readonly', ['product.view'], false, 2)).toBe(true);
    expect(canAccessPath('/collect/monitor', 'readonly', ['product.view'], false, 2)).toBe(true);
  });
});
