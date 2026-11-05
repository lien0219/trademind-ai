import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { request, useModel } from '@umijs/max';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetTenantSwitchState } from '@/utils/tenantContext';
import { getBusinessSettings, putBusinessSettings, restoreBusinessSettings } from '@/services/tenantBusinessSettings';
import { setLanguage } from '@/i18n';
import { layout } from './app';

vi.mock('@/services/tenantBusinessSettings', async () => ({
  ...await vi.importActual<typeof import('@/services/tenantBusinessSettings')>('@/services/tenantBusinessSettings'),
  getBusinessSettings: vi.fn(),
}));

function renderWorkspaceBrand(user?: API.CurrentUser) {
  vi.mocked(useModel).mockReturnValue({ initialState: { currentUser: user } } as never);
  const config = layout({ initialState: { currentUser: user } });
  const menuHeaderRender = config.menuHeaderRender as (
    logoDom: React.ReactNode,
    titleDom: React.ReactNode,
    props?: { collapsed?: boolean; isMobile?: boolean },
  ) => React.ReactNode;
  const brand = menuHeaderRender(null, null, { collapsed: false });
  return brand;
}

const currentUser = (tenantId: number): API.CurrentUser => ({
  id: 'user-1',
  username: 'user@example.test',
  displayName: 'User',
  tenantId,
});

beforeEach(() => {
  vi.mocked(getBusinessSettings).mockReset();
  localStorage.clear();
  setLanguage('zh-CN');
});

afterEach(() => {
  cleanup();
  resetTenantSwitchState();
  setLanguage('zh-CN');
});

describe('workspace header branding', () => {
  it('shows tenant branding from the read-only tenant settings endpoint', async () => {
    vi.mocked(getBusinessSettings).mockResolvedValue({
      module: 'branding',
      version: 1,
      inherited: false,
      values: { displayName: 'Acme Trading', logoUrl: 'https://assets.example.test/acme.svg' },
    });

    render(renderWorkspaceBrand(currentUser(42)));

    expect(await screen.findByText('Acme Trading')).toBeInTheDocument();
    expect(document.querySelector('img.tm-app-brand-logo')).toHaveAttribute('src', 'https://assets.example.test/acme.svg');
    expect(getBusinessSettings).toHaveBeenCalledTimes(1);
    expect(getBusinessSettings).toHaveBeenCalledWith('branding');
  });

  it('keeps default branding in the root system workspace without a tenant request', () => {
    render(renderWorkspaceBrand({ ...currentUser(0), isRoot: true }));

    expect(screen.getByText('贸灵')).toBeInTheDocument();
    expect(screen.getByText('TradeMind')).toBeInTheDocument();
    expect(getBusinessSettings).not.toHaveBeenCalled();
  });

  it('uses default branding while the authenticated profile is unavailable', () => {
    render(renderWorkspaceBrand());
    expect(screen.getByText('TradeMind')).toBeInTheDocument();
    expect(getBusinessSettings).not.toHaveBeenCalled();
  });

  it('ignores branding returned after switching to another tenant', async () => {
    let finishFirst!: (value: Awaited<ReturnType<typeof getBusinessSettings>>) => void;
    let finishSecond!: (value: Awaited<ReturnType<typeof getBusinessSettings>>) => void;
    vi.mocked(getBusinessSettings)
      .mockReturnValueOnce(new Promise((resolve) => { finishFirst = resolve; }))
      .mockReturnValueOnce(new Promise((resolve) => { finishSecond = resolve; }));

    const { rerender } = render(renderWorkspaceBrand(currentUser(42)));
    await waitFor(() => expect(getBusinessSettings).toHaveBeenCalledTimes(1));

    rerender(renderWorkspaceBrand(currentUser(84)));
    await waitFor(() => expect(getBusinessSettings).toHaveBeenCalledTimes(2));

    await act(async () => {
      finishSecond({
        module: 'branding', version: 1, inherited: false,
        values: { displayName: 'Current Workspace' },
      });
    });
    expect(await screen.findByText('Current Workspace')).toBeInTheDocument();

    await act(async () => {
      finishFirst({
        module: 'branding', version: 1, inherited: false,
        values: { displayName: 'Previous Workspace' },
      });
    });
    expect(screen.getByText('Current Workspace')).toBeInTheDocument();
    expect(screen.queryByText('Previous Workspace')).not.toBeInTheDocument();
  });

  it.each(['save', 'restore'])('refreshes the header after a successful branding %s', async (action) => {
    const snapshot = { module: 'branding' as const, version: 1, inherited: false, values: { displayName: 'Original workspace' } };
    vi.mocked(getBusinessSettings).mockResolvedValueOnce(snapshot);
    render(renderWorkspaceBrand(currentUser(42)));
    expect(await screen.findByText('Original workspace')).toBeInTheDocument();

    const updated = { ...snapshot, version: 2, values: { displayName: 'Updated workspace' } };
    vi.mocked(getBusinessSettings).mockResolvedValueOnce(updated);
    vi.mocked(request).mockResolvedValueOnce({ code: 0, message: 'ok', data: updated });
    await act(async () => {
      if (action === 'save') await putBusinessSettings('branding', { expectedVersion: 1, values: updated.values });
      else await restoreBusinessSettings('branding', { expectedVersion: 1, targetVersion: 1 });
    });
    expect(await screen.findByText('Updated workspace')).toBeInTheDocument();
  });

  it('preserves business branding verbatim when switching language without refetching', async () => {
    vi.mocked(getBusinessSettings).mockResolvedValue({
      module: 'branding', version: 1, inherited: false, values: { displayName: '设置' },
    });
    render(renderWorkspaceBrand(currentUser(42)));
    expect(await screen.findByText('设置')).toBeInTheDocument();
    act(() => setLanguage('en-US'));
    expect(screen.getByText('设置')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back to workspace' })).toBeInTheDocument();
    expect(getBusinessSettings).toHaveBeenCalledTimes(1);
  });
});
