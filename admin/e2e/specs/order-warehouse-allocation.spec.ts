import { test, expect } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { ok } from "../mocks/envelope";
import {
  E2E_ALLOCATION_ORDER_ID,
  E2E_ALLOCATION_REVISION,
  E2E_ALLOCATION_WAREHOUSE_ID,
  e2eWarehouseAllocation,
} from "../mocks/order-warehouse-allocation";
import {
  expectModalWithinViewport,
  expectNoRootOverflow,
} from "../utils/assertions";

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe("@smoke order warehouse allocation V1", () => {
  for (const viewport of viewports) {
    test(`renders the workbench and responsive candidate drawer at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto("/orders/warehouse-allocations");
      await expect(
        page.getByText("SO-E2E-ALLOC-0001", { exact: true }),
      ).toBeVisible();
      await expectNoRootOverflow(page);

      await page.getByRole("button", { name: "查看" }).click();
      await expect(page.getByText("分仓候选详情")).toBeVisible();
      const allocationDrawer = page.locator(".tm-app-drawer");
      await expect(
        allocationDrawer.getByText("MAIN · E2E 华东主仓", { exact: true }),
      ).toBeVisible();
      await expect(
        allocationDrawer.getByText("SECOND · E2E 华南备仓", { exact: true }),
      ).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount("unexpected", 0);
    });
  }

  test("keeps cancel side-effect free and confirms exactly one revision-bound allocation", async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: "confirm-warehouse-allocation",
      method: "POST",
      path: new RegExp(
        `^/api/v1/orders/${E2E_ALLOCATION_ORDER_ID}/warehouse-allocation$`,
      ),
      response: ok({
        allocation: {
          ...e2eWarehouseAllocation,
          status: "allocated",
          warehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
          warehouseCode: "MAIN",
          warehouseName: "E2E 华东主仓",
          candidates: [],
        },
        inventoryReserve: { action: "reserve", linesSynced: 1 },
      }),
    });

    await admin.goto("/orders/warehouse-allocations");
    await page.getByRole("button", { name: "查看" }).click();
    await expect(
      page
        .locator(".tm-app-drawer")
        .getByText("MAIN · E2E 华东主仓", { exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "确认分仓并预占库存" }).click();
    let dialog = page.getByRole("dialog", { name: "确认整单分配到该仓库？" });
    await expectModalWithinViewport(page);
    await dialog.getByRole("button", { name: /取\s*消/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount(
      "confirm-warehouse-allocation",
      0,
    );

    await page.getByRole("button", { name: "确认分仓并预占库存" }).click();
    dialog = page.getByRole("dialog", { name: "确认整单分配到该仓库？" });
    await dialog.getByRole("button", { name: /确认分仓并预占/ }).click();
    await admin.writeGuard.expectRequestCount(
      "confirm-warehouse-allocation",
      1,
    );
    const payload = admin.writeGuard.calls("confirm-warehouse-allocation")[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      warehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
      expectedRevision: E2E_ALLOCATION_REVISION,
    });
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-order-warehouse-allocation-/,
    );
  });

  test("shows loading, empty, and error states without issuing writes", async ({
    admin,
    page,
  }) => {
    let mode: "loading" | "error" = "loading";
    await page.route(
      "**/api/v1/orders/warehouse-allocations**",
      async (route) => {
        if (mode === "loading") {
          await new Promise((resolve) => setTimeout(resolve, 600));
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify(
              ok({
                list: [],
                pagination: { page: 1, pageSize: 20, total: 0, totalPages: 0 },
              }),
            ),
          });
          return;
        }
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 50000,
            message: "E2E 分仓列表失败",
            data: null,
          }),
        });
      },
    );

    await admin.goto("/orders/warehouse-allocations");
    await expect(page.locator(".ant-spin-spinning")).toBeVisible();
    await expect(page.getByText("暂无待处理的已付款订单")).toBeVisible();
    mode = "error";
    await page.getByRole("button", { name: "刷新" }).click();
    await expect(
      page.locator("#root .ant-alert-message").getByText("E2E 分仓列表失败"),
    ).toBeVisible();
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("keeps allocation confirmation disabled for readonly users", async ({
    admin,
    page,
  }) => {
    await page.route("**/api/v1/auth/profile", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({ ...e2eUser, role: "readonly", permissions: ["order.view"] }),
        ),
      });
    });
    await admin.goto("/orders/warehouse-allocations");
    await expect(page.getByText("当前账号为只读模式")).toBeVisible();
    await page.getByRole("button", { name: "查看" }).click();
    await expect(
      page.getByRole("button", { name: "确认分仓并预占库存" }),
    ).toBeDisabled();
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });
});
