import type { Page, Request, Response } from "@playwright/test";
import { expect } from "@playwright/test";
import {
  translateMessage,
  type AppLanguage,
} from "../../src/i18n/dictionaries";
import { e2eProduct } from "../mocks/product.fixture";
import { e2eOperationTask } from "../mocks/operation-tasks";
import {
  e2eAiImageBatchDetail,
  e2eAiTextBatchDetail,
} from "../mocks/ai-product-batches";
import { e2eFulfillmentWave } from "../mocks/fulfillment-waves";
import {
  e2eSalesOrder,
  e2eSalesReturn,
  e2ePlatformAfterSale,
  e2eRefundExecution,
} from "../mocks/sales-returns";
import { e2ePurchaseOrder, e2ePurchaseReturn } from "../mocks/procurement";
import { e2eConversation, e2eConversationMessages } from "../mocks/customer";
import { e2eTenant } from "../mocks/tenants";
import { E2E_PUBLISH_BATCH_ID } from "../mocks/publish";
import { resolveConfiguredPath, type ConfiguredRoute } from "./route-catalogue";

export async function withInitialApiReads(
  page: Page,
  inspect: () => Promise<void>,
) {
  const pending = new Set<Request>();
  const failures: string[] = [];
  const onRequest = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (
      request.method() === "GET" &&
      (path.startsWith("/api/v1/") || path.startsWith("/api/collector/"))
    ) {
      pending.add(request);
    }
  };
  const onResponse = async (response: Response) => {
    const request = response.request();
    if (!pending.has(request)) return;
    const path = new URL(request.url()).pathname;
    try {
      if (!response.ok()) {
        failures.push(`GET ${path}: HTTP ${response.status()}`);
      } else {
        const body = await response.json();
        if (body.code !== 0) {
          failures.push(`GET ${path}: business code ${String(body.code)}`);
        }
      }
    } catch {
      failures.push(`GET ${path}: unreadable API response`);
    } finally {
      pending.delete(request);
    }
  };
  const onRequestFailed = (request: Request) => {
    if (!pending.delete(request)) return;
    failures.push(
      `GET ${new URL(request.url()).pathname}: ${request.failure()?.errorText ?? "request failed"}`,
    );
  };

  page.on("request", onRequest);
  page.on("response", onResponse);
  page.on("requestfailed", onRequestFailed);
  try {
    await inspect();
    // Let initial React effects and chained reads start before checking completion.
    for (let cycle = 0; cycle < 2; cycle += 1) {
      await page.evaluate(
        () =>
          new Promise<void>((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
          ),
      );
      await expect
        .poll(() => pending.size, { message: "completed initial API reads" })
        .toBe(0);
    }
    expect(failures, "successful initial API reads").toEqual([]);
  } finally {
    page.off("request", onRequest);
    page.off("response", onResponse);
    page.off("requestfailed", onRequestFailed);
  }
}

const pageTitles: Record<string, string> = {
  "/ai/image-tasks": "AI 图片任务",
  "/ai/operation-workbench": "AI 商品运营工作台",
  "/product/drafts/:id": e2eProduct.title,
  "/product/ai-text-batches/:id": "AI 商品文案批量复核",
  "/product/ai-image-batches/:id": "AI 图片批量复核",
  "/product/publish-tasks": "商品刊登任务",
  "/orders/list": "订单管理",
  "/orders/logistics-channels": "物流渠道与运费模板",
  "/orders/sales-returns/:id": e2eSalesReturn.returnNo,
  "/orders/sales-return-reconciliation/:id":
    e2ePlatformAfterSale.externalAfterSaleId,
  "/orders/refund-executions/:id": e2eRefundExecution.executionNo,
  "/orders/sync-tasks": "订单同步任务",
  "/orders/sku-matches": "规格绑定",
  "/orders/exceptions": "订单异常工作台",
  "/finance/warehouse-fees": "仓库操作费台账",
  "/inventory": "库存中心",
  "/procurement/purchase-orders/:id": e2ePurchaseOrder.purchaseOrderNo,
  "/procurement/purchase-returns/:id": e2ePurchaseReturn.returnNo,
  "/customer/message-sync-tasks": "客服消息同步任务",
  "/platform/tenants/:id": e2eTenant.name,
  "/settings/image": "图片处理设置",
  "/settings/collector": "采集设置",
  "/settings/inventory": "库存与订单",
  "/settings/platform-publish": "刊登设置",
};

export function routeTitle(route: ConfiguredRoute, language: AppLanguage) {
  if (route.path === "/orders/:id") {
    return translateMessage(language, "订单 {value0}").replace(
      "{value0}",
      e2eSalesOrder.orderNo,
    );
  }
  const title = pageTitles[route.path] ?? route.name;
  return title ? translateMessage(language, title) : undefined;
}

