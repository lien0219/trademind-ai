import { test, expect } from "../fixtures/admin.fixture";
import type { Page } from "@playwright/test";
import {
  supportedLanguages,
  translateMessage,
  type AppLanguage,
} from "../../src/i18n/dictionaries";
import { expectNoRootOverflow } from "../utils/assertions";
import {
  isPublicPagePath,
  productionPageRoutes,
  productionRedirectRoutes,
  resolveConfiguredPath,
  resolveRedirectTarget,
} from "../utils/route-catalogue";
import { e2eUser } from "../mocks/auth";
import { ok } from "../mocks/envelope";
import {
  expectLoadedRouteContent,
  openConfiguredRoute,
  routeTitle,
  withInitialApiReads,
} from "../utils/route-readiness";

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe.configure({ mode: "parallel", timeout: 15 * 60 * 1000 });
test.use({
  anonymousRoutePaths: productionPageRoutes
    .filter((route) => isPublicPagePath(route.path))
    .map((route) => resolveConfiguredPath(route.path)),
});

test("@route-catalogue resolves every dynamic production route", () => {
  for (const route of productionPageRoutes) {
    expect(resolveConfiguredPath(route.path), route.path).not.toMatch(
      /:[A-Za-z]/,
    );
  }
});

async function gotoRoute(page: Page, path: string) {
  await page.goto(path, { waitUntil: "commit", timeout: 30_000 });
  await expect(page).not.toHaveURL(/\/user\/login/);
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

async function mockRootWorkspace(page: Page) {
  await page.route("**/api/v1/auth/profile", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      json: ok({
        ...e2eUser,
        tenantId: 101,
        isRoot: true,
        platformOperator: true,
      }),
    });
  });
}

async function expectRouteReady(
  page: Page,
  route: { path: string; name?: string },
  language: AppLanguage,
) {
  const main = page.locator("main").first();
  await expect(main, `main content for ${route.path}`).toBeVisible();
  await expect
    .poll(async () => (await main.innerText()).trim().length, {
      message: `non-empty main content for ${route.path}`,
    })
    .toBeGreaterThan(0);

  const label = routeTitle(route, language)?.trim();
  if (label) {
    await expect(
      main
        .locator(".ant-page-header-heading-title")
        .filter({ hasText: new RegExp(escapeRegExp(label), "i") })
        .first(),
      `route title in main content for ${route.path}`,
    ).toBeVisible();
  }

  await expectLoadedRouteContent(page, route);

  const errorMessages = [
    "当前账号无权限访问此页面",
    "页面不存在",
    "加载失败",
    "网络或服务异常",
  ].map((message) => escapeRegExp(translateMessage(language, message)));
  await expect(main.getByText(new RegExp(errorMessages.join("|")))).toHaveCount(
    0,
  );
}

