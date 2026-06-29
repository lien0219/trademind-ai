import { ModalForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import {
  Badge,
  Alert,
  Button,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import { formatDateTime } from '@/utils/formatTime';
import { history, useLocation } from '@umijs/max';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  ORDER_FULFILLMENT_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  ORDER_SYNC_SUMMARY,
} from '@/constants/status';
import {
  createOrder,
  createOrderItem,
  createOrderShipment,
  deductOrderInventory,
  deleteOrder,
  deleteOrderItem,
  deleteOrderShipment,
  getOrderInventoryEffects,
  getOrder,
  queryOrders,
  restoreOrderInventory,
  updateOrder,
  updateOrderItem,
  updateOrderShipment,
  type OrderDetailDTO,
  type OrderItemRow,
  type OrderListRow,
  type OrderShipmentRow,
} from '@/services/orders';
import OrderSkuMatchTab from '@/pages/Orders/SkuMatchTab';
import type { OrderInventoryEffectRow } from '@/services/inventory';
import { fetchSettingsList } from '@/services/settings';
import { queryShops } from '@/services/shops';
import { pickGroup } from '@/utils/settingsForm';

function truthyInventorySetting(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function summarizeInvResp(sum?: Record<string, unknown>) {
  if (!sum) return '';
  if (sum.skipped) return `跳过：${String(sum.skipReason || '')}`;
  if (typeof sum.message === 'string' && sum.message) return sum.message;
  return '已完成';
}

const ORDER_STATUS_OPTS = Object.keys(ORDER_STATUS).map((v) => ({
  label: ORDER_STATUS[v as keyof typeof ORDER_STATUS].text,
  value: v,
}));
const PAY_OPTS = Object.keys(ORDER_PAYMENT_STATUS).map((v) => ({
  label: ORDER_PAYMENT_STATUS[v as keyof typeof ORDER_PAYMENT_STATUS].text,
  value: v,
}));
const FULL_OPTS = Object.keys(ORDER_FULFILLMENT_STATUS).map((v) => ({
  label: ORDER_FULFILLMENT_STATUS[v as keyof typeof ORDER_FULFILLMENT_STATUS].text,
  value: v,
}));
const SHIP_OPTS = Object.keys(ORDER_SHIPMENT_STATUS).map((v) => ({
  label: ORDER_SHIPMENT_STATUS[v as keyof typeof ORDER_SHIPMENT_STATUS].text,
  value: v,
}));

type StatusTagMap = Record<string, { text: string; color: string }>;

function statusTag(raw: string, map: StatusTagMap) {
  const cfg = map[raw];
  if (!cfg) return <Tag>{raw}</Tag>;
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

export default function OrdersPage() {
  const actionRef = useRef<ActionType>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detail, setDetail] = useState<OrderDetailDTO | null>(null);
  const [editForm] = Form.useForm();
  const [itemModal, setItemModal] = useState<{ open: boolean; row?: OrderItemRow | null }>({ open: false });
  const [itemForm] = Form.useForm();
  const [shipModal, setShipModal] = useState<{ open: boolean; row?: OrderShipmentRow | null }>({ open: false });
  const [shipForm] = Form.useForm();
  const [shopOptions, setShopOptions] = useState<{ label: string; value: string }[]>([]);
  const { search: ordersSearch } = useLocation();
  const [createInvDefaults, setCreateInvDefaults] = useState<{ deduct: boolean; sync: boolean }>({
    deduct: false,
    sync: false,
  });
  const [invEffectRows, setInvEffectRows] = useState<OrderInventoryEffectRow[]>([]);
  const [invActionLoading, setInvActionLoading] = useState(false);
  const detailIdRef = useRef<string | undefined>();

  const invEffectFailures = useMemo(
    () => invEffectRows.filter((r) => r.status === 'failed'),
    [invEffectRows],
  );

  useEffect(() => {
    detailIdRef.current = detail?.id;
  }, [detail?.id]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 500 });
        setShopOptions(
          res.list.map((s) => ({
            label: `${s.shopName} (${s.platform})`,
            value: s.id,
          })),
        );
      } catch {
        /* ignore */
      }
    })();
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const { items } = await fetchSettingsList();
        const g = pickGroup(items, 'inventory');
        setCreateInvDefaults({
          deduct: truthyInventorySetting(g.auto_deduct_manual_orders),
          sync:
            truthyInventorySetting(g.auto_sync_inventory_after_order_deduct) ||
            truthyInventorySetting(g.auto_sync_platform_inventory_after_deduct),
        });
      } catch {
        /* ignore */
      }
    })();
  }, []);

  const refreshDetail = useCallback(async (id?: string) => {
    const oid = id ?? detailIdRef.current;
    if (!oid) return;
    const d = await getOrder(oid);
    setDetail(d);
    editForm.setFieldsValue({
      customerName: d.customerName,
      customerEmail: d.customerEmail,
      customerPhone: d.customerPhone,
      status: d.status,
      paymentStatus: d.paymentStatus,
      fulfillmentStatus: d.fulfillmentStatus,
      currency: d.currency,
      totalAmount: d.totalAmount,
      shopId: d.shopId,
    });
  }, [editForm]);

  const loadInvEffects = useCallback(async (orderId: string) => {
    try {
      const r = await getOrderInventoryEffects(orderId, { page: 1, pageSize: 100 });
      setInvEffectRows(r.list);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载库存影响失败');
    }
  }, []);

  useEffect(() => {
    const q = new URLSearchParams(ordersSearch);
    const jid = q.get('jumpOrder')?.trim();
    if (!jid) return;
    history.replace(`/orders/${encodeURIComponent(jid)}`);
  }, [ordersSearch]);

  const columns: ProColumns<OrderListRow>[] = useMemo(
    () => [
      {
        title: '关联店铺',
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'select',
        fieldProps: { options: shopOptions, allowClear: true, showSearch: true },
      },
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: '订单号 / 买家 / 平台单号' },
      },
      { title: '订单号', dataIndex: 'orderNo', copyable: true, width: 148 },
      {
        title: '外部单号',
        dataIndex: 'externalOrderId',
        width: 140,
        search: false,
        copyable: true,
        ellipsis: true,
        render: (_, r) => r.externalOrderId || '—',
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 96,
        fieldProps: { allowClear: true },
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        search: false,
        width: 140,
        ellipsis: true,
        render: (_, r) =>
          r.shopName ? (
            <span>
              {r.shopName}
              {r.shopPlatform ? ` / ${r.shopPlatform}` : ''}
            </span>
          ) : (
            '—'
          ),
      },
      { title: '客户', dataIndex: 'customerName', ellipsis: true, width: 120 },
      {
        title: '订单状态',
        dataIndex: 'status',
        width: 108,
        valueType: 'select',
        valueEnum: ORDER_STATUS,
        render: (_, r) => statusTag(r.status, ORDER_STATUS),
      },
      {
        title: '支付',
        dataIndex: 'paymentStatus',
        width: 94,
        valueType: 'select',
        valueEnum: ORDER_PAYMENT_STATUS,
        render: (_, r) => statusTag(r.paymentStatus, ORDER_PAYMENT_STATUS),
      },
      {
        title: '商品数',
        dataIndex: 'itemCount',
        search: false,
        width: 72,
        render: (_, r) => r.itemCount ?? '—',
      },
      {
        title: '规格匹配',
        dataIndex: 'skuMatchStatus',
        width: 108,
        valueType: 'select',
        valueEnum: ORDER_SKU_MATCH_SUMMARY,
        render: (_, r) => {
          const st = r.skuMatchStatus || 'none';
          const cfg = ORDER_SKU_MATCH_SUMMARY[st as keyof typeof ORDER_SKU_MATCH_SUMMARY];
          const label = cfg?.text || st;
          return (
            <span>
              <Tag color={cfg?.color}>{label}</Tag>
              {r.skuTotalCount ? (
                <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                  {' '}
                  {r.skuMatchedCount ?? 0}/{r.skuTotalCount}
                </Typography.Text>
              ) : null}
            </span>
          );
        },
      },
      {
        title: '库存扣减',
        dataIndex: 'inventoryDeductStatus',
        width: 100,
        valueType: 'select',
        valueEnum: ORDER_INVENTORY_DEDUCT_SUMMARY,
        search: false,
        render: (_, r) => {
          const st = r.inventoryDeductStatus || 'none';
          const cfg = ORDER_INVENTORY_DEDUCT_SUMMARY[st as keyof typeof ORDER_INVENTORY_DEDUCT_SUMMARY];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: '同步',
        dataIndex: 'syncStatus',
        width: 96,
        valueType: 'select',
        valueEnum: ORDER_SYNC_SUMMARY,
        search: false,
        render: (_, r) => {
          const st = r.syncStatus || 'unknown';
          const cfg = ORDER_SYNC_SUMMARY[st as keyof typeof ORDER_SYNC_SUMMARY];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: '是否有异常',
        dataIndex: 'hasException',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          true: { text: '有异常' },
          false: { text: '无异常' },
        },
      },
      {
        title: '异常',
        dataIndex: 'openExceptionCount',
        width: 72,
        search: false,
        render: (_, r) =>
          (r.openExceptionCount ?? 0) > 0 ? (
            <Badge count={r.openExceptionCount} size="small">
              <Tag color="error">待处理</Tag>
            </Badge>
          ) : (
            <Tag>无</Tag>
          ),
      },
      {
        title: '履约',
        dataIndex: 'fulfillmentStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: ORDER_FULFILLMENT_STATUS,
      },
      {
        title: '金额',
        search: false,
        width: 120,
        render: (_, r) => `${r.currency} ${r.totalAmount}`,
      },
      {
        title: '物流',
        dataIndex: 'latestShipmentStatus',
        search: false,
        width: 96,
        render: (_, r) =>
          r.latestShipmentStatus ? statusTag(r.latestShipmentStatus, ORDER_SHIPMENT_STATUS) : '—',
      },
      {
        title: '下单时间',
        dataIndex: 'orderedAt',
        search: false,
        width: 160,
        render: (_, r) => (r.orderedAt ? formatDateTime(r.orderedAt) : '—'),
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 160,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '更新时间',
        dataIndex: 'updatedAt',
        width: 160,
        search: false,
        render: (_, r) => (r.updatedAt ? formatDateTime(r.updatedAt) : '—'),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 220,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size={4}>
            <a onClick={() => history.push(`/orders/${encodeURIComponent(r.id)}`)}>详情</a>
            {(r.openExceptionCount ?? 0) > 0 ? (
              <a
                onClick={() =>
                  history.push(`/orders/exceptions?orderId=${encodeURIComponent(r.id)}`)
                }
              >
                异常
              </a>
            ) : null}
            <a onClick={() => history.push(`/orders/sync-tasks?shopId=${encodeURIComponent(r.shopId || '')}`)}>
              同步
            </a>
          </Space>
        ),
      },
    ],
    [shopOptions],
  );

  const openItemModal = (row?: OrderItemRow) => {
    setItemModal({ open: true, row: row ?? null });
    itemForm.resetFields();
    if (row) itemForm.setFieldsValue(row);
  };

  const openShipModal = (row?: OrderShipmentRow) => {
    setShipModal({ open: true, row: row ?? null });
    shipForm.resetFields();
    if (row) shipForm.setFieldsValue(row);
  };

  const itemColumns = detail
    ? [
        { title: '商品标题', dataIndex: 'productTitle', ellipsis: true },
        { title: '规格编号', dataIndex: 'skuCode', width: 120 },
        { title: '数量', dataIndex: 'quantity', width: 72 },
        { title: '单价', dataIndex: 'unitPrice', width: 88 },
        { title: '小计', dataIndex: 'totalPrice', width: 88 },
        {
          title: '操作',
          key: 'op',
          width: 132,
          render: (_: unknown, row: OrderItemRow) => (
            <Space>
              <a onClick={() => openItemModal(row)}>编辑</a>
              <Popconfirm
                title="删除？"
                onConfirm={async () => {
                  await deleteOrderItem(detail.id, row.id);
                  message.success('已删除');
                  await refreshDetail();
                }}
              >
                <a>删除</a>
              </Popconfirm>
            </Space>
          ),
        },
      ]
    : [];

  const shipColumns = detail
    ? [
        { title: '承运商', dataIndex: 'carrier', width: 110 },
        { title: '运单号', dataIndex: 'trackingNo', width: 150 },
        {
          title: '状态',
          dataIndex: 'status',
          width: 94,
          render: (v: string) => statusTag(v, ORDER_SHIPMENT_STATUS),
        },
        {
          title: '追踪',
          dataIndex: 'trackingUrl',
          render: (u: string) =>
            u ? (
              <a href={u} target="_blank" rel="noopener noreferrer">
                打开
              </a>
            ) : (
              '—'
            ),
        },
        {
          title: '操作',
          width: 132,
          render: (_: unknown, row: OrderShipmentRow) => (
            <Space>
              <a onClick={() => openShipModal(row)}>编辑</a>
              <Popconfirm
                title="删除？"
                onConfirm={async () => {
                  await deleteOrderShipment(detail.id, row.id);
                  message.success('已删除');
                  await refreshDetail();
                }}
              >
                <a>删除</a>
              </Popconfirm>
            </Space>
          ),
        },
      ]
    : [];

  const inventoryEffectCols = useMemo(
    () => [
      { title: '规格编号', dataIndex: 'productSkuId', ellipsis: true, width: 120 },
      { title: '类型', dataIndex: 'effectType', width: 100 },
      { title: '状态', dataIndex: 'status', width: 92 },
      { title: '数量', dataIndex: 'quantity', width: 64 },
      {
        title: '原因 / 错误',
        key: 'msg',
        ellipsis: true,
        render: (_: unknown, r: OrderInventoryEffectRow) => r.errorMessage || r.reason || '—',
      },
      {
        title: '时间',
        dataIndex: 'createdAt',
        width: 152,
        render: (v: string) => formatDateTime(v),
      },
    ],
    [],
  );

  return (
    <TmPageContainer title={PAGE_COPY.orderList.title} subTitle={PAGE_COPY.orderList.description}>
      <ProTable<OrderListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ layout: 'vertical', defaultCollapsed: false }}
        toolBarRender={() => [
          <ModalForm
            key={`c-${createInvDefaults.deduct}-${createInvDefaults.sync}`}
            initialValues={{
              deductInventory: createInvDefaults.deduct,
              syncInventory: createInvDefaults.sync,
            }}
            title="新建手工订单"
            trigger={<Button type="primary">新建订单</Button>}
            onFinish={async (vals) => {
              await createOrder(vals as Record<string, unknown>);
              message.success('已创建');
              actionRef.current?.reload();
              return true;
            }}
          >
            <ProFormText name="platform" label="平台" placeholder="manual" extra="手工订单可填 manual 或留空" />
            <ProFormSelect
              name="shopId"
              label="关联店铺（可选）"
              options={shopOptions}
              fieldProps={{ allowClear: true, showSearch: true }}
            />
            <ProFormText name="orderNo" label="订单号" rules={[{ required: true }]} />
            <ProFormText name="customerName" label="客户名称" rules={[{ required: true }]} />
            <ProFormText name="customerEmail" label="邮箱" />
            <ProFormText name="customerPhone" label="电话" />
            <ProFormSelect name="status" label="订单状态" options={ORDER_STATUS_OPTS} initialValue="pending" />
            <ProFormSelect name="paymentStatus" label="支付状态" options={PAY_OPTS} initialValue="unpaid" />
            <ProFormSelect name="fulfillmentStatus" label="履约状态" options={FULL_OPTS} initialValue="unfulfilled" />
            <ProFormText name="currency" label="币种" initialValue="USD" />
            <ProFormDigit name="totalAmount" label="订单总额" min={0} fieldProps={{ precision: 2 }} initialValue={0} />
            <ProFormSwitch
              name="deductInventory"
              label="创建后扣减本地库存"
              tooltip="与「设置 → 库存 / 订单 → 手工订单默认扣库存」并联"
            />
            <ProFormSwitch
              name="syncInventory"
              label="扣减后触发平台出库同步队列"
              tooltip="需在策略中放行并具备刊登出库路由"
            />
          </ModalForm>,
        ]}
        request={async (params) => {
          const res = await queryOrders({
            page: params.current,
            pageSize: params.pageSize,
            platform: params.platform as string | undefined,
            shopId: params.shopId as string | undefined,
            orderNo: params.orderNo as string | undefined,
            customerName: params.customerName as string | undefined,
            keyword: params.keyword as string | undefined,
            status: params.status as string | undefined,
            paymentStatus: params.paymentStatus as string | undefined,
            fulfillmentStatus: params.fulfillmentStatus as string | undefined,
            skuMatchStatus: params.skuMatchStatus as string | undefined,
            inventoryDeductStatus: params.inventoryDeductStatus as string | undefined,
            syncStatus: params.syncStatus as string | undefined,
            hasException:
              params.hasException === 'true' || params.hasException === true ? true : undefined,
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
        pagination={{ pageSize: 20 }}
      />

      <Drawer
        title={detail ? `订单 ${detail.orderNo}` : '订单详情'}
        width={720}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setDetail(null);
          setInvEffectRows([]);
        }}
        destroyOnHidden
      >
        {detail && (
          <>
            <Space wrap style={{ marginBottom: 12 }}>
              <Badge status="processing" text={`platform ${detail.platform}`} />
              {detail.shopSummary ? (
                <Badge
                  status="default"
                  text={`店铺 ${detail.shopSummary.shopName} (${detail.shopSummary.platform})`}
                />
              ) : null}
              <Popconfirm
                title="软删除此订单？"
                onConfirm={async () => {
                  await deleteOrder(detail.id);
                  message.success('已删除');
                  setDrawerOpen(false);
                  actionRef.current?.reload();
                }}
              >
                <Button danger size="small">
                  删除
                </Button>
              </Popconfirm>
            </Space>
            <Tabs
              onChange={(k) => {
                if (k === 'inv') void loadInvEffects(detail.id);
              }}
              items={[
                {
                  key: 'b',
                  label: '基础',
                  children: (
                    <Form
                      layout="vertical"
                      form={editForm}
                      onFinish={async (v) => {
                        const payload: Record<string, unknown> = {
                          customerName: v.customerName,
                          customerEmail: v.customerEmail ?? undefined,
                          customerPhone: v.customerPhone ?? undefined,
                          status: v.status,
                          paymentStatus: v.paymentStatus,
                          fulfillmentStatus: v.fulfillmentStatus,
                          currency: v.currency,
                          totalAmount: v.totalAmount,
                        };
                        const sid = v.shopId as string | undefined;
                        if (sid === undefined || sid === null || sid === '') {
                          payload.setShopIdNil = true;
                        } else {
                          payload.shopId = sid;
                        }
                        await updateOrder(detail.id, payload);
                        message.success('已保存');
                        await refreshDetail();
                        actionRef.current?.reload();
                      }}
                    >
                      <Form.Item name="customerName" label="客户名称" rules={[{ required: true }]}>
                        <Input />
                      </Form.Item>
                      <Form.Item name="customerEmail" label="邮箱">
                        <Input />
                      </Form.Item>
                      <Form.Item name="customerPhone" label="电话">
                        <Input />
                      </Form.Item>
                      <Form.Item name="shopId" label="关联店铺">
                        <Select
                          allowClear
                          showSearch
                          optionFilterProp="label"
                          placeholder="可选"
                          options={shopOptions}
                        />
                      </Form.Item>
                      <Form.Item name="status" label="订单状态" rules={[{ required: true }]}>
                        <Select options={ORDER_STATUS_OPTS} />
                      </Form.Item>
                      <Form.Item name="paymentStatus" label="支付" rules={[{ required: true }]}>
                        <Select options={PAY_OPTS} />
                      </Form.Item>
                      <Form.Item name="fulfillmentStatus" label="履约" rules={[{ required: true }]}>
                        <Select options={FULL_OPTS} />
                      </Form.Item>
                      <Form.Item name="currency" label="币种" rules={[{ required: true }]}>
                        <Input style={{ width: 120 }} />
                      </Form.Item>
                      <Form.Item name="totalAmount" label="总额" rules={[{ required: true }]}>
                        <InputNumber style={{ width: '100%' }} min={0} />
                      </Form.Item>
                      <Button type="primary" htmlType="submit">
                        保存
                      </Button>
                    </Form>
                  ),
                },
                {
                  key: 'i',
                  label: '商品明细',
                  children: (
                    <>
                      <Button type="primary" style={{ marginBottom: 8 }} onClick={() => openItemModal()}>
                        添加明细
                      </Button>
                      <Table<OrderItemRow> rowKey="id" columns={itemColumns as never} dataSource={detail.items} pagination={false} />
                    </>
                  ),
                },
                {
                  key: 's',
                  label: '物流',
                  children: (
                    <>
                      <Button type="primary" style={{ marginBottom: 8 }} onClick={() => openShipModal()}>
                        添加物流
                      </Button>
                      <Table<OrderShipmentRow> rowKey="id" columns={shipColumns as never} dataSource={detail.shipments} pagination={false} />
                    </>
                  ),
                },
                {
                  key: 'inv',
                  label: '库存',
                  children: (
                    <>
                      {detail && invEffectFailures.length > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          style={{ marginBottom: 12 }}
                          message="存在失败的库存扣减或恢复记录"
                          description={
                            <span>
                              请在异常工作台查看是否需要重新绑定 SKU、补库存或重试扣减。{' '}
                              <Typography.Link
                                onClick={() =>
                                  history.push(
                                    `/orders/exceptions?orderId=${encodeURIComponent(detail.id)}`,
                                  )
                                }
                              >
                                打开异常工作台
                              </Typography.Link>
                            </span>
                          }
                        />
                      ) : null}
                      <Space wrap style={{ marginBottom: 12 }}>
                        {detail.inventorySummary ? (
                          <>
                            <Tag color={detail.inventorySummary.hasDeductionSuccess ? 'success' : 'default'}>
                              扣库存{detail.inventorySummary.hasDeductionSuccess ? '：已有成功记录' : '：尚未成功'}
                            </Tag>
                            <Tag color={detail.inventorySummary.hasRestoreSuccess ? 'processing' : 'default'}>
                              回滚{detail.inventorySummary.hasRestoreSuccess ? '：有过成功记录' : '：未记录'}
                            </Tag>
                          </>
                        ) : (
                          <Tag>库存摘要不可用</Tag>
                        )}
                        <Popconfirm
                          title="扣减绑定 SKU 的本地库存（幂等；见错误提示）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, { syncInventory: false });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(r.inventoryDeduction as Record<string, unknown>),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button size="small" loading={invActionLoading}>
                            手工扣库存
                          </Button>
                        </Popconfirm>
                        <Popconfirm
                          title="扣库存并触发平台出库同步队列（仍需刊登与 outbound 就绪）"
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, { syncInventory: true });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(r.inventoryDeduction as Record<string, unknown>),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button size="small" loading={invActionLoading}>
                            扣库存 + 推平台任务
                          </Button>
                        </Popconfirm>
                        <Popconfirm
                          title='回滚本订单已成功扣掉的库存（需尚未被标记为「已完全对冲」等特殊状态）'
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await restoreOrderInventory(detail.id, {
                                syncInventory: false,
                                reason: 'manual_ui',
                              });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(r.inventoryRestoration as Record<string, unknown>),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button size="small" danger loading={invActionLoading}>
                            手工回滚库存
                          </Button>
                        </Popconfirm>
                      </Space>
                      <Space wrap style={{ marginBottom: 8 }}>
                        <Typography.Link href={`/inventory/effects?orderId=${encodeURIComponent(detail.id)}`}>
                          全局影响流水
                        </Typography.Link>
                        <Typography.Link href={`/inventory/logs?orderId=${encodeURIComponent(detail.id)}`}>
                          全局库存变更
                        </Typography.Link>
                      </Space>
                      <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
                        策略见「设置 → 库存 / 订单」。平台同步失败不参与本地数据库事务。
                      </Typography.Paragraph>
                      <Table<OrderInventoryEffectRow>
                        rowKey="id"
                        size="small"
                        columns={inventoryEffectCols as never}
                        dataSource={invEffectRows}
                        pagination={{ pageSize: 8 }}
                      />
                    </>
                  ),
                },
                {
                  key: 'sku',
                  label: '规格匹配',
                  children: (
                    <OrderSkuMatchTab
                      orderId={detail.id}
                      onRefreshOrder={async () => {
                        await refreshDetail();
                        await loadInvEffects(detail.id);
                      }}
                    />
                  ),
                },
              ]}
            />
          </>
        )}
      </Drawer>

      <Modal
        title={itemModal.row ? '编辑明细' : '新增明细'}
        open={itemModal.open}
        onCancel={() => setItemModal({ open: false })}
        destroyOnHidden
        onOk={async () => {
          const v = await itemForm.validateFields();
          if (!detail) return;
          if (itemModal.row) await updateOrderItem(detail.id, itemModal.row.id, v as Record<string, unknown>);
          else await createOrderItem(detail.id, v as Record<string, unknown>);
          message.success('已保存');
          setItemModal({ open: false });
          await refreshDetail();
        }}
      >
        <Form form={itemForm} layout="vertical">
          <Form.Item name="productTitle" label="标题" rules={[{ required: true, message: '必填' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="skuCode" label="规格编码">
            <Input />
          </Form.Item>
          <Form.Item name="skuName" label="规格名称">
            <Input />
          </Form.Item>
          <Form.Item name="quantity" label="数量" initialValue={1} rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="unitPrice" label="单价">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="totalPrice" label="小计">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={shipModal.row ? '编辑物流' : '新增物流'}
        open={shipModal.open}
        onCancel={() => setShipModal({ open: false })}
        destroyOnHidden
        onOk={async () => {
          const v = await shipForm.validateFields();
          if (!detail) return;
          if (shipModal.row) await updateOrderShipment(detail.id, shipModal.row.id, v as Record<string, unknown>);
          else await createOrderShipment(detail.id, v as Record<string, unknown>);
          message.success('已保存');
          setShipModal({ open: false });
          await refreshDetail();
        }}
      >
        <Form form={shipForm} layout="vertical">
          <Form.Item name="carrier" label="承运商" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="trackingNo" label="运单号">
            <Input />
          </Form.Item>
          <Form.Item name="trackingUrl" label="追踪 URL">
            <Input />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]} initialValue="pending">
            <Select options={SHIP_OPTS} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
