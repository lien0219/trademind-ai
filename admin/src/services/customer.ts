import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';

export type ConversationRow = {
  id: string;
  platform: string;
  customerName: string;
  customerLanguage: string;
  status: string;
  lastMessageAt?: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
  latestMessage?: string;
};

export type ConversationOrderShipment = {
  carrier: string;
  trackingNo: string;
  trackingUrl?: string;
  status: string;
  shippedAt?: string;
  deliveredAt?: string;
};

export type ConversationOrderSummary = {
  id: string;
  orderNo: string;
  platform: string;
  status: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  currency: string;
  totalAmount: number;
  orderedAt?: string;
  latestShipmentStatus?: string;
  shipments?: ConversationOrderShipment[];
};

export type ConversationDetail = {
  id: string;
  platform: string;
  shopId?: string;
  externalConversationId?: string;
  customerName: string;
  customerAvatar?: string;
  customerLanguage: string;
  status: string;
  lastMessageAt?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  orderId?: string;
  orderSummary?: ConversationOrderSummary | null;
};

export type CustomerMessageRow = {
  id: string;
  conversationId: string;
  role: string;
  content: string;
  language: string;
  source: string;
  externalMessageId?: string;
  rawData?: unknown;
  createdBy?: string;
  createdAt: string;
};

export type GenerateReplyResult = {
  suggestionId: string;
  reply: string;
  intent: string;
  sentiment: string;
  riskLevel: string;
  notes: string;
  taskId: string;
};

type Paginated<T> = {
  list: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
};

export async function queryConversations(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  status?: string;
  customerName?: string;
  start?: string;
  end?: string;
}): Promise<Paginated<ConversationRow>> {
  return getWithParams('/api/v1/customer/conversations', {
    page: params.page,
    pageSize: params.pageSize,
    platform: params.platform,
    status: params.status,
    customerName: params.customerName,
    start: params.start,
    end: params.end,
  });
}

export async function createConversation(payload: {
  platform?: string;
  customerName: string;
  customerLanguage?: string;
  customerAvatar?: string;
}): Promise<ConversationDetail> {
  return postJSON('/api/v1/customer/conversations', payload);
}

export async function getConversation(id: string): Promise<ConversationDetail> {
  return getJSON(`/api/v1/customer/conversations/${id}`);
}

export async function updateConversation(
  id: string,
  payload: { customerName?: string; customerLanguage?: string; status?: string; orderId?: string },
): Promise<ConversationDetail> {
  return putJSON(`/api/v1/customer/conversations/${id}`, payload);
}

export async function deleteConversation(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/customer/conversations/${id}`);
}

export async function queryMessages(conversationId: string): Promise<{ list: CustomerMessageRow[] }> {
  return getJSON(`/api/v1/customer/conversations/${conversationId}/messages`);
}

export async function createMessage(
  conversationId: string,
  payload: { role: string; content: string; language?: string; source?: string },
): Promise<CustomerMessageRow> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/messages`, payload);
}

export async function markConversationReplied(conversationId: string, reply: string): Promise<CustomerMessageRow> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/mark-replied`, { reply });
}

export async function generateCustomerReply(
  conversationId: string,
  payload: {
    messageId?: string;
    language?: string;
    tone?: string;
    platform?: string;
    shopPolicy?: string;
  },
): Promise<GenerateReplyResult> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/ai/generate-reply`, payload);
}

export async function updateReplySuggestion(
  id: string,
  payload: { editedReply: string },
): Promise<{ ok: boolean }> {
  return putJSON(`/api/v1/customer/reply-suggestions/${id}`, payload);
}

export async function acceptReplySuggestion(
  id: string,
  payload: { finalReply: string },
): Promise<{ ok: boolean }> {
  return postJSON(`/api/v1/customer/reply-suggestions/${id}/accept`, payload);
}

export async function discardReplySuggestion(id: string): Promise<{ ok: boolean }> {
  return postJSON(`/api/v1/customer/reply-suggestions/${id}/discard`, {});
}
