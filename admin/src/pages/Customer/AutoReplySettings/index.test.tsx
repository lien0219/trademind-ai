import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { message } from 'antd';
import CustomerAutoReplySettingsPage from './index';

const customerService = vi.hoisted(() => ({
  getCustomerAutoReplyPolicy: vi.fn(),
  getCustomerAutoReplySetting: vi.fn(),
  queryCustomerAutoReplyRuns: vi.fn(),
  updateCustomerAutoReplyPolicy: vi.fn(),
  updateCustomerAutoReplySetting: vi.fn(),
}));
const shopService = vi.hoisted(() => ({ queryShops: vi.fn() }));

// Static notices have an independent React root and duration timers. Flush
// their public destroy operation before jsdom tears down window.
afterEach(async () => {
  cleanup();
  await act(async () => { message.destroy(); });
});

vi.mock('@/hooks/usePermission', () => ({
  usePermission: () => ({ canManageSettings: true, canWriteCustomer: true }),
}));
vi.mock('@/services/customer', () => customerService);
vi.mock('@/services/shops', () => shopService);
vi.mock('@/utils/sensitiveConfirm', () => ({ confirmSensitiveAction: vi.fn() }));

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason?: unknown) => void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function policy(shopId: string, shopPolicy: string) {
  return {
    shopId,
    shopName: shopId,
    platform: 'mock',
    globalEnabled: false,
    workerAvailable: true,
    enabled: false,
    effectiveEnabled: false,
    tone: 'professional',
    shopPolicy,
    maxReplyRunes: 600,
    maxRepliesPerHour: 20,
    requireOrderContext: true,
    lowRiskOnly: true,
    updatedAt: '2026-08-12T00:00:00Z',
  };
}

async function selectShop(label: string) {
  const user = userEvent.setup();
  fireEvent.mouseDown(screen.getByRole('combobox', { name: '店铺' }));
  await user.click(await screen.findByText(label));
}

beforeEach(() => {
	vi.clearAllMocks();
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
  customerService.getCustomerAutoReplySetting.mockResolvedValue({
    messageSyncEnabled: false,
    autoReplyEnabled: false,
    pollIntervalSeconds: 60,
    workerAvailable: true,
    effectiveEnabled: false,
  });
  customerService.queryCustomerAutoReplyRuns.mockResolvedValue([]);
  shopService.queryShops.mockResolvedValue({
    list: [
      { id: 'shop-a', shopName: 'Shop A', platform: 'mock' },
      { id: 'shop-b', shopName: 'Shop B', platform: 'mock' },
    ],
    pagination: { page: 1, pageSize: 100, total: 2, totalPages: 1 },
  });
});

describe('CustomerAutoReplySettingsPage shop request isolation', () => {
  it('ignores a stale shop response that resolves after the active shop', async () => {
    const shopA = deferred<ReturnType<typeof policy>>();
    customerService.getCustomerAutoReplyPolicy.mockImplementation((shopId: string) =>
      shopId === 'shop-a' ? shopA.promise : Promise.resolve(policy('shop-b', 'B policy')),
    );
    render(<CustomerAutoReplySettingsPage />);

    await waitFor(() => expect(customerService.getCustomerAutoReplyPolicy).toHaveBeenCalledWith('shop-a'));
    await selectShop('Shop B (mock)');
    await waitFor(() => expect(screen.getByLabelText('店铺政策摘要')).toHaveValue('B policy'));

    shopA.resolve(policy('shop-a', 'A stale policy'));
    await waitFor(() => expect(screen.getByLabelText('店铺政策摘要')).toHaveValue('B policy'));
  });

  it('clears and disables the form when the active shop load fails', async () => {
    const shopA = deferred<ReturnType<typeof policy>>();
    customerService.getCustomerAutoReplyPolicy.mockImplementation((shopId: string) =>
      shopId === 'shop-a' ? shopA.promise : Promise.reject(new Error('B load failed')),
    );
    render(<CustomerAutoReplySettingsPage />);

    await waitFor(() => expect(customerService.getCustomerAutoReplyPolicy).toHaveBeenCalledWith('shop-a'));
    await selectShop('Shop B (mock)');
    expect(await screen.findByText('店铺策略加载失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存策略' })).toBeDisabled();

    shopA.resolve(policy('shop-a', 'A stale policy'));
    await waitFor(() => expect(screen.getByRole('button', { name: '保存策略' })).toBeDisabled());
    expect(screen.queryByDisplayValue('A stale policy')).not.toBeInTheDocument();
  });
});

describe('CustomerAutoReplySettingsPage 运行设置安全状态', () => {
  it('加载失败后保持禁用，仅在显式重试成功后恢复编辑', async () => {
    customerService.getCustomerAutoReplySetting
      .mockRejectedValueOnce(new Error('setting unavailable'))
      .mockResolvedValueOnce({
        messageSyncEnabled: true,
        autoReplyEnabled: false,
        pollIntervalSeconds: 120,
        workerAvailable: true,
        effectiveEnabled: false,
      });
    customerService.getCustomerAutoReplyPolicy.mockResolvedValue(policy('shop-a', 'A policy'));
    render(<CustomerAutoReplySettingsPage />);

    expect(await screen.findByText('运行设置加载失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存运行设置' })).toBeDisabled();
    expect(customerService.updateCustomerAutoReplySetting).not.toHaveBeenCalled();

    const retryButton = screen.getByRole('button', { name: /重新加载/ });
    await waitFor(() => expect(retryButton).toBeEnabled());
    await userEvent.click(retryButton);
    await waitFor(() => expect(screen.getByRole('button', { name: '保存运行设置' })).toBeEnabled());
    expect(screen.getByRole('switch', { name: '自动同步客服消息' })).toBeChecked();
    expect(screen.getByRole('spinbutton', { name: '消息轮询间隔（秒）' })).toHaveValue('120');
  });
});
