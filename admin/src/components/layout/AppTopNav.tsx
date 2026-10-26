import type { CSSProperties } from "react";
import {
  ApartmentOutlined,
  DownOutlined,
  GithubOutlined,
  LoadingOutlined,
  LogoutOutlined,
  SearchOutlined,
  SwapOutlined,
  TeamOutlined,
} from "@ant-design/icons";
import { history } from "@umijs/max";
import { Avatar, Button, Dropdown, Empty, Input, Popover, Spin, message } from "antd";
import { useCallback, useEffect, useState } from "react";
import { themeTokens, tmSemanticTokens } from "@/constants/layoutTokens";
import { canAccessPath } from "@/utils/menuAccess";
import { queryAuthTenants, switchAuthTenant, type AuthTenant } from "@/services/tenants";
import {
  installTenantSwitchListener,
  isTenantSwitching,
  subscribeTenantSwitch,
  switchTenantAndReload,
} from "@/utils/tenantContext";
import ThemeToggleButton from "./ThemeToggleButton";
import { visibleWorkspaceOptions, workspaceDisplayName, workspaceLandingPath } from "@/utils/tenantWorkspace";
import "./AppTopNav.less";

const TRADEMIND_GITHUB_URL = "https://github.com/lien0219/trademind-ai";

const avatarStyle: CSSProperties = {
  color: "#fff",
  background: `linear-gradient(135deg, ${themeTokens.colorPrimary} 0%, ${tmSemanticTokens.dataAccent} 100%)`,
};

export function resolveUserLabels(user?: API.CurrentUser) {
  const displayName = user?.displayName?.trim() || "管理员";
  const email = user?.email?.trim() || "";
  const username = user?.username?.trim() || "";
  const loginId = email || username;

  if (displayName.includes("@") && loginId && displayName === loginId) {
    const local = displayName.split("@")[0]?.trim() || displayName;
    return {
      primary: local,
      secondary: displayName,
      initial: local.slice(0, 1).toUpperCase(),
    };
  }

  return {
    primary: displayName,
    secondary: loginId && loginId !== displayName ? loginId : "",
    initial: displayName.slice(0, 1).toUpperCase(),
  };
}

type AppTopNavProps = {
  user?: API.CurrentUser;
  onLogout: () => void | Promise<void>;
  showThemeToggle?: boolean;
};

