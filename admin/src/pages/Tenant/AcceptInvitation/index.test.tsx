import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { history } from '@umijs/max';
import { AUTH_SESSION_MODE_KEY, AUTH_TOKEN_KEY } from '@/constants/auth';
import { acceptTenantInvitation } from '@/services/tenants';
import AcceptInvitationPage from './index';

vi.mock('@/services/tenants', () => ({
  acceptTenantInvitation: vi.fn(),
}));

describe('AcceptInvitationPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(message, 'success').mockImplementation(
      () => (() => {}) as ReturnType<typeof message.success>,
    );
    window.localStorage.clear();
    window.sessionStorage.clear();
    window.history.replaceState(null, '', '/tenant/invitations/accept');
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: false,
        media: '',
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  it('clears the revoked session and asks the user to log in again after acceptance', async () => {
    window.localStorage.setItem(AUTH_TOKEN_KEY, 'revoked-session-token');
    window.localStorage.setItem(AUTH_SESSION_MODE_KEY, 'secure_session');
    vi.mocked(acceptTenantInvitation).mockResolvedValueOnce({ tenantId: 101 });

    const user = userEvent.setup();
    render(<AcceptInvitationPage />);
    await user.type(screen.getByLabelText('一次性邀请令牌'), 'e2e-invitation-token');
    await user.click(screen.getByRole('button', { name: /接受邀请/ }));

    await waitFor(() => expect(screen.getByText('邀请已接受')).toBeInTheDocument());
    expect(acceptTenantInvitation).toHaveBeenCalledWith('e2e-invitation-token');
    expect(window.localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
    expect(window.localStorage.getItem(AUTH_SESSION_MODE_KEY)).toBeNull();
    expect(screen.getByText('当前会话已失效，请重新登录后手动切换到受邀企业。')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /重新登录/ }));
    expect(history.push).toHaveBeenCalledWith('/user/login');
  });
});
