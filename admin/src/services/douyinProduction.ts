import { getJSON, postJSON } from '@/services/request';

export type PreflightCheckItem = {
  key: string;
  status: 'passed' | 'warning' | 'failed';
  title: string;
  message: string;
  suggestion?: string;
  technicalDetails?: Record<string, unknown>;
};

export type DouyinPreflightResult = {
  status: 'passed' | 'warning' | 'failed';
  checks: PreflightCheckItem[];
  passedCount: number;
  warningCount: number;
  failedCount: number;
  checkedAt: string;
  liveTest?: boolean;
  blockedByRealCredentials?: boolean;
};

/** POST /api/v1/platform/douyin/production-preflight */
export async function runDouyinProductionPreflight(liveTest = false) {
  return postJSON<DouyinPreflightResult, { liveTest: boolean }>(
    '/api/v1/platform/douyin/production-preflight',
    { liveTest },
  );
}

/** GET /api/v1/platform/douyin/production-preflight/latest */
export async function getDouyinProductionPreflightLatest() {
  return getJSON<DouyinPreflightResult | { result: null; message?: string }>(
    '/api/v1/platform/douyin/production-preflight/latest',
  );
}

export type StoragePublicAccessResult = {
  ok: boolean;
  message?: string;
  errorCode?: string;
  technicalDetails?: Record<string, unknown>;
  storageKind?: string;
  testDeleted?: boolean;
};

/** POST /api/v1/storage/test-public-access */
export async function testStoragePublicAccess() {
  return postJSON<StoragePublicAccessResult>('/api/v1/storage/test-public-access');
}

export type DouyinRuntimeStatus = {
  status: 'normal' | 'paused' | 'emergency_disabled';
  reason?: string;
  changedAt?: string;
  message?: string;
};

export async function getDouyinRuntimeStatus() {
  return getJSON<DouyinRuntimeStatus>('/api/v1/platform/douyin/runtime-status');
}

export async function pauseDouyinRuntime(reason: string) {
  return postJSON<DouyinRuntimeStatus, { reason: string }>('/api/v1/platform/douyin/runtime-status/pause', { reason });
}

export async function resumeDouyinRuntime(reason: string) {
  return postJSON<DouyinRuntimeStatus, { reason: string }>('/api/v1/platform/douyin/runtime-status/resume', { reason });
}

export async function emergencyDisableDouyinRuntime(reason: string) {
  return postJSON<DouyinRuntimeStatus, { reason: string }>(
    '/api/v1/platform/douyin/runtime-status/emergency-disable',
    { reason },
  );
}

export type HealthLayerStatus = 'healthy' | 'degraded' | 'unhealthy' | 'disabled';

export type DouyinHealthSection = {
  status: HealthLayerStatus;
  label: string;
  details?: Record<string, unknown>;
};

export type DouyinGrayRelease = {
  enabled: boolean;
  writeOperationsEnabled: boolean;
  scheduledOrderSyncEnabled: boolean;
  scheduledInventorySyncEnabled: boolean;
  shopIds?: string[];
};

export type DouyinHealth = {
  overallStatus: HealthLayerStatus;
  overallLabel: string;
  checkedAt: string;
  config: DouyinHealthSection;
  auth: DouyinHealthSection;
  storage: DouyinHealthSection;
  tasks: DouyinHealthSection;
  api: DouyinHealthSection;
  runtime?: DouyinRuntimeStatus;
  grayRelease: DouyinGrayRelease;
};

export type DouyinMetricsSummary = {
  generatedAt: string;
  apiRequestsTotal: number;
  apiSuccessTotal: number;
  apiFailedTotal: number;
  apiSuccessRate: number;
  apiDurationAvgMs: number;
  apiTimeoutTotal: number;
  apiRateLimitedTotal: number;
  apiRetryTotal: number;
  tokenRefreshTotal: number;
  tokenRefreshFailedTotal: number;
  runtimeBlockedTasksTotal: number;
  staleTasksTotal: number;
  recoverySuccessTotal: number;
  recoveryFailedTotal: number;
  productDraftCreateTotal: number;
  productDraftCreateFailedTotal: number;
  imageUploadTotal: number;
  imageUploadFailedTotal: number;
  skuAutoBoundTotal: number;
  skuManualBoundTotal: number;
  skuUnmatchedTotal: number;
  skuAmbiguousTotal: number;
  orderFetchedTotal: number;
  orderCreatedTotal: number;
  orderUpdatedTotal: number;
  orderPartialSuccessTotal: number;
  orderUnmatchedItemsTotal: number;
  orderInventoryDeductedTotal: number;
  inventorySyncTotal: number;
  inventorySyncSuccessTotal: number;
  inventorySyncFailedTotal: number;
  inventorySyncSkippedTotal: number;
  failureTasksPending: number;
  authorizationsExpiring: number;
};

export type ReleaseGateStatus = 'not_checked' | 'blocked' | 'failed' | 'warning' | 'passed';

export type DouyinReleaseGateItem = {
  key: string;
  label: string;
  status: ReleaseGateStatus;
  message?: string;
};

export type DouyinReleaseGate = {
  overallConclusion: string;
  checkedAt: string;
  items: DouyinReleaseGateItem[];
};

/** GET /api/v1/platform/douyin/health */
export async function getDouyinHealth() {
  return getJSON<DouyinHealth>('/api/v1/platform/douyin/health');
}

/** GET /api/v1/platform/douyin/metrics-summary */
export async function getDouyinMetricsSummary() {
  return getJSON<DouyinMetricsSummary>('/api/v1/platform/douyin/metrics-summary');
}

/** GET /api/v1/platform/douyin/release-gate */
export async function getDouyinReleaseGate() {
  return getJSON<DouyinReleaseGate>('/api/v1/platform/douyin/release-gate');
}

/** POST /api/v1/platform/douyin/run-health-check */
export async function runDouyinHealthCheck() {
  return postJSON<DouyinHealth>('/api/v1/platform/douyin/run-health-check');
}