function TenantSwitcher({ user }: { user: API.CurrentUser }) {
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [tenants, setTenants] = useState<AuthTenant[]>([]);
  const [currentTenantId, setCurrentTenantId] = useState<number | undefined>(user.tenantId);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [switchingTenantId, setSwitchingTenantId] = useState<number | null>(null);

  const loadTenants = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const result = await queryAuthTenants();
      setTenants(result.items || []);
      setCurrentTenantId(result.currentTenantId ?? user.tenantId);
    } catch {
      setTenants([]);
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, [user.tenantId]);

  useEffect(() => {
    if (user.tenantId != null) void loadTenants();
  }, [loadTenants, user.tenantId]);

  const sourceTenantId = user.tenantId;
  if (sourceTenantId == null) return null;

  const needle = keyword.trim().toLowerCase();
  const availableWorkspaces = visibleWorkspaceOptions(tenants, user.isRoot === true);
  const visibleTenants = availableWorkspaces.filter((tenant) =>
    !needle || `${workspaceDisplayName(tenant)} ${tenant.code}`.toLowerCase().includes(needle),
  );
  const current = availableWorkspaces.find((tenant) => tenant.id === currentTenantId);
  const currentLabel = current ? workspaceDisplayName(current) :
    (user.tenantId === 0 ? "系统控制台" : (user.isRoot === true ? "Root 工作台" : user.tenantName || "当前企业"));
  const businessTenants = availableWorkspaces.filter((tenant) => tenant.id > 0);
  const isSystemWorkspace = user.tenantId === 0;

  const selectTenant = async (tenant: AuthTenant) => {
    if (tenant.id === currentTenantId || switchingTenantId !== null) return;
    setSwitchingTenantId(tenant.id);
    try {
      await switchTenantAndReload(tenant.id, switchAuthTenant, workspaceLandingPath(sourceTenantId, tenant.id));
    } catch (error) {
      setSwitchingTenantId(null);
      message.error((error as Error)?.message || "租户切换失败，请重试");
    }
  };

  const content = (
    <div className="tm-tenant-switcher__panel">
      <div className="tm-tenant-switcher__heading">切换工作空间</div>
      {isSystemWorkspace && user.isRoot !== true ? <div className="tm-tenant-switcher__hint">平台权限与企业业务权限独立；仅可进入已授权企业。</div> : null}
      <Input
        allowClear
        prefix={<SearchOutlined aria-hidden="true" />}
        value={keyword}
        placeholder="搜索企业名称或标识"
        aria-label="搜索企业"
        onChange={(event) => setKeyword(event.target.value)}
      />
      <div className="tm-tenant-switcher__list" role="listbox" aria-label="可访问企业">
        {loading ? (
          <div className="tm-tenant-switcher__state"><Spin size="small" /> 加载企业列表</div>
        ) : loadError ? (
          <div className="tm-tenant-switcher__state">
            <span>企业列表加载失败</span>
            <Button type="link" size="small" onClick={() => void loadTenants()}>重试</Button>
          </div>
        ) : visibleTenants.length ? (
          visibleTenants.map((tenant) => (
            <button
              type="button"
              key={tenant.id}
              role="option"
              aria-selected={tenant.id === currentTenantId}
              className={`tm-tenant-switcher__item${tenant.id === currentTenantId ? " is-current" : ""}`}
              disabled={tenant.status !== "active" || switchingTenantId !== null}
              onClick={() => void selectTenant(tenant)}
            >
              <span className="tm-tenant-switcher__item-main">
                <strong>{workspaceDisplayName(tenant)}</strong>
                <span>{tenant.code} · {tenant.role}</span>
              </span>
              {switchingTenantId === tenant.id ? <LoadingOutlined spin aria-label="切换中" /> : null}
              {tenant.id === currentTenantId && switchingTenantId === null ? <span aria-label="当前企业">当前</span> : null}
            </button>
          ))
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可访问工作空间" />
        )}
      </div>
      {!loading && !loadError && isSystemWorkspace && businessTenants.length === 0 ? (
        <div className="tm-tenant-switcher__hint">
          尚未加入任何企业。企业管理员可向该账号发出邀请。
          {user.platformOperator === true ? (
            <Button size="small" type="link" onClick={() => { setOpen(false); history.push('/platform/tenants'); }}>前往租户管理</Button>
          ) : null}
        </div>
      ) : null}
      {switchingTenantId !== null ? <div className="tm-tenant-switcher__hint">正在切换企业，页面即将刷新</div> : null}
    </div>
  );

  return (
    <Popover
      open={open}
      onOpenChange={setOpen}
      trigger="click"
      placement="bottomRight"
      content={content}
      overlayStyle={{ width: 320, maxWidth: "calc(100vw - 24px)" }}
    >
      <button
        type="button"
        className="tm-app-top-nav__tenant"
        aria-label={`${isSystemWorkspace || user.isRoot === true ? "当前工作空间" : "当前企业"} ${currentLabel}`}
        aria-expanded={open}
      >
        <ApartmentOutlined aria-hidden="true" />
        <span className="tm-app-top-nav__tenant-name" title={currentLabel}>{currentLabel}</span>
        <SwapOutlined aria-hidden="true" />
      </button>
    </Popover>
  );
}

function TenantSwitchOverlay() {
  const [visible, setVisible] = useState(isTenantSwitching());
  useEffect(() => {
    const unsubscribe = subscribeTenantSwitch(setVisible);
    const teardown = installTenantSwitchListener();
    return () => {
      unsubscribe();
      teardown();
    };
  }, []);
  return visible ? (
    <div className="tm-tenant-switch-overlay" role="status" aria-live="polite">
      <Spin size="small" />
      <span>正在切换企业，正在刷新工作区</span>
    </div>
  ) : null;
}

