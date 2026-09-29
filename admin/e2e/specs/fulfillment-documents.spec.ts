import { expect, test } from "../fixtures/admin.fixture";
import { e2eUser } from "../mocks/auth";
import { ok } from "../mocks/envelope";
import {
  E2E_FULFILLMENT_DOCUMENT_ID,
  E2E_FULFILLMENT_WAVE_ID,
  e2eFulfillmentDocument,
} from "../mocks/fulfillment-waves";
import { expectNoRootOverflow } from "../utils/assertions";

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

const documentPath = `/orders/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/documents`;

test.describe("@smoke fulfillment document center", () => {
  for (const viewport of viewports) {
    test(`renders immutable document preview at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await admin.goto(documentPath);
      await expect(
        page.getByText("履约出库单据中心", { exact: true }).first(),
      ).toBeVisible();
      await expect(page.getByLabel("拣货单打印预览")).toContainText("BLUE-01");
      await page.getByLabel("单据类型").first().click();
      await page.getByText("本地包裹标签", { exact: true }).last().click();
      await expect(page.getByLabel("本地包裹标签打印预览")).toContainText(
        "非承运商面单",
      );
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount("unexpected", 0);
    });
  }

  test("records one revision-bound snapshot and one browser print initiation", async ({
    admin,
    page,
  }) => {
    await page.addInitScript(() => {
      window.print = () =>
        window.sessionStorage.setItem("e2e-print-opened", "1");
    });
    admin.writeGuard.allow({
      operation: "generate-wave-document",
      method: "POST",
      path: new RegExp(
        `^/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/documents$`,
      ),
      response: ok({
        ...e2eFulfillmentDocument,
        id: "e2e-fulfillment-document-2",
        version: 2,
      }),
    });
    admin.writeGuard.allow({
      operation: "print-wave-document",
      method: "POST",
      path: new RegExp(
        `^/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/documents/[^/]+/print-events$`,
      ),
      response: ok({
        id: "e2e-print-event-1",
        waveId: E2E_FULFILLMENT_WAVE_ID,
        documentId: E2E_FULFILLMENT_DOCUMENT_ID,
        documentType: "pick_list",
        copies: 1,
        reprint: false,
        createdAt: "2026-08-24T02:06:00Z",
      }),
    });
    await admin.goto(documentPath);

    await page.getByRole("button", { name: "生成新版本" }).click();
    await admin.writeGuard.expectRequestCount("generate-wave-document", 1);
    expect(
      admin.writeGuard.calls("generate-wave-document")[0]?.postDataJSON,
    ).toMatchObject({ expectedRevision: 2 });
    expect(
      String(
        (
          admin.writeGuard.calls("generate-wave-document")[0]
            ?.postDataJSON as Record<string, unknown>
        ).idempotencyKey,
      ),
    ).toMatch(/^admin-fulfillment-wave-document-generate-/);

    await page.getByRole("button", { name: "登记并打开打印" }).click();
    await admin.writeGuard.expectRequestCount("print-wave-document", 1);
    expect(
      admin.writeGuard.calls("print-wave-document")[0]?.postDataJSON,
    ).toMatchObject({ documentType: "pick_list", copies: 1 });
    await expect
      .poll(() =>
        page.evaluate(() => window.sessionStorage.getItem("e2e-print-opened")),
      )
      .toBe("1");
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("keeps snapshot generation and print registration disabled for readonly users", async ({
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
    await admin.goto(documentPath);
    await expect(page.getByText("当前账号为只读模式")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "生成新版本" }),
    ).toBeDisabled();
    await expect(
      page.getByRole("button", { name: "登记并打开打印" }),
    ).toBeDisabled();
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });

  test("requires a reason before recording a reprint", async ({
    admin,
    page,
  }) => {
    await page.addInitScript(() => {
      window.print = () => undefined;
    });
    await page.route(
      `**/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/documents/${E2E_FULFILLMENT_DOCUMENT_ID}`,
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            ok({
              ...e2eFulfillmentDocument,
              printEvents: [
                {
                  id: "e2e-print-event-existing",
                  waveId: E2E_FULFILLMENT_WAVE_ID,
                  documentId: E2E_FULFILLMENT_DOCUMENT_ID,
                  documentType: "pick_list",
                  copies: 1,
                  reprint: false,
                  createdAt: "2026-08-24T02:06:00Z",
                },
              ],
            }),
          ),
        });
      },
    );
    admin.writeGuard.allow({
      operation: "reprint-wave-document",
      method: "POST",
      path: new RegExp(
        `^/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}/documents/${E2E_FULFILLMENT_DOCUMENT_ID}/print-events$`,
      ),
      response: ok({
        id: "e2e-print-event-reprint",
        waveId: E2E_FULFILLMENT_WAVE_ID,
        documentId: E2E_FULFILLMENT_DOCUMENT_ID,
        documentType: "pick_list",
        copies: 1,
        reprint: true,
        reason: "卡纸后重新打印",
        createdAt: "2026-08-24T02:07:00Z",
      }),
    });
    await admin.goto(documentPath);
    await page.getByRole("button", { name: "登记重打并打开打印" }).click();
    await expect(
      page.getByText("重打必须填写至少 2 个字符的原因。"),
    ).toBeVisible();
    await admin.writeGuard.expectRequestCount("reprint-wave-document", 0);

    await page.getByLabel("重打原因").fill("卡纸后重新打印");
    await page.getByRole("button", { name: "登记重打并打开打印" }).click();
    await admin.writeGuard.expectRequestCount("reprint-wave-document", 1);
    expect(
      admin.writeGuard.calls("reprint-wave-document")[0]?.postDataJSON,
    ).toMatchObject({
      documentType: "pick_list",
      copies: 1,
      reason: "卡纸后重新打印",
    });
    await admin.writeGuard.expectRequestCount("unexpected", 0);
  });
});
