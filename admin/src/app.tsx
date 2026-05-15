import type { CSSProperties, KeyboardEvent } from 'react';
import { LogoutOutlined } from '@ant-design/icons';
import { Avatar, Dropdown, Space } from 'antd';
import { history, useModel, type RequestConfig, type RunTimeLayoutConfig } from '@umijs/max';
import BrandLogo from '@/components/BrandLogo';
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

const TM_AVATAR_GRADIENT_BG = 'linear-gradient(135deg, #2563eb 0%, #0891b2 100%)';

const TM_AVATAR_STYLE: CSSProperties = { background: TM_AVATAR_GRADIENT_BG };

/** 侧栏 / 顶栏品牌图形（与登录页同一 `logo.png`） */
const TM_BRAND_MARK = <BrandLogo height={28} />;

async function logoutAndClear(
  setInitialState: (fn: (s: { currentUser?: API.CurrentUser }) => {
    currentUser?: API.CurrentUser;
  }) => Promise<unknown>,
) {
  try {
    await postJSON('/api/v1/auth/logout');
  } catch {
    /* ignore */
  }
  localStorage.removeItem(AUTH_TOKEN_KEY);
  setInitialState((s) => ({ ...s, currentUser: undefined }));
  history.push('/user/login');
}

/** 侧栏底部账号：悬停头像在右侧弹出菜单；勿把整行放进 Dropdown，便于「只在头像上触发」 */
function SiderUserFooter({ collapsed }: { collapsed?: boolean }) {
  const { setInitialState, initialState } = useModel('@@initialState');
  const name = initialState?.currentUser?.displayName?.trim() || '管理员';

  const menu = {
    items: [
      {
        key: 'logout',
        icon: <LogoutOutlined />,
        label: '退出登录',
        onClick: () => void logoutAndClear(setInitialState),
      },
    ],
  };

  return (
    <div
      className="tm-sider-user"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: collapsed ? 'center' : 'flex-start',
        gap: collapsed ? 0 : 10,
        padding: collapsed ? '12px 0' : '10px 16px',
        width: '100%',
        minWidth: 0,
      }}
    >
      <Dropdown
        menu={menu}
        trigger={['hover', 'click']}
        mouseEnterDelay={0.12}
        placement="rightTop"
        overlayStyle={{ minWidth: 128 }}
      >
        <span
          style={{
            display: 'inline-flex',
            cursor: 'pointer',
            lineHeight: 0,
          }}
        >
          <Avatar size={collapsed ? 30 : 28} style={TM_AVATAR_STYLE}>
            {name.slice(0, 1).toUpperCase()}
          </Avatar>
        </span>
      </Dropdown>
      {!collapsed ? (
        <span
          style={{
            flex: 1,
            minWidth: 0,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            fontSize: 13,
            color: 'rgba(0, 0, 0, 0.85)',
          }}
        >
          {name}
        </span>
      ) : null}
    </div>
  );
}

function RightActions() {
  const { setInitialState, initialState } = useModel('@@initialState');
  const name = initialState?.currentUser?.displayName?.trim() || '管理员';

  return (
    <Dropdown
      menu={{
        items: [
          {
            key: 'logout',
            icon: <LogoutOutlined />,
            label: '退出登录',
            onClick: () => void logoutAndClear(setInitialState),
          },
        ],
      }}
      placement="bottomRight"
    >
      <Space size={10} style={{ cursor: 'pointer', paddingInline: 4 }}>
        <Avatar size={32} style={{ ...TM_AVATAR_STYLE, fontSize: 14 }}>
          {name.slice(0, 1).toUpperCase()}
        </Avatar>
        <span className="tm-layout-user-name">{name}</span>
      </Space>
    </Dropdown>
  );
}

export const layout: RunTimeLayoutConfig = ({ initialState }) => ({
  title: false,
  logo: TM_BRAND_MARK,
  /** ProLayout 在侧栏会把 avatar 区域与（未定义 actionsRender 时的）rightContentRender 各渲染一遍，导致两行相同账号 */
  actionsRender: () => [],
  menuHeaderRender: (logoDom, _titleDom, props) => {
    const collapsed = props?.collapsed;
    const goHome = () => history.push('/dashboard');
    const interactive = {
      role: 'button' as const,
      tabIndex: 0,
      onClick: goHome,
      onKeyDown: (e: KeyboardEvent) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          goHome();
        }
      },
    };

    if (collapsed) {
      return (
        <div
          {...interactive}
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            padding: '14px 0 10px',
            cursor: 'pointer',
            width: '100%',
          }}
        >
          {logoDom}
        </div>
      );
    }

    return (
      <div
        {...interactive}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '14px 16px 10px',
          cursor: 'pointer',
          width: '100%',
          minWidth: 0,
        }}
      >
        {logoDom}
        <span
          style={{
            fontWeight: 600,
            fontSize: 16,
            letterSpacing: '-0.02em',
            color: '#0f172a',
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          贸灵 <span style={{ fontWeight: 500, color: '#64748b' }}>TradeMind</span>
        </span>
      </div>
    );
  },
  avatarProps: initialState?.currentUser
    ? {
        render: (_avatarProps, _defaultDom, menuProps) => (
          <SiderUserFooter collapsed={menuProps?.collapsed} />
        ),
      }
    : false,
  token: {
    headerHeight: 56,
    colorBgLayout: '#f4f6f9',
    colorTextMenuSelected: '#2563eb',
    colorBgMenuItemSelected: 'rgba(37, 99, 235, 0.09)',
  },
  menu: { locale: false },
  onPageChange: () => {
    const { pathname } = history.location;
    if (pathname === '/user/login' || pathname.startsWith('/user/login')) return;
    // 必须用 token 判断：initialState 在此闭包里不会在登录后刷新，会一直当作未登录并反复 push 登录页，触发 Navigate 死循环。
    if (!localStorage.getItem(AUTH_TOKEN_KEY)) {
      history.replace(`/user/login?redirect=${encodeURIComponent(pathname)}`);
    }
  },
  rightContentRender: () => <RightActions key="nav-right" />,
});
