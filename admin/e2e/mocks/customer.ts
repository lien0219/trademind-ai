import { ok } from "./envelope";
import type {
  ConversationDetail,
  CustomerMessageRow,
  CustomerAutoReplySetting,
  CustomerAutoReplyPolicy,
} from "../../src/services/customer";
import { e2eSalesOrder } from "./sales-returns";
import { e2eShop } from "./tenants";
import { e2eShops } from "./publish";

export const E2E_CONVERSATION_ID = "e2e-conversation-1";

export const e2eConversation: ConversationDetail = {
  id: E2E_CONVERSATION_ID,
  platform: "manual",
  externalConversationId: "e2e-external-conversation-1",
  customerName: "E2E 客户",
  customerNameMasked: "E2E 客户",
  customerLanguage: "en",
  status: "open",
  lastMessageAt: "2026-01-03T01:00:00Z",
  createdBy: "e2e-user",
  createdAt: "2026-01-03T00:00:00Z",
  updatedAt: "2026-01-03T01:00:00Z",
  shopId: e2eShop.id,
  orderId: e2eSalesOrder.id,
  shopSummary: {
    id: e2eShop.id,
    platform: e2eShop.platform,
    shopName: e2eShop.shopName,
    status: e2eShop.status,
    authStatus: e2eShop.authStatus,
  },
  orderSummary: {
    id: e2eSalesOrder.id,
    orderNo: e2eSalesOrder.orderNo,
    platform: e2eSalesOrder.platform,
    status: e2eSalesOrder.status,
    paymentStatus: e2eSalesOrder.paymentStatus,
    fulfillmentStatus: e2eSalesOrder.fulfillmentStatus,
    currency: e2eSalesOrder.currency,
    totalAmount: e2eSalesOrder.totalAmount,
    orderedAt: e2eSalesOrder.orderedAt,
    skuMatchStatus: "all_matched",
    inventoryDeductStatus: "success",
    latestShipmentStatus: "delivered",
    shipments: [
      {
        carrier: "E2E Carrier",
        trackingNo: "E2E-TRACK-0001",
        status: "delivered",
        shippedAt: "2026-08-21T02:00:00Z",
        deliveredAt: "2026-08-21T03:00:00Z",
      },
    ],
  },
  productContexts: [],
  inventoryContexts: [],
  contextSummary: {
    orderStatus: "delivered",
    skuMatchStatus: "all_matched",
    inventoryStatus: "success",
    productTitle: e2eSalesOrder.items[0].productTitle,
    customerQuestion: "E2E customer question",
  },
  canWrite: true,
};

export const e2eConversationMessages: CustomerMessageRow[] = [
  {
    id: "e2e-conversation-message-1",
    conversationId: E2E_CONVERSATION_ID,
    role: "customer",
    content: "Can you share the latest product details?",
    language: "en",
    messageType: "text",
    source: "manual",
    createdBy: "e2e-user",
    createdAt: "2026-01-03T01:00:00Z",
  },
  {
    id: "e2e-internal-reply",
    conversationId: E2E_CONVERSATION_ID,
    role: "agent",
    content: "E2E internal reply awaiting platform delivery",
    language: "en",
    messageType: "text",
    source: "manual",
    createdBy: "e2e-user",
    createdAt: "2026-01-03T01:01:00Z",
  },
  {
    id: "e2e-platform-reply",
    conversationId: E2E_CONVERSATION_ID,
    role: "agent",
    content: "E2E reply delivered to the platform",
    language: "en",
    messageType: "text",
    source: "platform",
    createdBy: "e2e-user",
    createdAt: "2026-01-03T01:02:00Z",
  },
  {
    id: "e2e-ai-suggestion",
    conversationId: E2E_CONVERSATION_ID,
    role: "ai",
    content: "E2E AI suggestion pending human review",
    language: "en",
    messageType: "text",
    source: "ai_suggestion",
    createdBy: "e2e-user",
    createdAt: "2026-01-03T01:03:00Z",
  },
];

export function customerResponse(path: string) {
  if (path === "/api/v1/customer/auto-reply-setting") {
    return ok({
      messageSyncEnabled: false,
      autoReplyEnabled: false,
      pollIntervalSeconds: 60,
      workerAvailable: false,
      effectiveEnabled: false,
    } satisfies CustomerAutoReplySetting);
  }
  const shop = e2eShops.find(
    (candidate) =>
      path === `/api/v1/customer/shops/${candidate.id}/auto-reply-policy`,
  );
  if (shop) {
    return ok({
      shopId: shop.id,
      shopName: shop.shopName,
      platform: shop.platform,
      globalEnabled: false,
      workerAvailable: false,
      enabled: false,
      effectiveEnabled: false,
      tone: "professional",
      shopPolicy: "",
      maxReplyRunes: 600,
      maxRepliesPerHour: 20,
      requireOrderContext: true,
      lowRiskOnly: true,
    } satisfies CustomerAutoReplyPolicy);
  }
  if (
    e2eShops.some(
      (candidate) =>
        path === `/api/v1/customer/shops/${candidate.id}/auto-reply-runs`,
    )
  )
    return ok([]);
  if (path === `/api/v1/customer/conversations/${E2E_CONVERSATION_ID}`) {
    return ok(e2eConversation);
  }
  if (
    path === `/api/v1/customer/conversations/${E2E_CONVERSATION_ID}/messages`
  ) {
    return ok({ list: e2eConversationMessages });
  }
  return null;
}
