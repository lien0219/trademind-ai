import { describe, expect, it } from 'vitest';
import routes, { createInternalInventorySyncRoutes } from '../../../config/routes';

type RouteNode = {
  path?: string;
  name?: string;
  hideInMenu?: boolean;
  routes?: RouteNode[];
};

function collectVisibleMenuPaths(nodes: RouteNode[], paths: string[] = []): string[] {
  for (const node of nodes) {
    if (node.name && node.path && !node.hideInMenu) {
      paths.push(node.path);
    }
    if (node.routes) {
      collectVisibleMenuPaths(node.routes, paths);
    }
  }
  return paths;
}

function collectVisibleMenuNames(nodes: RouteNode[], names: string[] = []): string[] {
  for (const node of nodes) {
    if (node.name && !node.hideInMenu) {
      names.push(node.name);
    }
    if (node.routes) {
      collectVisibleMenuNames(node.routes, names);
    }
  }
  return names;
}

function collectRoutePaths(nodes: RouteNode[], paths: string[] = []): string[] {
  for (const node of nodes) {
    if (node.path) {
      paths.push(node.path);
    }
    if (node.routes) {
      collectRoutePaths(node.routes, paths);
    }
  }
  return paths;
}

describe('Admin route menu configuration', () => {
  it('exposes separate public home, login, and registration routes', () => {
    expect(routes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ path: '/', layout: false, component: './Index' }),
        expect.objectContaining({ path: '/user/login', layout: false, component: './User/Login' }),
        expect.objectContaining({ path: '/user/register', layout: false, component: './User/Login' }),
      ]),
    );
  });

  it('uses a unique path for every visible menu item', () => {
    const paths = collectVisibleMenuPaths(routes);
    const duplicates = paths.filter((path, index) => paths.indexOf(path) !== index);

    expect([...new Set(duplicates)]).toEqual([]);
  });

  it('keeps the legacy inventory URL outside the menu and exposes a unique overview path', () => {
    const inventory = routes.find((route) => route.path === '/inventory');
    const legacyInventoryRoute = inventory?.routes?.find(
      (route) => route.path === '/inventory',
    );

    expect(inventory?.routes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ path: '/inventory', component: './Inventory' }),
        expect.objectContaining({
          path: '/inventory/overview',
          name: '库存中心',
          component: './Inventory',
        }),
      ]),
    );
    expect(legacyInventoryRoute).not.toHaveProperty('name');
  });

  it('keeps internal phase and fixture routes out of the production menu', () => {
    const paths = collectVisibleMenuPaths(routes);
    const names = collectVisibleMenuNames(routes);

    expect(paths.filter((path) => path.startsWith('/ops/inventory-sync'))).toEqual([]);
    expect(names.filter((name) => /P\d+|Batch|Gate|Fixture|夹具|人工验收/i.test(name))).toEqual([]);
  });

  it('exposes the controlled procurement workspace and keeps detail as a deep link', () => {
    const procurement = routes.find((route) => route.path === '/procurement');

    expect(procurement?.routes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ path: '/procurement/purchase-orders', name: '采购单' }),
        expect.objectContaining({ path: '/procurement/replenishment-suggestions', name: '补货建议' }),
        expect.objectContaining({ path: '/procurement/warehouses', name: '仓库管理' }),
        expect.objectContaining({ path: '/procurement/suppliers', name: '供应商管理' }),
        expect.objectContaining({ path: '/procurement/purchase-orders/:id', hideInMenu: true }),
      ]),
    );
  });

  it('excludes internal inventory fixture routes from production builds', () => {
    expect(createInternalInventorySyncRoutes(false)).toEqual([]);
    expect(createInternalInventorySyncRoutes(true)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ path: '/ops/inventory-sync', hideInMenu: true }),
      ]),
    );
    expect(createInternalInventorySyncRoutes(true).every((route) => route.hideInMenu)).toBe(true);
  });

  it('does not register retired application backup and restore routes', () => {
    const paths = collectRoutePaths(routes);

    expect(paths.filter((path) => path.startsWith('/ops/backups'))).toEqual([]);
    expect(paths.filter((path) => path.startsWith('/ops/restores'))).toEqual([]);
  });

  it('keeps historical AI batches outside the menu for deep-link compatibility', () => {
    const aiRoutes = routes.find((route) => route.path === '/ai')?.routes;

    expect(aiRoutes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: '/ai/batches',
          name: '历史 AI 批次',
          hideInMenu: true,
        }),
      ]),
    );
  });
});
