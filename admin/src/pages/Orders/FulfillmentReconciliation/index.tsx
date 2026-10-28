import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import { EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  Descriptions,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  type TableColumnsType,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import AppDrawer from '@/components/AppDrawer';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { useUrlDrawerState, useUrlQueryState } from '@/hooks/useUrlState';
import {
  ORDER_FULFILLMENT_RECONCILIATION_ISSUES,
  ORDER_FULFILLMENT_RECONCILIATION_STATUS,
  ORDER_FULFILLMENT_STATUS,
  ORDER_STATUS,
} from '@/constants/status';
import {
  queryFulfillmentReconciliation,
  getOrderFulfillmentReconciliation,
  type FulfillmentActionSummary,
  type FulfillmentReconciliation,
  type FulfillmentTimelineEntry,
} from '@/services/orders';
import { listInventoryWarehouses, type InventoryWarehouse } from '@/services/inventory';
import { formatDateTime } from '@/utils/formatTime';
import { parsePositiveInt } from '@/utils/urlState';
import { PERMISSIONS } from '@/utils/permission';

const QUERY_KEYS = [
  'page',
  'pageSize',
  'orderNo',
  'warehouseId',
  'status',
  'fulfillmentStatus',
  'reconciliationStatus',
  'drawer',
  'id',
] as const;

const ACTION_LABELS: Record<'reserve' | 'deduct' | 'release' | 'restore', string> = {
  reserve: '预占',
  deduct: '扣减',
  release: '释放',
  restore: '回补',
};

const TIMELINE_TYPE_LABELS: Record<string, string> = {
  effect: '库存 effect',
  movement: '库存流水',
  shipment: '发货单',
};

function statusTag(
  value: string | undefined,
  map: Record<string, { text: string; color: string }>,
) {
  const meta = value ? map[value] : undefined;
  return <Tag color={meta?.color}>{meta?.text || value || '—'}</Tag>;
}

function actionSummary(summary: FulfillmentActionSummary) {
  return `${summary.actual} / ${summary.expected}`;
}

function timelineType(value: string) {
  return tr(TIMELINE_TYPE_LABELS[value] || value || '—');
}

function issueLabel(value: string) {
  return ORDER_FULFILLMENT_RECONCILIATION_ISSUES[value] || value;
}

export default function FulfillmentReconciliationPage() {
  useLanguage();
  const reconciliationOptions = Object.entries(ORDER_FULFILLMENT_RECONCILIATION_STATUS).map(
    ([value, meta]) => ({ value, label: meta.text }),
  );
  const orderStatusOptions = Object.entries(ORDER_STATUS).map(([value, meta]) => ({
    value,
    label: meta.text,
  }));
  const fulfillmentStatusOptions = Object.entries(ORDER_FULFILLMENT_STATUS).map(
    ([value, meta]) => ({ value, label: meta.text }),
  );
  const { readonly } = usePermission();
  const actionRef = useRef<ActionType>();
  const initializedRef = useRef(false);
  const { state: urlState, setState: setUrlState } = useUrlQueryState<
    Record<(typeof QUERY_KEYS)[number], string | undefined>
  >(QUERY_KEYS);
  const drawer = useUrlDrawerState('fulfillment-reconciliation');
  const [orderNoInput, setOrderNoInput] = useState(urlState.orderNo || '');
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const [warehouseError, setWarehouseError] = useState('');
  const [listError, setListError] = useState('');
  const [detail, setDetail] = useState<FulfillmentReconciliation>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');

  const page = parsePositiveInt(urlState.page, 1);
  const pageSize = parsePositiveInt(urlState.pageSize, 20);
  const warehouseOptions = useMemo(
    () => warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` })),
    [warehouses],
  );

  useEffect(() => {
    setOrderNoInput(urlState.orderNo || '');
  }, [urlState.orderNo]);

  useEffect(() => {
    let active = true;
    void listInventoryWarehouses()
      .then((result) => {
        if (active) {
          setWarehouses(result.list || []);
          setWarehouseError('');
        }
      })
      .catch((error) => {
        if (active) setWarehouseError((error as Error)?.message || '仓库筛选加载失败');
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!initializedRef.current) {
      initializedRef.current = true;
      return;
    }
    void actionRef.current?.reload();
  }, [
    urlState.orderNo,
    urlState.warehouseId,
    urlState.status,
    urlState.fulfillmentStatus,
    urlState.reconciliationStatus,
  ]);

  const loadDetail = useCallback(async (id: string) => {
    setDetailLoading(true);
    setDetailError('');
    try {
      setDetail(await getOrderFulfillmentReconciliation(id));
    } catch (error) {
      setDetail(undefined);
      setDetailError((error as Error)?.message || '履约对账详情加载失败');
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (drawer.id) {
      void loadDetail(drawer.id);
    } else {
      setDetail(undefined);
      setDetailError('');
    }
  }, [drawer.id, loadDetail]);

  const updateFilter = useCallback(
    (key: 'orderNo' | 'warehouseId' | 'status' | 'fulfillmentStatus' | 'reconciliationStatus', value?: string) => {
      setUrlState({ [key]: value || undefined, page: undefined }, { replace: true });
    },
    [setUrlState],
  );

  const columns: ProColumns<FulfillmentReconciliation>[] = [
    {
      title: tr("订单"),
      dataIndex: 'orderNo',
      width: 190,
      copyable: true,
      ellipsis: true,
      render: (_, row) => (
        <Button type="link" size="small" onClick={() => drawer.openDrawer(row.orderId)}>
          {row.orderNo || row.orderId}
        </Button>
      ),
    },
    {
      title: tr("订单状态"),
      dataIndex: 'status',
      width: 108,
      render: (_, row) => statusTag(row.status, ORDER_STATUS),
    },
    {
      title: tr("履约状态"),
      dataIndex: 'fulfillmentStatus',
      width: 108,
      render: (_, row) => statusTag(row.fulfillmentStatus, ORDER_FULFILLMENT_STATUS),
    },
    {
      title: tr("仓库"),
      dataIndex: 'warehouseId',
      width: 180,
      ellipsis: true,
      render: (_, row) =>
        row.warehouseId
          ? warehouseOptions.find((item) => item.value === row.warehouseId)?.label || row.warehouseId
          : tr("未绑定"),
    },
    ...(['reserve', 'deduct', 'release', 'restore'] as const).map((key) => ({
      title: trFormat("{value0} 实际 / 预期", { value0: tr(ACTION_LABELS[key]) }),
      dataIndex: key,
      width: 120,
      align: 'right' as const,
      render: (_: unknown, row: FulfillmentReconciliation) => {
        const summary = row[key];
        return (
          <Typography.Text type={summary.actual === summary.expected ? undefined : 'danger'}>
            {actionSummary(summary)}
          </Typography.Text>
        );
      },
    })),
    {
      title: tr("发货 / effect"),
      width: 116,
      align: 'right',
      render: (_, row) => `${row.shipmentCount} / ${row.effectCount}`,
    },
    {
      title: tr("对账状态"),
      dataIndex: 'reconciliationStatus',
      width: 112,
      render: (_, row) => statusTag(row.reconciliationStatus, ORDER_FULFILLMENT_RECONCILIATION_STATUS),
    },
    {
      title: tr("最后库存动作"),
      dataIndex: 'lastInventoryActionAt',
      width: 168,
      render: (_, row) =>
        row.lastInventoryActionAt ? formatDateTime(row.lastInventoryActionAt) : '—',
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 88,
      fixed: 'right',
      render: (_, row) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.orderId)}>
          {tr("查看")}
        </Button>
      ),
    },
  ];

  const timelineColumns: TableColumnsType<FulfillmentTimelineEntry> = [
    {
      title: tr("事实"),
      dataIndex: 'type',
      width: 110,
      render: (_, row) => timelineType(row.type),
    },
    { title: tr("动作"), dataIndex: 'action', width: 150, ellipsis: true },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 96,
      render: (_, row) => row.status || '—',
    },
    {
      title: tr("数量"),
      dataIndex: 'quantity',
      width: 72,
      align: 'right',
      render: (_, row) => row.quantity ?? '—',
    },
    {
      title: tr("时间"),
      dataIndex: 'createdAt',
      width: 180,
      render: (_, row) => formatDateTime(row.createdAt),
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.ORDER_VIEW} showForbiddenPage>
      <TmPageContainer
        title={tr("履约库存对账")}
        subTitle={tr("订单生命周期事实与库存 effect、流水和发货单的只读核对")}
      >
        {readonly ? (
          <Alert type="info" showIcon message={tr("当前账号为只读模式")} description={tr("本工作台仅提供查询、筛选和详情查看，不提供修复或重试操作。")} style={{ marginBottom: 16 }} />
        ) : null}
        {warehouseError ? (
          <Alert type="warning" showIcon message={tr("仓库筛选暂不可用")} description={tr(warehouseError)} style={{ marginBottom: 16 }} />
        ) : null}
        {listError ? (
          <ErrorAlert title={tr(listError)} actionHint={<Button icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>{tr("重新加载")}</Button>} />
        ) : null}
        <TmProTable<FulfillmentReconciliation>
          rowKey="orderId"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1500 }}
          locale={{ emptyText: listError ? tr('对账数据暂不可用') : tr('暂无符合条件的履约对账记录') }}
          filterBar={
            <Space wrap>
              <Input.Search
                allowClear
                aria-label={tr("订单号")}
                placeholder={tr("搜索订单号")}
                value={orderNoInput}
                style={{ width: 210, maxWidth: '100%' }}
                onChange={(event) => setOrderNoInput(event.target.value)}
                onSearch={(value) => updateFilter('orderNo', value.trim())}
              />
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                aria-label={tr("履约仓库")}
                placeholder={tr("全部仓库")}
                value={urlState.warehouseId}
                style={{ width: 190 }}
                options={warehouseOptions}
                onChange={(value) => updateFilter('warehouseId', value)}
              />
              <Select
                allowClear
                aria-label={tr("订单状态")}
                placeholder={tr("全部订单状态")}
                value={urlState.status}
                style={{ width: 150 }}
                options={orderStatusOptions}
                onChange={(value) => updateFilter('status', value)}
              />
              <Select
                allowClear
                aria-label={tr("履约状态")}
                placeholder={tr("全部履约状态")}
                value={urlState.fulfillmentStatus}
                style={{ width: 150 }}
                options={fulfillmentStatusOptions}
                onChange={(value) => updateFilter('fulfillmentStatus', value)}
              />
              <Select
                allowClear
                aria-label={tr("对账状态")}
                placeholder={tr("全部对账状态")}
                value={urlState.reconciliationStatus}
                style={{ width: 150 }}
                options={reconciliationOptions}
                onChange={(value) => updateFilter('reconciliationStatus', value)}
              />
            </Space>
          }
          pagination={{
            current: page,
            pageSize,
            showSizeChanger: true,
            onChange: (nextPage, nextPageSize) =>
              setUrlState({
                page: nextPage > 1 ? String(nextPage) : undefined,
                pageSize: nextPageSize !== 20 ? String(nextPageSize) : undefined,
              }, { replace: true }),
          }}
          request={async (params) => {
            try {
              const result = await queryFulfillmentReconciliation({
                page: params.current || page,
                pageSize: params.pageSize || pageSize,
                orderNo: urlState.orderNo,
                warehouseId: urlState.warehouseId,
                status: urlState.status,
                fulfillmentStatus: urlState.fulfillmentStatus,
                reconciliationStatus: urlState.reconciliationStatus as FulfillmentReconciliation['reconciliationStatus'] | undefined,
              });
              setListError('');
              return {
                data: result.list || [],
                success: true,
                total: result.pagination?.total || 0,
              };
            } catch (error) {
              setListError((error as Error)?.message || '履约对账加载失败');
              return { data: [], success: false, total: 0 };
            }
          }}
        />
        <AppDrawer
          title={detail?.orderNo ? `${tr('订单')} ${detail.orderNo} · ${tr('履约对账')}` : tr('履约对账详情')}
          open={drawer.open}
          onClose={drawer.closeDrawer}
          loading={detailLoading}
          extra={
            drawer.id ? (
              <Button
                type="link"
                icon={<EyeOutlined />}
                onClick={() => history.push(`/orders/${encodeURIComponent(drawer.id!)}?tab=reconciliation`)}
              >
                {tr("打开订单详情")}
              </Button>
            ) : null
          }
        >
          {detailError ? <ErrorAlert title={tr(detailError)} /> : null}
          {detail ? (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                <Descriptions.Item label={tr("对账状态")}>
                  {statusTag(detail.reconciliationStatus, ORDER_FULFILLMENT_RECONCILIATION_STATUS)}
                </Descriptions.Item>
                <Descriptions.Item label={tr("订单状态")}>{statusTag(detail.status, ORDER_STATUS)}</Descriptions.Item>
                <Descriptions.Item label={tr("履约状态")}>{statusTag(detail.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}</Descriptions.Item>
                <Descriptions.Item label={tr("履约仓库")}>
                  {detail.warehouseId ? warehouseOptions.find((item) => item.value === detail.warehouseId)?.label || detail.warehouseId : tr("未绑定")}
                </Descriptions.Item>
                <Descriptions.Item label={tr("发货单数量")}>{detail.shipmentCount}</Descriptions.Item>
                <Descriptions.Item label={tr("库存 effect 数量")}>{detail.effectCount}</Descriptions.Item>
                <Descriptions.Item label={tr("最后库存动作")}>
                  {detail.lastInventoryActionAt ? formatDateTime(detail.lastInventoryActionAt) : '—'}
                </Descriptions.Item>
              </Descriptions>
              {detail.issues?.length ? (
                <Alert
                  type={detail.reconciliationStatus === 'blocked' ? 'warning' : 'error'}
                  showIcon
                  message={tr("需要人工核对")}
                  description={detail.issues.map(issueLabel).join('；')}
                />
              ) : null}
              <Space wrap>
                {(['reserve', 'deduct', 'release', 'restore'] as const).map((key) => {
                  const summary = detail[key];
                  return (
                    <Tag key={key} color={summary.expected === summary.actual ? 'success' : 'error'}>
                      {tr(ACTION_LABELS[key])} {actionSummary(summary)}
                    </Tag>
                  );
                })}
              </Space>
              <Table<FulfillmentTimelineEntry>
                rowKey="id"
                size="small"
                pagination={false}
                scroll={{ x: 600 }}
                locale={{ emptyText: tr("暂无库存或发货事实") }}
                dataSource={detail.timeline || []}
                columns={timelineColumns}
              />
            </Space>
          ) : null}
        </AppDrawer>
      </TmPageContainer>
    </PermissionGuard>
  );
}
