import { tr } from '@/i18n';
import { CheckCircleOutlined, CloseCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { Button, Card, Col, Descriptions, Form, Input, Modal, Row, Select, Space, Table, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { SectionCard, TmPageContainer, TmProTable } from '@/components/ui';
import { useUrlQueryState } from '@/hooks/useUrlState';
import {
  confirmManualBindingRequest,
  createInventorySyncIdempotencyKey,
  extractInventorySyncAPIError,
  getManualBindingRequest,
  listManualBindingRequests,
  rejectManualBindingRequest,
  type ManualBindingDecision,
  type ManualBindingDetail,
  type ManualBindingRequest,
  type InventorySyncAPIError,
} from '@/services/inventorySync';
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';
import { formatDateTime } from '@/utils/formatTime';
import {
  ActionSourceHint,
  CursorPager,
  MANUAL_REQUEST_STATUS_LABELS,
  StatusTag,
  copyableText,
  errorMessage,
  renderInventorySyncError,
} from './components/InventorySyncShared';

const { Text } = Typography;
const QUERY_KEYS = ['shopConnectionId', 'status', 'cursor', 'requestId'] as const;
const LIMIT = 20;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;
type ModalKind = 'confirm' | 'reject';

const statusOptions = Object.entries(MANUAL_REQUEST_STATUS_LABELS).map(([value, item]) => ({ value, label: item.text }));
const rejectReasonOptions = [
  { value: 'no_binding_candidate', get label() { return tr("无可用候选"); } },
  { value: 'calibration_threshold_not_met', get label() { return tr("置信度不足"); } },
  { value: 'existing_binding_conflict', get label() { return tr("已有绑定冲突"); } },
  { value: 'multiple_normalized_matches', get label() { return tr("多个候选匹配"); } },
  { value: 'manual_review_required', get label() { return tr("人工判定拒绝"); } },
];

function skuOption(hit: ProductSkuSearchHit) {
  return {
    value: hit.productSkuId,
    label: `${hit.skuCode || hit.productSkuId} / ${hit.skuName || hit.productTitle || '-'}`,
  };
}

export default function InventorySyncManualBindingsPage() {
  const { state, setState, clearState } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [decisionForm] = Form.useForm();
  const [items, setItems] = useState<ManualBindingRequest[]>([]);
  const [detail, setDetail] = useState<ManualBindingDetail | null>(null);
  const [skuOptions, setSkuOptions] = useState<{ value: string; label: string }[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [skuLoading, setSkuLoading] = useState(false);
  const [error, setError] = useState<InventorySyncAPIError | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [modal, setModal] = useState<ModalKind | null>(null);
  const requestSeq = useRef(0);

  useEffect(() => {
    form.setFieldsValue({ shopConnectionId: state.shopConnectionId, status: state.status });
  }, [form, state.shopConnectionId, state.status]);

  const loadList = useCallback(async () => {
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const page = await listManualBindingRequests({
        shopConnectionId: state.shopConnectionId,
        status: state.status,
        cursor: state.cursor,
        limit: LIMIT,
      });
      if (requestSeq.current !== seq) return;
      setItems(page.items || []);
      setNextCursor(page.nextCursor);
      setHasMore(page.hasMore);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractInventorySyncAPIError(e));
      setItems([]);
      setNextCursor(undefined);
      setHasMore(false);
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [state.cursor, state.shopConnectionId, state.status]);

  const loadDetail = useCallback(async () => {
    if (!state.requestId) {
      setDetail(null);
      return;
    }
    setDetailLoading(true);
    setError(null);
    try {
      const next = await getManualBindingRequest(state.requestId);
      setDetail(next);
      const suggested = next.request.suggestedLocalSkuId || next.request.selectedLocalSkuId;
      setSkuOptions(suggested ? [{ value: suggested, label: suggested }] : []);
    } catch (e) {
      setError(extractInventorySyncAPIError(e));
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  }, [state.requestId]);

  useEffect(() => {
    void loadList();
  }, [loadList]);

  useEffect(() => {
    void loadDetail();
  }, [loadDetail]);

  const updateFilters = (values: QueryState) => {
    setCursorStack([]);
    setState({ shopConnectionId: values.shopConnectionId, status: values.status, cursor: undefined }, { replace: true });
  };

  const clearFilters = () => {
    setCursorStack([]);
    form.resetFields();
    clearState(['shopConnectionId', 'status', 'cursor'], { replace: true });
  };

  const searchSku = async (keyword: string) => {
    const text = keyword.trim();
    if (!text) return;
    setSkuLoading(true);
    try {
      const res = await searchProductSkus({ keyword: text, limit: 10 });
      setSkuOptions((res.list || []).map(skuOption));
    } finally {
      setSkuLoading(false);
    }
  };

  const openDecision = (kind: ModalKind) => {
    setModal(kind);
    decisionForm.resetFields();
    if (kind === 'confirm' && detail?.request.suggestedLocalSkuId) {
      decisionForm.setFieldsValue({ selectedLocalSkuId: detail.request.suggestedLocalSkuId });
    }
    if (kind === 'reject') decisionForm.setFieldsValue({ reasonCode: detail?.request.reasonCode || 'manual_review_required' });
  };

  const submitDecision = async () => {
    if (!detail || !modal) return;
    setActionLoading(true);
    setError(null);
    try {
      const values = await decisionForm.validateFields();
      if (modal === 'confirm') {
        await confirmManualBindingRequest(
          detail.request.id,
          {
            expectedRevision: detail.request.revision,
            selectedLocalSkuId: values.selectedLocalSkuId,
            comment: values.comment,
          },
          createInventorySyncIdempotencyKey('manual-confirm'),
        );
        message.success(tr("已确认人工绑定"));
      }
      if (modal === 'reject') {
        await rejectManualBindingRequest(
          detail.request.id,
          {
            expectedRevision: detail.request.revision,
            reasonCode: values.reasonCode,
            comment: values.comment,
          },
          createInventorySyncIdempotencyKey('manual-reject'),
        );
        message.success(tr("已拒绝人工绑定"));
      }
      setModal(null);
      await Promise.all([loadList(), loadDetail()]);
    } catch (e) {
      const next = extractInventorySyncAPIError(e);
      setError(next);
      message.error(errorMessage(next));
      void loadDetail();
    } finally {
      setActionLoading(false);
    }
  };

  const columns = useMemo<ProColumns<ManualBindingRequest>[]>(() => [
    { title: tr("请求 ID"), dataIndex: 'id', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("店铺连接"), dataIndex: 'shopConnectionId', width: 150, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("平台规格"), dataIndex: 'externalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("状态"), dataIndex: 'status', width: 120, render: (v) => <StatusTag map={MANUAL_REQUEST_STATUS_LABELS} value={String(v || '')} /> },
    { title: tr("原因码"), dataIndex: 'reasonCode', width: 190 },
    { title: tr("候选数"), dataIndex: 'candidateCount', width: 90 },
    { title: tr("建议规格"), dataIndex: 'suggestedLocalSkuId', width: 160, render: (v) => copyableText(String(v || ''), 12) },
    { title: 'Revision', dataIndex: 'revision', width: 90 },
    { title: tr("更新时间"), dataIndex: 'updatedAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 120,
      render: (_, row) => [
        <Button key="view" type="link" onClick={() => setState({ requestId: row.id }, { replace: true })}>{tr("处理")}</Button>,
      ],
    },
  ], [setState]);

  const decisionColumns: ColumnsType<ManualBindingDecision> = [
    { title: tr("时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: tr("操作"), dataIndex: 'operation', width: 110 },
    { title: tr("操作者"), dataIndex: 'actorId', width: 160, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("规格"), dataIndex: 'selectedLocalSkuId', width: 160, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("原因码"), dataIndex: 'reasonCode', width: 190 },
    { title: 'Revision', dataIndex: 'requestRevision', width: 90 },
    { title: tr("备注"), dataIndex: 'comment', ellipsis: true },
  ];

  return (
    <TmPageContainer
      title={tr("人工绑定工作台")}
      subTitle={tr("处理库存同步生成的 SKU 绑定人工确认请求")}
      extra={<Button icon={<ReloadOutlined />} onClick={() => void Promise.all([loadList(), loadDetail()])} loading={loading || detailLoading}>{tr("刷新")}</Button>}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderInventorySyncError(error)}
        <Card>
          <Form form={form} layout="vertical" onFinish={updateFilters}>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="shopConnectionId" label={tr("店铺连接 ID")}>
                  <Input allowClear placeholder={tr("仅传 shopConnectionId")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="status" label={tr("请求状态")}>
                  <Select allowClear options={statusOptions} placeholder={tr("全部状态")} />
                </Form.Item>
              </Col>
              <Col xs={24} lg={8}>
                <Form.Item label={tr("操作")}>
                  <Space wrap>
                    <Button type="primary" htmlType="submit">{tr("应用筛选")}</Button>
                    <Button onClick={clearFilters}>{tr("清除")}</Button>
                  </Space>
                </Form.Item>
              </Col>
            </Row>
          </Form>
          <ActionSourceHint />
        </Card>
        <TmProTable<ManualBindingRequest>
          rowKey="id"
          search={false}
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={false}
          scroll={{ x: 1420 }}
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
        {detail ? (
          <SectionCard
            title={tr("请求详情")}
            description={tr("确认会生成手工绑定；拒绝只记录受控决策，不触发真实平台库存写入。")}
            headerExtra={<StatusTag map={MANUAL_REQUEST_STATUS_LABELS} value={detail.request.status} />}
          >
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
                <Descriptions.Item label={tr("请求 ID")}>{copyableText(detail.request.id, 18)}</Descriptions.Item>
                <Descriptions.Item label={tr("运行 ID")}>{copyableText(detail.request.inventorySyncRunId, 18)}</Descriptions.Item>
                <Descriptions.Item label={tr("快照 ID")}>{copyableText(detail.request.inventorySnapshotItemId, 18)}</Descriptions.Item>
                <Descriptions.Item label={tr("平台 SKU")}>{copyableText(detail.request.externalSkuId, 18)}</Descriptions.Item>
                <Descriptions.Item label={tr("建议 SKU")}>{copyableText(detail.request.suggestedLocalSkuId, 18)}</Descriptions.Item>
                <Descriptions.Item label="Revision">{detail.request.revision}</Descriptions.Item>
                <Descriptions.Item label={tr("原因码")}>{detail.request.reasonCode}</Descriptions.Item>
                <Descriptions.Item label="Resolution">{detail.request.resolution || '-'}</Descriptions.Item>
                <Descriptions.Item label={tr("更新时间")}>{formatDateTime(detail.request.updatedAt)}</Descriptions.Item>
              </Descriptions>
              <Space wrap>
                <Button
                  type="primary"
                  icon={<CheckCircleOutlined />}
                  disabled={!detail.request.allowedActions.canConfirm}
                  onClick={() => openDecision('confirm')}
                >
                  {tr("确认绑定")}
                </Button>
                <Button
                  danger
                  icon={<CloseCircleOutlined />}
                  disabled={!detail.request.allowedActions.canReject}
                  onClick={() => openDecision('reject')}
                >
                  {tr("拒绝")}
                </Button>
                <Text type="secondary">{tr("按钮状态来自 allowedActions；后端仍会校验权限、状态和 revision。")}</Text>
              </Space>
              <Table rowKey="id" columns={decisionColumns} dataSource={detail.decisions || []} pagination={false} scroll={{ x: 1040 }} />
            </Space>
          </SectionCard>
        ) : <Card><Text type="secondary">{tr("从列表选择一条人工绑定请求。")}</Text></Card>}
      </Space>
      <Modal
        title={modal === 'confirm' ? tr("确认人工绑定") : tr("拒绝人工绑定")}
        open={!!modal}
        onCancel={() => setModal(null)}
        onOk={() => void submitDecision()}
        confirmLoading={actionLoading}
        okText={modal === 'confirm' ? tr("确认绑定") : tr("确认拒绝")}
        cancelText={tr("取消")}
        destroyOnHidden
      >
        <Form form={decisionForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item label="Expected Revision">
            <Input disabled value={detail?.request.revision} />
          </Form.Item>
          {modal === 'confirm' ? (
          <Form.Item name="selectedLocalSkuId" label={tr("选择本地规格")} rules={[{ required: true, message: tr("请选择本地规格") }]}>
              <Select
                showSearch
                filterOption={false}
                onSearch={(value) => void searchSku(value)}
                options={skuOptions}
                loading={skuLoading}
                placeholder={tr("搜索 SKU 编码、名称或商品标题")}
              />
            </Form.Item>
          ) : (
            <Form.Item name="reasonCode" label={tr("拒绝原因码")} rules={[{ required: true, message: tr("请选择拒绝原因码") }]}>
              <Select options={rejectReasonOptions} />
            </Form.Item>
          )}
          <Form.Item name="comment" label={tr("备注")}>
            <Input.TextArea rows={3} maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
