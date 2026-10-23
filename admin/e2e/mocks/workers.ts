import type { FailuresSummary } from '../../src/services/taskCenter';
import type { WorkerMonitorData } from '../../src/services/workers';
import { ok } from './envelope';

export function workersResponse(path: string) {
  if (path === '/api/v1/workers/monitor') {
    const data: WorkerMonitorData = {
      summary: { running: 0, stale: 0, stopped: 0 },
      byType: {},
      instances: [],
      leasedTasks: { collect: [], image: [], orderSync: [], customerMessageSync: [], productPublish: [], inventorySync: [] },
    };
    return ok(data);
  }
  if (path === '/api/v1/task-center/summary') {
    const summary: FailuresSummary = { totalFailed: 0, byType: {}, byPlatform: {}, retryableCount: 0, ignoredCount: 0, handledCount: 0 };
    return ok(summary);
  }
  return null;
}
