import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';

export type ConversationRow = {
  id: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  shopPlatform?: string;
  customerName: string;
  customerNameMasked?: string;
  customerLanguage: string;
  status: string;
  lastMessageAt?: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
  latestMessage?: string;
  orderId?: string;
  orderNo?: string;
  productTitle?: string;
  aiSuggestionStatus?: string;
  sendStatus?: string;
  openFailureCount?: number;
};

export type ConversationShopSummary = {
  id: string;
  platform: string;
  shopName: string;
  shopCode?: string;
  status: string;
  authStatus: string;
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
  itemCount?: number;
  skuMatchStatus?: string;
  inventoryDeductStatus?: string;
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
  customerNameMasked?: string;
  customerAvatar?: string;
  customerLanguage: string;
  status: string;
  lastMessageAt?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  orderId?: string;
  orderSummary?: ConversationOrderSummary | null;
  shopSummary?: ConversationShopSummary | null;
  productContexts?: ProductContextItem[];
  inventoryContexts?: InventoryContextItem[];
  contextSummary?: ContextSummary | null;
  openFailureCount?: number;
  canWrite?: boolean;
};

export type ContextSummary = {
  orderStatus?: string;
  skuMatchStatus?: string;
  inventoryStatus?: string;
  productTitle?: string;
  customerQuestion?: string;
  incompleteWarning?: string;
};

export type ProductContextItem = {
  productId?: string;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
  stockStatus?: string;
  publishStatus?: string;
  aiOpsStatus?: string;
};

export type InventoryContextItem = {
  skuCode?: string;
  skuName?: string;
  stock?: number;
  stockStatus?: string;
  bindStatus?: string;
};

export type CustomerDashboardSummary = {
  pendingReplyCount: number;
  todayNewMessages: number;
  aiSuggestionPendingCount: number;
  sendFailureCount: number;
  unauthorizedShopCount: number;
  syncTaskFailureCount: number;
  openConversationCount: number;
};

export type CustomerAutoReplyPolicy = {
  shopId: string;
  shopName: string;
  platform: string;
  globalEnabled: boolean;
  workerAvailable: boolean;
  enabled: boolean;
  effectiveEnabled: boolean;
  tone: string;
  shopPolicy?: string;
  maxReplyRunes: number;
  maxRepliesPerHour: number;
  requireOrderContext: boolean;
  lowRiskOnly: boolean;
  updatedAt?: string;
};

export type CustomerAutoReplySetting = {
  messageSyncEnabled: boolean;
  autoReplyEnabled: boolean;
  pollIntervalSeconds: number;
  workerAvailable: boolean;
  effectiveEnabled: boolean;
  updatedAt?: string;
};

export type UpdateCustomerAutoReplySetting = Pick<
  CustomerAutoReplySetting,
  'messageSyncEnabled' | 'autoReplyEnabled' | 'pollIntervalSeconds'
>;

export type UpdateCustomerAutoReplyPolicy = Pick<
  CustomerAutoReplyPolicy,
  | 'enabled'
  | 'tone'
  | 'shopPolicy'
  | 'maxReplyRunes'
  | 'maxRepliesPerHour'
  | 'requireOrderContext'
  | 'lowRiskOnly'
>;

export type CustomerAutoReplyRun = {
  id: string;
  conversationId: string;
  messageId: string;
  suggestionId?: string;
  sentMessageId?: string;
  status: 'pending' | 'generating' | 'sending' | 'sent' | 'human_required' | 'skipped' | 'failed';
  riskLevel?: string;
  reasonCode?: string;
  errorMessage?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
};

export type SuggestionRow = {
  id: string;
  conversationId: string;
  messageId?: string;
  status: string;
  suggestedReply?: string;
  editedReply?: string;
  rejectReason?: string;
  language?: string;
  tone?: string;
  contextSummary?: ContextSummary;
  createdAt: string;
  updatedAt: string;
};

export type CustomerMessageRow = {
  id: string;
  conversationId: string;
  role: string;
  content: string;
  language: string;
  messageType?: string;
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
  contextSummary?: ContextSummary;
};

type Paginated<T> = {
  list: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
};

function boolQueryFlag(v: boolean | string | undefined): string | undefined {
  if (v === true || v === 'true' || v === '1') return '1';
  return undefined;
}

