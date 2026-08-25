import { test, expect } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { ok } from "../mocks/envelope";
import {
  E2E_ALLOCATION_ORDER_ID,
  E2E_ALLOCATION_REVISION,
  E2E_ALLOCATION_WAREHOUSE_ID,
  allocatedWarehouseAllocationListResponse,
  blockedWarehouseAllocationListResponse,
  e2eBlockedWarehouseAllocation,
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
      const recommendedCandidate = allocationDrawer.locator(".ant-card").first();
      await expect(
        recommendedCandidate.getByText("推荐排序 #1"),
      ).toBeVisible();
      await expect(
        recommendedCandidate.getByText("可用库存覆盖整单需求；默认仓优先", {
          exact: true,
        }),
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

  test("links a blocked allocation to the existing exception workbench without writing", async ({
    admin,
    page,
  }) => {
    await page.route(
      "**/api/v1/orders/warehouse-allocations*",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(blockedWarehouseAllocationListResponse()),
        });
      },
    );
    await page.route(
      `**/api/v1/orders/${E2E_ALLOCATION_ORDER_ID}/warehouse-allocation`,
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(ok(e2eBlockedWarehouseAllocation)),
        });
      },
    );

    await admin.goto("/orders/warehouse-allocations");
    await page.getByRole("button", { name: "查看" }).click();
    await expect(
      page.getByRole("button", { name: "打开异常工作台" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "打开异常工作台" }).click();
    await expect(page).toHaveURL(
      new RegExp(`/orders/exceptions\\?orderId=${E2E_ALLOCATION_ORDER_ID}`),
    );
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

  test("creates one same-warehouse picking wave and keeps cancel side-effect free", async ({
    admin,
    page,
  }) => {
    await page.route(
      "**/api/v1/orders/warehouse-allocations*",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(allocatedWarehouseAllocationListResponse()),
        });
      },
    );
    admin.writeGuard.allow({
      operation: "create-fulfillment-wave",
      method: "POST",
      path: /^\/api\/v1\/fulfillment-waves$/,
      response: ok({
        id: "e2e-fulfillment-wave-1",
        waveNo: "FW20260824-E2E0000001",
        warehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
        status: "draft",
        revision: 1,
        orderCount: 1,
        lineCount: 1,
        requiredQuantity: 3,
        pickedQuantity: 0,
        shortageQuantity: 0,
        fulfilledCount: 0,
        failedCount: 0,
      }),
    });

    await admin.goto("/orders/warehouse-allocations");
    await page.getByRole("checkbox").nth(1).check();
    const waveButton = page.getByRole("button", { name: /创建拣货波次（1）/ });
    await expect(waveButton).toBeEnabled();
    await waveButton.click();
    const modal = page.getByRole("dialog", { name: "创建拣货波次" });
    await expect(modal).toBeVisible();
    await page.getByLabel("波次备注").fill("E2E 早班");
    await modal.getByRole("button", { name: /取\s*消/ }).click();
    await expect(modal).toBeHidden();
    await admin.writeGuard.expectRequestCount("create-fulfillment-wave", 0);

    await waveButton.click();
    await page.getByLabel("波次备注").fill("E2E 早班");
    await modal.getByRole("button", { name: "创建波次" }).click();
    await admin.writeGuard.expectRequestCount("create-fulfillment-wave", 1);
    const payload = admin.writeGuard.calls("create-fulfillment-wave")[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      warehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
      orderIds: [E2E_ALLOCATION_ORDER_ID],
      remark: "E2E 早班",
    });
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-fulfillment-wave-create-/,
    );
    await expect(page).toHaveURL(/\/orders\/fulfillment-waves/);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("keeps the create-wave modal inside a narrow viewport", async ({
    admin,
    page,
  }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.route(
      "**/api/v1/orders/warehouse-allocations*",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(allocatedWarehouseAllocationListResponse()),
        });
      },
    );
    await admin.goto("/orders/warehouse-allocations");
    await page.getByRole("checkbox").nth(1).check();
    await page.getByRole("button", { name: /创建拣货波次（1）/ }).click();
    await expectModalWithinViewport(page);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });
});
