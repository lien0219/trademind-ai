import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { history, useAntdConfigSetter } from "@umijs/max";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { THEME_MODE_STORAGE_KEY } from "@/theme";
import AppTopNav, { resolveUserLabels } from "../AppTopNav";

const user: API.CurrentUser = {
  id: "test-user",
  username: "operator@example.test",
  email: "operator@example.test",
  displayName: "运营账号",
};

beforeEach(() => {
  window.localStorage.removeItem(THEME_MODE_STORAGE_KEY);
  delete document.documentElement.dataset.theme;
  document.documentElement.style.removeProperty("color-scheme");
  document.documentElement.classList.remove("tm-theme-switching");
  document.documentElement.scrollTop = 0;
  document.body.scrollTop = 0;
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }),
  });
});

describe("AppTopNav", () => {
  it("shows the current account in the content navigation", () => {
    render(<AppTopNav user={user} onLogout={vi.fn()} />);

    const navigation = screen.getByRole("navigation", { name: "内容导航栏" });
    expect(navigation).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "当前用户 运营账号" }),
    ).toBeInTheDocument();
    expect(screen.getByText("operator@example.test")).toBeInTheDocument();
  });

  it("keeps logout available from the account menu", async () => {
    const onLogout = vi.fn();
    const interaction = userEvent.setup();
    render(<AppTopNav user={user} onLogout={onLogout} />);

    const accountTrigger = screen.getByRole("button", {
      name: "当前用户 运营账号",
    });
    expect(accountTrigger).toHaveAttribute("aria-expanded", "false");

    await interaction.click(accountTrigger);
    expect(accountTrigger).toHaveAttribute("aria-expanded", "true");
    await interaction.click(
      await screen.findByRole("menuitem", { name: /退出登录/ }),
    );

    expect(onLogout).toHaveBeenCalledTimes(1);
    expect(accountTrigger).toHaveAttribute("aria-expanded", "false");
  });

  it("groups account details and permitted destinations in the menu", async () => {
    const interaction = userEvent.setup();
    const openWindow = vi.spyOn(window, "open").mockReturnValue(null);
    render(<AppTopNav user={user} onLogout={vi.fn()} />);

    await interaction.click(
      screen.getByRole("button", { name: "当前用户 运营账号" }),
    );

    expect(await screen.findByText("用户与权限")).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /API 密钥/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("联系客服")).not.toBeInTheDocument();

    await interaction.click(
      screen.getByRole("menuitem", { name: "用户与权限" }),
    );
    expect(history.push).toHaveBeenCalledWith("/settings/users");

    await interaction.click(
      screen.getByRole("button", { name: "当前用户 运营账号" }),
    );
    await interaction.click(screen.getByRole("menuitem", { name: "GitHub" }));
    expect(openWindow).toHaveBeenCalledWith(
      "https://github.com/lien0219/trademind-ai",
      "_blank",
      "noopener,noreferrer",
    );
  });

  it("shows the theme action as an icon with an accessible tooltip", async () => {
    const interaction = userEvent.setup();
    render(<AppTopNav user={user} onLogout={vi.fn()} />);

    const themeAction = screen.getByRole("button", {
      name: "切换到深色模式",
    });
    expect(themeAction.textContent).toBe("");

    await interaction.hover(themeAction);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "切换到深色模式",
    );
  });

  it("defaults to light mode and persists theme changes", async () => {
    const setAntdConfig = vi.fn();
    vi.mocked(useAntdConfigSetter).mockReturnValue(setAntdConfig);
    const interaction = userEvent.setup();
    render(<AppTopNav user={user} onLogout={vi.fn()} />);

    const darkModeButton = screen.getByRole("button", {
      name: "切换到深色模式",
    });
    expect(darkModeButton).toHaveAttribute("aria-pressed", "false");
    expect(document.documentElement.dataset.theme).toBe("light");

    await interaction.click(darkModeButton);

    expect(
      screen.getByRole("button", { name: "切换到浅色模式" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(window.localStorage.getItem(THEME_MODE_STORAGE_KEY)).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");
    expect(setAntdConfig).toHaveBeenCalledTimes(1);

    await interaction.click(
      screen.getByRole("button", { name: "切换到浅色模式" }),
    );

    expect(
      screen.getByRole("button", { name: "切换到深色模式" }),
    ).toHaveAttribute("aria-pressed", "false");
    expect(window.localStorage.getItem(THEME_MODE_STORAGE_KEY)).toBe("light");
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(document.documentElement.style.colorScheme).toBe("light");
    expect(setAntdConfig).toHaveBeenCalledTimes(2);
    expect(setAntdConfig).toHaveBeenLastCalledWith(
      expect.objectContaining({
        theme: expect.objectContaining({
          cssVar: { key: "trademind-admin-light" },
          token: expect.objectContaining({ colorBgElevated: "#ffffff" }),
        }),
      }),
    );
  });

  it("shortens an email display name while retaining the full account", () => {
    expect(
      resolveUserLabels({ ...user, displayName: "operator@example.test" }),
    ).toEqual({
      primary: "operator",
      secondary: "operator@example.test",
      initial: "O",
    });
  });
});
