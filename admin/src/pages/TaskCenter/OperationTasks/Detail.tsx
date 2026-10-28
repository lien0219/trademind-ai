import { tr, trFormat } from '@/i18n';
import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import { Button, Descriptions, Form, Input, Modal, Select, Space, Spin, Table, Tabs, Tag, Timeline, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { EmptyState, ErrorAlert, OperationToolbar, SectionCard, TaskJsonBlock, TmPageContainer } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useLanguage } from '@/i18n/useLanguage';
import {
  approveTask,
  cancelTask,
  createDraft,
  createOperationIdempotencyKey,
  executeTask,
  extractOperationTaskAPIError,
  getTask,
  listAttempts,
  listDrafts,
  listEvents,
  rejectTask,
  retryTask,
  type ExecutionAttemptSummary,
  type OperationTaskAPIError,
  type OperationTaskDetail,
  type OperationTaskEventDTO,
  type PlatformDraftSummary,
} from '@/services/operationTasks';
import {
  getProductionRuntimeStatus,
  productionDraftBlockReason,
  type ProductionRuntimeStatus,
} from '@/services/productionControl';
import { formatDateTime } from '@/utils/formatTime';
import { isProductionBuild } from '@/utils/runtimeEnvironment';
import {
  NonProductionBoundary,
  OperationAttemptStatusTag,
  OperationDraftStatusTag,
  OperationPriorityTag,
  OperationTaskStatusTag,
  ProductionRuntimeBoundary,
  actorTypeLabel,
  adapterModeLabel,
  copyableText,
  diffJSON,
  eventTypeLabel,
  jsonPreview,
  normalizeOperationTaskTab,
  operationErrorMessage,
  parseJSONInput,
  platformLabel,
  renderOperationError,
  resultTypeLabel,
  safeMetadata,
  operationSourceLabel,
  taskTypeLabel,
} from './components/OperationTaskShared';
import './index.less';

const { Text, Paragraph } = Typography;

type ModalKind = 'draft' | 'approve' | 'reject' | 'execute' | 'retry' | 'cancel';
type DetailTab = 'drafts' | 'attempts' | 'events';
type DetailQueryState = { tab?: string; from?: string };
type ResourceErrors = { drafts?: OperationTaskAPIError; attempts?: OperationTaskAPIError; events?: OperationTaskAPIError };

type FrozenDraftMapping = {
  categoryId?: string;
  categoryPath?: string;
  title?: string;
  description?: string;
  mainImages?: { platformImageUrl?: string; url?: string }[];
  detailImages?: { platformImageUrl?: string; url?: string }[];
  skus?: { localSkuId?: string; name?: string; price?: number; stock?: number | null; attrs?: Record<string, unknown> }[];
  price?: { currency?: string; min?: number; max?: number };
  stock?: { total?: number; min?: number; unconfirmed?: boolean };
};

type FrozenProductionDraft = {
  schemaVersion?: string;
  productId?: string;
  shopId?: string;
  publishMode?: string;
  skuCount?: number;
  review?: Record<string, unknown>;
  mappingSnapshot?: FrozenDraftMapping;
  mappingHash?: string;
};

function productionDraftPayload(value: unknown): FrozenProductionDraft | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const draft = value as FrozenProductionDraft;
  return draft.schemaVersion === 'douyin_draft_v1' && draft.mappingSnapshot ? draft : null;
}

function imageURL(image?: { platformImageUrl?: string; url?: string }) {
  return image?.platformImageUrl || image?.url;
}

