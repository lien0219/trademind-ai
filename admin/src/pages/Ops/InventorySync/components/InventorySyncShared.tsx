import { tr, trFormat } from '@/i18n';
import { Alert, Button, Space, Tag, Tooltip, Typography } from 'antd';
import { TaskJsonBlock } from '@/components/ui';
import type { InventorySyncAPIError } from '@/services/inventorySync';

const { Text } = Typography;

const SENSITIVE_KEY = /(token|secret|credential|password|cookie|authorization|oauth|accesskey|refresh|metadata)/i;

export const RUN_STATUS_LABELS: Record<string, { text: string; color: string }> = {
  queued: { get text() { return tr("等待执行"); }, color: 'default' },
  running: { get text() { return tr("夹具执行中"); }, color: 'processing' },
  succeeded: { get text() { return tr("夹具完成"); }, color: 'success' },
  failed: { get text() { return tr("夹具失败"); }, color: 'error' },
  cancelled: { get text() { return tr("已取消"); }, color: 'default' },
};

export const BINDING_RESULT_LABELS: Record<string, { text: string; color: string }> = {
  matched: { get text() { return tr("已匹配"); }, color: 'success' },
  manual_review: { get text() { return tr("待人工确认"); }, color: 'warning' },
  unmatched: { get text() { return tr("未匹配"); }, color: 'default' },
};

export const BINDING_STATUS_LABELS: Record<string, { text: string; color: string }> = {
  confirmed: { get text() { return tr("已确认"); }, color: 'success' },
  pending: { get text() { return tr("待确认"); }, color: 'warning' },
  rejected: { get text() { return tr("已拒绝"); }, color: 'error' },
  superseded: { get text() { return tr("已被取代"); }, color: 'default' },
};

export const MANUAL_REQUEST_STATUS_LABELS: Record<string, { text: string; color: string }> = {
  pending: { get text() { return tr("待处理"); }, color: 'warning' },
  confirmed: { get text() { return tr("已确认"); }, color: 'success' },
  rejected: { get text() { return tr("已拒绝"); }, color: 'error' },
  cancelled: { get text() { return tr("已取消"); }, color: 'default' },
};

export function labelOf(map: Record<string, { text: string }>, value?: string | null) {
  const key = String(value || '').trim();
  if (!key) return '-';
  return map[key]?.text || key;
}

export function StatusTag({ map, value }: { map: Record<string, { text: string; color: string }>; value?: string | null }) {
  const key = String(value || '').trim();
  const item = key ? map[key] : undefined;
  return <Tag color={item?.color || 'default'}>{item?.text || key || '-'}</Tag>;
}

export function copyableText(value?: string | null, max = 14) {
  const text = String(value || '').trim();
  if (!text) return '-';
  const short = text.length > max ? `${text.slice(0, max)}...` : text;
  return <Typography.Text copyable={{ text }}>{short}</Typography.Text>;
}

export function errorMessage(error?: InventorySyncAPIError | null) {
  if (!error) return undefined;
  return error.errorCode ? `${error.message} (${error.errorCode})` : error.message;
}

export function renderInventorySyncError(error?: InventorySyncAPIError | null) {
  if (!error) return null;
  return (
    <Alert
      type="error"
      showIcon
      message={errorMessage(error)}
      description={error.traceId ? trFormat("排查编号：{value0}", { value0: String(error.traceId) }) : undefined}
    />
  );
}

export function safeAuditSummary(metadata: unknown) {
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return {};
  const allowlist = new Set([
    'runId',
    'snapshotId',
    'bindingId',
    'manualRequestId',
    'expectedRevision',
    'expectedCalibrationVersion',
    'reasonCode',
    'providerMode',
    'fixtureScenario',
  ]);
  return Object.fromEntries(
    Object.entries(metadata as Record<string, unknown>)
      .filter(([key]) => allowlist.has(key) && !SENSITIVE_KEY.test(key))
      .map(([key, raw]) => [key, redactSensitiveValue(raw)]),
  );
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

export function AuditMetadataBlock({ metadata }: { metadata: unknown }) {
  return <TaskJsonBlock title={tr("安全摘要")} value={safeAuditSummary(metadata)} maxHeight={160} last />;
}

export function CursorPager({
  count,
  hasMore,
  canPrev,
  loading,
  onPrev,
  onNext,
}: {
  count: number;
  hasMore: boolean;
  canPrev: boolean;
  loading?: boolean;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <Space wrap>
      <Button disabled={!canPrev || loading} onClick={onPrev}>{tr("上一批")}</Button>
      <Button type="primary" disabled={!hasMore || loading} onClick={onNext}>{tr("下一批")}</Button>
      <Text type="secondary">{tr("当前显示")} {count} {tr("条；列表使用 keyset cursor，不显示页码或总数。")}</Text>
    </Space>
  );
}

export function ActionSourceHint() {
  return (
    <Text type="secondary">
      {tr("可用操作完全来自后端 allowedActions；提交时携带隐藏的 Idempotency-Key 与 expected revision。")}
    </Text>
  );
}

export function ConfidenceText({ value }: { value?: number | null }) {
  if (value === undefined || value === null || Number.isNaN(value)) return <>-</>;
  const normalized = value > 1 ? value / 100 : value * 100;
  return <Tooltip title={`confidence=${value}`}>{Math.round(normalized)}%</Tooltip>;
}
