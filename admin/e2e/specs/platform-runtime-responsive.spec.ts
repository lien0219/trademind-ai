import { test, expect } from "../fixtures/admin.fixture";
import { ok } from "../mocks/envelope";
import { e2eUser } from "../mocks/auth";
import {
  expectHeaderContentAligned,
  expectNoRootOverflow,
} from "../utils/assertions";

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
] as const;

test.describe("@smoke @platform-runtime-responsive 平台运行状态", () => {
  for (const viewport of viewports) {
    test(`keeps the Douyin operations panel usable at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.route("**/api/v1/platform/providers", async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              list: [
                {
                  platform: "douyin_shop",
                  name: "抖店 / Douyin Shop",
                  status: "beta",
                  authType: "oauth",
                  capabilities: ["product_publish", "inventory_sync"],
                  authSchema: [],
                  appConfigSchema: {
                    groupKey: "douyin_shop",
                    title: "抖店",
                    fields: [],
                  },
                  settingsGroupKey: "platform_douyin_shop",
                },
              ],
            }),
          ),
        });
      });

      await page.route("**/api/v1/auth/profile", (route) => route.fulfill({ json: ok({ ...e2eUser, isRoot: true, tenantId: 101, platformOperator: true }) }));
      await admin.goto("/ops/platform-runtime?platform=douyin_shop");

      await expect(
        page
          .locator(".tm-page-container")
          .getByText("平台运行状态", { exact: true })
          .first(),
      ).toBeVisible();
      await expect(
        page.getByRole("tab", { name: /抖店 \/ Douyin Shop/ }),
      ).toHaveAttribute("aria-selected", "true");
      await expect(
        page.getByRole("region", { name: "抖店运行概览" }),
      ).toBeVisible();
      await expect(page.getByText("运行控制", { exact: true })).toBeVisible();
      await expect(
        page.getByText("24 小时指标", { exact: true }),
      ).toBeVisible();
      await expect(
        page.getByText("发布门禁清单", { exact: true }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "刷新全部状态" }),
      ).toBeVisible();
      await expectHeaderContentAligned(page);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount("unexpected", 0);
    });
  }
});
