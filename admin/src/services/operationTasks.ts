import { tr } from '@/i18n';
import { ApiRequestError, getJSON, getWithParams, patchJSON, postJSON } from '@/services/request';

export type AllowedTaskActions = {
  canEditDraft: boolean;
  canApprove: boolean;
  canReject: boolean;
  canExecute: boolean;
  canRetry: boolean;
  canCancel: boolean;
};

export type OperationTaskSummary = {
  id: string;
  sourceType: string;
  sourceReference?: string;
  taskType: string;
  platform: string;
  title: string;
  summary?: string;
  status: string;
  priority: string;
  revision: number;
  latestDraftVersion?: number;
  latestExecutionStatus?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type PlatformDraftSummary = {
  draftId: string;
  draftVersion: number;
  adapterMode: 'mock' | 'sandbox' | 'local_draft_only' | 'production_draft' | string;
  payloadHash: string;
  status: string;
  changeReason?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  payload?: unknown;
};

export type ApprovalSummary = {
  approvalId: string;
  decision: string;
  draftVersion: number;
  draftPayloadHash: string;
  reviewerId: string;
  reason?: string;
  comment?: string;
  requestId?: string;
  createdAt: string;
};

export type ExecutionAttemptSummary = {
  attemptId: string;
  attemptNumber: number;
  status: string;
  adapterMode: string;
  platform: string;
  approvedDraftVersion: number;
  approvedDraftPayloadHash: string;
  executedDraftVersion: number;
  executedDraftPayloadHash: string;
  resultType?: string;
  requestId?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
};

export type OperationTaskDetail = OperationTaskSummary & {
  payload?: unknown;
  latestDraft?: PlatformDraftSummary;
  latestApproval?: ApprovalSummary;
  latestAttempt?: ExecutionAttemptSummary;
  allowedActions: AllowedTaskActions;
};

export type ExecutionFailure = {
  category: string;
  code: string;
  safeMessage: string;
  retryable: boolean;
};

export type ExecutionResponse = {
  status: string;
  attempt: ExecutionAttemptSummary;
  resultType?: string;
  taskStatus?: string;
  requestId?: string;
  failure?: ExecutionFailure;
};

export type OperationTaskEventDTO = {
  eventId: string;
  sequence: number;
  eventType: string;
  actorType: string;
  actorId?: string;
  beforeState?: string;
  afterState?: string;
  platformDraftId?: string;
  draftVersion?: number;
  requestId?: string;
  reason?: string;
  metadata?: unknown;
  occurredAt: string;
};

export type CursorPage<T> = {
  items: T[];
  nextCursor?: string;
  hasMore: boolean;
  limit: number;
};

export type EventCursorPage<T> = {
  items: T[];
  nextSequence?: number;
  hasMore: boolean;
  limit: number;
};

export type DraftListResponse = {
  items: PlatformDraftSummary[];
  limit: number;
};

export type CreateTaskRequest = {
  sourceType: string;
  sourceReference: string;
  taskType: string;
  platform: string;
  title: string;
  summary?: string;
  payload: unknown;
  priority: string;
};

export type CreateDraftRequest = {
  payload: unknown;
  changeReason: string;
  expectedTaskRevision: number;
};

export type EditDraftRequest = CreateDraftRequest & {
  expectedDraftVersion: number;
};

export type ApproveTaskRequest = {
  draftVersion: number;
  draftPayloadHash: string;
  reason: string;
  comment?: string;
  expectedTaskRevision: number;
};

export type RejectTaskRequest = ApproveTaskRequest;

export type ExecuteTaskRequest = {
  expectedTaskRevision: number;
  adapterMode: 'mock' | 'sandbox' | 'local_draft_only' | 'production_draft';
};

export type RetryTaskRequest = {
  failedAttemptId?: string;
  reason: string;
  expectedTaskRevision: number;
};

export type CancelTaskRequest = {
  reason: string;
  expectedTaskRevision: number;
};

export type ListTasksParams = {
  status?: string;
  platform?: string;
  taskType?: string;
  limit?: number;
  cursor?: string;
};

export type ListCursorParams = {
  limit?: number;
  cursor?: string;
};

export type ListEventsParams = {
  limit?: number;
  afterSequence?: number;
};

export type OperationTaskAPIError = {
  message: string;
  errorCode?: string;
  traceId?: string;
};

const BASE = '/api/v1/operation-tasks';

function enc(value: string) {
  return encodeURIComponent(value);
}

function idempotencyHeaders(key: string) {
  return { 'Idempotency-Key': key };
}

export function createOperationIdempotencyKey(action: string) {
  const random = globalThis.crypto?.randomUUID?.() ?? Math.random().toString(36).slice(2, 12);
  return `operation-task-ui:${action}:${Date.now()}:${random}`;
}

export function extractOperationTaskAPIError(error: unknown): OperationTaskAPIError {
  if (error instanceof ApiRequestError) {
    const data = error.data as { errorCode?: string } | null | undefined;
    return {
      message: error.message || tr('操作失败，请稍后重试'),
      errorCode: data?.errorCode,
      traceId: error.traceId,
    };
  }
  if (error instanceof Error) {
    return { message: error.message || tr('操作失败，请稍后重试') };
  }
  return { message: tr("操作失败，请稍后重试") };
}

export async function createTask(body: CreateTaskRequest, idempotencyKey: string) {
  return postJSON<OperationTaskDetail>(BASE, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function listTasks(params: ListTasksParams) {
  return getWithParams<CursorPage<OperationTaskSummary>>(BASE, params as Record<string, string | number | undefined>);
}

export async function getTask(taskId: string) {
  return getJSON<OperationTaskDetail>(`${BASE}/${enc(taskId)}`);
}

export async function createDraft(taskId: string, body: CreateDraftRequest, idempotencyKey: string) {
  return postJSON<OperationTaskDetail>(`${BASE}/${enc(taskId)}/drafts`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function editDraft(taskId: string, body: EditDraftRequest, idempotencyKey: string) {
  return patchJSON<OperationTaskDetail, EditDraftRequest>(`${BASE}/${enc(taskId)}/drafts/latest`, body, {
    headers: idempotencyHeaders(idempotencyKey),
  });
}

export async function listDrafts(taskId: string, limit = 50) {
  return getWithParams<DraftListResponse>(`${BASE}/${enc(taskId)}/drafts`, { limit });
}

export async function approveTask(taskId: string, body: ApproveTaskRequest, idempotencyKey: string) {
  return postJSON<OperationTaskDetail>(`${BASE}/${enc(taskId)}/approve`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function rejectTask(taskId: string, body: RejectTaskRequest, idempotencyKey: string) {
  return postJSON<OperationTaskDetail>(`${BASE}/${enc(taskId)}/reject`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function cancelTask(taskId: string, body: CancelTaskRequest, idempotencyKey: string) {
  return postJSON<OperationTaskDetail>(`${BASE}/${enc(taskId)}/cancel`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function executeTask(taskId: string, body: ExecuteTaskRequest, idempotencyKey: string) {
  return postJSON<ExecutionResponse>(`${BASE}/${enc(taskId)}/execute`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function retryTask(taskId: string, body: RetryTaskRequest, idempotencyKey: string) {
  return postJSON<ExecutionResponse>(`${BASE}/${enc(taskId)}/retry`, body, { headers: idempotencyHeaders(idempotencyKey) });
}

export async function listAttempts(taskId: string, params: ListCursorParams) {
  return getWithParams<CursorPage<ExecutionAttemptSummary>>(`${BASE}/${enc(taskId)}/attempts`, params as Record<string, string | number | undefined>);
}

export async function listEvents(taskId: string, params: ListEventsParams) {
  return getWithParams<EventCursorPage<OperationTaskEventDTO>>(`${BASE}/${enc(taskId)}/events`, params as Record<string, string | number | undefined>);
}
