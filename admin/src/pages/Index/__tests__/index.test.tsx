import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { setLanguage } from "@/i18n";
import { THEME_MODE_STORAGE_KEY } from "@/theme";
import IndexPage from "../index";

beforeEach(() => {
  setLanguage("zh-CN");
  window.localStorage.removeItem(THEME_MODE_STORAGE_KEY);
  delete document.documentElement.dataset.theme;
  document.documentElement.style.removeProperty("color-scheme");
  document.documentElement.classList.remove("tm-theme-switching");
});

describe("IndexPage", () => {
  it("presents the public product homepage with direct auth routes", () => {
    render(<IndexPage />);

    expect(
      screen.getByRole("heading", { name: "让商品运营，从采集到刊登更顺畅" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "登录" })[0]).toHaveAttribute(
      "href",
      "/user/login",
    );
    expect(
      screen.getAllByRole("link", { name: "免费注册" })[0],
    ).toHaveAttribute("href", "/user/register");
    expect(
      screen.getByRole("heading", { name: "多来源商品采集" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "私有化部署" }),
    ).toBeInTheDocument();
  });

  it("switches and persists the public homepage theme", async () => {
    const interaction = userEvent.setup();
    render(<IndexPage />);

    const darkModeButton = screen.getByRole("button", {
      name: "切换到深色模式",
    });
    expect(darkModeButton).toHaveAttribute("aria-pressed", "false");

    await interaction.click(darkModeButton);

    expect(
      screen.getByRole("button", { name: "切换到浅色模式" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");
    expect(window.localStorage.getItem(THEME_MODE_STORAGE_KEY)).toBe("dark");
  });

  it("updates translated homepage copy without replacing the page", () => {
    const { container } = render(<IndexPage />);
    const page = container.firstElementChild;

    act(() => setLanguage("en-US"));

    expect(
      screen.getByRole("heading", { name: "Multi-platform publishing collaboration" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Sign in to workspace" }),
    ).toHaveAttribute("href", "/user/login");
    expect(container.firstElementChild).toBe(page);
  });
});
