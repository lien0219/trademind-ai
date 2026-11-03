import { test, expect } from "../fixtures/admin.fixture";
import { expectNoRootOverflow } from "../utils/assertions";
import { e2eConversation, E2E_CONVERSATION_ID } from "../mocks/customer";
import { ok } from "../mocks/envelope";

const conversationPath = `/customer/conversations/${E2E_CONVERSATION_ID}`;

for (const viewport of [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
]) {
  test(`@p1 customer conversation keeps linked context and labelled reply controls at ${viewport.width}`, async ({
    admin,
    page,
  }) => {
    await page.setViewportSize(viewport);
    await admin.goto(conversationPath);
    await expect(
      page.getByText("E2E-TRACK-0001", { exact: false }),
    ).toBeVisible();
    await expect(
      page.getByText(e2eConversation.orderSummary!.orderNo, { exact: true }),
    ).toBeVisible();
    for (const [content, label] of [
      ["E2E internal reply awaiting platform delivery", "内部回复"],
      ["E2E reply delivered to the platform", "平台消息"],
      ["E2E AI suggestion pending human review", "AI 建议"],
    ]) {
      const item = page.getByRole("listitem").filter({ hasText: content });
      await expect(item.getByText(label, { exact: true })).toBeVisible();
    }
    await expect(
      page
        .getByRole("listitem")
        .filter({ hasText: "E2E AI suggestion pending human review" })
        .getByText("AI", { exact: true }),
    ).toBeVisible();
    for (const label of [
      "回复语言",
      "回复语气",
      "会话平台",
      "店铺政策（可选）",
    ]) {
      const field = page.getByLabel(label, { exact: true });
      await expect(field).toBeVisible();
      await expect(field).toBeEnabled();
      const box = await field.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(viewport.width);
    }
    await expect(
      page.getByRole("button", { name: "AI 生成建议回复", exact: true }),
    ).toBeEnabled();
    await page.getByRole("button", { name: "选择店铺", exact: true }).click();
    const shopPicker = page.getByRole("dialog", {
      name: "选择关联店铺",
      exact: true,
    });
    await expect(shopPicker).toBeVisible();
    await shopPicker.getByRole("button", { name: "取 消" }).click();
    await expect(shopPicker).toBeHidden();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectNoUnexpectedWrites();
  });
}

test("@p1 customer conversation preserves empty and readonly states without writes", async ({
  admin,
  page,
}) => {
  await page.route(
    `**/api/v1/customer/conversations/${E2E_CONVERSATION_ID}`,
    (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      return route.fulfill({
        json: ok({
          ...e2eConversation,
          shopId: undefined,
          shopSummary: null,
          orderId: undefined,
          orderSummary: null,
          canWrite: false,
        }),
      });
    },
  );
  await page.route(
    `**/api/v1/customer/conversations/${E2E_CONVERSATION_ID}/messages`,
    (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      return route.fulfill({ json: ok({ list: [] }) });
    },
  );
  await page.setViewportSize({ width: 375, height: 812 });
  await admin.goto(conversationPath);
  await expect(page.getByText("未关联店铺", { exact: true })).toBeVisible();
  await expect(page.getByText("暂无消息", { exact: true })).toBeVisible();
  await expect(
    page.getByText("当前为只读账号，不可生成建议或发送消息。", { exact: true }),
  ).toBeVisible();
  for (const name of [
    "AI 生成建议回复",
    "保存编辑",
    "采纳为内部回复",
    "发送到平台",
  ]) {
    await expect(
      page.getByRole("button", { name, exact: true }),
    ).toBeDisabled();
  }
  await expectNoRootOverflow(page);
  await admin.writeGuard.expectNoUnexpectedWrites();
});
