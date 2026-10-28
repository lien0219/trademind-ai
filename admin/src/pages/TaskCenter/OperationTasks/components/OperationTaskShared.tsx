import { tr, trFormat } from '@/i18n';
import { Alert, Descriptions, Tag, Tooltip, Typography } from 'antd';
import { TaskJsonBlock } from '@/components/ui';
import {
  NON_PRODUCTION_BOUNDARY_COPY,
  OPERATION_ACTOR_TYPE_LABELS,
  OPERATION_ADAPTER_MODE_LABELS,
  OPERATION_ATTEMPT_STATUS_LABELS,
  OPERATION_DRAFT_STATUS_LABELS,
  OPERATION_ERROR_LABELS,
  OPERATION_EVENT_TYPE_LABELS,
  OPERATION_METADATA_ALLOWLIST,
  OPERATION_PLATFORM_LABELS,
  OPERATION_PRIORITY_LABELS,
  OPERATION_RESULT_TYPE_LABELS,
  OPERATION_SOURCE_TYPE_LABELS,
  OPERATION_TASK_STATUS_LABELS,
  OPERATION_TASK_TYPE_LABELS,
  operationLabel,
  operationLabelColor,
  type LocalizedLabel,
} from '@/constants/operationTasks';
import type { OperationTaskAPIError } from '@/services/operationTasks';
import type { ProductionRuntimeStatus } from '@/services/productionControl';
import { productionDraftBlockReason } from '@/services/productionControl';

const SENSITIVE_KEY = /(token|secret|credential|password|cookie|authorization|oauth|accesskey|refresh)/i;

export function labelFromRecord(map: Record<string, string>, value?: string | null) {
  const key = (value ?? '').trim();
  return key ? tr(map[key] ?? key) : '—';
}

function statusTag(map: Record<string, LocalizedLabel>, status?: string | null) {
  const label = tr(operationLabel(map, status));
  const color = operationLabelColor(map, status);
  const description = status && map[status]?.descriptionSource ? tr(map[status].descriptionSource) : undefined;
  const tag = <Tag color={color as never}>{label}</Tag>;
  return description ? <Tooltip title={description}>{tag}</Tooltip> : tag;
}

export function OperationTaskStatusTag({ status }: { status?: string | null }) {
  return statusTag(OPERATION_TASK_STATUS_LABELS, status);
}

export function OperationDraftStatusTag({ status }: { status?: string | null }) {
  return statusTag(OPERATION_DRAFT_STATUS_LABELS, status);
}

export function OperationAttemptStatusTag({ status }: { status?: string | null }) {
  return statusTag(OPERATION_ATTEMPT_STATUS_LABELS, status);
}

export function OperationPriorityTag({ priority }: { priority?: string | null }) {
  return statusTag(OPERATION_PRIORITY_LABELS, priority);
}

export function taskTypeLabel(value?: string | null) {
  return labelFromRecord(OPERATION_TASK_TYPE_LABELS, value);
}

export function operationSourceLabel(value?: string | null) {
  return labelFromRecord(OPERATION_SOURCE_TYPE_LABELS, value);
}

export function platformLabel(value?: string | null) {
  return labelFromRecord(OPERATION_PLATFORM_LABELS, value);
}

export function adapterModeLabel(value?: string | null) {
  return labelFromRecord(OPERATION_ADAPTER_MODE_LABELS, value);
}

export function resultTypeLabel(value?: string | null) {
  return labelFromRecord(OPERATION_RESULT_TYPE_LABELS, value);
}

export function eventTypeLabel(value?: string | null) {
  const key = (value ?? '').trim();
  if (!key) return '—';
  return OPERATION_EVENT_TYPE_LABELS[key]?.source ? tr(OPERATION_EVENT_TYPE_LABELS[key].source) : trFormat('未知事件（{key}）', { key });
}

export function actorTypeLabel(value?: string | null) {
  return labelFromRecord(OPERATION_ACTOR_TYPE_LABELS, value);
}

const OPERATION_TASK_DETAIL_TABS = new Set(['drafts', 'attempts', 'events']);

export function normalizeOperationTaskTab(value?: string | null) {
  const key = (value ?? '').trim();
  return OPERATION_TASK_DETAIL_TABS.has(key) ? key : 'drafts';
}

export function operationErrorMessage(error?: OperationTaskAPIError | null) {
  if (!error) return undefined;
  if (error.errorCode && OPERATION_ERROR_LABELS[error.errorCode]) return tr(OPERATION_ERROR_LABELS[error.errorCode]);
  return error.message || tr('操作失败，请稍后重试。');
}

