import { test, expect } from "../fixtures/admin.fixture";
import type { Locator, Page } from "@playwright/test";
import {
  expectAccountInTopNavbar,
  expectNoRootOverflow,
} from "../utils/assertions";
import { AUTH_TOKEN_KEY } from "../../src/constants/auth";
import { THEME_MODE_STORAGE_KEY } from "../../src/theme/themeMode";
import { ok } from "../mocks/envelope";

const themeSurfaceColors = {
  light: {
    container: "rgb(255, 255, 255)",
    elevated: "rgb(255, 255, 255)",
  },
  dark: {
    container: "rgb(23, 26, 33)",
    elevated: "rgb(28, 32, 40)",
  },
} as const;

type ThemeFrame = {
  mode: string | undefined;
  switching: boolean;
  headerBackground: string | null;
  siderBackground: string | null;
  surfaceBackground: string | null;
};

type ThemeProbeWindow = Window & {
  __tmThemeFrames?: ThemeFrame[];
};

async function expectMobileDrawerOpaque(
  page: Page,
  mode: keyof typeof themeSurfaceColors,
) {
  const drawer = page.locator(".ant-drawer-content:visible").first();
  await expect(drawer).toBeVisible();
  await expect
    .poll(() =>
      drawer.evaluate((content) => content.getBoundingClientRect().left),
    )
    .toBeGreaterThanOrEqual(-1);

  const state = await drawer.evaluate((content) => {
    const body = content.querySelector<HTMLElement>(".ant-drawer-body");
    const sider = content.querySelector<HTMLElement>(".ant-pro-sider");
    const rect = content.getBoundingClientRect();
    const probeX = Math.max(
      1,
      Math.min(window.innerWidth - 1, rect.left + Math.max(24, rect.width / 2)),
    );
    const probeY = Math.min(
      rect.bottom - 24,
      Math.max(rect.top + 24, window.innerHeight / 2),
    );
    const topElement = document.elementFromPoint(probeX, probeY);

    return {
      contentBackground: window.getComputedStyle(content).backgroundColor,
      bodyBackground: body
        ? window.getComputedStyle(body).backgroundColor
        : null,
      siderBackground: sider
        ? window.getComputedStyle(sider).backgroundColor
        : null,
      topElementWithinDrawer: Boolean(
        topElement && content.contains(topElement),
      ),
      topElementClassName:
        topElement instanceof HTMLElement ? String(topElement.className) : null,
      drawerRect: {
        left: rect.left,
        right: rect.right,
        width: rect.width,
      },
    };
  });

  expect(
    [themeSurfaceColors[mode].container, themeSurfaceColors[mode].elevated],
    `${mode} drawer content surface ${JSON.stringify(state)}`,
  ).toContain(state.contentBackground);
  expect(
    state.bodyBackground,
    `${mode} drawer body ${JSON.stringify(state)}`,
  ).toBe(themeSurfaceColors[mode].container);
  expect(
    state.siderBackground,
    `${mode} drawer sider ${JSON.stringify(state)}`,
  ).toBe(themeSurfaceColors[mode].container);
  expect(
    state.topElementWithinDrawer,
    `drawer stacking ${JSON.stringify(state)}`,
  ).toBe(true);
}

async function expectAccountMenuSurface(
  page: Page,
  mode: keyof typeof themeSurfaceColors,
) {
  const accountTrigger = page.getByRole("button", { name: /^当前用户 / });
  await accountTrigger.click();
  const menu = page
    .locator(".tm-app-account-dropdown .ant-dropdown-menu:visible")
    .first();
  await expect(menu).toBeVisible();
  await expect(menu).toHaveCSS(
    "background-color",
    themeSurfaceColors[mode].elevated,
  );
  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
}

async function readThemeStyles(locator: Locator) {
  return locator.evaluate((element) => {
    const styles = window.getComputedStyle(element);
    return {
      backgroundColor: styles.backgroundColor,
      backgroundImage: styles.backgroundImage,
      borderColor: styles.borderColor,
      color: styles.color,
    };
  });
}

