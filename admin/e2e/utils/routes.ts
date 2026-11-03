import type { Page } from "@playwright/test";
import { ok } from "../mocks/envelope";
import { e2eUser, E2E_TOKEN } from "../mocks/auth";
import { e2eTenant } from "../mocks/tenants";
import { productsResponse } from "../mocks/products";
import { readinessResponse } from "../mocks/readiness";
import { publishResponse, skuBindingsResponse } from "../mocks/publish";
import { inventoryResponse } from "../mocks/inventory";
import { inventorySyncResponse } from "../mocks/inventory-sync";
import { imageProviderCapabilities } from "../mocks/image-providers";
import { observabilityResponse } from "../mocks/observability";
import { operationTaskResponse } from "../mocks/operation-tasks";
import { platformRuntimeResponse } from "../mocks/platform-runtime";
import { procurementResponse } from "../mocks/procurement";
import { salesReturnResponse } from "../mocks/sales-returns";
import { fulfillmentOrderResponse } from "../mocks/order-fulfillment";
import { warehouseAllocationResponse } from "../mocks/order-warehouse-allocation";
import { fulfillmentWaveResponse } from "../mocks/fulfillment-waves";
import { fulfillmentReconciliationResponse } from "../mocks/order-fulfillment-reconciliation";
import { orderProfitResponse } from "../mocks/order-profits";
import { fxReportResponse } from "../mocks/fx-report";
import { settlementResponse } from "../mocks/settlement";
import { warehouseFeeResponse } from "../mocks/warehouse-fees";
import { advertisingFeeResponse } from "../mocks/advertising-fees";
import { freightFeeResponse } from "../mocks/freight-fees";
import { workersResponse } from "../mocks/workers";
import { aiProductBatchResponse } from "../mocks/ai-product-batches";
import { customerResponse } from "../mocks/customer";

export async function seedAdminAuth(
  page: Page,
  anonymousRoutePaths: string[] = [],
) {
  await page.addInitScript(
    ({ key, token, anonymousRoutePaths: anonymousPaths }) => {
      if (anonymousPaths.includes(location.pathname)) {
        window.localStorage.removeItem(key);
        return;
      }
      if (!window.localStorage.getItem(key))
        window.localStorage.setItem(key, token);
    },
    { key: "trademind_admin_token", token: E2E_TOKEN, anonymousRoutePaths },
  );
}

export async function routeStaticAssets(page: Page) {
  await page.route("**/*.{png,jpg,jpeg,webp,gif,svg}", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" />',
    });
  });
}

export async function routeAdminApi(page: Page) {
  await routeStaticAssets(page);
  await page.route(
    "**/api/collector/providers/1688/auth-status",
    async (route) => {
      if (route.request().method() !== "GET") {
        await route.fallback();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          ok({
            provider: "1688",
            status: "not_logged_in",
            loggedIn: false,
            needVerification: false,
            message: "E2E collector browser login is not configured.",
            lastCheckedAt: "2026-01-01T00:00:00Z",
            profilePath: "",
          }),
        ),
      });
    },
  );
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method().toUpperCase())) {
      await route.fallback();
      return;
    }

    const url = new URL(request.url());
    const path = url.pathname;
    const response =
      (path === "/api/v1/auth/profile" ? ok(e2eUser) : null) ??
      (path === "/api/v1/settings" ? ok({ items: [] }) : null) ??
      (path === "/api/v1/tenant/current"
        ? ok({ tenant: e2eTenant, memberCount: 1, shopCount: 1 })
        : null) ??
      (path === "/api/v1/tenant/settings/branding"
        ? ok({
            module: "branding",
            version: 1,
            inherited: false,
            values: { displayName: e2eTenant.name, logoUrl: "", labels: [] },
          })
        : null) ??
      (path === "/api/v1/platform/tenants/101"
        ? ok({ tenant: e2eTenant, memberCount: 1, shopCount: 1 })
        : null) ??
      (path === "/api/v1/platform/tenants/101/audit"
        ? ok({ items: [] })
        : null) ??
      (path === "/api/v1/settings/integrations/overview"
        ? ok({
            ai: { configured: false, provider: "", model: "" },
            image: {
              providerCurrent: "",
              removebg: false,
              openaiImage: false,
              comfyui: false,
            },
            storage: { kind: "local", configured: false },
            mail: { configured: false },
            platforms: [],
            collectRulesCount: 0,
            disclaimerShort: "",
          })
        : null) ??
      (path === "/api/v1/image/providers"
        ? ok(imageProviderCapabilities)
        : null) ??
      workersResponse(path) ??
      observabilityResponse(path) ??
      platformRuntimeResponse(path) ??
      operationTaskResponse(path) ??
      aiProductBatchResponse(path) ??
      customerResponse(path) ??
      inventorySyncResponse(path) ??
      procurementResponse(path) ??
      warehouseAllocationResponse(path) ??
      fulfillmentWaveResponse(path) ??
      fulfillmentReconciliationResponse(path) ??
      orderProfitResponse(path) ??
      fxReportResponse(path) ??
      settlementResponse(path) ??
      warehouseFeeResponse(path) ??
      advertisingFeeResponse(path) ??
      freightFeeResponse(path) ??
      fulfillmentOrderResponse(path) ??
      salesReturnResponse(path) ??
      productsResponse(path) ??
      readinessResponse(path) ??
      publishResponse(path) ??
      inventoryResponse(path) ??
      (path.includes("/product-publications/") &&
      path.endsWith("/douyin/sku-bindings")
        ? skuBindingsResponse(path.split("/").at(-3) || undefined)
        : null) ??
      ok({
        list: [],
        pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 },
      });

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(response),
    });
  });
}