async function expectPublicRouteReady(
  page: Page,
  path: string,
  language: AppLanguage,
) {
  const tr = (message: string) => translateMessage(language, message);
  if (path === "/") {
    await expect(
      page.getByRole("heading", {
        name: tr("AI 跨境电商运营平台"),
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: tr("免费注册"), exact: true }).first(),
    ).toBeVisible();
  } else if (path === "*") {
    await expect(page.getByText("404", { exact: true })).toBeVisible();
  } else if (path === "/tenant/invitations/accept") {
    await expect(
      page.getByLabel(tr("一次性邀请令牌"), { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", {
        name: new RegExp(escapeRegExp(tr("登录并继续"))),
      }),
    ).toBeEnabled();
  } else {
    const registering = path === "/user/register";
    await expect(
      page.getByRole("tab", {
        name: tr(registering ? "注册" : "登录"),
        exact: true,
      }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(
      page.getByLabel(tr(registering ? "邮箱" : "邮箱 / 手机号"), {
        exact: true,
      }),
    ).toBeVisible();
    if (registering) {
      await expect(
        page.getByText(tr("请联系管理员创建账号，当前不开放自助注册"), {
          exact: true,
        }),
      ).toBeVisible();
      await expect(page.getByLabel(tr("邮箱"), { exact: true })).toBeDisabled();
    } else {
      await expect(
        page.getByRole("button", {
          name: new RegExp(escapeRegExp(tr("登录工作台"))),
        }),
      ).toBeEnabled();
    }
  }
}

for (const language of supportedLanguages) {
  for (const viewport of viewports) {
    test(`@route-catalogue renders every production page in ${language} at ${viewport.width}`, async ({
      admin,
      page,
    }) => {
      await page.addInitScript(
        ([key, value]) => {
          localStorage.setItem(key, value);
        },
        ["trademind_admin_language", language],
      );
      await mockRootWorkspace(page);
      await page.setViewportSize(viewport);

      for (const route of productionPageRoutes) {
        await test
          .step(route.path, () =>
            withInitialApiReads(page, async () => {
              const resolvedPath = resolveConfiguredPath(route.path);
              await openConfiguredRoute(page, route);
              await expect(page).toHaveURL(
                (url) => url.pathname === resolvedPath,
              );
              await expect(page.locator("#root")).toBeVisible();
              if (isPublicPagePath(route.path)) {
                await expectPublicRouteReady(page, route.path, language);
              } else {
                await expect(
                  page.locator(".tm-page-container").first(),
                  `page shell for ${route.path}`,
                ).toBeVisible();
                await expectRouteReady(page, route, language);
              }
              await expect(page.locator("html")).toHaveAttribute(
                "lang",
                language,
              );
              await expectNoRootOverflow(page);
            }),
          )
          .catch((error: unknown) => {
            expect
              .soft(error, `complete route verification for ${route.path}`)
              .toBeUndefined();
          });
      }

      for (const route of productionRedirectRoutes) {
        await test
          .step(`redirect ${route.path}`, () =>
            withInitialApiReads(page, async () => {
              await gotoRoute(page, resolveConfiguredPath(route.path));
              const target = new URL(
                resolveRedirectTarget(route.redirect ?? ""),
                "http://localhost",
              );
              await expect(page).toHaveURL(
                (url) =>
                  url.pathname === target.pathname &&
                  url.search === target.search,
              );
              const targetRoute = productionPageRoutes.find(
                (candidate) =>
                  resolveConfiguredPath(candidate.path) === target.pathname,
              );
              expect(
                targetRoute,
                `configured destination for ${route.path}`,
              ).toBeDefined();
              await expectRouteReady(page, targetRoute!, language);
              await expect(page.locator("html")).toHaveAttribute(
                "lang",
                language,
              );
              await expectNoRootOverflow(page);
            }),
          )
          .catch((error: unknown) => {
            expect
              .soft(error, `complete redirect verification for ${route.path}`)
              .toBeUndefined();
          });
      }

      await admin.writeGuard.expectNoUnexpectedWrites();
    });
  }
}

test("@route-catalogue rejects a static page whose initial GET has a business error", async ({
  page,
}) => {
  await page.route("**/api/v1/tenant/current", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      json: {
        code: 50001,
        message: "E2E organization load failure",
        data: null,
      },
    });
  });
  await expect(
    withInitialApiReads(page, async () => {
      await gotoRoute(page, "/tenant/settings");
      await expect(page.locator(".tm-page-container").first()).toBeVisible();
      await expect(
        page.getByText("E2E organization load failure", { exact: true }),
      ).toBeVisible();
    }),
  ).rejects.toThrow(/business code 50001/);
});

test("@route-catalogue rejects a failed legacy collector initial GET", async ({
  page,
}) => {
  await mockRootWorkspace(page);
  await page.route("**/api/collector/providers/1688/auth-status", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      json: {
        code: 50002,
        message: "E2E collector auth load failure",
        data: null,
      },
    });
  });
  await expect(
    withInitialApiReads(page, async () => {
      const initialRead = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname ===
          "/api/collector/providers/1688/auth-status",
        { timeout: 10_000 },
      );
      await gotoRoute(page, "/settings/collector");
      await initialRead;
      await expectRouteReady(page, { path: "/settings/collector" }, "zh-CN");
    }),
  ).rejects.toThrow(/business code 50002/);
});

test("@route-catalogue renders wildcard 404 without authenticated API reads", async ({
  admin,
  page,
}) => {
  let profileReads = 0;
  await page.route("**/api/v1/auth/profile", async (route) => {
    profileReads += 1;
    await route.fallback();
  });
  await gotoRoute(page, "/__e2e-route-not-found__");
  await expect(page.getByText("404", { exact: true })).toBeVisible();
  await expect(page.locator("#root")).toBeVisible();
  expect(profileReads).toBeLessThanOrEqual(1);
  await admin.writeGuard.expectNoUnexpectedWrites();
});
