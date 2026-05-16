import { getJSON } from './request';

export type WorkerMonitorSummary = {
  running: number;
  stale: number;
  stopped: number;
};

export type WorkerMonitorInstance = {
  workerId: string;
  workerType: string;
  instanceName?: string;
  hostname?: string;
  pid: number;
  status: string;
  effectiveStatus?: string;
  lastHeartbeatAt?: string;
  startedAt: string;
  stoppedAt?: string;
  meta?: Record<string, unknown>;
  workerInstanceId?: string;
};

export type LeasedTaskRow = {
  id: string;
  status: string;
  lockedBy?: string | null;
  lockedUntil?: string | null;
  createdAt: string;
  updatedAt: string;
};

export type WorkerMonitorData = {
  summary: WorkerMonitorSummary;
  byType: Record<string, WorkerMonitorSummary>;
  instances: WorkerMonitorInstance[];
  leasedTasks: {
    collect: LeasedTaskRow[];
    image: LeasedTaskRow[];
    orderSync: LeasedTaskRow[];
    customerMessageSync: LeasedTaskRow[];
  };
};

export async function getWorkersMonitor() {
  return getJSON<WorkerMonitorData>('/api/v1/workers/monitor');
}
