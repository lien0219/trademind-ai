import { test, expect } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { ok } from "../mocks/envelope";
import {
  E2E_FULFILLMENT_WAVE_ID,
  E2E_FULFILLMENT_WAVE_LINE_ID,
  e2eFulfillmentWave,
} from "../mocks/fulfillment-waves";
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

test.describe("@smoke fulfillment picking waves", () => {
  for (const viewport of viewports) {
    test(`renders the wave list and detail at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto("/orders/fulfillment-waves");
      await expect(
        page.getByText("FW20260824-E2E0000001", { exact: true }).first(),
      ).toBeVisible();
      await expectNoRootOverflow(page);
      await page.getByRole("button", { name: "查看" }).click();
      const drawer = page.getByRole("dialog", {
        name: /拣货波次 FW20260824-E2E0000001/,
      });
      await expect(drawer).toBeVisible();
      await expect(
        drawer.getByText("SO-E2E-ALLOC-0001", { exact: true }).first(),
      ).toBeVisible();
      await expect(
        drawer.getByText("BLUE-01", { exact: true }).first(),
      ).toBeVisible();
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount("unexpected", 0);
    });
  }

  test("keeps pick cancellation side-effect free and records one revision-bound result", async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: "record-wave-pick",
      method: "POST",
      path: new RegExp(
        `^/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/picks$`,
      ),
      response: ok({
        ...e2eFulfillmentWave,
        status: "packing",
        revision: 3,
        pickedQuantity: 3,
      }),
    });
    await admin.goto("/orders/fulfillment-waves");
    await page.getByRole("button", { name: "查看" }).click();
    await page.getByRole("button", { name: "录入拣货结果" }).click();
    let dialog = page.getByRole("dialog", { name: "录入拣货结果" });
    await expectModalWithinViewport(page);
    await dialog.getByRole("button", { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount("record-wave-pick", 0);

    await page.getByRole("button", { name: "录入拣货结果" }).click();
    dialog = page.getByRole("dialog", { name: "录入拣货结果" });
    await dialog
      .getByRole("spinbutton", { name: "BLUE-01 实际拣到数量" })
      .fill("3");
    await dialog.getByRole("button", { name: "保存拣货结果" }).click();
    await admin.writeGuard.expectRequestCount("record-wave-pick", 1);
    const payload = admin.writeGuard.calls("record-wave-pick")[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      expectedRevision: 2,
      lines: [
        {
          lineId: E2E_FULFILLMENT_WAVE_LINE_ID,
          pickedQuantity: 3,
          shortageQuantity: 0,
        },
      ],
    });
    expect(String(payload.idempotencyKey)).toMatch(
      /^admin-fulfillment-wave-pick-/,
    );
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("reviews packing once and completes only an already packed order", async ({
    admin,
    page,
  }) => {
    let detail = {
      ...e2eFulfillmentWave,
      status: "packing",
      revision: 3,
      pickedQuantity: 3,
      orders: [{ ...e2eFulfillmentWave.orders[0], status: "ready_to_pack" }],
      lines: [
        { ...e2eFulfillmentWave.lines[0], status: "picked", pickedQuantity: 3 },
      ],
    };
    await page.route(
      `**/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}`,
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(ok(detail)),
        });
      },
    );
    admin.writeGuard.allow({
      operation: "pack-wave-order",
      method: "POST",
      path: new RegExp(
        `^/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/orders/e2e-order-warehouse-allocation/pack$`,
      ),
      response: ok(detail),
    });
    admin.writeGuard.allow({
      operation: "complete-wave",
      method: "POST",
      path: new RegExp(
        `^/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/complete$`,
      ),
      response: ok({
        wave: {
          ...detail,
          status: "completed",
          revision: 6,
          fulfilledCount: 1,
        },
        processed: 1,
        succeeded: 1,
        failed: 0,
        items: [],
      }),
    });

    await admin.goto(
      `/orders/fulfillment-waves?drawer=fulfillment-wave&id=${E2E_FULFILLMENT_WAVE_ID}`,
    );
    await page.getByRole("button", { name: "打包复核" }).click();
    let dialog = page.getByRole("dialog", { name: /打包复核/ });
    await dialog.getByRole("button", { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount("pack-wave-order", 0);

    await page.getByRole("button", { name: "打包复核" }).click();
    dialog = page.getByRole("dialog", { name: /打包复核/ });
    await dialog.getByLabel("承运商").fill("顺丰");
    await dialog.getByLabel("运单号").fill("SF-E2E-WAVE-1");
    await dialog.getByRole("button", { name: "保存复核" }).click();
    await admin.writeGuard.expectRequestCount("pack-wave-order", 1);
    const packPayload = admin.writeGuard.calls("pack-wave-order")[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(packPayload).toMatchObject({
      expectedRevision: 3,
      carrier: "顺丰",
      trackingNo: "SF-E2E-WAVE-1",
    });

    detail = {
      ...detail,
      revision: 4,
      orders: [
        {
          ...detail.orders[0],
          status: "packed",
          carrier: "顺丰",
          trackingNo: "SF-E2E-WAVE-1",
        },
      ],
    };
    await page
      .getByRole("dialog", { name: /拣货波次 FW20260824-E2E0000001/ })
      .getByRole("button", { name: /刷新/ })
      .click();
    await page.getByRole("button", { name: /完成已打包订单（1）/ }).click();
    dialog = page.getByRole("dialog", { name: "确认完成已打包订单？" });
    await dialog.getByRole("button", { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount("complete-wave", 0);
    await page.getByRole("button", { name: /完成已打包订单（1）/ }).click();
    await page
      .getByRole("dialog", { name: "确认完成已打包订单？" })
      .getByRole("button", { name: "确认完成" })
      .click();
    await admin.writeGuard.expectRequestCount("complete-wave", 1);
    const completePayload = admin.writeGuard.calls("complete-wave")[0]
      ?.postDataJSON as Record<string, unknown>;
    expect(completePayload.expectedRevision).toBe(4);
    expect(String(completePayload.idempotencyKey)).toMatch(
      /^admin-fulfillment-wave-complete-/,
    );
  });

  test("shows loading, empty, error, and readonly states without writes", async ({
    admin,
    page,
  }) => {
    let mode: "loading" | "error" = "loading";
    await page.route("**/api/v1/fulfillment-waves*", async (route) => {
      if (
        new URL(route.request().url()).pathname !== "/api/v1/fulfillment-waves"
      ) {
        await route.fallback();
        return;
      }
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
          message: "E2E 波次列表失败",
          data: null,
        }),
      });
    });
    await admin.goto("/orders/fulfillment-waves");
    await expect(page.locator(".ant-spin-spinning")).toBeVisible();
    await expect(
      page.getByText("暂无拣货波次，可从履约分仓页选择已分仓订单创建"),
    ).toBeVisible();
    mode = "error";
    await page.getByRole("button", { name: "刷新" }).click();
    await expect(
      page.locator("#root .ant-alert-message").getByText("E2E 波次列表失败"),
    ).toBeVisible();

    await page.unroute("**/api/v1/fulfillment-waves*");
    await page.route("**/api/v1/auth/profile", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({ ...e2eUser, role: "readonly", permissions: ["order.view"] }),
        ),
      });
    });
    await admin.goto("/orders/fulfillment-waves");
    await expect(page.getByText("当前账号为只读模式")).toBeVisible();
    await page.getByRole("button", { name: "查看" }).click();
    await expect(
      page.getByRole("button", { name: "录入拣货结果" }),
    ).toBeDisabled();
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });
});
