import { tr, trFormat } from '@/i18n';
import { ArrowLeftOutlined, HistoryOutlined, ReloadOutlined, RetweetOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { Button, Card, Descriptions, Modal, Space, Spin, Statistic, Table, Tabs, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { SectionCard, TmPageContainer, TmProTable } from '@/components/ui';
import {
  createInventorySyncIdempotencyKey,
  extractInventorySyncAPIError,
  getInventorySyncRun,
  listInventoryRunAuditEvents,
  listInventorySnapshots,
  rerunInventorySyncRun,
  type AuditEvent,
  type InventorySnapshot,
  type InventorySyncAPIError,
  type InventorySyncRun,
} from '@/services/inventorySync';
import { formatDateTime } from '@/utils/formatTime';
import {
  ActionSourceHint,
  AuditMetadataBlock,
  BINDING_RESULT_LABELS,
  ConfidenceText,
  CursorPager,
  RUN_STATUS_LABELS,
  StatusTag,
  copyableText,
  errorMessage,
  renderInventorySyncError,
} from './components/InventorySyncShared';

const { Text } = Typography;
const LIMIT = 20;

export default function InventorySyncRunDetailPage() {
  const params = useParams<{ runId: string }>();
  const runId = params.runId || '';
  const [run, setRun] = useState<InventorySyncRun | null>(null);
  const [snapshots, setSnapshots] = useState<InventorySnapshot[]>([]);
  const [auditEvents, setAuditEvents] = useState<AuditEvent[]>([]);
  const [snapshotCursor, setSnapshotCursor] = useState<string | undefined>();
  const [snapshotHasMore, setSnapshotHasMore] = useState(false);
  const [auditCursor, setAuditCursor] = useState<string | undefined>();
  const [auditHasMore, setAuditHasMore] = useState(false);
  const [snapshotStack, setSnapshotStack] = useState<string[]>([]);
  const [auditStack, setAuditStack] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<InventorySyncAPIError | null>(null);
  const requestSeq = useRef(0);

  const load = useCallback(async (nextSnapshotCursor?: string, nextAuditCursor?: string) => {
    if (!runId) return;
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const detail = await getInventorySyncRun(runId);
      const [snapshotPage, auditPage] = await Promise.all([
        detail.allowedActions.canViewSnapshots
          ? listInventorySnapshots(runId, { limit: LIMIT, cursor: nextSnapshotCursor })
          : Promise.resolve({ items: [], nextCursor: undefined, hasMore: false, limit: LIMIT }),
        detail.allowedActions.canViewAudit
          ? listInventoryRunAuditEvents(runId, { limit: LIMIT, cursor: nextAuditCursor })
          : Promise.resolve({ items: [], nextCursor: undefined, hasMore: false, limit: LIMIT }),
      ]);
      if (requestSeq.current !== seq) return;
      setRun(detail);
      setSnapshots(snapshotPage.items || []);
      setSnapshotCursor(snapshotPage.nextCursor);
      setSnapshotHasMore(snapshotPage.hasMore);
      setAuditEvents(auditPage.items || []);
      setAuditCursor(auditPage.nextCursor);
      setAuditHasMore(auditPage.hasMore);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractInventorySyncAPIError(e));
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [runId]);

  useEffect(() => {
    void load();
  }, [load]);

  const runRerun = async () => {
    if (!run) return;
    setActionLoading(true);
    setError(null);
    try {
      const next = await rerunInventorySyncRun(run.id, { expectedRevision: run.revision }, createInventorySyncIdempotencyKey('rerun'));
      message.success(tr("已提交夹具重跑请求"));
      history.push(`/ops/inventory-sync/runs/${encodeURIComponent(next.id)}`);
    } catch (e) {
      const next = extractInventorySyncAPIError(e);
      setError(next);
      message.error(errorMessage(next));
      void load();
    } finally {
      setActionLoading(false);
    }
  };

  const snapshotColumns = useMemo<ProColumns<InventorySnapshot>[]>(() => [
    { title: tr("快照 ID"), dataIndex: 'id', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("平台商品"), dataIndex: 'externalProductId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("平台规格"), dataIndex: 'externalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("商品标题"), dataIndex: 'productTitle', width: 220, ellipsis: true },
    { title: tr("规格"), dataIndex: 'variantTitle', width: 160, ellipsis: true },
    { title: tr("可用"), dataIndex: 'availableQuantity', width: 80 },
    { title: tr("预留"), dataIndex: 'reservedQuantity', width: 80 },
    { title: tr("总量"), dataIndex: 'totalQuantity', width: 80 },
    { title: tr("绑定结果"), dataIndex: ['binding', 'result'], width: 130, render: (v) => <StatusTag map={BINDING_RESULT_LABELS} value={String(v || '')} /> },
    { title: tr("置信度"), dataIndex: ['binding', 'confidence'], width: 100, render: (v) => <ConfidenceText value={Number(v)} /> },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 150,
      render: (_, row) => [
        row.binding?.manualRequestId ? (
          <Button key="manual" type="link" onClick={() => history.push(`/ops/inventory-sync/manual-bindings?requestId=${encodeURIComponent(row.binding.manualRequestId || '')}`)}>
            {tr("人工处理")}
          </Button>
        ) : null,
        row.binding?.bindingId ? (
          <Button key="binding" type="link" onClick={() => history.push(`/ops/inventory-sync/bindings/${encodeURIComponent(row.binding.bindingId || '')}`)}>
            {tr("绑定历史")}
          </Button>
        ) : null,
      ].filter(Boolean),
    },
  ], []);

  const auditColumns: ColumnsType<AuditEvent> = [
    { title: tr("时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: tr("动作"), dataIndex: 'action', width: 180 },
    { title: tr("资源"), dataIndex: 'resource', width: 120 },
    { title: tr("状态"), dataIndex: 'status', width: 110 },
    { title: 'Request ID', dataIndex: 'requestId', width: 150, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("操作者"), dataIndex: 'actorId', width: 150, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("安全摘要"), dataIndex: 'metadata', render: (v) => <AuditMetadataBlock metadata={v} /> },
  ];

  if (loading && !run) return <Spin fullscreen tip={tr("正在加载库存同步运行")} />;

  return (
    <TmPageContainer
      title={tr("库存同步运行详情")}
      subTitle={run ? `Run ${run.id}` : runId}
      extra={
        <Space wrap>
          <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/ops/inventory-sync')}>{tr("返回列表")}</Button>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>{tr("刷新")}</Button>
          <Button
            type="primary"
            icon={<RetweetOutlined />}
            disabled={!run?.allowedActions.canRerun}
            loading={actionLoading}
            onClick={() => {
              Modal.confirm({
                title: tr("确认重跑夹具同步？"),
                content: trFormat("将以 expectedRevision={value0} 发起一次受控重跑，不会连接真实平台。", { value0: String(run?.revision || 0) }),
                okText: tr("重跑"),
                cancelText: tr("取消"),
                onOk: runRerun,
              });
            }}
          >
            {tr("重跑")}
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderInventorySyncError(error)}
        {run ? (
          <SectionCard title={tr("运行概览")} description={tr("成功状态表示 fixture/test 完成，不代表真实库存同步完成。")} headerExtra={<StatusTag map={RUN_STATUS_LABELS} value={run.status} />}>
            <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
              <Descriptions.Item label={tr("运行 ID")}>{copyableText(run.id, 18)}</Descriptions.Item>
              <Descriptions.Item label={tr("店铺连接")}>{copyableText(run.shopConnectionId, 14)}</Descriptions.Item>
              <Descriptions.Item label={tr("平台")}>{run.platform}</Descriptions.Item>
              <Descriptions.Item label={tr("接入模式")}>{run.providerMode}</Descriptions.Item>
              <Descriptions.Item label={tr("夹具场景")}>{run.fixtureScenario || '-'}</Descriptions.Item>
              <Descriptions.Item label={tr("触发方式")}>{run.triggerType}</Descriptions.Item>
              <Descriptions.Item label="Revision">{run.revision}</Descriptions.Item>
              <Descriptions.Item label="Cursor Hash">{copyableText(run.cursorHash, 16)}</Descriptions.Item>
              <Descriptions.Item label={tr("创建时间")}>{formatDateTime(run.createdAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("开始时间")}>{formatDateTime(run.startedAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("完成时间")}>{formatDateTime(run.finishedAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("安全错误")}>{run.safeError?.message || run.safeError?.code || '-'}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
        ) : null}
        {run ? (
          <Card>
            <Space wrap size={24}>
              <Statistic title={tr("夹具记录")} value={run.statistics.totalRecordCount} />
              <Statistic title={tr("已匹配")} value={run.statistics.matchedRecordCount} />
              <Statistic title={tr("未匹配")} value={run.statistics.unmatchedRecordCount} />
              <Statistic title={tr("冲突")} value={run.statistics.conflictRecordCount} />
              <Statistic title={tr("待人工")} value={run.statistics.manualBindingRequestCount} />
              <Statistic title={tr("确认绑定")} value={run.statistics.confirmedBindingCount} />
            </Space>
          </Card>
        ) : null}
        <ActionSourceHint />
        <Tabs
          items={[
            {
              key: 'snapshots',
              label: tr("快照"),
              children: run?.allowedActions.canViewSnapshots ? (
                <Space direction="vertical" style={{ width: '100%' }} size={12}>
                  <TmProTable<InventorySnapshot>
                    rowKey="id"
                    search={false}
                    columns={snapshotColumns}
                    dataSource={snapshots}
                    loading={loading}
                    pagination={false}
                    scroll={{ x: 1500 }}
                    options={false}
                    toolBarRender={false}
                  />
                  <Card size="small">
                    <CursorPager
                      count={snapshots.length}
                      hasMore={snapshotHasMore && !!snapshotCursor}
                      canPrev={snapshotStack.length > 0}
                      loading={loading}
                      onNext={() => {
                        if (!snapshotCursor) return;
                        setSnapshotStack((prev) => [...prev, '']);
                        void load(snapshotCursor, undefined);
                      }}
                      onPrev={() => {
                        setSnapshotStack((prev) => {
                          const next = [...prev];
                          next.pop();
                          void load(undefined, undefined);
                          return next;
                        });
                      }}
                    />
                  </Card>
                </Space>
              ) : <Text type="secondary">{tr("后端未开放查看快照操作。")}</Text>,
            },
            {
              key: 'audit',
              label: <Space><HistoryOutlined />{tr("审计")}</Space>,
              children: run?.allowedActions.canViewAudit ? (
                <Space direction="vertical" style={{ width: '100%' }} size={12}>
                  <Table rowKey="id" columns={auditColumns} dataSource={auditEvents} pagination={false} scroll={{ x: 1260 }} />
                  <Card size="small">
                    <CursorPager
                      count={auditEvents.length}
                      hasMore={auditHasMore && !!auditCursor}
                      canPrev={auditStack.length > 0}
                      loading={loading}
                      onNext={() => {
                        if (!auditCursor) return;
                        setAuditStack((prev) => [...prev, '']);
                        void load(undefined, auditCursor);
                      }}
                      onPrev={() => {
                        setAuditStack((prev) => {
                          const next = [...prev];
                          next.pop();
                          void load(undefined, undefined);
                          return next;
                        });
                      }}
                    />
                  </Card>
                </Space>
              ) : <Text type="secondary">{tr("后端未开放查看审计操作。")}</Text>,
            },
          ]}
        />
      </Space>
    </TmPageContainer>
  );
}