async function startThemeFrameProbe(page: Page, surfaceSelector: string) {
  await page.evaluate((selector) => {
    const probeWindow = window as ThemeProbeWindow;
    probeWindow.__tmThemeFrames = [];
    let frameCount = 0;

    const capture = () => {
      const header = document.querySelector<HTMLElement>(".tm-app-top-nav");
      const sider = document.querySelector<HTMLElement>(".ant-pro-sider");
      const surface = document.querySelector<HTMLElement>(selector);
      const background = (element: HTMLElement | null) =>
        element ? window.getComputedStyle(element).backgroundColor : null;
      const paintedBackground = (element: HTMLElement | null) => {
        let current = element;

        while (current) {
          const color = background(current);
          if (
            color &&
            color !== "transparent" &&
            color !== "rgba(0, 0, 0, 0)"
          ) {
            return color;
          }
          current = current.parentElement;
        }

        return null;
      };

      probeWindow.__tmThemeFrames?.push({
        mode: document.documentElement.dataset.theme,
        switching:
          document.documentElement.classList.contains("tm-theme-switching"),
        headerBackground: paintedBackground(header),
        siderBackground: background(sider),
        surfaceBackground: background(surface),
      });

      frameCount += 1;
      if (frameCount < 12) window.requestAnimationFrame(capture);
    };

    window.requestAnimationFrame(capture);
  }, surfaceSelector);
}

async function readThemeFrames(page: Page) {
  await page.waitForFunction(
    () => ((window as ThemeProbeWindow).__tmThemeFrames?.length ?? 0) >= 12,
  );
  return page.evaluate(
    () => (window as ThemeProbeWindow).__tmThemeFrames ?? [],
  );
}

function expectThemeFramesConsistent(
  frames: ThemeFrame[],
  mode: keyof typeof themeSurfaceColors,
) {
  const themedFrames = frames.filter((frame) => frame.mode === mode);
  expect(themedFrames.length, `${mode} frame count`).toBeGreaterThan(0);
  const settledFrame = themedFrames.at(-1);

  expect(settledFrame, `${mode} settled frame`).toBeDefined();
  expect(
    settledFrame?.headerBackground,
    `${mode} visible header ${JSON.stringify(settledFrame)}`,
  ).not.toBeNull();
  expect(settledFrame?.siderBackground).toBe(
    themeSurfaceColors[mode].container,
  );
  expect(settledFrame?.surfaceBackground).toBe(
    themeSurfaceColors[mode].container,
  );

  for (const frame of themedFrames) {
    expect(frame, `${mode} mixed frame ${JSON.stringify(frame)}`).toMatchObject(
      {
        headerBackground: settledFrame?.headerBackground,
        siderBackground: settledFrame?.siderBackground,
        surfaceBackground: settledFrame?.surfaceBackground,
      },
    );
  }
}

const smokeRoutes = [
  { path: "/dashboard/product-operations", name: /运营总览|工作台/ },
  { path: "/collect/hub", name: /采集中心/ },
  { path: "/ai/operation-workbench", name: /商品运营工作台/ },
  { path: "/product/drafts", name: /商品草稿|E2E 商品草稿/ },
  { path: "/inventory/overview", name: /库存中心/ },
  { path: "/procurement/purchase-orders", name: /采购单/ },
  { path: "/ops/task-center/alerts", name: /告警中心/ },
  { path: "/ops/task-center/operation-tasks", name: /运营任务中心/ },
  { path: "/ops/observability", name: /可观测性中心/ },
  { path: "/settings/alert-notify", name: /告警通知配置/ },
  { path: "/files", name: /文件管理/ },
];