export async function queryConversations(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  status?: string;
  shopId?: string;
  customerName?: string;
  keyword?: string;
  pendingReply?: boolean | string;
  hasAiSuggestion?: boolean | string;
  sendFailed?: boolean | string;
  hasOrder?: boolean | string;
  start?: string;
  end?: string;
  updatedStart?: string;
  updatedEnd?: string;
}): Promise<Paginated<ConversationRow>> {
  return getWithParams('/api/v1/customer/conversations', {
    page: params.page,
    pageSize: params.pageSize,
    platform: params.platform,
    status: params.status,
    shopId: params.shopId,
    customerName: params.customerName,
    keyword: params.keyword,
    pendingReply: boolQueryFlag(params.pendingReply),
    hasAiSuggestion: boolQueryFlag(params.hasAiSuggestion),
    sendFailed: boolQueryFlag(params.sendFailed),
    hasOrder: boolQueryFlag(params.hasOrder),
    start: params.start,
    end: params.end,
    updatedStart: params.updatedStart,
    updatedEnd: params.updatedEnd,
  });
}

export async function getCustomerDashboard(filters: { platform?: string; shopId?: string } = {}): Promise<CustomerDashboardSummary> {
  return getWithParams('/api/v1/customer/dashboard', filters);
}

export async function getCustomerAutoReplyPolicy(shopId: string): Promise<CustomerAutoReplyPolicy> {
  return getJSON(`/api/v1/customer/shops/${shopId}/auto-reply-policy`);
}

export async function getCustomerAutoReplySetting(): Promise<CustomerAutoReplySetting> {
  return getJSON('/api/v1/customer/auto-reply-setting');
}

export async function updateCustomerAutoReplySetting(
  payload: UpdateCustomerAutoReplySetting,
): Promise<CustomerAutoReplySetting> {
  return putJSON('/api/v1/customer/auto-reply-setting', payload);
}

export async function updateCustomerAutoReplyPolicy(
  shopId: string,
  payload: UpdateCustomerAutoReplyPolicy,
): Promise<CustomerAutoReplyPolicy> {
  return putJSON(`/api/v1/customer/shops/${shopId}/auto-reply-policy`, payload);
}

export async function queryCustomerAutoReplyRuns(shopId: string): Promise<CustomerAutoReplyRun[]> {
  return getJSON(`/api/v1/customer/shops/${shopId}/auto-reply-runs`);
}

export async function querySuggestions(conversationId: string): Promise<{ list: SuggestionRow[] }> {
  return getJSON(`/api/v1/customer/conversations/${conversationId}/ai-suggestions`);
}

export async function rejectReplySuggestion(id: string, payload: { reason?: string }): Promise<{ ok: boolean }> {
  return postJSON(`/api/v1/customer/ai-suggestions/${id}/reject`, payload);
}

export async function createConversation(payload: {
  platform?: string;
  shopId?: string;
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
  payload: {
    customerName?: string;
    customerLanguage?: string;
    status?: string;
    shopId?: string;
    orderId?: string;
  },
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

export type CustomerMessageSyncTaskRow = {
  id: string;
  shopId: string;
  shopName?: string;
  platform: string;
  taskType: string;
  status: string;
  mode: string;
  cursor?: string;
  startedAt?: string;
  finishedAt?: string;
  totalCount: number;
  successCount: number;
  failedCount: number;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function syncCustomerMessages(
  shopId: string,
  payload: { mode?: string; start?: string; end?: string; cursor?: string; limit?: number },
): Promise<CustomerMessageSyncTaskRow> {
  return postJSON(`/api/v1/shops/${shopId}/sync-customer-messages`, payload);
}

export async function queryCustomerMessageSyncTasks(params: {
  page?: number;
  pageSize?: number;
  shopId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}): Promise<{ list: CustomerMessageSyncTaskRow[]; pagination: { page: number; pageSize: number; total: number; totalPages: number } }> {
  return getWithParams('/api/v1/customer/message-sync/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    shopId: params.shopId,
    platform: params.platform,
    status: params.status,
    start: params.start,
    end: params.end,
  });
}

export async function getCustomerMessageSyncTask(id: string): Promise<CustomerMessageSyncTaskRow> {
  return getJSON(`/api/v1/customer/message-sync/tasks/${id}`);
}

export async function retryCustomerMessageSyncTask(id: string): Promise<CustomerMessageSyncTaskRow> {
  return postJSON(`/api/v1/customer/message-sync/tasks/${id}/retry`, {});
}

export async function sendPlatformMessage(
  conversationId: string,
  payload: {
    reply: string;
    clientMessageId: string;
    suggestionId?: string;
    idempotencyKey?: string;
  },
): Promise<CustomerMessageRow> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/send-platform-message`, payload);
}
