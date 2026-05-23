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
  orderExceptionTotal: number;
  skuUnmatchedOrderItems: number;
  inventoryDeductFailedOrders: number;
  /** Workbench compact KPI aliases */
  draftTotal: number;
  todayNewProducts: number;
  missingAiTitle: number;
  missingAiDescription: number;
  imageTaskPending: number;
  imageTaskFailed: number;
  readinessBlocked: number;
  publishable: number;
  published: number;
  imageProcessedProducts: number;
  inventoryAlerts: number;
  orderExceptions: number;
  collectFailedCount: number;
  aiTitleCompletedCount: number;
  aiDescriptionCompletedCount: number;
  collectedProductsCount: number;
  aiTextCompletedCount: number;
  readinessPassedCount: number;
};

export type DashboardTodo = {
  id: string;
  key: string;
  title: string;
  count: number;
  severity: string;
  level: string;
  description: string;
  link: string;
};

export type DashboardFunnelStep = {
  key: string;
  title: string;
  count: number;
  link: string;
  description?: string;
};

export type DashboardException = {
  key: string;
  title: string;
  count: number;
  lastOccurredAt?: string;
  link: string;
  description: string;
};

export type DashboardQuickLink = {
  title: string;
  link: string;
  description?: string;
  icon?: string;
};

export type DashboardRecentItem = {
  type: string;
  title: string;
  subtitle?: string;
  status?: string;
  occurredAt: string;
  link: string;
};

export type ProductOperationDashboard = {
  summary: DashboardSummary;
  todos: DashboardTodo[];
  funnel: DashboardFunnelStep[];
  exceptions: DashboardException[];
  charts: Record<string, unknown>;
  quickLinks: DashboardQuickLink[];
  recent: {
    products?: DashboardRecentItem[];
    collectedProducts?: DashboardRecentItem[];
    aiTasks?: DashboardRecentItem[];
    aiBatches?: DashboardRecentItem[];
    imageTasks?: DashboardRecentItem[];
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