export function renderOperationError(error?: OperationTaskAPIError | null) {
  if (!error) return null;
  const detail = [error.errorCode ? trFormat('错误码：{code}', { code: error.errorCode }) : undefined, error.traceId ? trFormat('排查编号：{traceId}', { traceId: error.traceId }) : undefined]
    .filter(Boolean)
    .join('；');
  return <Alert type="error" showIcon message={operationErrorMessage(error)} description={detail || undefined} />;
}

export function redactSensitiveValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map((item) => redactSensitiveValue(item));
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).map(([key, raw]) => [
      key,
      SENSITIVE_KEY.test(key) ? '******' : redactSensitiveValue(raw),
    ]),
  );
}

export function safeMetadata(metadata: unknown) {
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return {};
  return Object.fromEntries(
    Object.entries(metadata as Record<string, unknown>)
      .filter(([key]) => OPERATION_METADATA_ALLOWLIST.has(key))
      .map(([key, raw]) => [key, redactSensitiveValue(raw)]),
  );
}

export function jsonPreview(value: unknown) {
  return <TaskJsonBlock title={tr("安全 JSON")} value={redactSensitiveValue(value)} maxHeight={320} />;
}

export function NonProductionBoundary() {
  return (
    <Alert
      type="warning"
      showIcon
      message={tr(NON_PRODUCTION_BOUNDARY_COPY.titleSource)}
      description={
        <div>
          <Typography.Paragraph style={{ marginBottom: 8 }}>{tr(NON_PRODUCTION_BOUNDARY_COPY.descriptionSource)}</Typography.Paragraph>
          <Descriptions size="small" column={{ xs: 1, sm: 3 }}>
            <Descriptions.Item label={tr("真实平台写入")}>{tr("未启用")}</Descriptions.Item>
            <Descriptions.Item label={tr("自动发布")}>{tr("未启用")}</Descriptions.Item>
            <Descriptions.Item label={tr("自动上架")}>{tr("未启用")}</Descriptions.Item>
          </Descriptions>
        </div>
      }
    />
  );
}

export function ProductionRuntimeBoundary({
  status,
  loading = false,
  error,
}: {
  status?: ProductionRuntimeStatus | null;
  loading?: boolean;
  error?: string;
}) {
  const blockedReason = productionDraftBlockReason(status);
  const ready = !!status?.productionReady && !error;
  return (
    <Alert
      type={ready ? 'success' : 'warning'}
      showIcon
      message={ready ? tr('平台草稿写入已受控启用') : tr('平台草稿写入未就绪')}
      description={(
        <div>
          <Typography.Paragraph style={{ marginBottom: 8 }}>
            {loading
              ? tr('正在核对实时运行控制，核对完成前禁止真实平台写入。')
              : error || blockedReason || tr('仅允许对人工审核冻结快照执行创建平台草稿，不会发布或上架。')}
          </Typography.Paragraph>
          <Descriptions size="small" column={{ xs: 1, sm: 3 }}>
            <Descriptions.Item label={tr("能力级别")}>{status?.currentAllowedLevel || tr("未确认")}</Descriptions.Item>
            <Descriptions.Item label={tr("允许店铺")}>{status?.allowlist?.enabled ? copyableText(status.allowlist.shopId, 12) : tr("无")}</Descriptions.Item>
            <Descriptions.Item label={tr("执行边界")}>{tr("仅创建平台草稿")}</Descriptions.Item>
          </Descriptions>
        </div>
      )}
    />
  );
}

export function copyableText(value?: string | null, max = 14) {
  const text = (value ?? '').trim();
  if (!text) return '—';
  const short = text.length > max ? `${text.slice(0, max)}…` : text;
  return <Typography.Text copyable={{ text }}>{short}</Typography.Text>;
}

export function parseJSONInput(raw: string) {
  try {
    return { ok: true as const, value: JSON.parse(raw) };
  } catch (error) {
    return { ok: false as const, message: error instanceof Error ? error.message : tr('JSON 格式不正确') };
  }
}

export function diffJSON(before: unknown, after: unknown) {
  const beforeText = JSON.stringify(redactSensitiveValue(before), null, 2).split('\n');
  const afterText = JSON.stringify(redactSensitiveValue(after), null, 2).split('\n');
  const size = Math.max(beforeText.length, afterText.length);
  const rows: { line: number; before?: string; after?: string; changed: boolean }[] = [];
  for (let i = 0; i < size; i += 1) {
    const b = beforeText[i];
    const a = afterText[i];
    rows.push({ line: i + 1, before: b, after: a, changed: b !== a });
  }
  return rows.filter((row) => row.changed).slice(0, 200);
}
