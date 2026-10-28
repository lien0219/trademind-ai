import { tr } from '@/i18n';
import { ReloadOutlined, SyncOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { Button, Card, Col, Form, Input, Modal, Row, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { SectionCard, TmPageContainer, TmProTable } from '@/components/ui';
import { useUrlQueryState } from '@/hooks/useUrlState';
import {
  createInventorySyncIdempotencyKey,
  extractInventorySyncAPIError,
  getInventorySnapshot,
  listInventorySnapshotCalibrations,
  recalibrateInventorySnapshot,
  type BindingCalibration,
  type InventorySnapshot,
  type InventorySyncAPIError,
} from '@/services/inventorySync';
import { formatDateTime } from '@/utils/formatTime';
import {
  ActionSourceHint,
  BINDING_RESULT_LABELS,
  ConfidenceText,
  CursorPager,
  StatusTag,
  copyableText,
  errorMessage,
  renderInventorySyncError,
} from './components/InventorySyncShared';

const { Text } = Typography;
const QUERY_KEYS = ['snapshotId', 'cursor'] as const;
const LIMIT = 20;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;

export default function InventorySyncCalibrationPage() {
  const { state, setState, clearState } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [reasonForm] = Form.useForm<{ reason: string }>();
  const [snapshot, setSnapshot] = useState<InventorySnapshot | null>(null);
  const [items, setItems] = useState<BindingCalibration[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<InventorySyncAPIError | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [modalOpen, setModalOpen] = useState(false);
  const requestSeq = useRef(0);

  useEffect(() => {
    form.setFieldsValue({ snapshotId: state.snapshotId });
  }, [form, state.snapshotId]);

  const load = useCallback(async () => {
    if (!state.snapshotId) {
      setSnapshot(null);
      setItems([]);
      setNextCursor(undefined);
      setHasMore(false);
      return;
    }
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const [nextSnapshot, page] = await Promise.all([
        getInventorySnapshot(state.snapshotId),
        listInventorySnapshotCalibrations(state.snapshotId, { limit: LIMIT, cursor: state.cursor }),
      ]);
      if (requestSeq.current !== seq) return;
      setSnapshot(nextSnapshot);
      setItems(page.items || []);
      setNextCursor(page.nextCursor);
      setHasMore(page.hasMore);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractInventorySyncAPIError(e));
      setSnapshot(null);
      setItems([]);
      setNextCursor(undefined);
      setHasMore(false);
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [state.cursor, state.snapshotId]);

  useEffect(() => {
    void load();
  }, [load]);

  const expectedCalibrationVersion = useMemo(() => {
    const fromRows = items.reduce((max, item) => Math.max(max, item.calibrationVersion || 0), 0);
    return Math.max(snapshot?.binding?.calibrationVersion || 0, fromRows, 1);
  }, [items, snapshot?.binding?.calibrationVersion]);

  const submitFilter = (values: QueryState) => {
    setCursorStack([]);
    setState({ snapshotId: values.snapshotId, cursor: undefined }, { replace: true });
  };

  const clear = () => {
    setCursorStack([]);
    form.resetFields();
    clearState(QUERY_KEYS, { replace: true });
  };

  const submitRecalibrate = async () => {
    if (!state.snapshotId) return;
    setActionLoading(true);
    setError(null);
    try {
      const values = await reasonForm.validateFields();
      await recalibrateInventorySnapshot(
        state.snapshotId,
        { expectedCalibrationVersion, reason: values.reason },
        createInventorySyncIdempotencyKey('recalibrate'),
      );
      message.success(tr("已提交夹具校准请求"));
      setModalOpen(false);
      reasonForm.resetFields();
      void load();
    } catch (e) {
      const next = extractInventorySyncAPIError(e);
      setError(next);
      message.error(errorMessage(next));
    } finally {
      setActionLoading(false);
    }
  };

  const columns = useMemo<ProColumns<BindingCalibration>[]>(() => [
    { title: tr("校准 ID"), dataIndex: 'id', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("平台规格"), dataIndex: 'externalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("候选商品"), dataIndex: 'candidateLocalProductId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("候选规格"), dataIndex: 'candidateLocalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("策略"), dataIndex: 'matchStrategy', width: 180 },
    { title: tr("置信度"), dataIndex: 'confidence', width: 100, render: (v) => <ConfidenceText value={typeof v === 'number' ? v : undefined} /> },
    { title: tr("版本"), dataIndex: 'calibrationVersion', width: 90 },
    { title: tr("状态"), dataIndex: 'status', width: 120 },
    { title: tr("原因码"), dataIndex: 'reasonCodes', width: 240, render: (v) => Array.isArray(v) ? v.join(', ') : '-' },
    { title: tr("创建时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
  ], []);

  return (
    <TmPageContainer
      title={tr("SKU 校准工作台")}
      subTitle={tr("按快照查看候选校准并发起受控 fixture recalibrate")}
      extra={
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>{tr("刷新")}</Button>
          <Button type="primary" icon={<SyncOutlined />} disabled={!snapshot} onClick={() => setModalOpen(true)}>{tr("重新校准")}</Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderInventorySyncError(error)}
        <Card>
          <Form form={form} layout="vertical" onFinish={submitFilter}>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={14} lg={10}>
                <Form.Item name="snapshotId" label={tr("快照 ID")}>
                  <Input allowClear placeholder={tr("输入 inventory snapshot ID")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={10}>
                <Form.Item label={tr("操作")}>
                  <Space wrap>
                    <Button type="primary" htmlType="submit">{tr("加载校准")}</Button>
                    <Button onClick={clear}>{tr("清除")}</Button>
                  </Space>
                </Form.Item>
              </Col>
            </Row>
          </Form>
          <ActionSourceHint />
        </Card>
        {snapshot ? (
          <SectionCard title={tr("快照摘要")} description={tr("来自 fixture 快照，不代表线上库存读数。")} headerExtra={<StatusTag map={BINDING_RESULT_LABELS} value={snapshot.binding?.result} />}>
            <Space direction="vertical" size={4}>
              <Text>{tr("平台 SKU：")}{copyableText(snapshot.externalSkuId, 18)} / {snapshot.externalSkuCode || '-'}</Text>
              <Text>{tr("本地 SKU：")}{copyableText(snapshot.binding?.localSkuId, 18)}{tr("；当前校准版本：")}{expectedCalibrationVersion}</Text>
              <Text>{tr("商品：")}{snapshot.productTitle || '-'} / {snapshot.variantTitle || '-'}</Text>
            </Space>
          </SectionCard>
        ) : (
          <Card><Text type="secondary">{tr("请输入快照 ID 后加载校准记录。")}</Text></Card>
        )}
        <TmProTable<BindingCalibration>
          rowKey="id"
          search={false}
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={false}
          scroll={{ x: 1480 }}
          options={false}
          toolBarRender={false}
        />
        <Card size="small">
          <CursorPager
            count={items.length}
            hasMore={hasMore && !!nextCursor}
            canPrev={cursorStack.length > 0}
            loading={loading}
            onNext={() => {
              if (!nextCursor) return;
              setCursorStack((prev) => [...prev, state.cursor || '']);
              setState({ cursor: nextCursor }, { replace: true });
            }}
            onPrev={() => {
              setCursorStack((prev) => {
                const next = [...prev];
                const prevCursor = next.pop();
                setState({ cursor: prevCursor || undefined }, { replace: true });
                return next;
              });
            }}
          />
        </Card>
      </Space>
      <Modal
        title={tr("重新校准 SKU 绑定")}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void submitRecalibrate()}
        confirmLoading={actionLoading}
        okText={tr("提交校准")}
        cancelText={tr("取消")}
        destroyOnHidden
      >
        <Form form={reasonForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item label="Expected Calibration Version">
            <Input disabled value={expectedCalibrationVersion} />
          </Form.Item>
          <Form.Item name="reason" label={tr("校准原因")} rules={[{ required: true, whitespace: true, message: tr("请输入校准原因") }]}>
            <Input.TextArea rows={3} maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
