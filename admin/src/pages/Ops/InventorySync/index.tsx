import { tr } from '@/i18n';
import { EyeOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Card, Col, Form, Input, Modal, Row, Select, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { MetricCard, TmPageContainer, TmProTable } from '@/components/ui';
import { useUrlQueryState } from '@/hooks/useUrlState';
import {
  createInventorySyncIdempotencyKey,
  createInventorySyncRun,
  extractInventorySyncAPIError,
  listInventorySyncRuns,
  type CreateRunRequest,
  type InventorySyncAPIError,
  type InventorySyncRun,
} from '@/services/inventorySync';
import { formatDateTime } from '@/utils/formatTime';
import {
  ActionSourceHint,
  CursorPager,
  RUN_STATUS_LABELS,
  StatusTag,
  copyableText,
  errorMessage,
  renderInventorySyncError,
} from './components/InventorySyncShared';

const { Text } = Typography;

const QUERY_KEYS = ['shopConnectionId', 'status', 'providerMode', 'cursor'] as const;
const LIMIT = 20;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;

const providerModeOptions = [
  { value: 'mock', get label() { return tr("模拟夹具"); } },
  { value: 'sandbox', get label() { return tr("Sandbox 夹具"); } },
  { value: 'local_draft_only', get label() { return tr("本地草稿夹具"); } },
];

const runStatusOptions = Object.entries(RUN_STATUS_LABELS).map(([value, item]) => ({ value, label: item.text }));

const fixtureScenarioOptions = [
  { value: 'success_single_page', get label() { return tr("单页成功夹具"); } },
  { value: 'success_multi_page', get label() { return tr("多页成功夹具"); } },
  { value: 'empty_inventory', get label() { return tr("空库存夹具"); } },
  { value: 'low_confidence_binding', get label() { return tr("低置信绑定夹具"); } },
  { value: 'binding_conflict', get label() { return tr("绑定冲突夹具"); } },
  { value: 'unmatched_sku', get label() { return tr("未匹配规格夹具"); } },
  { value: 'provider_timeout', get label() { return tr("平台超时夹具"); } },
  { value: 'provider_rejected', get label() { return tr("平台拒绝夹具"); } },
  { value: 'malformed_item', get label() { return tr("异常数据夹具"); } },
  { value: 'duplicate_external_sku', get label() { return tr("重复外部规格夹具"); } },
  { value: 'cursor_loop', get label() { return tr("游标循环夹具"); } },
  { value: 'cancelled_context', get label() { return tr("上下文取消夹具"); } },
];

function sum(items: InventorySyncRun[], key: keyof InventorySyncRun['statistics']) {
  return items.reduce((total, run) => total + Number(run.statistics?.[key] || 0), 0);
}

function shortText(value: string, max = 16) {
  return value.length > max ? `${value.slice(0, max)}...` : value;
}

export default function InventorySyncDashboardPage() {
  const { state: urlState, setState, clearState } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [createForm] = Form.useForm<CreateRunRequest>();
  const [items, setItems] = useState<InventorySyncRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<InventorySyncAPIError | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const requestSeq = useRef(0);

  useEffect(() => {
    form.setFieldsValue({
      shopConnectionId: urlState.shopConnectionId,
      status: urlState.status,
      providerMode: urlState.providerMode,
    });
  }, [form, urlState.providerMode, urlState.shopConnectionId, urlState.status]);

  const load = useCallback(async () => {
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const page = await listInventorySyncRuns({
        shopConnectionId: urlState.shopConnectionId,
        status: urlState.status,
        providerMode: urlState.providerMode,
        cursor: urlState.cursor,
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
  }, [urlState.cursor, urlState.providerMode, urlState.shopConnectionId, urlState.status]);

  useEffect(() => {
    void load();
  }, [load]);

  const updateFilters = (values: QueryState) => {
    setCursorStack([]);
    setState({ ...values, cursor: undefined }, { replace: true });
  };

  const clearFilters = () => {
    setCursorStack([]);
    form.resetFields();
    clearState(QUERY_KEYS, { replace: true });
  };

  const goNext = () => {
    if (!nextCursor) return;
    setCursorStack((prev) => [...prev, urlState.cursor || '']);
    setState({ cursor: nextCursor }, { replace: true });
  };

  const goPrev = () => {
    setCursorStack((prev) => {
      const next = [...prev];
      const prevCursor = next.pop();
      setState({ cursor: prevCursor || undefined }, { replace: true });
      return next;
    });
  };

  const submitCreateRun = async () => {
    setActionLoading(true);
    setError(null);
    try {
      const values = await createForm.validateFields();
      const body: CreateRunRequest = {
        shopConnectionId: values.shopConnectionId,
        platform: 'douyin',
        providerMode: values.providerMode,
        fixtureScenario: values.fixtureScenario,
      };
      const run = await createInventorySyncRun(body, createInventorySyncIdempotencyKey('create-run'));
      message.success(tr("夹具同步运行已创建"));
      setCreateOpen(false);
      createForm.resetFields();
      history.push(`/ops/inventory-sync/runs/${encodeURIComponent(run.id)}`);
    } catch (e) {
      const next = extractInventorySyncAPIError(e);
      setError(next);
      message.error(errorMessage(next));
    } finally {
      setActionLoading(false);
    }
  };

  const columns = useMemo<ProColumns<InventorySyncRun>[]>(() => [
    {
      title: tr("运行 ID"),
      dataIndex: 'id',
      width: 180,
      render: (_, row) => (
        <Space direction="vertical" size={2}>
          <Button type="link" style={{ padding: 0 }} onClick={() => history.push(`/ops/inventory-sync/runs/${encodeURIComponent(row.id)}`)}>
            {shortText(row.id, 16)}
          </Button>
          <Text type="secondary">{copyableText(row.id, 16)}</Text>
          <Text type="secondary">{row.fixtureScenario || 'default fixture'}</Text>
        </Space>
      ),
    },
    { title: tr("店铺连接"), dataIndex: 'shopConnectionId', width: 160, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("平台"), dataIndex: 'platform', width: 90 },
    { title: tr("模式"), dataIndex: 'providerMode', width: 130 },
    { title: tr("状态"), dataIndex: 'status', width: 120, render: (v) => <StatusTag map={RUN_STATUS_LABELS} value={String(v || '')} /> },
    { title: tr("记录"), dataIndex: ['statistics', 'totalRecordCount'], width: 90 },
    { title: tr("匹配"), dataIndex: ['statistics', 'matchedRecordCount'], width: 90 },
    { title: tr("待人工"), dataIndex: ['statistics', 'manualBindingRequestCount'], width: 90 },
    { title: tr("失败"), dataIndex: ['statistics', 'failedRecordCount'], width: 90 },
    { title: 'Revision', dataIndex: 'revision', width: 90 },
    { title: tr("开始时间"), dataIndex: 'startedAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: tr("完成时间"), dataIndex: 'finishedAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 130,
      render: (_, row) => [
        <Button key="detail" type="link" icon={<EyeOutlined />} onClick={() => history.push(`/ops/inventory-sync/runs/${encodeURIComponent(row.id)}`)}>
          {tr("查看")}
        </Button>,
      ],
    },
  ], []);

  return (
    <TmPageContainer
      title={tr("库存同步与绑定中心")}
      subTitle={tr("夹具库存同步运行、SKU 校准、人工绑定和审计查看")}
      extra={
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>{tr("刷新")}</Button>
          <Button data-testid="inventory-sync-create-run" type="primary" icon={<PlayCircleOutlined />} onClick={() => setCreateOpen(true)}>{tr("创建夹具运行")}</Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderInventorySyncError(error)}
        <Row gutter={[16, 16]}>
          <Col xs={24} sm={12} lg={6}><MetricCard title={tr("当前批次运行")} value={items.length} description={tr("当前 cursor 窗口")} intent="data" /></Col>
          <Col xs={24} sm={12} lg={6}><MetricCard title={tr("夹具记录")} value={sum(items, 'totalRecordCount')} description={tr("非真实库存读取")} intent="primary" /></Col>
          <Col xs={24} sm={12} lg={6}><MetricCard title={tr("已匹配")} value={sum(items, 'matchedRecordCount')} description={tr("后端快照统计")} intent="success" /></Col>
          <Col xs={24} sm={12} lg={6}><MetricCard title={tr("待人工")} value={sum(items, 'manualBindingRequestCount')} description={tr("需进入人工绑定")} intent="warning" /></Col>
        </Row>
        <Card>
          <Form form={form} layout="vertical" onFinish={updateFilters}>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="shopConnectionId" label={tr("店铺连接 ID")}>
                  <Input allowClear placeholder={tr("仅传 shopConnectionId")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="status" label={tr("运行状态")}>
                  <Select allowClear options={runStatusOptions} placeholder={tr("全部状态")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="providerMode" label={tr("接入模式")}>
                  <Select allowClear options={providerModeOptions} placeholder={tr("全部 fixture 模式")} />
                </Form.Item>
              </Col>
              <Col xs={24} lg={6}>
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
        <TmProTable<InventorySyncRun>
          rowKey="id"
          search={false}
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={false}
          scroll={{ x: 1460 }}
          options={false}
          toolBarRender={false}
        />
        <Card size="small">
          <CursorPager count={items.length} hasMore={hasMore && !!nextCursor} canPrev={cursorStack.length > 0} loading={loading} onPrev={goPrev} onNext={goNext} />
        </Card>
      </Space>
      <Modal
        title={tr("创建夹具库存同步运行")}
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => void submitCreateRun()}
        confirmLoading={actionLoading}
        okText={tr("创建")}
        cancelText={tr("取消")}
        okButtonProps={{ 'data-testid': 'inventory-sync-create-run-submit' } as never}
        destroyOnHidden
      >
        <Form form={createForm} layout="vertical" style={{ marginTop: 16 }} initialValues={{ platform: 'douyin', providerMode: 'mock', fixtureScenario: 'success_single_page' }}>
          <Form.Item name="shopConnectionId" label={tr("店铺连接 ID")} rules={[{ required: true, whitespace: true, message: tr("请输入店铺连接 ID") }]}>
            <Input data-testid="inventory-sync-create-run-shop" placeholder={tr("后端按登录态校验店铺权限")} />
          </Form.Item>
          <Form.Item name="platform" label={tr("平台")}>
            <Input disabled />
          </Form.Item>
          <Form.Item name="providerMode" label={tr("接入模式")} rules={[{ required: true, message: tr("请选择接入模式") }]}>
            <Select options={providerModeOptions} />
          </Form.Item>
          <Form.Item name="fixtureScenario" label={tr("夹具场景")}>
            <Select allowClear options={fixtureScenarioOptions} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
