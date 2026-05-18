import { getWithParams } from './request';

export type DashboardSummary = {
  totalProducts: number;
  draftProducts: number;
  readyProducts: number;
  publishedProducts: number;
  archivedProducts: number;
  aiPendingProducts: number;
  readinessBlockedProducts: number;
  publishFailedTasks: number;
  lowStockSkus: number;
  customerPendingConversations: number;
  failedTasks: number;
  missingAiTitleCount: number;
  missingAiDescriptionCount: number;
  aiTaskFailedCount: number;
  aiBatchRunningCount: number;
  aiBatchFailedCount: number;
  readinessWarningProducts: number;
  readinessReadyProducts: number;
  publishPendingTasks: number;
  publishRunningTasks: number;
  publishedPublicationCount: number;
  outOfStockSkus: number;
  platformStockMismatchCount: number;
  inventorySyncFailedCount: number;
  customerOpenConversations: number;
  customerPendingReplyCount: number;
  aiReplySuggestionPendingCount: number;
  failedTaskTotal: number;
  criticalAlertCount: number;
  openAlertCount: number;
};

export type DashboardTodo = {
  id: string;
  title: string;
  count: number;
  severity: string;
  description: string;
  link: string;
};

export type DashboardQuickLink = {
  title: string;
  link: string;
};

export type DashboardRecentItem = {
  type: string;
  title: string;
  subtitle?: string;
  occurredAt: string;
  link: string;
};

export type ProductOperationDashboard = {
  summary: DashboardSummary;
  todos: DashboardTodo[];
  charts: Record<string, unknown>;
  quickLinks: DashboardQuickLink[];
  recent: {
    collectedProducts?: DashboardRecentItem[];
    aiBatches?: DashboardRecentItem[];
    publishTasks?: DashboardRecentItem[];
    inventoryAlerts?: DashboardRecentItem[];
    customerConversations?: DashboardRecentItem[];
    failedTasks?: DashboardRecentItem[];
    alerts?: DashboardRecentItem[];
  };
  filters?: Record<string, unknown>;
};

export async function queryProductOperationDashboard(params?: {
  start?: string;
  end?: string;
  platform?: string;
  shopId?: string;
  source?: string;
}) {
  return getWithParams<ProductOperationDashboard>('/api/v1/dashboard/product-operations', {
    start: params?.start,
    end: params?.end,
    platform: params?.platform,
    shopId: params?.shopId,
    source: params?.source,
  });
}