export default function AppTopNav({
  user,
  onLogout,
  showThemeToggle = true,
}: AppTopNavProps) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);

  if (!user) {
    return <nav className="tm-app-top-nav" aria-label="内容导航栏" />;
  }

  const { primary, secondary, initial } = resolveUserLabels(user);
  const account = secondary || user.username || "管理员";
  const canManageUsers = canAccessPath(
    "/settings/users",
    user.role,
    user.permissions,
    user.platformOperator === true,
    user.tenantId,
    user.isRoot === true,
  );

  const closeAndNavigate = (path: string) => {
    setIsMenuOpen(false);
    history.push(path);
  };

  return (
    <nav className="tm-app-top-nav" aria-label="内容导航栏">
      <TenantSwitchOverlay />
      <TenantSwitcher user={user} />
      {showThemeToggle ? (
        <ThemeToggleButton className="tm-app-top-nav__theme-toggle" />
      ) : null}
      <Dropdown
        menu={{
          items: [
            ...(user.tenantId != null && user.tenantId > 0
              ? [
                  {
                    key: "tenant-settings",
                    icon: <ApartmentOutlined aria-hidden="true" />,
                    label: "企业信息与设置",
                    onClick: () => closeAndNavigate("/tenant/settings"),
                  },
                  ...(canManageUsers
                    ? [
                        {
                          key: "tenant-members",
                          icon: <TeamOutlined aria-hidden="true" />,
                          label: "企业成员与邀请",
                          onClick: () => closeAndNavigate("/tenant/members"),
                        },
                      ]
                    : []),
                ]
              : []),
            ...(user.platformOperator === true
              ? [
                  {
                    key: "platform-tenants",
                    icon: <ApartmentOutlined aria-hidden="true" />,
                    label: "平台租户管理",
                    onClick: () => closeAndNavigate("/platform/tenants"),
                  },
                ]
              : []),
            ...(canManageUsers
              ? [
                  {
                    key: "users",
                    icon: <TeamOutlined aria-hidden="true" />,
                    label: "用户与权限",
                    onClick: () => closeAndNavigate("/settings/users"),
                  },
                ]
              : []),
            {
              key: "github",
              icon: <GithubOutlined aria-hidden="true" />,
              label: "GitHub",
              onClick: () => {
                setIsMenuOpen(false);
                window.open(
                  TRADEMIND_GITHUB_URL,
                  "_blank",
                  "noopener,noreferrer",
                );
              },
            },
            { type: "divider" },
            {
              key: "logout",
              danger: true,
              icon: (
                <LogoutOutlined
                  className="tm-app-account-dropdown__logout-icon"
                  aria-hidden="true"
                />
              ),
              label: (
                <span className="tm-app-account-dropdown__label">
                  <span className="tm-app-account-dropdown__title">
                    退出登录
                  </span>
                  <span className="tm-app-account-dropdown__description">
                    返回登录页面
                  </span>
                </span>
              ),
              onClick: () => {
                setIsMenuOpen(false);
                void onLogout();
              },
            },
          ],
        }}
        popupRender={(menu) => (
          <div className="tm-app-account-dropdown__panel">
            <div className="tm-app-account-dropdown__profile">
              <Avatar
                size={40}
                className="tm-app-account-dropdown__profile-avatar"
                style={avatarStyle}
              >
                {initial}
              </Avatar>
              <span className="tm-app-account-dropdown__profile-meta">
                <span className="tm-app-account-dropdown__profile-name">
                  {primary}
                </span>
                <span
                  className="tm-app-account-dropdown__profile-account"
                  title={account}
                >
                  {account}
                </span>
              </span>
            </div>
            {menu}
          </div>
        )}
        open={isMenuOpen}
        onOpenChange={setIsMenuOpen}
        overlayClassName="tm-app-account-dropdown"
        placement="bottomRight"
        trigger={["click"]}
        overlayStyle={{ width: 252, maxWidth: "calc(100vw - 16px)" }}
      >
        <button
          type="button"
          className="tm-app-top-nav__user"
          aria-label={`当前用户 ${primary}`}
          aria-haspopup="menu"
          aria-expanded={isMenuOpen}
        >
          <Avatar
            size={32}
            className="tm-app-top-nav__avatar"
            style={avatarStyle}
          >
            {initial}
          </Avatar>
          <span className="tm-app-top-nav__user-meta">
            <span className="tm-app-top-nav__user-name" title={primary}>
              {primary}
            </span>
            <span className="tm-app-top-nav__user-account" title={account}>
              {account}
            </span>
          </span>
          <DownOutlined
            className={`tm-app-top-nav__user-chevron${isMenuOpen ? " is-open" : ""}`}
            aria-hidden="true"
          />
        </button>
      </Dropdown>
    </nav>
  );
}
