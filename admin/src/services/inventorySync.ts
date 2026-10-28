import { tr } from '@/i18n';
import { ApiRequestError, getJSON, getWithParams, postJSON } from '@/services/request';

export type CursorPage<T> = {
  items: T[];
  nextCursor?: string;
  hasMore: boolean;
  limit: number;
};

export type InventorySyncRunAllowedActions = {
  canViewSnapshots: boolean;
  canRerun: boolean;
  canViewAudit: boolean;
};

export type InventorySyncRunStatistics = {
  totalRecordCount: number;
  matchedRecordCount: number;
  unmatchedRecordCount: number;
  conflictRecordCount: number;
  failedRecordCount: number;
  manualBindingRequestCount: number;
  confirmedBindingCount: number;
  pagesProcessed: number;
};

export type InventorySyncRunSafeError = {
  code?: string;
  message?: string;
};

export type InventorySyncRun = {
  id: string;
  shopConnectionId: string;
  platform: string;
  providerMode: string;
  status: string;
  triggerType: string;
  fixtureScenario?: string;
  rerunOfRunId?: string;
  statistics: InventorySyncRunStatistics;
  safeError?: InventorySyncRunSafeError;
  cursorHash: string;
  revision: number;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
  allowedActions: InventorySyncRunAllowedActions;
};

export type SnapshotBindingSummary = {
  result: string;
  bindingId?: string;
  bindingStatus?: string;
  localProductId?: string;
  localSkuId?: string;
  confidence?: number;
  calibrationVersion?: number;
  manualRequestId?: string;
};

export type InventorySnapshot = {
  id: string;
  inventorySyncRunId: string;
  shopConnectionId: string;
  platform: string;
  externalProductId: string;
  externalSkuId: string;
  externalProductCode?: string;
  externalSkuCode?: string;
  barcode?: string;
  productTitle?: string;
  variantTitle?: string;
  availableQuantity: number;
  reservedQuantity: number;
  totalQuantity: number;
  sourceUpdatedAt?: string;
  observedAt: string;
  binding: SnapshotBindingSummary;
  createdAt: string;
};

export type SKUBindingAllowedActions = {
  canViewHistory: boolean;
  canViewCalibration: boolean;
};

export type SKUBinding = {
  id: string;
  shopConnectionId: string;
  platform: string;
  externalProductId: string;
  externalSkuId: string;
  externalSkuCode?: string;
  localProductId: string;
  localSkuId: string;
  bindingSource: string;
  bindingStatus: string;
  confidence: number;
  calibrationVersion: number;
  calibrationReason?: string;
  confirmedBy?: string;
  confirmedAt?: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
  allowedActions: SKUBindingAllowedActions;
};

export type CalibrationScoreBreakdown = {
  signal: string;
  score: number;
};

export type BindingCalibration = {
  id: string;
  inventorySyncRunId: string;
  inventorySnapshotItemId: string;
  externalSkuId: string;
  candidateLocalProductId: string;
  candidateLocalSkuId: string;
  matchStrategy: string;
  confidence: number;
  scoreBreakdown: CalibrationScoreBreakdown[];
  reasonCodes: string[];
  calibrationVersion: number;
  status: string;
  createdAt: string;
};

export type ManualBindingAllowedActions = {
  canConfirm: boolean;
  canReject: boolean;
};

export type ManualBindingRequest = {
  id: string;
  inventorySyncRunId: string;
  inventorySnapshotItemId: string;
  shopConnectionId: string;
  externalSkuId: string;
  status: string;
  reasonCode: string;
  candidateCount: number;
  suggestedLocalSkuId?: string;
  assignedTo?: string;
  resolvedBy?: string;
  resolvedAt?: string;
  resolution?: string;
  selectedLocalProductId?: string;
  selectedLocalSkuId?: string;
  comment?: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
  allowedActions: ManualBindingAllowedActions;
};

export type ManualBindingDecision = {
  id: string;
  operation: string;
  actorId: string;
  selectedLocalProductId?: string;
  selectedLocalSkuId?: string;
  reasonCode: string;
  comment?: string;
  requestRevision: number;
  createdAt: string;
};

export type ManualBindingDetail = {
  request: ManualBindingRequest;
  decisions: ManualBindingDecision[];
};

export type BindingHistory = {
  binding: SKUBinding;
  calibrations: BindingCalibration[];
  manualDecisions: ManualBindingDecision[];
};

export type AuditEvent = {
  id: string;
  action: string;
  resource: string;
  resourceId?: string;
  shopId?: string;
  platform?: string;
  permission?: string;
  requestId?: string;
  status: string;
  metadata?: unknown;
  actorId?: string;
  actorRole?: string;
  createdAt: string;
};

export type CreateRunRequest = {
  shopConnectionId: string;
  platform: string;
  providerMode: string;
  fixtureScenario?: string;
};

export type RerunRequest = {
  expectedRevision: number;
};

export type RecalibrateRequest = {
  expectedCalibrationVersion: number;
  reason: string;
};

export type ConfirmManualBindingRequest = {
  expectedRevision: number;
  selectedLocalSkuId: string;
  comment?: string;
};

export type RejectManualBindingRequest = {
  expectedRevision: number;
  reasonCode: string;
  comment?: string;
};

