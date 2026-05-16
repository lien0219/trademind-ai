import { getJSON } from './request';

export type CollectMonitorData = {
  queue: {
    enabled: boolean;
    name: string;
    redisAvailable: boolean;
    depth: number;
    oldestPendingSeconds?: number;
  };
  worker: {
    enabled: boolean;
    concurrency: number;
    running: boolean;
  };
  tasks: {
    pending: number;
    retrying: number;
    retryingCount: number;
    running: number;
    success: number;
    failed: number;
    cancelled: number;
  };
  batches: {
    running: number;
    partialSuccess: number;
    success: number;
    failed: number;
    cancelled: number;
  };
  recentFailures: Array<{
    id: string;
    source: string;
    sourceUrl: string;
    batchId?: string;
    errorMessage: string;
    updatedAt: string;
  }>;
  recentRetrying: Array<{
    id: string;
    source: string;
    sourceUrl: string;
    batchId?: string;
    retryCount: number;
    maxRetries: number;
    nextRetryAt?: string;
    errorMessage?: string;
    updatedAt: string;
  }>;
  retry: {
    enabled: boolean;
    maxRetries: number;
    baseDelaySeconds: number;
    maxDelaySeconds: number;
    nextRetryDueCount: number;
    oldestRetryingSeconds?: number;
  };
  collector: {
    baseUrl: string;
    timeoutSeconds: number;
    reachable: boolean;
    message: string;
  };
};

export async function getCollectMonitor() {
  return getJSON<CollectMonitorData>('/api/v1/collect/monitor');
}
