import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import {
  Badge,
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
  message,
} from 'antd';
import dayjs from 'dayjs';
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  ORDER_FULFILLMENT_STATUS,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_STATUS,
} from '@/constants/status';
import {
  createOrder,
  createOrderItem,
  createOrderShipment,
  deleteOrder,
  deleteOrderItem,
  deleteOrderShipment,
  getOrder,
  queryOrders,
  updateOrder,
  updateOrderItem,
  updateOrderShipment,
  type OrderDetailDTO,
  type OrderItemRow,
  type OrderListRow,
  type OrderShipmentRow,
} from '@/services/orders';
import { queryShops } from '@/services/shops';

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

function statusTag(
  raw: string,
  map:
    | typeof ORDER_STATUS
    | typeof ORDER_PAYMENT_STATUS
    | typeof ORDER_FULFILLMENT_STATUS
    | typeof ORDER_SHIPMENT_STATUS,
) {
  const cfg = map[raw as keyof typeof map];
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

  const refreshDetail = async (id?: string) => {
    const oid = id || detail?.id;
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
  };

  const columns: ProColumns<OrderListRow>[] = useMemo(
    () => [
      {
        title: '关联店铺',
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'select',
        fieldProps: { options: shopOptions, allowClear: true, showSearch: true },
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
        search: false,
        render: (_, r) => statusTag(r.paymentStatus, ORDER_PAYMENT_STATUS),
      },
      {
        title: '履约',
        dataIndex: 'fulfillmentStatus',
        width: 94,
        valueType: 'select',
        valueEnum: ORDER_FULFILLMENT_STATUS,
        render: (_, r) => statusTag(r.fulfillmentStatus, ORDER_FULFILLMENT_STATUS),
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
        render: (_, r) => (r.orderedAt ? dayjs(r.orderedAt).format('YYYY-MM-DD HH:mm') : '—'),
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
        render: (_, r) => dayjs(r.createdAt).format('YYYY-MM-DD HH:mm'),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 80,
        render: (_, r) => (
          <a
            onClick={() => {
              setDrawerOpen(true);
              void refreshDetail(r.id);
            }}
          >
            详情
          </a>
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
        { title: 'SKU', dataIndex: 'skuCode', width: 120 },
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

  return (
    <PageContainer title="订单管理">
      <ProTable<OrderListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ layout: 'vertical', defaultCollapsed: false }}
        toolBarRender={() => [
          <ModalForm
            key="c"
            title="新建手工订单"
            trigger={<Button type="primary">新建订单</Button>}
            onFinish={async (vals) => {
              await createOrder(vals as Record<string, unknown>);
              message.success('已创建');
              actionRef.current?.reload();
              return true;
            }}
          >
            <ProFormText name="platform" label="platform" placeholder="manual" />
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
            status: params.status as string | undefined,
            fulfillmentStatus: params.fulfillmentStatus as string | undefined,
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
        }}
        destroyOnClose
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
              ]}
            />
          </>
        )}
      </Drawer>

      <Modal
        title={itemModal.row ? '编辑明细' : '新增明细'}
        open={itemModal.open}
        onCancel={() => setItemModal({ open: false })}
        destroyOnClose
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
          <Form.Item name="skuCode" label="SKU 编码">
            <Input />
          </Form.Item>
          <Form.Item name="skuName" label="SKU 名称">
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
        destroyOnClose
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
    </PageContainer>
  );
}
