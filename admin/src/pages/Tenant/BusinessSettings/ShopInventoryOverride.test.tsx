import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { message } from 'antd';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { queryShops } from '@/services/shops';
import { getJSON, putJSON } from '@/services/request';
import { markTenantContextChanged, resetTenantSwitchState } from '@/utils/tenantContext';
import { setLanguage } from '@/i18n';
import ShopInventoryOverride from './ShopInventoryOverride';

vi.mock('@/services/shops', () => ({ queryShops: vi.fn() }));
vi.mock('@/services/request', () => ({ getJSON: vi.fn(), putJSON: vi.fn() }));
vi.mock('@/hooks/usePermission', () => ({
  usePermission: () => ({ user: { tenantId: 23, id: 'operator', role: 'operator' } }),
}));
vi.mock('@/utils/shopActionAccess', () => ({ canOperateShop: () => true }));

const store = { id: 'shop-a', shopName: 'Example store', platform: 'demo' };
const initial = { shopId: 'shop-a', warningStock: 9, origin: 'shop' as const, version: 2 };

beforeEach(() => {
  setLanguage('zh-CN');
  vi.mocked(queryShops).mockReset();
  vi.mocked(getJSON).mockReset();
  vi.mocked(putJSON).mockReset();
  vi.mocked(queryShops).mockResolvedValue({ list: [{ ...store, status: 'active', authStatus: 'connected', updatedAt: '2026-10-27T00:00:00Z' }], pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 } });
});
afterEach(async () => {
  cleanup();
  resetTenantSwitchState();
  await act(async () => { message.destroy(); });
});

it('does not display a store override fetched under a previous tenant context', async () => {
  let resolve!: (value: typeof initial) => void;
  vi.mocked(getJSON).mockReturnValue(new Promise(done => { resolve = done; }));
  render(<ShopInventoryOverride />);
  await waitFor(() => expect(getJSON).toHaveBeenCalledWith('/api/v1/tenant/shops/shop-a/settings/inventory'));
  markTenantContextChanged();
  await act(async () => { resolve(initial); });
  expect(screen.queryByRole('button', { name: '保存店铺覆盖' })).not.toBeInTheDocument();
});

it('does not report a successful write from a previous tenant context', async () => {
  vi.mocked(getJSON).mockResolvedValue(initial);
  let resolve!: (value: typeof initial) => void;
  vi.mocked(putJSON).mockReturnValue(new Promise(done => { resolve = done; }));
  const success = vi.spyOn(message, 'success');
  render(<ShopInventoryOverride />);
  const save = await screen.findByRole('button', { name: '保存店铺覆盖' });
  fireEvent.click(save);
  await waitFor(() => expect(putJSON).toHaveBeenCalledTimes(1));
  markTenantContextChanged();
  await act(async () => { resolve({ ...initial, version: 3 }); });
  expect(success).not.toHaveBeenCalled();
});
