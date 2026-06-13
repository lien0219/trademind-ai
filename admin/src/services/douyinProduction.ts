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
