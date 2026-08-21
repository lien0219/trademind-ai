import { test, expect } from "../fixtures/admin.fixture";
import { ok } from "../mocks/envelope";
import { expectNoRootOverflow } from "../utils/assertions";

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe("@smoke order warehouse inventory lifecycle", () => {
  for (const viewport of viewports) {
    test(`renders the warehouse-bound manual order form at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto("/orders");
      await page.getByRole("button", { name: "新建订单" }).click();
      const dialog = page.getByRole("dialog", { name: "新建手工订单" });
      await expect(dialog.getByLabel("履约仓库")).toBeVisible();
      await expect(dialog.getByText("创建后应用预占 / 出库")).toBeVisible();
      await expectNoRootOverflow(page);
      await dialog.getByRole("button", { name: /取\s*消/ }).click();
      await expect(dialog).toBeHidden();
      await admin.writeGuard.expectRequestCount("unexpected", 0);
    });
  }

  test("submits the selected order warehouse without an early write", async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: "create-warehouse-bound-order",
      method: "POST",
      path: /^\/api\/v1\/orders$/,
      response: ok({ id: "e2e-order-warehouse-bound" }),
    });

    await admin.goto("/orders");
    await page.getByRole("button", { name: "新建订单" }).click();
    let dialog = page.getByRole("dialog", { name: "新建手工订单" });
    await dialog.getByRole("button", { name: /取\s*消/ }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount(
      "create-warehouse-bound-order",
      0,
    );

    await page.getByRole("button", { name: "新建订单" }).click();
    dialog = page.getByRole("dialog", { name: "新建手工订单" });
    await dialog.locator("input#orderNo").fill("E2E-ORDER-WAREHOUSE-1");
    await dialog.locator("input#customerName").fill("E2E 客户");
    await dialog.getByRole("button", { name: /确\s*定/ }).click();

    await admin.writeGuard.expectRequestCount(
      "create-warehouse-bound-order",
      1,
    );
    expect(
      admin.writeGuard.calls("create-warehouse-bound-order")[0]?.postDataJSON,
    ).toMatchObject({
      orderNo: "E2E-ORDER-WAREHOUSE-1",
      customerName: "E2E 客户",
      warehouseId: "e2e-warehouse-main",
    });
  });
});
