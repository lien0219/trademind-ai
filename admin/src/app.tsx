import type { CSSProperties, KeyboardEvent, ReactElement } from 'react';
import { LogoutOutlined } from '@ant-design/icons';
import { Avatar, Dropdown, Space, Tooltip } from 'antd';
import { history, useModel, type RequestConfig, type RunTimeLayoutConfig } from '@umijs/max';
import AppMessageBridge from '@/components/AppMessageBridge';
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

/**
 * Runs inside umi antd innerProvider `<App>` (under ConfigProvider).
 * Do not add another `<App>` in rootContainer — that wraps outside ConfigProvider and breaks static message.
 */
export function innerProvider(container: ReactElement) {
  return (
    <>
      <AppMessageBridge />
      {container}
    </>
  );
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

function looksLikeEmail(value: string) {
  return value.includes('@');
}

/** 侧栏/顶栏展示：邮箱账号优先显示 @ 前昵称，完整邮箱放副行 */
function resolveUserLabels(user?: API.CurrentUser) {
  const displayName = user?.displayName?.trim() || '管理员';
  const email = user?.email?.trim() || '';
  const username = user?.username?.trim() || '';
  const loginId = email || username;

  if (looksLikeEmail(displayName) && loginId && displayName === loginId) {
    const local = displayName.split('@')[0]?.trim() || displayName;
    return {
      primary: local,
      secondary: displayName,
      initial: local.slice(0, 1).toUpperCase(),
    };
  }

  return {
    primary: displayName,
    secondary: loginId && loginId !== displayName ? loginId : '',
    initial: displayName.slice(0, 1).toUpperCase(),
  };
}

function buildLogoutMenu(
  setInitialState: (fn: (s: { currentUser?: API.CurrentUser }) => {
    currentUser?: API.CurrentUser;
  }) => Promise<unknown>,
) {
  return {
    items: [
      {
        key: 'logout',
        icon: <LogoutOutlined />,
        label: '退出登录',
        onClick: () => void logoutAndClear(setInitialState),
      },
    ],
  };
}

/** 侧栏底部账号：整行可点，向上弹出菜单；邮箱账号双行展示避免截断 */
function SiderUserFooter({ collapsed }: { collapsed?: boolean }) {
  const { setInitialState, initialState } = useModel('@@initialState');
  const { primary, secondary, initial } = resolveUserLabels(initialState?.currentUser);
  const menu = buildLogoutMenu(setInitialState);
  const tooltipTitle = secondary ? `${primary}\n${secondary}` : primary;

  const avatar = (
    <Avatar size={32} style={TM_AVATAR_STYLE}>
      {initial}
    </Avatar>
  );

  const body = (
    <div className="tm-sider-user">
      {avatar}
      <div className="tm-sider-user__meta">
        <span className="tm-sider-user__name" title={primary}>
          {primary}
        </span>
        {secondary ? (
          <span className="tm-sider-user__sub" title={secondary}>
            {secondary}
          </span>
        ) : (
          <span className="tm-sider-user__sub">管理员</span>
        )}
      </div>
    </div>
  );

  return (
    <Dropdown
      menu={menu}
      trigger={['click']}
      placement={collapsed ? 'rightTop' : 'topLeft'}
      overlayStyle={{ minWidth: 140 }}
    >
      <div
        className="tm-sider-user-trigger"
        role="button"
        tabIndex={0}
        aria-label={`当前用户 ${primary}`}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            (e.currentTarget as HTMLDivElement).click();
          }
        }}
      >
        {collapsed ? (
          <Tooltip title={tooltipTitle} placement="right">
            <span className="tm-sider-user tm-sider-user--collapsed">{avatar}</span>
          </Tooltip>
        ) : (
          body
        )}
      </div>
    </Dropdown>
  );
}

function RightActions() {
  const { setInitialState, initialState } = useModel('@@initialState');
  const { primary, initial } = resolveUserLabels(initialState?.currentUser);

  return (
    <Dropdown menu={buildLogoutMenu(setInitialState)} placement="bottomRight" trigger={['click']}>
      <Space size={10} className="tm-header-user" style={{ cursor: 'pointer', paddingInline: 4 }}>
        <Avatar size={32} style={{ ...TM_AVATAR_STYLE, fontSize: 14 }}>
          {initial}
        </Avatar>
        <span className="tm-layout-user-name" title={primary}>
          {primary}
        </span>
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
