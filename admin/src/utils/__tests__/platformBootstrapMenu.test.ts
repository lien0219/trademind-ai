import { describe, expect, it } from 'vitest';
import type { MenuDataItem } from '@umijs/route-utils';
import { canAccessPath, filterMenuByPermission } from '../menuAccess';

describe('tenant-zero bootstrap navigation', () => {
  const permissions = ['settings.manage', 'config.read', 'store.view'];
  it('keeps tenant business out of the platform-only workspace', () => {
    expect(canAccessPath('/platform/tenants', 'admin', permissions, true, 0)).toBe(true);
    expect(canAccessPath('/settings/system', 'admin', permissions, true, 0)).toBe(true);
    for (const path of ['/dashboard', '/shops/manage', '/tenant/members', '/settings/collector', '/ops/platform-runtime']) {
      expect(canAccessPath(path, 'admin', permissions, true, 0)).toBe(false);
    }
    expect(canAccessPath('/platform/tenants', 'admin', permissions, false, 0)).toBe(false);
  });
  it('filters navigation without loosening ordinary tenant RBAC', () => {
    const menu: MenuDataItem[] = [
      { name: '平台管理', path: '/platform', children: [{ name: '租户管理', path: '/platform/tenants' }] },
      { name: '店铺', path: '/shops/manage' },
      { name: '设置', path: '/settings', children: [
        { name: '系统设置', path: '/settings/system' },
        { name: '采集服务', path: '/settings/collector' },
      ] },
    ];
    const bootstrap = filterMenuByPermission(menu, 'admin', permissions, true, 0);
    expect(bootstrap.map(item => item.name)).toEqual(['平台管理', '设置']);
    expect(bootstrap[1].children?.map(item => item.name)).toEqual(['系统设置']);
    const tenant = filterMenuByPermission(menu, 'admin', ['store.view', 'settings.manage'], false, 42);
    expect(tenant.map(item => item.name)).toEqual(['店铺']);
  });
});
