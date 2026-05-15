import { Button, Space } from 'antd';
import { history, useModel, type RequestConfig, type RunTimeLayoutConfig } from '@umijs/max';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { postJSON } from '@/services/request';

async function loadProfileFromToken(token: string): Promise<API.CurrentUser | undefined> {
  const res = await fetch('/api/v1/auth/profile', {
    headers: { Authorization: `Bearer ${token}` },
  });
  const json = (await res.json()) as { code: number; data?: API.CurrentUser };
  if (!res.ok || json.code !== 0 || !json.data) return undefined;
  return json.data;
}

export async function getInitialState(): Promise<{ currentUser?: API.CurrentUser }> {
  const token = localStorage.getItem(AUTH_TOKEN_KEY);
  if (!token) {
    return {};
  }
  const user = await loadProfileFromToken(token);
  if (!user) {
    localStorage.removeItem(AUTH_TOKEN_KEY);
    return {};
  }
  return { currentUser: user };
}

export const request: RequestConfig = {
  requestInterceptors: [
    (url, options) => {
      const token = localStorage.getItem(AUTH_TOKEN_KEY);
      const headers: Record<string, string> = {
        ...((options.headers as Record<string, string>) || {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      };
      return { url, options: { ...options, headers } };
    },
  ],
  errorConfig: {
    errorHandler: (error: any) => {
      if (error?.info?.skipErrorHandler) {
        throw error;
      }
      const status = error?.response?.status;
      const reqUrl = String(error?.config?.url || '');
      if (status === 401 && !reqUrl.includes('/auth/login')) {
        localStorage.removeItem(AUTH_TOKEN_KEY);
        const path = history.location.pathname;
        if (path !== '/user/login' && !path.startsWith('/user/login')) {
          const q = encodeURIComponent(path);
          window.location.assign(`${window.location.origin}/user/login?redirect=${q}`);
          return;
        }
      }
      throw error;
    },
  },
};

function RightActions() {
  const { setInitialState, initialState } = useModel('@@initialState');

  return (
    <Space>
      <span style={{ color: 'rgba(0,0,0,0.65)' }}>{initialState?.currentUser?.displayName}</span>
      <Button
        size="small"
        onClick={async () => {
          try {
            await postJSON('/api/v1/auth/logout');
          } catch {
            /* ignore */
          }
          localStorage.removeItem(AUTH_TOKEN_KEY);
          setInitialState((s) => ({ ...s, currentUser: undefined }));
          history.push('/user/login');
        }}
      >
        退出
      </Button>
    </Space>
  );
}

export const layout: RunTimeLayoutConfig = ({ initialState }) => ({
  logo: (
    <span style={{ fontWeight: 600, fontSize: 16, letterSpacing: 0.5 }}>
      <span style={{ color: '#1677ff' }}>贸灵</span>
      <span style={{ color: '#262626' }}> TradeMind</span>
    </span>
  ),
  menu: { locale: false },
  onPageChange: () => {
    const { pathname } = history.location;
    if (pathname === '/user/login') return;
    if (!initialState?.currentUser) {
      history.push(`/user/login?redirect=${encodeURIComponent(pathname)}`);
    }
  },
  rightContentRender: () => <RightActions key="nav-right" />,
});