export type ListRunsParams = {
  shopConnectionId?: string;
  status?: string;
  providerMode?: string;
  limit?: number;
  cursor?: string;
};

export type ListSnapshotsParams = {
  bindingResult?: string;
  limit?: number;
  cursor?: string;
};

export type ListBindingsParams = {
  shopConnectionId?: string;
  bindingStatus?: string;
  bindingSource?: string;
  limit?: number;
  cursor?: string;
};

export type ListManualRequestsParams = {
  shopConnectionId?: string;
  status?: string;
  limit?: number;
  cursor?: string;
};

export type ListCursorParams = {
  limit?: number;
  cursor?: string;
};

export type InventorySyncAPIError = {
  message: string;
  errorCode?: string;
  traceId?: string;
};

const BASE = '/api/v1/inventory-sync';

function enc(value: string) {
  return encodeURIComponent(value);
}

function idempotencyHeaders(key: string) {
  return { 'Idempotency-Key': key };
}

export function createInventorySyncIdempotencyKey(action: string) {
  const random = Math.random().toString(36).slice(2, 12);
  return `inventory-sync-ui:${action}:${Date.now()}:${random}`;
}

export function extractInventorySyncAPIError(error: unknown): InventorySyncAPIError {
  if (error instanceof ApiRequestError) {
    const data = error.data as { errorCode?: string } | null | undefined;
    return {
      message: error.message || tr('操作失败，请稍后重试'),
      errorCode: data?.errorCode,
      traceId: error.traceId,
    };
  }
  if (error instanceof Error) return { message: error.message || tr('操作失败，请稍后重试') };
  return { message: tr("操作失败，请稍后重试") };
}

export async function createInventorySyncRun(body: CreateRunRequest, idempotencyKey: string) {
  return postJSON<InventorySyncRun>(`${BASE}/runs`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function listInventorySyncRuns(params: ListRunsParams) {
  return getWithParams<CursorPage<InventorySyncRun>>(`${BASE}/runs`, params as Record<string, string | number | undefined>);
}

export async function getInventorySyncRun(runId: string) {
  return getJSON<InventorySyncRun>(`${BASE}/runs/${enc(runId)}`);
}

export async function rerunInventorySyncRun(runId: string, body: RerunRequest, idempotencyKey: string) {
  return postJSON<InventorySyncRun>(`${BASE}/runs/${enc(runId)}/rerun`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function listInventorySnapshots(runId: string, params: ListSnapshotsParams) {
  return getWithParams<CursorPage<InventorySnapshot>>(
    `${BASE}/runs/${enc(runId)}/snapshots`,
    params as Record<string, string | number | undefined>,
  );
}

export async function listInventoryRunAuditEvents(runId: string, params: ListCursorParams) {
  return getWithParams<CursorPage<AuditEvent>>(
    `${BASE}/runs/${enc(runId)}/audit-events`,
    params as Record<string, string | number | undefined>,
  );
}

export async function getInventorySnapshot(snapshotId: string) {
  return getJSON<InventorySnapshot>(`${BASE}/snapshots/${enc(snapshotId)}`);
}

export async function listInventorySnapshotCalibrations(snapshotId: string, params: ListCursorParams) {
  return getWithParams<CursorPage<BindingCalibration>>(
    `${BASE}/snapshots/${enc(snapshotId)}/calibrations`,
    params as Record<string, string | number | undefined>,
  );
}

export async function recalibrateInventorySnapshot(snapshotId: string, body: RecalibrateRequest, idempotencyKey: string) {
  return postJSON<InventorySnapshot>(`${BASE}/snapshots/${enc(snapshotId)}/recalibrate`, body, {
    headers: idempotencyHeaders(idempotencyKey),
  });
}

export async function listInventoryBindings(params: ListBindingsParams) {
  return getWithParams<CursorPage<SKUBinding>>(`${BASE}/bindings`, params as Record<string, string | number | undefined>);
}

export async function getInventoryBinding(bindingId: string) {
  return getJSON<SKUBinding>(`${BASE}/bindings/${enc(bindingId)}`);
}

export async function getInventoryBindingHistory(bindingId: string) {
  return getJSON<BindingHistory>(`${BASE}/bindings/${enc(bindingId)}/history`);
}

export async function listManualBindingRequests(params: ListManualRequestsParams) {
  return getWithParams<CursorPage<ManualBindingRequest>>(
    `${BASE}/manual-binding-requests`,
    params as Record<string, string | number | undefined>,
  );
}

export async function getManualBindingRequest(requestId: string) {
  return getJSON<ManualBindingDetail>(`${BASE}/manual-binding-requests/${enc(requestId)}`);
}

export async function confirmManualBindingRequest(
  requestId: string,
  body: ConfirmManualBindingRequest,
  idempotencyKey: string,
) {
  return postJSON<ManualBindingDetail>(`${BASE}/manual-binding-requests/${enc(requestId)}/confirm`, body, {
    headers: idempotencyHeaders(idempotencyKey),
  });
}

export async function rejectManualBindingRequest(
  requestId: string,
  body: RejectManualBindingRequest,
  idempotencyKey: string,
) {
  return postJSON<ManualBindingDetail>(`${BASE}/manual-binding-requests/${enc(requestId)}/reject`, body, {
    headers: idempotencyHeaders(idempotencyKey),
  });
}
