import type { Page } from '@playwright/test';
import { ok } from '../mocks/envelope';
import { e2eUser, E2E_TOKEN } from '../mocks/auth';
import { productsResponse } from '../mocks/products';
import { readinessResponse } from '../mocks/readiness';
import { publishResponse, skuBindingsResponse } from '../mocks/publish';
import { inventoryResponse } from '../mocks/inventory';
import { inventorySyncResponse } from '../mocks/inventory-sync';
import { imageProviderCapabilities } from '../mocks/image-providers';
import { observabilityResponse } from '../mocks/observability';
import { operationTaskResponse } from '../mocks/operation-tasks';
import { platformRuntimeResponse } from '../mocks/platform-runtime';
import { procurementResponse } from '../mocks/procurement';

export async function seedAdminAuth(page: Page) {
  await page.addInitScript(([key, token]) => {
    window.localStorage.setItem(key, token);
  }, ['trademind_admin_token', E2E_TOKEN]);
}

export async function routeStaticAssets(page: Page) {
  await page.route('**/*.{png,jpg,jpeg,webp,gif,svg}', async (route) => {
    await route.fulfill({ status: 200, contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" />' });
  });
}

export async function routeAdminApi(page: Page) {
  await routeStaticAssets(page);
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method().toUpperCase())) {
      await route.fallback();
      return;
    }

    const url = new URL(request.url());
    const path = url.pathname;
    const response =
      (path === '/api/v1/auth/profile' ? ok(e2eUser) : null) ??
      (path === '/api/v1/settings' ? ok({ items: [] }) : null) ??
      (path === '/api/v1/image/providers' ? ok(imageProviderCapabilities) : null) ??
      observabilityResponse(path) ??
      platformRuntimeResponse(path) ??
      operationTaskResponse(path) ??
      inventorySyncResponse(path) ??
      procurementResponse(path) ??
      productsResponse(path) ??
      readinessResponse(path) ??
      publishResponse(path) ??
      inventoryResponse(path) ??
      (path.includes('/product-publications/') && path.endsWith('/douyin/sku-bindings') ? skuBindingsResponse(path.split('/').at(-3) || undefined) : null) ??
      ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });

    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(response) });
  });
}