test.describe("@smoke Admin route smoke", () => {
  for (const route of smokeRoutes) {
    test(`renders ${route.path} without login, fatal error, or writes`, async ({
      admin,
      page,
    }) => {
      await admin.goto(route.path);
      await expect(page.locator("#root")).toBeVisible();
      await expect(page.getByText(route.name).first()).toBeVisible();
      await expect(page).not.toHaveURL(/\/user\/login/);
      await expectAccountInTopNavbar(page);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount("unexpected", 0);
    });
  }

  test("opens the shared table density and column setting controls", async ({
    admin,
    page,
  }) => {
    await admin.goto("/files");

    const table = page.locator(".tm-pro-table").first();
    await table.getByRole("button", { name: "表格密度" }).click();
    await expect(page.getByRole("menuitem", { name: "宽松" })).toBeVisible();

    await page.keyboard.press("Escape");
    await table.getByRole("button", { name: "列设置" }).click();
    await expect(page.getByText("列展示", { exact: true })).toBeVisible();
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("does not render persistent instructional banners", async ({
    admin,
    page,
  }) => {
    const removedBanners = [
      {
        path: "/collect/hub",
        messages: ["店铺归属与权限提示", "不要承诺百分百采集成功"],
      },
      { path: "/collect/tasks", messages: ["店铺归属与权限提示"] },
      { path: "/orders/sync-tasks", messages: ["抖店订单同步说明"] },
      { path: "/settings/inventory", messages: ["默认值说明"] },
      { path: "/settings/security", messages: ["空闲超时说明"] },
      {
        path: "/settings/integrations",
        messages: ["贸灵不提供也不内置任何第三方密钥"],
      },
    ];

    for (const entry of removedBanners) {
      await admin.goto(entry.path);
      await expect(page.locator("#root")).toBeVisible();
      for (const message of entry.messages) {
        await expect(page.getByText(message, { exact: true })).toHaveCount(0);
      }
    }

    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("shows native Feishu and Enterprise WeChat robot channels", async ({
    admin,
    page,
  }) => {
    await admin.goto("/settings/alert-notify");

    await expect(page.getByText("飞书", { exact: true })).toBeVisible();
    await expect(page.getByText("企业微信", { exact: true })).toBeVisible();
    await expect(page.getByText(/当前保存后发送结果为 skipped/)).toHaveCount(0);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("renders platform runtime tabs without console warnings", async ({
    admin,
    page,
  }) => {
    await page.route("**/api/v1/platform/providers", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({
            list: [
              {
                platform: "shopify",
                name: "Shopify",
                status: "planned",
                authType: "oauth",
                capabilities: [],
              },
            ],
          }),
        ),
      });
    });

    await admin.goto("/ops/platform-runtime");

    await expect(page.getByText("平台运行状态").first()).toBeVisible();
    await expect(page.getByRole("tab", { name: /Shopify/ })).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("renders operational observability status without writes", async ({
    admin,
    page,
  }) => {
    await admin.goto("/ops/observability");

    await expect(page.getByText("需要处理").first()).toBeVisible();
    await expect(page.getByText("活跃系统告警")).toBeVisible();
    await expect(page.getByText("最近评估成功").first()).toBeVisible();
    await expect(page.getByText("未配置导出后端").first()).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("opens logout from the account dropdown without triggering a write", async ({
    admin,
    page,
  }) => {
    await admin.goto("/dashboard/product-operations");

    const accountTrigger = page.getByRole("button", { name: /^当前用户 / });
    await expect(accountTrigger).toHaveAttribute("aria-haspopup", "menu");
    await expect(accountTrigger).toHaveAttribute("aria-expanded", "false");

    await accountTrigger.click();
    await expect(accountTrigger).toHaveAttribute("aria-expanded", "true");
    await expect(page.getByText("用户与权限")).toBeVisible();
    await expect(page.getByRole("menuitem", { name: /API 密钥/ })).toHaveCount(
      0,
    );
    await expect(page.getByText("联系客服")).toHaveCount(0);
    await expect(
      page.getByRole("menuitem", { name: /退出登录/ }),
    ).toBeVisible();
    await expect(page.getByText("返回登录页面")).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(accountTrigger).toHaveAttribute("aria-expanded", "false");
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("keeps the brand in the sider and switches theme without mixed frames", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto("/dashboard/product-operations");

    await expect(
      page.locator(".ant-pro-sider .tm-app-brand-header"),
    ).toBeVisible();
    await expect(
      page.locator(".ant-pro-global-header .tm-app-brand-header"),
    ).toHaveCount(0);
    const collapseControl = page.getByRole("button", { name: "收起侧栏" });
    await expect(collapseControl).toBeVisible();
    await expect(page.locator(".tm-app-sider-footer")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "搜索功能或页面" }),
    ).toBeVisible();
    const searchBox = await page
      .getByRole("button", { name: "搜索功能或页面" })
      .boundingBox();
    expect(searchBox).not.toBeNull();
    if (!searchBox) {
      throw new Error("desktop search control must have a layout box");
    }
    expect(searchBox.x).toBeGreaterThanOrEqual(0);
    const footerBox = await page.locator(".tm-app-sider-footer").boundingBox();
    const siderBox = await page.locator(".ant-pro-sider").boundingBox();
    expect(footerBox).not.toBeNull();
    expect(siderBox).not.toBeNull();
    if (!footerBox || !siderBox) {
      throw new Error("sider footer and sider must have layout boxes");
    }
    expect(footerBox.y + footerBox.height).toBeGreaterThanOrEqual(
      siderBox.y + siderBox.height - 24,
    );
    const brandLogoBox = await page.locator(".tm-app-brand-logo").boundingBox();
    const firstNavigationIconBox = await page
      .getByRole("menuitem", { name: /工作台/ })
      .first()
      .locator(".anticon")
      .first()
      .boundingBox();
    expect(brandLogoBox).not.toBeNull();
    expect(firstNavigationIconBox).not.toBeNull();
    if (!brandLogoBox || !firstNavigationIconBox) {
      throw new Error("brand and navigation icons must have layout boxes");
    }
    expect(
      Math.abs(brandLogoBox.x - firstNavigationIconBox.x),
      `brand left ${brandLogoBox.x} vs navigation icon left ${firstNavigationIconBox.x}`,
    ).toBeLessThanOrEqual(4);
    await expect(page.locator(".tm-app-brand-logo")).toHaveCount(1);

    const darkThemeAction = page.getByRole("button", {
      name: "切换到深色模式",
    });
    await expect(darkThemeAction).toHaveText("");
    await darkThemeAction.hover();
    await expect(page.getByRole("tooltip")).toHaveText("切换到深色模式");
    await page.mouse.move(720, 450);

    await expect(page.locator(".tm-metric-card").first()).toBeVisible();
    await startThemeFrameProbe(page, ".tm-metric-card");
    await darkThemeAction.click();
    expectThemeFramesConsistent(await readThemeFrames(page), "dark");
    await expect(page.locator("html")).not.toHaveClass(/tm-theme-switching/);

    await startThemeFrameProbe(page, ".tm-metric-card");
    await page.getByRole("button", { name: "切换到浅色模式" }).click();
    expectThemeFramesConsistent(await readThemeFrames(page), "light");
    await expect(page.locator("html")).not.toHaveClass(/tm-theme-switching/);

    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("searches the accessible navigation and opens the selected page", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto("/dashboard/product-operations");

    await page.getByRole("button", { name: "搜索功能或页面" }).click();
    const dialog = page.getByRole("dialog", { name: "搜索功能" });
    await expect(dialog).toBeVisible();
    await dialog
      .getByRole("textbox", { name: "搜索功能或页面" })
      .fill("告警中心");
    await dialog.getByRole("button", { name: /告警中心/ }).click();

    await expect(page).toHaveURL(
      /\/ops\/task-center\/alerts\?source=business$/,
    );
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("centers the selected navigation icon after the desktop sider collapses", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto("/ops/task-center/alerts");

    await page.getByRole("button", { name: "收起侧栏" }).click();
    await expect(page.locator(".ant-pro-sider-collapsed")).toBeVisible();
    await expect(
      page.locator(".ant-pro-sider-collapsed .tm-app-brand-header--collapsed"),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "展开侧栏" })).toBeVisible();

    const sider = page.locator(".ant-pro-sider-collapsed");
    const selectedItem = sider.locator(
      ".ant-menu-submenu-selected > .ant-menu-submenu-title",
    );
    const selectedIcon = selectedItem.locator(".anticon").first();
    await expect(sider).toBeVisible();
    await expect(selectedItem).toBeVisible();
    await expect(selectedIcon).toBeVisible();

    const centers = await Promise.all(
      [sider, selectedItem, selectedIcon].map((locator) =>
        locator.evaluate((element) => {
          const rect = element.getBoundingClientRect();
          return rect.left + rect.width / 2;
        }),
      ),
    );
    const [siderCenter, selectedItemCenter, selectedIconCenter] = centers;

    expect(
      Math.abs(selectedItemCenter - siderCenter),
      `selected background center ${selectedItemCenter} vs sider center ${siderCenter}`,
    ).toBeLessThanOrEqual(1);
    expect(
      Math.abs(selectedIconCenter - selectedItemCenter),
      `selected icon center ${selectedIconCenter} vs background center ${selectedItemCenter}`,
    ).toBeLessThanOrEqual(1);

    await page.getByRole("button", { name: "展开侧栏" }).click();
    await expect(
      page.locator(".ant-pro-sider .tm-app-brand-header"),
    ).toBeVisible();
    await expect(page.locator(".ant-pro-sider-collapsed")).toHaveCount(0);

    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("keeps the mobile account actions in one header and uses a readable themed menu", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await admin.goto("/dashboard/product-operations");

    await expectAccountInTopNavbar(page);
    await expect(
      page.locator(".ant-pro-global-header .tm-app-brand-logo"),
    ).toBeVisible();
    await page.locator(".ant-pro-global-header-collapsed-button").click();

    const drawer = page.locator(".ant-drawer-content:visible").first();
    await expect(drawer).toBeVisible();
    await expect(drawer.locator(".tm-app-brand-header")).toHaveCount(0);
    await expect(page.locator(".tm-app-brand-logo")).toHaveCount(1);
    await expect(
      page.getByRole("button", { name: "搜索功能或页面" }),
    ).toBeVisible();
    await expect(drawer.getByText("运维", { exact: true })).toBeVisible();
    await expectMobileDrawerOpaque(page, "light");

    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await page.getByRole("button", { name: "切换到深色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await page.evaluate(() => {
      const scroller = document.scrollingElement as HTMLElement | null;
      if (scroller)
        scroller.scrollTop = Math.min(
          640,
          scroller.scrollHeight - scroller.clientHeight,
        );
    });
    await page.locator(".ant-pro-global-header-collapsed-button").click();

    const darkDrawer = page.locator(".ant-drawer-content:visible").first();
    await expect(darkDrawer).toBeVisible();
    await expectMobileDrawerOpaque(page, "dark");

    await page.keyboard.press("Escape");
    await expect(darkDrawer).toBeHidden();
    await expectAccountMenuSurface(page, "dark");

    await page.getByRole("button", { name: "切换到浅色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await expectAccountMenuSurface(page, "light");
    await page.locator(".ant-pro-global-header-collapsed-button").click();
    await expectMobileDrawerOpaque(page, "light");

    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("keeps the public homepage responsive and opens both auth routes", async ({
    admin,
    page,
  }) => {
    await page.addInitScript((authKey) => {
      window.localStorage.removeItem(authKey);
    }, AUTH_TOKEN_KEY);
    await page.emulateMedia({ reducedMotion: "no-preference" });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/");

    const hero = page.locator(".landing-hero");
    const lightHeroBackground = await hero.evaluate(
      (element) => getComputedStyle(element).backgroundColor,
    );

    await page.getByRole("button", { name: "切换到深色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await expect(
      page.getByRole("button", { name: "切换到浅色模式" }),
    ).toHaveAttribute("aria-pressed", "true");
    await expect
      .poll(() =>
        hero.evaluate((element) => getComputedStyle(element).backgroundColor),
      )
      .not.toBe(lightHeroBackground);

    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await page.getByRole("button", { name: "切换到浅色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.reload();
    await expect
      .poll(() =>
        page
          .locator(".landing-capability-card")
          .first()
          .evaluate((element) => getComputedStyle(element).opacity),
      )
      .toBe("1");
    await page.emulateMedia({ reducedMotion: "no-preference" });

    for (const viewport of [
      { width: 1440, height: 900 },
      { width: 1280, height: 800 },
      { width: 1024, height: 768 },
      { width: 768, height: 900 },
      { width: 375, height: 812 },
    ]) {
      await page.setViewportSize(viewport);
      await page.goto("/");
      await expect(
        page.getByRole("heading", { name: /从采集到刊登更顺畅/ }),
      ).toBeVisible();
      await expectNoRootOverflow(page);
    }

    await page.locator("#capabilities").scrollIntoViewIfNeeded();
    await expect(page.locator(".landing-capability-card").first()).toHaveClass(
      /is-visible/,
    );

    await page.getByRole("link", { name: "免费注册" }).first().click();
    await expect(page).toHaveURL(/\/user\/register/);
    await expect(page.getByRole("tab", { name: "注册" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await page.goto("/");
    await page.getByRole("link", { name: "登录", exact: true }).click();
    await expect(page).toHaveURL(/\/user\/login/);
    await expect(page.getByRole("tab", { name: "登录" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("centers login and registration safely on mobile", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 659, height: 1301 });
    await page.addInitScript(
      ([authKey, themeKey]) => {
        window.localStorage.removeItem(authKey);
        window.localStorage.removeItem(themeKey);
      },
      [AUTH_TOKEN_KEY, THEME_MODE_STORAGE_KEY],
    );
    await page.goto("/user/register");
    await expect(page).toHaveURL(/\/user\/register/);

    const expectCentered = async (tab: "登录" | "注册") => {
      await expect(page.getByRole("tab", { name: tab })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      const metrics = await page
        .locator(".login-right-inner")
        .evaluate((element) => {
          const rect = element.getBoundingClientRect();
          return {
            horizontalDelta: Math.abs(
              (rect.left + rect.right) / 2 - window.innerWidth / 2,
            ),
            verticalDelta: Math.abs(
              rect.top - (window.innerHeight - rect.bottom),
            ),
            top: rect.top,
            bottom: rect.bottom,
          };
        });

      expect(
        metrics.horizontalDelta,
        `${tab} horizontal center ${JSON.stringify(metrics)}`,
      ).toBeLessThanOrEqual(1);
      expect(
        metrics.verticalDelta,
        `${tab} safe vertical center ${JSON.stringify(metrics)}`,
      ).toBeLessThanOrEqual(10);
      expect(
        metrics.top,
        `${tab} top in viewport ${JSON.stringify(metrics)}`,
      ).toBeGreaterThanOrEqual(0);
      expect(
        metrics.bottom,
        `${tab} bottom in viewport ${JSON.stringify(metrics)}`,
      ).toBeLessThanOrEqual(1301);
    };

    await expectCentered("注册");
    await page.getByRole("tab", { name: "登录" }).click();
    await expect(page).toHaveURL(/\/user\/login/);
    await expectCentered("登录");
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("repaints mounted controls and keeps the mobile avatar visible across a theme round trip", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 535, height: 1301 });
    await admin.goto("/settings/email");

    const sendTestButton = page.getByRole("button", {
      name: /发送测试邮件/,
    });
    const avatar = page.locator(".tm-app-top-nav__avatar");

    await expect(sendTestButton).toBeVisible();
    await expect(avatar).toBeVisible();
    await expect(avatar).toHaveText("E");

    const lightButton = await readThemeStyles(sendTestButton);
    const lightAvatar = await readThemeStyles(avatar);
    expect(lightAvatar.backgroundImage).not.toBe("none");
    expect(lightAvatar.color).toBe("rgb(255, 255, 255)");

    await page.getByRole("button", { name: "切换到深色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await expect
      .poll(() => readThemeStyles(sendTestButton))
      .not.toEqual(lightButton);
    await expect(avatar).toHaveText("E");
    expect((await readThemeStyles(avatar)).backgroundImage).not.toBe("none");
    expect((await readThemeStyles(avatar)).color).toBe("rgb(255, 255, 255)");

    await page.getByRole("button", { name: "切换到浅色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await expect
      .poll(() => readThemeStyles(sendTestButton))
      .toEqual(lightButton);
    await expect.poll(() => readThemeStyles(avatar)).toEqual(lightAvatar);
    await expect(avatar).toHaveText("E");

    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("switches theme and restores the stored preference after reload", async ({
    admin,
    page,
  }) => {
    await admin.goto("/dashboard/product-operations");

    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await page.getByRole("button", { name: "切换到深色模式" }).click();

    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await expect(
      page.getByRole("button", { name: "切换到浅色模式" }),
    ).toBeVisible();
    await expect
      .poll(() =>
        page.evaluate(
          (storageKey) => ({
            storedMode: window.localStorage.getItem(storageKey),
            colorScheme: document.documentElement.style.colorScheme,
          }),
          THEME_MODE_STORAGE_KEY,
        ),
      )
      .toEqual({ storedMode: "dark", colorScheme: "dark" });

    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await expect(
      page.getByRole("button", { name: "切换到浅色模式" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "切换到浅色模式" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await expect
      .poll(() =>
        page.evaluate(
          (storageKey) => ({
            storedMode: window.localStorage.getItem(storageKey),
            colorScheme: document.documentElement.style.colorScheme,
          }),
          THEME_MODE_STORAGE_KEY,
        ),
      )
      .toEqual({ storedMode: "light", colorScheme: "light" });

    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await expect(
      page.getByRole("button", { name: "切换到深色模式" }),
    ).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });
});