function frozenSkuRowKey(row: NonNullable<FrozenDraftMapping['skus']>[number]) {
  if (row.localSkuId) return row.localSkuId;
  const attrs = Object.entries(row.attrs ?? {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}:${String(value)}`)
    .join('|');
  return `${row.name || 'sku'}:${attrs}:${row.price ?? ''}:${row.stock ?? ''}`;
}

function latestDraft(drafts: PlatformDraftSummary[]) {
  return [...drafts].sort((a, b) => b.draftVersion - a.draftVersion)[0];
}

function requiredReasonRule(label: string) {
  return [{ required: true, whitespace: true, message: trFormat("请填写{value0}", { value0: label }) }];
}

function isFormValidationError(error: unknown) {
  return !!error && typeof error === 'object' && 'errorFields' in error;
}

function safeReturnPath(value?: string): string {
  if (!value) return '/ops/task-center/operation-tasks';
  try {
    const base = new URL('https://trademind.local');
    const target = new URL(value, base);
    if (target.origin === base.origin && target.pathname === '/ops/task-center/operation-tasks') {
      return `${target.pathname}${target.search}`;
    }
  } catch {
    // Invalid return targets fall back to the list route.
  }
  return '/ops/task-center/operation-tasks';
}

function resourceError(error: OperationTaskAPIError | undefined, hasPreviousData: boolean) {
  if (!error) return null;
  return (
    <ErrorAlert
      title={operationErrorMessage(error)}
      actionHint={hasPreviousData
        ? tr('刷新失败，当前显示上次成功加载的记录。')
        : error.traceId ? trFormat('排查编号：{traceId}', { traceId: error.traceId }) : tr('请稍后刷新重试。')}
    />
  );
}

export default function OperationTaskDetailPage() {
  useLanguage();
  const params = useParams<{ taskId: string }>();
  const taskId = params.taskId || '';
  const { state: urlState, setState: setUrlState } = useUrlQueryState<DetailQueryState>(['tab', 'from']);
  const activeTab = normalizeOperationTaskTab(urlState.tab) as DetailTab;
  const returnPath = safeReturnPath(urlState.from);
  const [detail, setDetail] = useState<OperationTaskDetail | null>(null);
  const [drafts, setDrafts] = useState<PlatformDraftSummary[]>([]);
  const [attempts, setAttempts] = useState<ExecutionAttemptSummary[]>([]);
  const [events, setEvents] = useState<OperationTaskEventDTO[]>([]);
  const [attemptCursor, setAttemptCursor] = useState<string | undefined>();
  const [eventsSequence, setEventsSequence] = useState<number | undefined>();
  const [attemptHasMore, setAttemptHasMore] = useState(false);
  const [eventsHasMore, setEventsHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [auxiliaryLoading, setAuxiliaryLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [attemptMoreLoading, setAttemptMoreLoading] = useState(false);
  const [eventMoreLoading, setEventMoreLoading] = useState(false);
  const [error, setError] = useState<OperationTaskAPIError | null>(null);
  const [resourceErrors, setResourceErrors] = useState<ResourceErrors>({});
  const [productionStatus, setProductionStatus] = useState<ProductionRuntimeStatus | null>(null);
  const [productionStatusLoading, setProductionStatusLoading] = useState(false);
  const [productionStatusError, setProductionStatusError] = useState<string>();
  const [modal, setModal] = useState<ModalKind | null>(null);
  const [jsonText, setJsonText] = useState('');
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [form] = Form.useForm();
  const requestSeq = useRef(0);
  const loadedTaskId = useRef('');
  const actionSubmittingRef = useRef(false);
  const actionIdempotencyRef = useRef<{ kind: ModalKind; fingerprint: string; key: string } | null>(null);
  const initializedModalRef = useRef<ModalKind | null>(null);
  const attemptMoreLoadingRef = useRef(false);
  const eventMoreLoadingRef = useRef(false);

  const currentDraft = detail?.latestDraft || latestDraft(drafts);
  const isProductionDraft = currentDraft?.adapterMode === 'production_draft';
  const frozenDraft = productionDraftPayload(currentDraft?.payload);
  const failedAttempts = useMemo(() => attempts.filter((attempt) => attempt.status === 'failed'), [attempts]);

  const loadAll = useCallback(async () => {
    if (!taskId) {
      setError({ message: tr("任务地址无效，请返回列表重新选择。"), errorCode: 'validation_error' });
      return;
    }
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    const identityChanged = loadedTaskId.current !== taskId;
    if (identityChanged) {
      loadedTaskId.current = taskId;
      setDetail(null);
      setDrafts([]);
      setAttempts([]);
      setEvents([]);
      setAttemptCursor(undefined);
      setEventsSequence(undefined);
      setAttemptHasMore(false);
      setEventsHasMore(false);
      setProductionStatus(null);
      setProductionStatusError(undefined);
    }
    setLoading(true);
    setError(null);
    setResourceErrors({});
    try {
      const nextDetail = await getTask(taskId);
      if (requestSeq.current !== seq) return;
      setDetail(nextDetail);
      const productionTask = nextDetail.latestDraft?.adapterMode === 'production_draft';
      setProductionStatusLoading(productionTask);
      setLoading(false);
      setAuxiliaryLoading(true);
      const [draftResult, attemptResult, eventResult, productionStatusResult] = await Promise.allSettled([
        listDrafts(taskId, 50),
        listAttempts(taskId, { limit: 20 }),
        listEvents(taskId, { limit: 30 }),
        productionTask ? getProductionRuntimeStatus() : Promise.resolve(null),
      ]);
      if (requestSeq.current !== seq) return;
      const nextResourceErrors: ResourceErrors = {};
      if (draftResult.status === 'fulfilled') {
        setDrafts([...draftResult.value.items].sort((a, b) => b.draftVersion - a.draftVersion));
      } else {
        nextResourceErrors.drafts = extractOperationTaskAPIError(draftResult.reason);
      }
      if (attemptResult.status === 'fulfilled') {
        setAttempts(attemptResult.value.items);
        setAttemptCursor(attemptResult.value.nextCursor);
        setAttemptHasMore(attemptResult.value.hasMore);
      } else {
        nextResourceErrors.attempts = extractOperationTaskAPIError(attemptResult.reason);
      }
      if (eventResult.status === 'fulfilled') {
        setEvents(eventResult.value.items);
        setEventsSequence(eventResult.value.nextSequence);
        setEventsHasMore(eventResult.value.hasMore);
      } else {
        nextResourceErrors.events = extractOperationTaskAPIError(eventResult.reason);
      }
      if (productionStatusResult.status === 'fulfilled') {
        setProductionStatus(productionStatusResult.value);
        setProductionStatusError(undefined);
      } else if (productionTask) {
        setProductionStatus(null);
        setProductionStatusError(operationErrorMessage(extractOperationTaskAPIError(productionStatusResult.reason)));
      }
      setResourceErrors(nextResourceErrors);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractOperationTaskAPIError(e));
      if (identityChanged) setDetail(null);
    } finally {
      if (requestSeq.current === seq) {
        setLoading(false);
        setAuxiliaryLoading(false);
        setProductionStatusLoading(false);
      }
    }
  }, [taskId]);

  useEffect(() => {
    void loadAll();
    return () => {
      requestSeq.current += 1;
    };
  }, [loadAll]);

  useLayoutEffect(() => {
    if (!modal) {
      initializedModalRef.current = null;
      return;
    }
    if (initializedModalRef.current === modal) return;
    initializedModalRef.current = modal;
    form.resetFields();
    if (modal === 'draft') form.setFieldsValue({ changeReason: '' });
    if (modal === 'execute') form.setFieldsValue({ adapterMode: isProductionDraft ? 'production_draft' : 'local_draft_only' });
    if (modal === 'retry') form.setFieldsValue({ failedAttemptId: failedAttempts[0]?.attemptId, reason: '' });
  }, [failedAttempts, form, isProductionDraft, modal]);

  const openModal = (kind: ModalKind) => {
    setModal(kind);
    setJsonError(null);
    actionIdempotencyRef.current = null;
    if (kind === 'draft') {
      setJsonText(JSON.stringify(detail?.payload ?? {}, null, 2));
    }
  };

  const closeModal = () => {
    if (actionSubmittingRef.current) return;
    setModal(null);
    setJsonError(null);
    actionIdempotencyRef.current = null;
  };

  const completeAction = async (successText: string) => {
    message.success(successText);
    setModal(null);
    setJsonError(null);
    actionIdempotencyRef.current = null;
    await loadAll();
  };

  const actionIdempotencyKey = (kind: ModalKind, body: unknown) => {
    const fingerprint = JSON.stringify(body) ?? '';
    let idempotency = actionIdempotencyRef.current;
    if (idempotency?.kind !== kind || idempotency.fingerprint !== fingerprint) {
      idempotency = { kind, fingerprint, key: createOperationIdempotencyKey(kind) };
      actionIdempotencyRef.current = idempotency;
    }
    return idempotency.key;
  };

  const runAction = async () => {
    if (!detail || !modal || actionSubmittingRef.current) return;
    actionSubmittingRef.current = true;
    setActionLoading(true);
    setError(null);
    try {
      if (modal === 'draft') {
        const parsed = parseJSONInput(jsonText);
        if (!parsed.ok) {
          setJsonError(parsed.message);
          setActionLoading(false);
          return;
        }
        const values = await form.validateFields();
        if (currentDraft) throw new Error(tr('当前接口未提供最新草稿正文，无法安全编辑已有草稿'));
        const body = {
          payload: parsed.value,
          changeReason: values.changeReason,
          expectedTaskRevision: detail.revision,
        };
        await createDraft(taskId, body, actionIdempotencyKey('draft', body));
        await completeAction(tr('首版草稿已创建。'));
      }
      if (modal === 'approve') {
        const values = await form.validateFields();
        if (!currentDraft) throw new Error(tr('当前没有可审核草稿'));
        const body = {
          draftVersion: currentDraft.draftVersion,
          draftPayloadHash: currentDraft.payloadHash,
          reason: values.reason,
          comment: values.comment,
          expectedTaskRevision: detail.revision,
        };
        await approveTask(taskId, body, actionIdempotencyKey('approve', body));
        await completeAction(tr('已批准最新草稿。'));
      }
      if (modal === 'reject') {
        const values = await form.validateFields();
        if (!currentDraft) throw new Error(tr('当前没有可拒绝草稿'));
        const body = {
          draftVersion: currentDraft.draftVersion,
          draftPayloadHash: currentDraft.payloadHash,
          reason: values.reason,
          comment: values.comment,
          expectedTaskRevision: detail.revision,
        };
        await rejectTask(taskId, body, actionIdempotencyKey('reject', body));
        await completeAction(tr('已拒绝最新草稿。'));
      }
      if (modal === 'execute') {
        const values = await form.validateFields();
        const body = {
          expectedTaskRevision: detail.revision,
          adapterMode: values.adapterMode,
        };
        const result = await executeTask(taskId, body, actionIdempotencyKey('execute', body));
        if (result.failure || result.status === 'failed_retryable' || result.status === 'failed_final') {
          await loadAll();
          throw new Error(result.failure?.safeMessage || tr('草稿生成失败，请查看执行历史。'));
        }
        await completeAction(isProductionDraft ? tr('平台草稿创建请求已排队。') : tr('草稿生成请求已提交。'));
      }
      if (modal === 'retry') {
        const values = await form.validateFields();
        const body = {
          failedAttemptId: values.failedAttemptId,
          reason: values.reason,
          expectedTaskRevision: detail.revision,
        };
        const result = await retryTask(taskId, body, actionIdempotencyKey('retry', body));
        if (result.failure || result.status === 'failed_retryable' || result.status === 'failed_final') {
          await loadAll();
          throw new Error(result.failure?.safeMessage || tr('人工重试失败，请查看执行历史。'));
        }
        await completeAction(tr('人工重试请求已提交。'));
      }
      if (modal === 'cancel') {
        const values = await form.validateFields();
        const body = {
          reason: values.reason,
          expectedTaskRevision: detail.revision,
        };
        await cancelTask(taskId, body, actionIdempotencyKey('cancel', body));
        await completeAction(tr('任务已取消。'));
      }
    } catch (e) {
      if (isFormValidationError(e)) return;
      const next = extractOperationTaskAPIError(e);
      setError(next);
      message.error(operationErrorMessage(next));
      if (next.errorCode?.includes('conflict') || next.errorCode?.includes('mismatch') || next.errorCode === 'state_conflict') {
        await loadAll();
      }
    } finally {
      actionSubmittingRef.current = false;
      setActionLoading(false);
    }
  };

  const loadMoreAttempts = async () => {
    if (!attemptCursor || attemptMoreLoadingRef.current) return;
    const seq = requestSeq.current;
    attemptMoreLoadingRef.current = true;
    setAttemptMoreLoading(true);
    setResourceErrors((prev) => ({ ...prev, attempts: undefined }));
    try {
      const page = await listAttempts(taskId, { limit: 20, cursor: attemptCursor });
      if (requestSeq.current !== seq) return;
      setAttempts((prev) => {
        const known = new Set(prev.map((item) => item.attemptId));
        return [...prev, ...page.items.filter((item) => !known.has(item.attemptId))];
      });
      setAttemptCursor(page.nextCursor);
      setAttemptHasMore(page.hasMore);
    } catch (nextError) {
      setResourceErrors((prev) => ({ ...prev, attempts: extractOperationTaskAPIError(nextError) }));
    } finally {
      attemptMoreLoadingRef.current = false;
      setAttemptMoreLoading(false);
    }
  };

  const loadMoreEvents = async () => {
    if (!eventsSequence || eventMoreLoadingRef.current) return;
    const seq = requestSeq.current;
    eventMoreLoadingRef.current = true;
    setEventMoreLoading(true);
    setResourceErrors((prev) => ({ ...prev, events: undefined }));
    try {
      const page = await listEvents(taskId, { limit: 30, afterSequence: eventsSequence });
      if (requestSeq.current !== seq) return;
      setEvents((prev) => {
        const known = new Set(prev.map((item) => item.eventId));
        return [...prev, ...page.items.filter((item) => !known.has(item.eventId))];
      });
      setEventsSequence(page.nextSequence);
      setEventsHasMore(page.hasMore);
    } catch (nextError) {
      setResourceErrors((prev) => ({ ...prev, events: extractOperationTaskAPIError(nextError) }));
    } finally {
      eventMoreLoadingRef.current = false;
      setEventMoreLoading(false);
    }
  };

  const draftColumns: ColumnsType<PlatformDraftSummary> = [
    { title: tr("版本"), dataIndex: 'draftVersion', width: 90, render: (v) => `v${v}` },
    { title: tr("状态"), dataIndex: 'status', width: 120, render: (v) => <OperationDraftStatusTag status={String(v)} /> },
    { title: 'Payload Hash', dataIndex: 'payloadHash', render: (v) => copyableText(String(v), 18) },
    { title: tr("变更原因"), dataIndex: 'changeReason', ellipsis: true, render: (v) => v || '—' },
    { title: tr("创建人"), dataIndex: 'createdBy', render: (v) => copyableText(String(v || ''), 10) },
    { title: tr("创建时间"), dataIndex: 'createdAt', render: (v) => formatDateTime(String(v)) },
  ];

  const attemptColumns: ColumnsType<ExecutionAttemptSummary> = [
    { title: 'Attempt', dataIndex: 'attemptNumber', width: 100 },
    { title: tr("状态"), dataIndex: 'status', width: 120, render: (v) => <OperationAttemptStatusTag status={String(v)} /> },
    { title: 'Adapter Mode', dataIndex: 'adapterMode', render: (v) => adapterModeLabel(String(v)) },
    { title: tr("批准草稿"), dataIndex: 'approvedDraftVersion', render: (_, row) => `v${row.approvedDraftVersion} / ${row.approvedDraftPayloadHash || '—'}` },
    { title: tr("执行草稿"), dataIndex: 'executedDraftVersion', render: (_, row) => row.executedDraftVersion ? `v${row.executedDraftVersion} / ${row.executedDraftPayloadHash || '—'}` : '—' },
    { title: tr("结果"), dataIndex: 'resultType', render: (v) => resultTypeLabel(String(v || '')) },
    { title: 'Request ID', dataIndex: 'requestId', render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("开始时间"), dataIndex: 'startedAt', render: (v) => formatDateTime(String(v || '')) },
    { title: tr("结束时间"), dataIndex: 'finishedAt', render: (v) => formatDateTime(String(v || '')) },
  ];

  const modalTitle = useMemo(() => {
    if (modal === 'draft') return tr('创建首版草稿');
    if (modal === 'approve') return tr('确认批准');
    if (modal === 'reject') return tr('拒绝草稿');
    if (modal === 'execute') return isProductionDraft ? tr('创建平台草稿') : tr('执行草稿生成');
    if (modal === 'retry') return tr('人工重试');
    if (modal === 'cancel') return tr('取消运营任务');
    return '';
  }, [isProductionDraft, modal]);

  const modalOkText = useMemo(() => {
    if (modal === 'draft') return tr('创建草稿');
    if (modal === 'approve') return tr('批准草稿');
    if (modal === 'reject') return tr('拒绝草稿');
    if (modal === 'execute') return isProductionDraft ? tr('确认创建平台草稿') : tr('提交草稿生成');
    if (modal === 'retry') return tr('发起人工重试');
    if (modal === 'cancel') return tr('取消任务');
    return tr('提交');
  }, [isProductionDraft, modal]);

  if (loading && !detail) {
    return <Spin fullscreen tip={tr("正在加载运营任务详情")} />;
  }

  if (!detail && error) {
    return (
      <TmPageContainer
        title={tr("运营任务详情")}
        extra={<Button icon={<ArrowLeftOutlined />} onClick={() => history.push(returnPath)}>{tr("返回列表")}</Button>}
      >
        <ErrorAlert
          title={operationErrorMessage(error)}
          actionHint={(
            <OperationToolbar>
              <Text>{error.traceId ? trFormat('排查编号：{traceId}', { traceId: error.traceId }) : tr('请检查任务地址或稍后重试。')}</Text>
              <Button size="small" onClick={() => void loadAll()}>{tr("重新加载")}</Button>
            </OperationToolbar>
          )}
        />
      </TmPageContainer>
    );
  }

  const { sourceType: taskSourceType } = detail ?? {};
  const sourceLabel = taskSourceType ? operationSourceLabel(taskSourceType) : '-';

  return (
    <TmPageContainer
      title={PAGE_COPY.operationTasks.title}
      subTitle={tr("运营任务详情")}
      extra={
        <Space wrap>
          <Button icon={<ArrowLeftOutlined />} onClick={() => history.push(returnPath)}>{tr("返回列表")}</Button>
          <Button icon={<ReloadOutlined />} onClick={() => void loadAll()} loading={loading}>{tr("刷新")}</Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} className="operation-task-detail">
        {renderOperationError(error)}
        {isProductionDraft ? (
          <ProductionRuntimeBoundary status={productionStatus} loading={productionStatusLoading} error={productionStatusError} />
        ) : <NonProductionBoundary />}
        {detail ? (
          <SectionCard
            title={detail.title}
            description={detail.summary || tr('运营任务概要')}
            headerExtra={<OperationTaskStatusTag status={detail.status} />}
          >
            <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
              <Descriptions.Item label={tr("任务 ID")}>{copyableText(detail.id, 18)}</Descriptions.Item>
              <Descriptions.Item label={tr("任务类型")}>{taskTypeLabel(detail.taskType)}</Descriptions.Item>
              <Descriptions.Item label={tr("平台")}>{platformLabel(detail.platform)}</Descriptions.Item>
              <Descriptions.Item label={tr("优先级")}><OperationPriorityTag priority={detail.priority} /></Descriptions.Item>
              <Descriptions.Item label={tr("数据版本")}>{detail.revision}</Descriptions.Item>
              <Descriptions.Item label={tr("来源")}>{sourceLabel} / {copyableText(detail.sourceReference, 18)}</Descriptions.Item>
              <Descriptions.Item label={tr("创建人")}>{copyableText(detail.createdBy, 12)}</Descriptions.Item>
              <Descriptions.Item label={tr("创建时间")}>{formatDateTime(detail.createdAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("更新时间")}>{formatDateTime(detail.updatedAt)}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
        ) : null}

        {detail ? (
          <SectionCard title={tr("可用操作")} description={tr("操作可用性由当前账号权限和任务状态共同决定。")} compact>
            <OperationToolbar className="operation-task-detail__actions">
              {!isProductionDraft ? <Button disabled={!detail.allowedActions.canEditDraft || !!currentDraft} onClick={() => openModal('draft')}>{tr("创建首版草稿")}</Button> : null}
              <Button type="primary" disabled={!detail.allowedActions.canApprove || !currentDraft} onClick={() => openModal('approve')}>{tr("确认批准")}</Button>
              <Button danger disabled={!detail.allowedActions.canReject || !currentDraft} onClick={() => openModal('reject')}>{tr("拒绝")}</Button>
              <Button
                disabled={!detail.allowedActions.canExecute || (isProductionDraft && (!productionStatus?.productionReady || productionStatusLoading || !!productionStatusError))}
                onClick={() => openModal('execute')}
              >
                {isProductionDraft ? tr('创建平台草稿') : tr('执行草稿生成')}
              </Button>
              {!isProductionDraft ? <Button disabled={!detail.allowedActions.canRetry || failedAttempts.length === 0} onClick={() => openModal('retry')}>{tr("人工重试")}</Button> : null}
              <Button danger disabled={!detail.allowedActions.canCancel} onClick={() => openModal('cancel')}>{tr("取消任务")}</Button>
            </OperationToolbar>
            {currentDraft && !isProductionDraft ? (
              <Paragraph type="secondary" className="operation-task-detail__action-hint">
                {tr('已有草稿仅展示摘要。当前接口未提供最新草稿正文，为避免覆盖历史修改，页面不开放继续编辑。')}
              </Paragraph>
            ) : null}
          </SectionCard>
        ) : null}

        {detail ? (
          <Tabs
            activeKey={activeTab}
            onChange={(tab) => setUrlState({ tab: tab === 'drafts' ? undefined : tab }, { replace: true })}
            items={[
              {
                key: 'drafts',
                label: tr("草稿版本"),
                children: (
                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    {resourceError(resourceErrors.drafts, drafts.length > 0)}
                    <SectionCard
                      title={tr("最新草稿")}
                      description={
                          isProductionDraft
                            ? tr('冻结快照已绑定当前草稿版本和哈希；执行只会创建抖店平台草稿，不会发布或上架。')
                            : isProductionBuild
                          ? tr('仅表示 TradeMind 内的本地草稿，不代表已发布商品。')
                          : tr('仅表示本地、模拟或沙箱草稿，不代表已发布商品。')
                      }
                    >
                      {currentDraft ? (
                        <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
                          <Descriptions.Item label={tr("版本")}>v{currentDraft.draftVersion}</Descriptions.Item>
                          <Descriptions.Item label={tr("状态")}><OperationDraftStatusTag status={currentDraft.status} /></Descriptions.Item>
                          <Descriptions.Item label={tr("执行方式")}>{adapterModeLabel(currentDraft.adapterMode)}</Descriptions.Item>
                          <Descriptions.Item label="Payload Hash">{copyableText(currentDraft.payloadHash, 18)}</Descriptions.Item>
                          <Descriptions.Item label={tr("变更原因")}>{currentDraft.changeReason || '—'}</Descriptions.Item>
                          <Descriptions.Item label={tr("创建人")}>{copyableText(currentDraft.createdBy, 12)}</Descriptions.Item>
                          <Descriptions.Item label={tr("更新时间")}>{formatDateTime(currentDraft.updatedAt)}</Descriptions.Item>
                        </Descriptions>
                      ) : <EmptyState compact title={tr("暂无草稿版本")} description={tr("创建首版草稿后，版本摘要会显示在这里。")} />}
                    </SectionCard>
                    {isProductionDraft && frozenDraft ? (
                      <SectionCard title={tr("审核冻结快照")} description={tr("以下内容在任务创建时冻结，审核和执行期间不可编辑。")}>
                        <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
                          <Descriptions.Item label={tr("商品 ID")}>{copyableText(frozenDraft.productId, 16)}</Descriptions.Item>
                          <Descriptions.Item label={tr("店铺 ID")}>{copyableText(frozenDraft.shopId, 16)}</Descriptions.Item>
                          <Descriptions.Item label={tr("刊登方式")}>{tr("保存为平台草稿")}</Descriptions.Item>
                          <Descriptions.Item label={tr("刊登标题")}>{frozenDraft.mappingSnapshot?.title || '—'}</Descriptions.Item>
                          <Descriptions.Item label={tr("平台类目")}>{frozenDraft.mappingSnapshot?.categoryPath || frozenDraft.mappingSnapshot?.categoryId || '—'}</Descriptions.Item>
                          <Descriptions.Item label={tr("SKU 数量")}>{frozenDraft.skuCount ?? frozenDraft.mappingSnapshot?.skus?.length ?? 0}</Descriptions.Item>
                          <Descriptions.Item label={tr("价格区间")}>
                            {frozenDraft.mappingSnapshot?.price?.min != null
                              ? `${frozenDraft.mappingSnapshot.price.currency || 'CNY'} ${frozenDraft.mappingSnapshot.price.min}${frozenDraft.mappingSnapshot.price.max != null && frozenDraft.mappingSnapshot.price.max !== frozenDraft.mappingSnapshot.price.min ? ` - ${frozenDraft.mappingSnapshot.price.max}` : ''}`
                              : '—'}
                          </Descriptions.Item>
                          <Descriptions.Item label={tr("总库存")}>{frozenDraft.mappingSnapshot?.stock?.total ?? '—'}</Descriptions.Item>
                          <Descriptions.Item label={tr("映射 Hash")}>{copyableText(frozenDraft.mappingHash, 18)}</Descriptions.Item>
                          <Descriptions.Item label={tr("刊登描述")} span={3}>
                            <Paragraph className="operation-task-detail__long-text">{frozenDraft.mappingSnapshot?.description || '—'}</Paragraph>
                          </Descriptions.Item>
                        </Descriptions>
                        {(frozenDraft.mappingSnapshot?.mainImages?.length || 0) > 0 ? (
                          <div className="operation-task-detail__images">
                            {frozenDraft.mappingSnapshot?.mainImages?.map((item, index) => imageURL(item) ? (
                              <img key={`${imageURL(item)}-${index}`} src={imageURL(item)} alt={trFormat("主图 {value0}", { value0: String(index + 1) })} />
                            ) : null)}
                          </div>
                        ) : null}
                        <Table
                          rowKey={frozenSkuRowKey}
                          size="small"
                          pagination={false}
                          dataSource={frozenDraft.mappingSnapshot?.skus || []}
                          columns={[
                            { title: tr("销售规格"), dataIndex: 'name', render: (value) => value || tr('默认规格') },
                            { title: tr("规格"), dataIndex: 'attrs', render: (value) => value ? Object.entries(value).map(([key, raw]) => `${key}: ${String(raw)}`).join(' / ') : '—' },
                            { title: tr("价格"), dataIndex: 'price', width: 120, render: (value) => value ?? '—' },
                            { title: tr("库存"), dataIndex: 'stock', width: 100, render: (value) => value ?? '—' },
                          ]}
                          locale={{ emptyText: tr('暂无 SKU 快照') }}
                          scroll={{ x: 640 }}
                        />
                      </SectionCard>
                    ) : null}
                    <Table rowKey="draftId" columns={draftColumns} dataSource={drafts} pagination={false} loading={auxiliaryLoading} scroll={{ x: 900 }} locale={{ emptyText: tr('暂无草稿版本') }} />
                    {!isProductionDraft ? (
                      <SectionCard title={tr("任务载荷预览")} description={tr("这里展示任务原始载荷；草稿版本列表仅提供摘要。")}>
                        {jsonPreview(detail.payload)}
                      </SectionCard>
                    ) : null}
                  </Space>
                ),
              },
              {
                key: 'attempts',
                label: tr("执行历史"),
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    {resourceError(resourceErrors.attempts, attempts.length > 0)}
                    <Table rowKey="attemptId" columns={attemptColumns} dataSource={attempts} pagination={false} loading={auxiliaryLoading} scroll={{ x: 1280 }} locale={{ emptyText: <EmptyState compact title={tr("暂无执行记录")} description={isProductionDraft ? tr('提交创建平台草稿后，执行结果会显示在这里。') : tr('提交草稿生成后，执行结果会显示在这里。')} /> }} />
                    {attemptHasMore ? <Button loading={attemptMoreLoading} disabled={attemptMoreLoading} onClick={() => void loadMoreAttempts()}>{tr("加载更多执行记录")}</Button> : null}
                  </Space>
                ),
              },
              {
                key: 'events',
                label: tr("审计时间线"),
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    {resourceError(resourceErrors.events, events.length > 0)}
                    {events.length > 0 ? <Timeline
                      items={events.map((event) => ({
                        key: event.eventId,
                        children: (
                          <Space direction="vertical" size={4} style={{ width: '100%' }}>
                            <Space wrap>
                              <Text strong>#{event.sequence}</Text>
                              <Tag>{eventTypeLabel(event.eventType)}</Tag>
                              <Text type="secondary">{formatDateTime(event.occurredAt)}</Text>
                            </Space>
                            <Text>{tr("状态：")}<OperationTaskStatusTag status={event.beforeState} /> → <OperationTaskStatusTag status={event.afterState} /></Text>
                            <Text>{tr("操作者：")}{actorTypeLabel(event.actorType)}{event.actorId ? <> / {copyableText(event.actorId, 12)}</> : null}</Text>
                            <Text>{tr("草稿版本：")}{event.draftVersion || '—'}{tr("；请求编号：")}{copyableText(event.requestId, 14)}</Text>
                            {event.reason ? <Text className="operation-task-detail__long-text">{tr("原因：")}{event.reason}</Text> : null}
                            <TaskJsonBlock title={tr("安全元数据")} value={safeMetadata(event.metadata)} maxHeight={160} last />
                          </Space>
                        ),
                      }))}
                    /> : <EmptyState compact title={tr("暂无审计事件")} description={tr("任务状态变化和人工操作会记录在这里。")} />}
                    {eventsHasMore ? <Button loading={eventMoreLoading} disabled={eventMoreLoading} onClick={() => void loadMoreEvents()}>{tr("加载更多事件")}</Button> : null}
                  </Space>
                ),
              },
            ]}
          />
        ) : null}
      </Space>

      <Modal
        title={modalTitle}
        open={!!modal}
        onCancel={closeModal}
        onOk={() => void runAction()}
        confirmLoading={actionLoading}
        okText={modalOkText}
        okButtonProps={{ danger: modal === 'reject' || modal === 'cancel', disabled: actionLoading }}
        cancelText={tr("取消")}
        width={760}
        destroyOnHidden
      >
        {detail ? <Paragraph type="secondary">{tr("当前任务：")}{detail.title}（{detail.id}）</Paragraph> : null}
        {modal === 'draft' ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Paragraph>{tr("创建后会生成首版草稿并进入人工审核，不会自动执行或发布。")}</Paragraph>
            <Input.TextArea value={jsonText} onChange={(e) => { setJsonText(e.target.value); setJsonError(null); }} rows={12} aria-label={tr("草稿 JSON")} aria-invalid={!!jsonError} aria-describedby={jsonError ? 'operation-draft-json-error' : undefined} />
            {jsonError ? <Text id="operation-draft-json-error" type="danger">{tr("JSON 格式错误：")}{jsonError}</Text> : null}
            <Form form={form} layout="vertical">
              <Form.Item name="changeReason" label={tr("变更原因")} rules={requiredReasonRule(tr("变更原因"))}>
                <Input.TextArea rows={3} maxLength={500} showCount />
              </Form.Item>
            </Form>
            <TaskJsonBlock title={tr("安全差异预览（仅展示变化行，最多 200 行）")} value={diffJSON(detail?.payload, parseJSONInput(jsonText).ok ? parseJSONInput(jsonText).value : {})} maxHeight={220} />
          </Space>
        ) : null}

        {modal === 'approve' || modal === 'reject' ? (
          <Form form={form} layout="vertical">
            <Paragraph>{tr("当前操作绑定最新草稿 v")}{currentDraft?.draftVersion || '—'} / {currentDraft?.payloadHash || '—'}。{isProductionDraft ? tr("批准后允许在运行控制就绪时人工创建平台草稿，但不会发布或上架。") : tr("不会自动发布或上架商品。")}</Paragraph>
            <Form.Item name="reason" label={modal === 'approve' ? tr('批准说明') : tr('拒绝原因')} rules={requiredReasonRule(modal === 'approve' ? tr('批准说明') : tr('拒绝原因'))}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
            <Form.Item name="comment" label={tr("补充说明")}>
              <Input.TextArea rows={2} maxLength={500} showCount />
            </Form.Item>
          </Form>
        ) : null}

        {modal === 'execute' ? (
          <Form form={form} layout="vertical">
            {isProductionDraft ? (
              <ProductionRuntimeBoundary status={productionStatus} loading={productionStatusLoading} error={productionStatusError} />
            ) : !isProductionBuild ? <NonProductionBoundary /> : null}
            <Paragraph style={{ marginTop: 12 }}>
              {isProductionDraft
                ? tr('将把人工批准的冻结快照提交到白名单抖店，仅保存为平台草稿。平台发布、上架和自动重试均不会执行。')
                : isProductionBuild
                ? tr('执行仅生成本地草稿，不调用真实平台发布或上架。')
                : tr('执行仅生成本地、模拟或沙箱草稿，不调用真实平台发布或上架。')}
            </Paragraph>
            <Form.Item name="adapterMode" label={tr("执行方式")} rules={[{ required: true, message: tr("请选择执行方式") }]}>
              {isProductionDraft ? (
                <Select disabled options={[{ value: 'production_draft', label: tr("创建平台草稿") }]} />
              ) : <Select
                options={[
                  { value: 'local_draft_only', label: adapterModeLabel('local_draft_only') },
                  ...(!isProductionBuild
                    ? [
                        { value: 'mock', label: adapterModeLabel('mock') },
                        { value: 'sandbox', label: adapterModeLabel('sandbox') },
                      ]
                    : []),
                ]}
              />}
            </Form.Item>
            {isProductionDraft && productionDraftBlockReason(productionStatus) ? <Text type="danger">{productionDraftBlockReason(productionStatus)}</Text> : null}
          </Form>
        ) : null}

        {modal === 'retry' ? (
          <Form form={form} layout="vertical">
            <Paragraph>{tr("仅对明确失败的执行记录发起一次人工重试，不会自动连续重试。")}</Paragraph>
            <Form.Item name="failedAttemptId" label={tr("失败执行记录")} rules={[{ required: true, message: tr("请选择失败执行记录") }]}>
              <Select options={failedAttempts.map((attempt) => ({ value: attempt.attemptId, label: trFormat("第 {value0} 次 / 失败", { value0: attempt.attemptNumber }) }))} />
            </Form.Item>
            <Form.Item name="reason" label={tr("重试原因")} rules={requiredReasonRule(tr("重试原因"))}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          </Form>
        ) : null}

        {modal === 'cancel' ? (
          <Form form={form} layout="vertical">
            <Paragraph>{tr("取消后该任务不能继续执行；已完成或已取消的任务由后端拒绝重复取消。")}</Paragraph>
            <Form.Item name="reason" label={tr("取消原因")} rules={requiredReasonRule(tr("取消原因"))}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          </Form>
        ) : null}
      </Modal>
    </TmPageContainer>
  );
}
