import { tr } from '@/i18n';
import { EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Card, Col, Form, Input, Row, Select, Space } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { TmPageContainer, TmProTable } from '@/components/ui';
import { useUrlQueryState } from '@/hooks/useUrlState';
import {
  extractInventorySyncAPIError,
  listInventoryBindings,
  type InventorySyncAPIError,
  type SKUBinding,
} from '@/services/inventorySync';
import { formatDateTime } from '@/utils/formatTime';
import {
  ActionSourceHint,
  BINDING_STATUS_LABELS,
  ConfidenceText,
  CursorPager,
  StatusTag,
  copyableText,
  renderInventorySyncError,
} from './components/InventorySyncShared';

const QUERY_KEYS = ['shopConnectionId', 'bindingStatus', 'bindingSource', 'cursor'] as const;
const LIMIT = 20;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;

const bindingStatusOptions = [
  { value: 'proposed', get label() { return tr("待确认"); } },
  { value: 'confirmed', get label() { return tr("已确认"); } },
  { value: 'rejected', get label() { return tr("已拒绝"); } },
  { value: 'stale', get label() { return tr("已过期"); } },
  { value: 'conflict', get label() { return tr("冲突"); } },
];

const bindingSourceOptions = [
  { value: 'automatic', get label() { return tr("自动校准"); } },
  { value: 'manual', get label() { return tr("人工确认"); } },
];

export default function InventorySyncBindingsPage() {
  const { state, setState, clearState } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [items, setItems] = useState<SKUBinding[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<InventorySyncAPIError | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const requestSeq = useRef(0);

  useEffect(() => {
    form.setFieldsValue({
      shopConnectionId: state.shopConnectionId,
      bindingStatus: state.bindingStatus,
      bindingSource: state.bindingSource,
    });
  }, [form, state.bindingSource, state.bindingStatus, state.shopConnectionId]);

  const load = useCallback(async () => {
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const page = await listInventoryBindings({
        shopConnectionId: state.shopConnectionId,
        bindingStatus: state.bindingStatus,
        bindingSource: state.bindingSource,
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
  }, [state.bindingSource, state.bindingStatus, state.cursor, state.shopConnectionId]);

  useEffect(() => {
    void load();
  }, [load]);

  const columns = useMemo<ProColumns<SKUBinding>[]>(() => [
    { title: tr("绑定 ID"), dataIndex: 'id', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("店铺连接"), dataIndex: 'shopConnectionId', width: 150, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("外部规格"), dataIndex: 'externalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("外部规格编码"), dataIndex: 'externalSkuCode', width: 140 },
    { title: tr("本地规格"), dataIndex: 'localSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("来源"), dataIndex: 'bindingSource', width: 110 },
    { title: tr("状态"), dataIndex: 'bindingStatus', width: 120, render: (v) => <StatusTag map={BINDING_STATUS_LABELS} value={String(v || '')} /> },
    { title: tr("置信度"), dataIndex: 'confidence', width: 100, render: (v) => <ConfidenceText value={typeof v === 'number' ? v : undefined} /> },
    { title: tr("版本"), dataIndex: 'calibrationVersion', width: 90 },
    { title: tr("更新时间"), dataIndex: 'updatedAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 130,
      render: (_, row) => [
        <Button
          key="history"
          type="link"
          icon={<EyeOutlined />}
          disabled={!row.allowedActions.canViewHistory}
          onClick={() => history.push(`/ops/inventory-sync/bindings/${encodeURIComponent(row.id)}`)}
        >
          {tr("历史")}
        </Button>,
      ],
    },
  ], []);

  return (
    <TmPageContainer
      title={tr("SKU 绑定列表")}
      subTitle={tr("查看库存同步绑定记录并进入历史")}
      extra={<Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>{tr("刷新")}</Button>}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderInventorySyncError(error)}
        <Card>
          <Form
            form={form}
            layout="vertical"
            onFinish={(values) => {
              setCursorStack([]);
              setState({ ...values, cursor: undefined }, { replace: true });
            }}
          >
            <Row gutter={[16, 8]}>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="shopConnectionId" label={tr("店铺连接 ID")}>
                  <Input allowClear placeholder={tr("仅传 shopConnectionId")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="bindingStatus" label={tr("绑定状态")}>
                  <Select allowClear options={bindingStatusOptions} placeholder={tr("全部状态")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="bindingSource" label={tr("绑定来源")}>
                  <Select allowClear options={bindingSourceOptions} placeholder={tr("全部来源")} />
                </Form.Item>
              </Col>
              <Col xs={24} lg={6}>
                <Form.Item label={tr("操作")}>
                  <Space wrap>
                    <Button type="primary" htmlType="submit">{tr("应用筛选")}</Button>
                    <Button onClick={() => { setCursorStack([]); form.resetFields(); clearState(QUERY_KEYS, { replace: true }); }}>{tr("清除")}</Button>
                  </Space>
                </Form.Item>
              </Col>
            </Row>
          </Form>
          <ActionSourceHint />
        </Card>
        <TmProTable<SKUBinding>
          rowKey="id"
          search={false}
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={false}
          scroll={{ x: 1450 }}
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
    </TmPageContainer>
  );
}