const detailData: Record<
  string,
  { apiPath: string; content: string[]; extraReads?: string[] }
> = {
  "/ops/task-center/operation-tasks/:taskId": {
    apiPath: `/operation-tasks/${e2eOperationTask.id}`,
    content: [e2eOperationTask.title],
  },
  "/product/drafts/:id": {
    apiPath: `/products/${e2eProduct.id}`,
    content: [e2eProduct.title],
  },
  "/product/ai-text-batches/:id": {
    apiPath: `/products/ai-text/batches/${e2eAiTextBatchDetail.id}`,
    content: [e2eAiTextBatchDetail.batchNo],
  },
  "/product/ai-image-batches/:id": {
    apiPath: `/products/ai-images/batches/${e2eAiImageBatchDetail.id}`,
    content: [e2eAiImageBatchDetail.batchNo],
  },
  "/product/publish-batches/:id": {
    apiPath: `/product-publish/batches/${E2E_PUBLISH_BATCH_ID}`,
    content: [e2eProduct.title],
  },
  "/orders/fulfillment-waves/:id/verify-pack": {
    apiPath: `/fulfillment-waves/${e2eFulfillmentWave.id}`,
    content: [e2eFulfillmentWave.waveNo],
  },
  "/orders/fulfillment-waves/:id/documents": {
    apiPath: `/fulfillment-waves/${e2eFulfillmentWave.id}`,
    extraReads: [`/fulfillment-waves/${e2eFulfillmentWave.id}/documents`],
    content: [e2eFulfillmentWave.waveNo],
  },
  "/orders/sales-returns/:id": {
    apiPath: `/sales-returns/${e2eSalesReturn.id}`,
    content: [e2eSalesReturn.returnNo],
  },
  "/orders/sales-return-reconciliation/:id": {
    apiPath: `/sales-return-reconciliation/${e2ePlatformAfterSale.id}`,
    content: [e2ePlatformAfterSale.externalAfterSaleId],
  },
  "/orders/refund-executions/:id": {
    apiPath: `/refund-executions/${e2eRefundExecution.id}`,
    content: [e2eRefundExecution.executionNo],
  },
  "/orders/:id": {
    apiPath: `/orders/${e2eSalesOrder.id}`,
    content: [e2eSalesOrder.orderNo],
  },
  "/procurement/purchase-orders/:id": {
    apiPath: `/purchase-orders/${e2ePurchaseOrder.id}`,
    content: [e2ePurchaseOrder.purchaseOrderNo],
  },
  "/procurement/purchase-returns/:id": {
    apiPath: `/purchase-returns/${e2ePurchaseReturn.id}`,
    content: [e2ePurchaseReturn.returnNo],
  },
  "/customer/conversations/:id": {
    apiPath: `/customer/conversations/${e2eConversation.id}`,
    extraReads: [`/customer/conversations/${e2eConversation.id}/messages`],
    content: [
      e2eConversation.customerName,
      e2eConversationMessages[0].content,
      "E2E-TRACK-0001",
    ],
  },
  "/platform/tenants/:id": {
    apiPath: `/platform/tenants/${e2eTenant.id}`,
    content: [e2eTenant.name],
  },
};

export async function openConfiguredRoute(page: Page, route: ConfiguredRoute) {
  const data = detailData[route.path];
  const apiPaths = data ? [data.apiPath, ...(data.extraReads ?? [])] : [];
  const reads = apiPaths.map(async (path) => {
    const response = await page.waitForResponse(
      (candidate) =>
        candidate.request().method() === "GET" &&
        new URL(candidate.url()).pathname === `/api/v1${path}`,
      { timeout: 10_000 },
    );
    expect(response.ok(), `initial GET ${path}`).toBe(true);
    const body = await response.json();
    expect(body.code, `initial GET ${path} business status`).toBe(0);
  });
  await Promise.all([
    page.goto(resolveConfiguredPath(route.path), {
      waitUntil: "commit",
      timeout: 30_000,
    }),
    ...reads,
  ]);
}

export async function expectLoadedRouteContent(
  page: Page,
  route: ConfiguredRoute,
) {
  const main = page.locator("main").first();
  for (const text of detailData[route.path]?.content ?? []) {
    await expect(
      main.getByText(text, { exact: false }).first(),
      `loaded business content for ${route.path}: ${text}`,
    ).toBeVisible();
  }
  await expect(
    main.locator(".ant-spin-spinning:visible, .ant-skeleton:visible"),
    `finished loading ${route.path}`,
  ).toHaveCount(0);
}
