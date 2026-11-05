import { describe, expect, it } from 'vitest';
import { canAdministerShops, canOperateShop } from '../shopActionAccess';
const user = (overrides: Partial<API.CurrentUser>): API.CurrentUser => ({
  id: 'one', username: 'test', displayName: 'Test', role: 'operator', tenantId: 9,
  permissions: ['store.view', 'store.operate'],
  storePermissions: [{ storeId: 'shop-a', permissionScope: 'operate' }], ...overrides,
});
describe('store-scoped UI preflight', () => {
  it('limits create/delete to the tenant admin', () => {
    expect(canAdministerShops(user({role:'admin'}))).toBe(true);
    expect(canAdministerShops(user({role:'operator'}))).toBe(false);
    expect(canAdministerShops(user({role:'admin',tenantId:0}))).toBe(false);
  });
  it('limits store mutation to granted store and scope', () => {
    expect(canOperateShop(user({}), 'shop-a')).toBe(true);
    expect(canOperateShop(user({}), 'shop-b')).toBe(false);
    expect(canOperateShop(user({storePermissions:[{storeId:'shop-a',permissionScope:'view'}]}), 'shop-a')).toBe(false);
    expect(canOperateShop(user({permissions:['store.view']}), 'shop-a')).toBe(false);
    expect(canOperateShop(user({role:'readonly'}), 'shop-a')).toBe(false);
    expect(canOperateShop(user({role:'admin', permissions:['store.operate'],storePermissions:[]}), 'shop-b')).toBe(true);
  });
});
