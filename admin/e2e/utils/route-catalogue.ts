import * as configuredRouteModule from "../../config/routes";
import { E2E_OPERATION_TASK_ID } from "../mocks/operation-tasks";
import {
  E2E_AI_IMAGE_BATCH_ID,
  E2E_AI_TEXT_BATCH_ID,
} from "../mocks/ai-product-batches";
import { E2E_PRODUCT_ID } from "../mocks/product.fixture";
import { E2E_PUBLISH_BATCH_ID } from "../mocks/publish";
import { E2E_FULFILLMENT_WAVE_ID } from "../mocks/fulfillment-waves";
import {
  E2E_PLATFORM_AFTER_SALE_ID,
  E2E_REFUND_EXECUTION_ID,
  E2E_SALES_ORDER_ID,
  E2E_SALES_RETURN_ID,
} from "../mocks/sales-returns";
import { E2E_CONVERSATION_ID } from "../mocks/customer";
import { isPublicAdminPath } from "../../src/utils/publicRoutes";
import {
  E2E_PURCHASE_ORDER_ID,
  E2E_PURCHASE_RETURN_ID,
} from "../mocks/procurement";

export type ConfiguredRoute = {
  path: string;
  component?: string;
  redirect?: string;
  name?: string;
  hideInMenu?: boolean;
};

type RouteNode = ConfiguredRoute & { routes?: RouteNode[] };

function unwrapRouteModule(value: unknown): unknown {
  if (Array.isArray(value)) return value;
  if (value && typeof value === "object" && "default" in value) {
    return unwrapRouteModule((value as { default: unknown }).default);
  }
  return value;
}

function flattenRoutes(
  nodes: RouteNode[],
  result: ConfiguredRoute[] = [],
): ConfiguredRoute[] {
  for (const node of nodes) {
    if (node.component || node.redirect || node.path === "*") result.push(node);
    if (Array.isArray(node.routes)) flattenRoutes(node.routes, result);
  }
  return result;
}

const configuredRoutes = unwrapRouteModule(
  configuredRouteModule,
) as RouteNode[];
const flattenedRoutes = flattenRoutes(configuredRoutes);

export const internalInventoryRoutes = flattenedRoutes.filter((route) =>
  route.path.startsWith("/ops/inventory-sync"),
);

export const productionPageRoutes = flattenedRoutes.filter(
  (route) =>
    typeof route.component === "string" &&
    route.component.startsWith("./") &&
    !route.path.startsWith("/ops/inventory-sync"),
);

export const productionRedirectRoutes = flattenedRoutes.filter(
  (route) => route.redirect && !route.path.startsWith("/ops/inventory-sync"),
);

const dynamicRouteValues: Record<string, string> = {
  "/ops/task-center/operation-tasks/:taskId": E2E_OPERATION_TASK_ID,
  "/product/drafts/:id": E2E_PRODUCT_ID,
  "/product/ai-text-batches/:id": E2E_AI_TEXT_BATCH_ID,
  "/product/ai-image-batches/:id": E2E_AI_IMAGE_BATCH_ID,
  "/product/publish-batches/:id": E2E_PUBLISH_BATCH_ID,
  "/orders/fulfillment-waves/:id/verify-pack": E2E_FULFILLMENT_WAVE_ID,
  "/orders/fulfillment-waves/:id/documents": E2E_FULFILLMENT_WAVE_ID,
  "/orders/sales-returns/:id": E2E_SALES_RETURN_ID,
  "/orders/sales-return-reconciliation/:id": E2E_PLATFORM_AFTER_SALE_ID,
  "/orders/refund-executions/:id": E2E_REFUND_EXECUTION_ID,
  "/orders/:id": E2E_SALES_ORDER_ID,
  "/procurement/purchase-orders/:id": E2E_PURCHASE_ORDER_ID,
  "/procurement/purchase-returns/:id": E2E_PURCHASE_RETURN_ID,
  "/customer/conversations/:id": E2E_CONVERSATION_ID,
  "/platform/tenants/:id": "101",
};

export function resolveConfiguredPath(path: string): string {
  if (path === "*") return "/__e2e-route-not-found__";
  return dynamicRouteValues[path]
    ? path.replace(/:[^/]+/g, dynamicRouteValues[path])
    : path;
}

export function resolveRedirectTarget(redirect: string): string {
  return redirect.replace(/:([A-Za-z0-9_]+)/g, (match) => {
    const key = Object.keys(dynamicRouteValues).find((routePath) =>
      routePath.includes(match),
    );
    return key ? dynamicRouteValues[key] : match;
  });
}

export function isPublicPagePath(path: string): boolean {
  return path === "*" || isPublicAdminPath(path);
}
