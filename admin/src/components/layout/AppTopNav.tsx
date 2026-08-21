import { useState } from "react";
import type { CSSProperties } from "react";
import {
  DownOutlined,
  GithubOutlined,
  LogoutOutlined,
  TeamOutlined,
} from "@ant-design/icons";
import { history } from "@umijs/max";
import { Avatar, Dropdown } from "antd";
import { themeTokens, tmSemanticTokens } from "@/constants/layoutTokens";
import { canAccessPath } from "@/utils/menuAccess";
import ThemeToggleButton from "./ThemeToggleButton";
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
  );

  const closeAndNavigate = (path: string) => {
    setIsMenuOpen(false);
    history.push(path);
  };

  return (
    <nav className="tm-app-top-nav" aria-label="内容导航栏">
      {showThemeToggle ? (
        <ThemeToggleButton className="tm-app-top-nav__theme-toggle" />
      ) : null}
      <Dropdown
        menu={{
          items: [
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
