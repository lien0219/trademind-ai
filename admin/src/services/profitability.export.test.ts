import { afterEach, describe, expect, it, vi } from 'vitest';
import { request } from '@umijs/max';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import { markTenantContextChanged, resetTenantSwitchState } from '@/utils/tenantContext';
import { downloadOrderProfits } from './profitability';

vi.mock('@umijs/max', () => ({
  request: vi.fn(),
}));

describe('order profit export tenant isolation', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.mocked(request).mockReset();
    localStorage.clear();
    resetTenantSwitchState();
  });

  it('downloads a valid CSV for the original tenant only', async () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'tenant-a');
    const guard = createTenantReadGuard();
    const blob = new Blob(['orderNo,currency\nPO-1,CNY'], { type: 'text/csv' });
    vi.mocked(request).mockResolvedValue(blob);
    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:test-export');
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    expect(await downloadOrderProfits({ currency: 'CNY' }, guard.begin())).toBe(true);
    expect(request).toHaveBeenCalledWith('/api/v1/order-profits?format=csv&currency=CNY', expect.objectContaining({
      method: 'GET',
      responseType: 'blob',
    }));
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(click).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:test-export');
  });

  it('does not download a response returned after a tenant change', async () => {
    localStorage.setItem(AUTH_TOKEN_KEY, 'tenant-a');
    const guard = createTenantReadGuard();
    let resolveRequest!: (value: Blob) => void;
    vi.mocked(request).mockImplementation(() => new Promise<Blob>((resolve) => { resolveRequest = resolve; }) as ReturnType<typeof request>);
    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:invalid');
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const result = downloadOrderProfits({}, guard.begin());
    markTenantContextChanged();
    resolveRequest(new Blob(['old-tenant']));
    expect(await result).toBe(false);
    expect(createObjectURL).not.toHaveBeenCalled();
    expect(click).not.toHaveBeenCalled();
  });

  it('does not request or download when the context is invalid before export', async () => {
    const guard = createTenantReadGuard();
    const isCurrent = guard.begin();
    guard.invalidate();
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    expect(await downloadOrderProfits({}, isCurrent)).toBe(false);
    expect(request).not.toHaveBeenCalled();
    expect(click).not.toHaveBeenCalled();
  });
});
