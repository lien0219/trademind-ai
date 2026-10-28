import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import {
  ModalForm,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  type ActionType,
  type ProColumns,
  type ProFormInstance,
} from "@ant-design/pro-components";
import { TmPageContainer, TmProTable as ProTable } from "@/components/ui";
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
} from "antd";
import dayjs from "dayjs";
import { formatDateTime } from "@/utils/formatTime";
import { history, useLocation } from "@umijs/max";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { PAGE_COPY } from "@/constants/copywriting";
import { useListEmptyLocale } from "@/hooks/useListEmptyLocale";
import {
  ORDER_FULFILLMENT_STATUS,
  ORDER_FULFILLMENT_RECONCILIATION_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  ORDER_SYNC_SUMMARY,
} from "@/constants/status";
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
  partialOrderCreateFromError,
  queryOrders,
  restoreOrderInventory,
  updateOrder,
  updateOrderItem,
  updateOrderShipment,
  type OrderDetailDTO,
  type OrderItemRow,
  type OrderListRow,
  type OrderShipmentRow,
} from "@/services/orders";
import OrderSkuMatchTab from "@/pages/Orders/SkuMatchTab";
import {
  listInventoryWarehouses,
  type InventoryWarehouse,
  type OrderInventoryEffectRow,
} from "@/services/inventory";
import { fetchSettingsList } from "@/services/settings";
import { usePermission } from "@/hooks/usePermission";
import { mayReadGlobalSettings } from "@/utils/globalSettingsAccess";
import { queryShops } from "@/services/shops";
import { pickGroup } from "@/utils/settingsForm";
import { useUrlQueryState } from "@/hooks/useUrlState";
import { useKeywordSearchField } from "@/hooks/useKeywordSearchField";
import KeywordSafetyHint from "@/components/common/KeywordSafetyHint";
import {
  appendSourceToUrl,
  parsePositiveInt,
  queryTimeRange,
} from "@/utils/urlState";

const ORDER_QUERY_KEYS = [
  "page",
  "pageSize",
  "keyword",
  "payStatus",
  "skuStatus",
  "inventoryStatus",
  "status",
  "fulfillmentStatus",
  "reconciliationStatus",
  "platform",
  "shopId",
  "source",
  "start",
  "end",
  "jumpOrder",
] as const;

function truthyInventorySetting(v: string | undefined): boolean {
  const s = String(v ?? "")
    .trim()
    .toLowerCase();
  return s === "1" || s === "true" || s === "yes" || s === "on";
}

function summarizeInvResp(sum?: Record<string, unknown>) {
  if (!sum) return "";
  if (sum.skipped) return trFormat("跳过：{reason}", { reason: tr(String(sum.skipReason || "")) });
  const action = String(sum.action || "");
  if (action === "reserve") return tr("库存预占已完成");
  if (action === "deduct") return tr("实际出库扣减已完成");
  if (action === "release") return tr("库存预占已释放");
  if (action === "restore") return tr("库存已回补");
  if (typeof sum.message === "string" && sum.message) return tr(sum.message);
  return tr("已完成");
}

type StatusTagMap = Record<string, { text: string; color: string }>;

function statusTag(raw: string, map: StatusTagMap) {
  const cfg = map[raw];
  if (!cfg) return <Tag>{raw}</Tag>;
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

export default function OrdersPage() {
  const language = useLanguage();
  const ORDER_STATUS_OPTS = useMemo(
    () => Object.keys(ORDER_STATUS).map((v) => ({
      label: ORDER_STATUS[v as keyof typeof ORDER_STATUS].text,
      value: v,
    })),
    [language],
  );
  const PAY_OPTS = useMemo(
    () => Object.keys(ORDER_PAYMENT_STATUS).map((v) => ({
      label: ORDER_PAYMENT_STATUS[v as keyof typeof ORDER_PAYMENT_STATUS].text,
      value: v,
    })),
    [language],
  );
  const FULL_OPTS = useMemo(
    () => Object.keys(ORDER_FULFILLMENT_STATUS).map((v) => ({
      label: ORDER_FULFILLMENT_STATUS[v as keyof typeof ORDER_FULFILLMENT_STATUS].text,
      value: v,
    })),
    [language],
  );
  const SHIP_OPTS = useMemo(
    () => Object.keys(ORDER_SHIPMENT_STATUS).map((v) => ({
      label: ORDER_SHIPMENT_STATUS[v as keyof typeof ORDER_SHIPMENT_STATUS].text,
      value: v,
    })),
    [language],
  );
  const { user, canWriteOrders } = usePermission();
  const mayReadSettings = mayReadGlobalSettings(user);
  const emptyLocale = useListEmptyLocale("orderList", {
    permissionScoped: true,
  });
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const {
    state: urlState,
    setState: setUrlState,
    clearState: clearUrlState,
  } = useUrlQueryState<
    Record<(typeof ORDER_QUERY_KEYS)[number], string | undefined>
  >(ORDER_QUERY_KEYS);
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detail, setDetail] = useState<OrderDetailDTO | null>(null);
  const [editForm] = Form.useForm();
  const [itemModal, setItemModal] = useState<{
    open: boolean;
    row?: OrderItemRow | null;
  }>({ open: false });
  const [itemForm] = Form.useForm();
  const [shipModal, setShipModal] = useState<{
    open: boolean;
    row?: OrderShipmentRow | null;
  }>({ open: false });
  const [shipForm] = Form.useForm();
  const [shopOptions, setShopOptions] = useState<
    { label: string; value: string }[]
  >([]);
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const warehouseOptions = useMemo(
    () => warehouses.map((warehouse) => ({
      label: trFormat("{value0} ({value1}){value2}", {
        value0: warehouse.name,
        value1: warehouse.code,
        value2: warehouse.isDefault ? tr(" / 默认") : "",
      }),
      value: warehouse.id,
      isDefault: warehouse.isDefault,
    })),
    [language, warehouses],
  );
  const { search: ordersSearch } = useLocation();
  const [createInvDefaults, setCreateInvDefaults] = useState<{
    deduct: boolean;
    sync: boolean;
  }>({
    deduct: false,
    sync: false,
  });
  const [invEffectRows, setInvEffectRows] = useState<OrderInventoryEffectRow[]>(
    [],
  );
  const [invActionLoading, setInvActionLoading] = useState(false);
  const detailIdRef = useRef<string | undefined>();

  const invEffectFailures = useMemo(
    () => invEffectRows.filter((r) => r.status === "failed"),
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
        const res = await listInventoryWarehouses();
        setWarehouses(res.list.filter((warehouse) => warehouse.status === "active"));
      } catch {
        setWarehouses([]);
      }
    })();
  }, []);

  useEffect(() => {
    if (!mayReadSettings) return;
    void (async () => {
      try {
        const { items } = await fetchSettingsList();
        const g = pickGroup(items, "inventory");
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
  }, [mayReadSettings]);

  const refreshDetail = useCallback(
    async (id?: string) => {
      const oid = id ?? detailIdRef.current;
      if (!oid) return;
      const d = await getOrder(oid);
      setDetail(d);
      editForm.setFieldsValue({
        customerName: d.customerName,
        customerEmail: d.customerEmail,
        customerPhone: d.customerPhone,
        destinationCountryCode: d.destinationCountryCode,
        destinationRegion: d.destinationRegion,
        destinationPostalCode: d.destinationPostalCode,
        status: d.status,
        paymentStatus: d.paymentStatus,
        fulfillmentStatus: d.fulfillmentStatus,
        currency: d.currency,
        totalAmount: d.totalAmount,
        shopId: d.shopId,
        warehouseId: d.warehouseId,
      });
    },
    [editForm],
  );

  const loadInvEffects = useCallback(async (orderId: string) => {
    try {
      const r = await getOrderInventoryEffects(orderId, {
        page: 1,
        pageSize: 100,
      });
      setInvEffectRows(r.list);
    } catch (e: unknown) {
      message.error(tr((e as Error)?.message || tr("加载库存影响失败")));
    }
  }, []);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      paymentStatus: urlState.payStatus,
      skuMatchStatus: urlState.skuStatus,
      inventoryDeductStatus: urlState.inventoryStatus,
      status: urlState.status,
      fulfillmentStatus: urlState.fulfillmentStatus,
      reconciliationStatus: urlState.reconciliationStatus,
      platform: urlState.platform,
      shopId: urlState.shopId,
      createdAt: queryTimeRange(urlState.start, urlState.end),
    });
  }, [
    urlState.fulfillmentStatus,
    urlState.reconciliationStatus,
    urlState.inventoryStatus,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.payStatus,
    urlState.platform,
    urlState.shopId,
    urlState.skuStatus,
    urlState.start,
    urlState.end,
    urlState.status,
  ]);

  useEffect(() => {
    const q = new URLSearchParams(ordersSearch);
    const jid = q.get("jumpOrder")?.trim();
    if (!jid) return;
    history.replace(`/orders/${encodeURIComponent(jid)}`);
  }, [ordersSearch]);

  const columns: ProColumns<OrderListRow>[] = useMemo(
    () => [
      {
        title: tr("关联店铺"),
        dataIndex: "shopId",
        hideInTable: true,
        valueType: "select",
        fieldProps: {
          options: shopOptions,
          allowClear: true,
          showSearch: true,
        },
      },
      {
        title: tr("关键词"),
        dataIndex: "keyword",
        hideInTable: true,
        fieldProps: {
          placeholder: tr("订单号 / 买家 / 平台单号"),
          ...keywordFieldProps,
        },
      },
      { title: tr("订单号"), dataIndex: "orderNo", copyable: true, width: 148 },
      {
        title: tr("外部单号"),
        dataIndex: "externalOrderId",
        width: 140,
        search: false,
        copyable: true,
        ellipsis: true,
        render: (_, r) => r.externalOrderId || "—",
      },
      {
        title: tr("平台"),
        dataIndex: "platform",
        width: 96,
        fieldProps: { allowClear: true },
      },
      {
        title: tr("店铺"),
        dataIndex: "shopName",
        search: false,
        width: 140,
        ellipsis: true,
        render: (_, r) =>
          r.shopName ? (
            <span>
              {r.shopName}
              {r.shopPlatform ? ` / ${r.shopPlatform}` : ""}
            </span>
          ) : (
            "—"
          ),
      },
      { title: tr("客户"), dataIndex: "customerName", ellipsis: true, width: 120 },
      {
        title: tr("订单状态"),
        dataIndex: "status",
        width: 108,
        valueType: "select",
        valueEnum: ORDER_STATUS,
        render: (_, r) => statusTag(r.status, ORDER_STATUS),
      },
      {
        title: tr("支付"),
        dataIndex: "paymentStatus",
        width: 94,
        valueType: "select",
        valueEnum: ORDER_PAYMENT_STATUS,
        render: (_, r) => statusTag(r.paymentStatus, ORDER_PAYMENT_STATUS),
      },
      {
        title: tr("商品数"),
        dataIndex: "itemCount",
        search: false,
        width: 72,
        render: (_, r) => r.itemCount ?? "—",
      },
      {
        title: tr("规格匹配"),
        dataIndex: "skuMatchStatus",
        width: 108,
        valueType: "select",
        valueEnum: ORDER_SKU_MATCH_SUMMARY,
        render: (_, r) => {
          const st = r.skuMatchStatus || "none";
          const cfg =
            ORDER_SKU_MATCH_SUMMARY[st as keyof typeof ORDER_SKU_MATCH_SUMMARY];
          const label = cfg?.text || st;
          return (
            <span>
              <Tag color={cfg?.color}>{label}</Tag>
              {r.skuTotalCount ? (
                <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                  {" "}
                  {r.skuMatchedCount ?? 0}/{r.skuTotalCount}
                </Typography.Text>
              ) : null}
            </span>
          );
        },
      },
      {
        title: tr("库存扣减"),
        dataIndex: "inventoryDeductStatus",
        width: 100,
        valueType: "select",
        valueEnum: ORDER_INVENTORY_DEDUCT_SUMMARY,
        search: false,
        render: (_, r) => {
          const st = r.inventoryDeductStatus || "none";
          const cfg =
            ORDER_INVENTORY_DEDUCT_SUMMARY[
              st as keyof typeof ORDER_INVENTORY_DEDUCT_SUMMARY
            ];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: tr("履约对账"),
        dataIndex: "reconciliationStatus",
        width: 108,
        valueType: "select",
        valueEnum: ORDER_FULFILLMENT_RECONCILIATION_STATUS,
        render: (_, r) => {
          const st = r.reconciliationStatus || "pending";
          const cfg = ORDER_FULFILLMENT_RECONCILIATION_STATUS[st];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: tr("同步"),
        dataIndex: "syncStatus",
        width: 96,
        valueType: "select",
        valueEnum: ORDER_SYNC_SUMMARY,
        search: false,
        render: (_, r) => {
          const st = r.syncStatus || "unknown";
          const cfg = ORDER_SYNC_SUMMARY[st as keyof typeof ORDER_SYNC_SUMMARY];
          return <Tag color={cfg?.color}>{cfg?.text || st}</Tag>;
        },
      },
      {
        title: tr("是否有异常"),
        dataIndex: "hasException",
        hideInTable: true,
        valueType: "select",
        valueEnum: {
          true: { text: tr("有异常") },
          false: { text: tr("无异常") },
        },
      },
      {
        title: tr("异常"),
        dataIndex: "openExceptionCount",
        width: 72,
        search: false,
        render: (_, r) =>
          (r.openExceptionCount ?? 0) > 0 ? (
            <Badge count={r.openExceptionCount} size="small">
              <Tag color="error">{tr("待处理")}</Tag>
            </Badge>
          ) : (
            <Tag>{tr("无")}</Tag>
          ),
      },
      {
        title: tr("履约"),
        dataIndex: "fulfillmentStatus",
        hideInTable: true,
        valueType: "select",
        valueEnum: ORDER_FULFILLMENT_STATUS,
      },
      {
        title: tr("金额"),
        search: false,
        width: 120,
        render: (_, r) => `${r.currency} ${r.totalAmount}`,
      },
      {
        title: tr("物流"),
        dataIndex: "latestShipmentStatus",
        search: false,
        width: 96,
        render: (_, r) =>
          r.latestShipmentStatus
            ? statusTag(r.latestShipmentStatus, ORDER_SHIPMENT_STATUS)
            : "—",
      },
      {
        title: tr("下单时间"),
        dataIndex: "orderedAt",
        search: false,
        width: 160,
        render: (_, r) => (r.orderedAt ? formatDateTime(r.orderedAt) : "—"),
      },
      {
        title: tr("创建时间"),
        dataIndex: "createdAt",
        width: 160,
        valueType: "dateTimeRange",
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: tr("更新时间"),
        dataIndex: "updatedAt",
        width: 160,
        search: false,
        render: (_, r) => (r.updatedAt ? formatDateTime(r.updatedAt) : "—"),
      },
      {
        title: tr("操作"),
        valueType: "option",
        width: 220,
        fixed: "right",
        render: (_, r) => (
          <Space wrap size={4}>
            <a
              onClick={() =>
                history.push(`/orders/${encodeURIComponent(r.id)}`)
              }
            >
              {tr("详情")}
            </a>
            {(r.openExceptionCount ?? 0) > 0 ? (
              <a
                onClick={() =>
                  history.push(
                    appendSourceToUrl(
                      `/orders/exceptions?orderId=${encodeURIComponent(r.id)}`,
                      "order_detail",
                    ),
                  )
                }
              >
                {tr("异常")}
              </a>
            ) : null}
            <a
              onClick={() =>
                history.push(
                  `/orders/sync-tasks?shopId=${encodeURIComponent(r.shopId || "")}`,
                )
              }
            >
              {tr("同步")}
            </a>
          </Space>
        ),
      },
    ],
    [shopOptions, keywordFieldProps, language],
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
        { title: tr("商品标题"), dataIndex: "productTitle", ellipsis: true },
        { title: tr("规格编号"), dataIndex: "skuCode", width: 120 },
        { title: tr("数量"), dataIndex: "quantity", width: 72 },
        { title: tr("单价"), dataIndex: "unitPrice", width: 88 },
        { title: tr("小计"), dataIndex: "totalPrice", width: 88 },
        {
          title: tr("操作"),
          key: "op",
          width: 132,
          render: (_: unknown, row: OrderItemRow) => (
            <Space>
              <a onClick={() => openItemModal(row)}>{tr("编辑")}</a>
              <Popconfirm
                title={tr("删除？")}
                onConfirm={async () => {
                  await deleteOrderItem(detail.id, row.id);
                  message.success(tr("已删除"));
                  await refreshDetail();
                }}
              >
                <a>{tr("删除")}</a>
              </Popconfirm>
            </Space>
          ),
        },
      ]
    : [];

  const shipColumns = detail
    ? [
        { title: tr("承运商"), dataIndex: "carrier", width: 110 },
        { title: tr("运单号"), dataIndex: "trackingNo", width: 150 },
        {
          title: tr("状态"),
          dataIndex: "status",
          width: 94,
          render: (v: string) => statusTag(v, ORDER_SHIPMENT_STATUS),
        },
        {
          title: tr("追踪"),
          dataIndex: "trackingUrl",
          render: (u: string) =>
            u ? (
              <a href={u} target="_blank" rel="noopener noreferrer">
                {tr("打开")}
              </a>
            ) : (
              "—"
            ),
        },
        {
          title: tr("操作"),
          width: 132,
          render: (_: unknown, row: OrderShipmentRow) => (
            <Space>
              <a onClick={() => openShipModal(row)}>{tr("编辑")}</a>
              <Popconfirm
                title={tr("删除？")}
                onConfirm={async () => {
                  await deleteOrderShipment(detail.id, row.id);
                  message.success(tr("已删除"));
                  await refreshDetail();
                }}
              >
                <a>{tr("删除")}</a>
              </Popconfirm>
            </Space>
          ),
        },
      ]
    : [];

  const inventoryEffectCols = useMemo(
    () => [
      {
        title: tr("规格编号"),
        dataIndex: "productSkuId",
        ellipsis: true,
        width: 120,
      },
      { title: tr("仓库"), dataIndex: "warehouseId", ellipsis: true, width: 120 },
      { title: tr("类型"), dataIndex: "effectType", width: 100 },
      { title: tr("状态"), dataIndex: "status", width: 92 },
      { title: tr("数量"), dataIndex: "quantity", width: 64 },
      {
        title: tr("原因 / 错误"),
        key: "msg",
        ellipsis: true,
        render: (_: unknown, r: OrderInventoryEffectRow) =>
          r.errorMessage || r.reason || "—",
      },
      {
        title: tr("时间"),
        dataIndex: "createdAt",
        width: 152,
        render: (v: string) => formatDateTime(v),
      },
    ],
    [language],
  );

  return (
    <TmPageContainer
      title={PAGE_COPY.orderList.title}
      subTitle={PAGE_COPY.orderList.description}
    >
      <KeywordSafetyHint visible={showSensitiveHint} />
      <ProTable<OrderListRow>
        rowKey="id"
        locale={emptyLocale}
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        params={{
          current: tablePage,
          pageSize: tablePageSize,
          keyword: urlState.keyword,
          paymentStatus: urlState.payStatus,
          skuMatchStatus: urlState.skuStatus,
          inventoryDeductStatus: urlState.inventoryStatus,
          status: urlState.status,
          fulfillmentStatus: urlState.fulfillmentStatus,
          reconciliationStatus: urlState.reconciliationStatus,
          platform: urlState.platform,
          shopId: urlState.shopId,
          start: urlState.start,
          end: urlState.end,
        }}
        search={{ layout: "vertical", defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(ORDER_QUERY_KEYS, { replace: true });
        }}
        toolBarRender={() => canWriteOrders ? [
          <ModalForm
            key={`c-${createInvDefaults.deduct}-${createInvDefaults.sync}`}
            initialValues={{
              deductInventory: createInvDefaults.deduct,
              syncInventory: createInvDefaults.sync,
              warehouseId: warehouseOptions.find(
                (warehouse) => warehouse.isDefault,
              )?.value,
            }}
            title={tr("新建手工订单")}
            trigger={<Button type="primary">{tr("新建订单")}</Button>}
            onFinish={async (vals) => {
              try {
                await createOrder(vals as Record<string, unknown>);
                message.success(tr("已创建"));
              } catch (error) {
                const partial = partialOrderCreateFromError(error);
                if (!partial) throw error;
                message.warning(
                  tr("订单已创建，但库存处理失败，请打开订单详情重试库存操作"),
                );
              }
              actionRef.current?.reload();
              return true;
            }}
          >
            <ProFormText
              name="platform"
              label={tr("平台")}
              placeholder="manual"
              extra={tr("手工订单可填 manual 或留空")}
            />
            <ProFormSelect
              name="shopId"
              label={tr("关联店铺（可选）")}
              options={shopOptions}
              fieldProps={{ allowClear: true, showSearch: true }}
            />
            <ProFormText
              name="orderNo"
              label={tr("订单号")}
              rules={[{ required: true }]}
            />
            <ProFormSelect
              name="warehouseId"
              label={tr("履约仓库")}
              options={warehouseOptions}
              rules={[{ required: true, message: tr("请选择履约仓库") }]}
              fieldProps={{ showSearch: true, optionFilterProp: "label" }}
            />
            <ProFormText
              name="customerName"
              label={tr("客户名称")}
              rules={[{ required: true }]}
            />
            <ProFormText name="customerEmail" label={tr("邮箱")} />
            <ProFormText name="customerPhone" label={tr("电话")} />
            <ProFormText
              name="destinationCountryCode"
              label={tr("目的国家/地区代码")}
              placeholder={tr("例如 CN")}
              fieldProps={{ maxLength: 2 }}
              rules={[
                {
                  pattern: /^[A-Za-z]{2}$/,
                  message: tr("请输入 2 位国家/地区代码"),
                },
              ]}
            />
            <ProFormText
              name="destinationRegion"
              label={tr("目的省/州")}
              fieldProps={{ maxLength: 120 }}
            />
            <ProFormText
              name="destinationPostalCode"
              label={tr("目的邮编")}
              fieldProps={{ maxLength: 32 }}
            />
            <ProFormSelect
              name="status"
              label={tr("订单状态")}
              options={ORDER_STATUS_OPTS}
              initialValue="pending"
            />
            <ProFormSelect
              name="paymentStatus"
              label={tr("支付状态")}
              options={PAY_OPTS}
              initialValue="unpaid"
            />
            <ProFormSelect
              name="fulfillmentStatus"
              label={tr("履约状态")}
              options={FULL_OPTS}
              initialValue="unfulfilled"
            />
            <ProFormText name="currency" label={tr("币种")} initialValue="USD" />
            <ProFormDigit
              name="totalAmount"
              label={tr("订单总额")}
              min={0}
              fieldProps={{ precision: 2 }}
              initialValue={0}
            />
            <ProFormSwitch
              name="deductInventory"
              label={tr("创建后应用预占 / 出库")}
              tooltip={tr("根据订单状态执行预占或实际出库，与库存策略中的手工订单开关联动")}
            />
            <ProFormSwitch
              name="syncInventory"
              label={tr("扣减后触发平台出库同步队列")}
              tooltip={tr("需在策略中放行并具备刊登出库路由")}
            />
          </ModalForm>,
        ] : []}
        request={async (params) => {
          const kw = prepareKeyword(params.keyword);
          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            platform: (params.platform as string | undefined)?.trim(),
            shopId: (params.shopId as string | undefined)?.trim(),
            keyword: kw,
            paymentStatus: (params.paymentStatus as string | undefined)?.trim(),
            skuMatchStatus: (
              params.skuMatchStatus as string | undefined
            )?.trim(),
            inventoryDeductStatus: (
              params.inventoryDeductStatus as string | undefined
            )?.trim(),
            status: (params.status as string | undefined)?.trim(),
            fulfillmentStatus: (
              params.fulfillmentStatus as string | undefined
            )?.trim(),
            reconciliationStatus: (
              params.reconciliationStatus as string | undefined
            )?.trim(),
            hasException:
              params.hasException === "true" || params.hasException === true
                ? true
                : undefined,
            start: typeof params.start === "string" ? params.start : undefined,
            end: typeof params.end === "string" ? params.end : undefined,
          };
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              keyword: qp.keyword,
              payStatus: qp.paymentStatus,
              skuStatus: qp.skuMatchStatus,
              inventoryStatus: qp.inventoryDeductStatus,
              status: qp.status,
              fulfillmentStatus: qp.fulfillmentStatus,
              reconciliationStatus: qp.reconciliationStatus,
              platform: qp.platform,
              shopId: qp.shopId,
              start: qp.start,
              end: qp.end,
              source: urlState.source,
            },
            { replace: true },
          );
          const res = await queryOrders({
            page: qp.page,
            pageSize: qp.pageSize,
            platform: qp.platform,
            shopId: qp.shopId,
            keyword: qp.keyword,
            status: qp.status,
            paymentStatus: qp.paymentStatus,
            fulfillmentStatus: qp.fulfillmentStatus,
            reconciliationStatus:
              qp.reconciliationStatus as OrderListRow["reconciliationStatus"],
            skuMatchStatus: qp.skuMatchStatus,
            inventoryDeductStatus: qp.inventoryDeductStatus,
            hasException: qp.hasException,
            start: qp.start,
            end: qp.end,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
        pagination={{
          current: tablePage,
          pageSize: tablePageSize,
          showSizeChanger: true,
          onChange: (page, pageSize) => {
            setTablePage(page);
            setTablePageSize(pageSize);
            setUrlState({
              page: page > 1 ? page : undefined,
              pageSize: pageSize !== 20 ? pageSize : undefined,
            });
          },
        }}
      />

      <Drawer
        title={detail ? trFormat("订单 {value0}", { value0: detail.orderNo }) : tr("订单详情")}
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
                  text={trFormat("店铺 {value0} ({value1})", { value0: String(detail.shopSummary.shopName), value1: String(detail.shopSummary.platform) })}
                />
              ) : null}
              <Popconfirm
                title={tr("软删除此订单？")}
                onConfirm={async () => {
                  await deleteOrder(detail.id);
                  message.success(tr("已删除"));
                  setDrawerOpen(false);
                  actionRef.current?.reload();
                }}
              >
                <Button danger size="small">
                  {tr("删除")}
                </Button>
              </Popconfirm>
            </Space>
            <Tabs
              onChange={(k) => {
                if (k === "inv") void loadInvEffects(detail.id);
              }}
              items={[
                {
                  key: "b",
                  label: tr("基础"),
                  children: (
                    <Form
                      layout="vertical"
                      form={editForm}
                      onFinish={async (v) => {
                        const payload: Record<string, unknown> = {
                          customerName: v.customerName,
                          customerEmail: v.customerEmail ?? undefined,
                          customerPhone: v.customerPhone ?? undefined,
                          destinationCountryCode:
                            v.destinationCountryCode ?? undefined,
                          destinationRegion: v.destinationRegion ?? undefined,
                          destinationPostalCode:
                            v.destinationPostalCode ?? undefined,
                          status: v.status,
                          paymentStatus: v.paymentStatus,
                          fulfillmentStatus: v.fulfillmentStatus,
                          currency: v.currency,
                          totalAmount: v.totalAmount,
                        };
                        const sid = v.shopId as string | undefined;
                        if (sid === undefined || sid === null || sid === "") {
                          payload.setShopIdNil = true;
                        } else {
                          payload.shopId = sid;
                        }
                        const warehouseId = v.warehouseId as string | undefined;
                        if (warehouseId) payload.warehouseId = warehouseId;
                        else payload.setWarehouseIdNil = true;
                        await updateOrder(detail.id, payload);
                        message.success(tr("已保存"));
                        await refreshDetail();
                        actionRef.current?.reload();
                      }}
                    >
                      <Form.Item
                        name="customerName"
                        label={tr("客户名称")}
                        rules={[{ required: true }]}
                      >
                        <Input />
                      </Form.Item>
                      <Form.Item name="customerEmail" label={tr("邮箱")}>
                        <Input />
                      </Form.Item>
                      <Form.Item name="customerPhone" label={tr("电话")}>
                        <Input />
                      </Form.Item>
                      <Form.Item
                        name="destinationCountryCode"
                        label={tr("目的国家/地区代码")}
                        rules={[
                          {
                            pattern: /^[A-Za-z]{2}$/,
                            message: tr("请输入 2 位国家/地区代码"),
                          },
                        ]}
                      >
                        <Input maxLength={2} placeholder={tr("例如 CN")} />
                      </Form.Item>
                      <Form.Item name="destinationRegion" label={tr("目的省/州")}>
                        <Input maxLength={120} />
                      </Form.Item>
                      <Form.Item name="destinationPostalCode" label={tr("目的邮编")}>
                        <Input maxLength={32} />
                      </Form.Item>
                      <Form.Item name="shopId" label={tr("关联店铺")}>
                        <Select
                          allowClear
                          showSearch
                          optionFilterProp="label"
                          placeholder={tr("可选")}
                          options={shopOptions}
                        />
                      </Form.Item>
                      <Form.Item
                        name="warehouseId"
                        label={tr("履约仓库")}
                        rules={[{ required: true, message: tr("请选择履约仓库") }]}
                      >
                        <Select
                          showSearch
                          optionFilterProp="label"
                          options={warehouseOptions}
                        />
                      </Form.Item>
                      <Form.Item
                        name="status"
                        label={tr("订单状态")}
                        rules={[{ required: true }]}
                      >
                        <Select options={ORDER_STATUS_OPTS} />
                      </Form.Item>
                      <Form.Item
                        name="paymentStatus"
                        label={tr("支付")}
                        rules={[{ required: true }]}
                      >
                        <Select options={PAY_OPTS} />
                      </Form.Item>
                      <Form.Item
                        name="fulfillmentStatus"
                        label={tr("履约")}
                        rules={[{ required: true }]}
                      >
                        <Select options={FULL_OPTS} />
                      </Form.Item>
                      <Form.Item
                        name="currency"
                        label={tr("币种")}
                        rules={[{ required: true }]}
                      >
                        <Input style={{ width: 120 }} />
                      </Form.Item>
                      <Form.Item
                        name="totalAmount"
                        label={tr("总额")}
                        rules={[{ required: true }]}
                      >
                        <InputNumber style={{ width: "100%" }} min={0} />
                      </Form.Item>
                      <Button type="primary" htmlType="submit">
                        {tr("保存")}
                      </Button>
                    </Form>
                  ),
                },
                {
                  key: "i",
                  label: tr("商品明细"),
                  children: (
                    <>
                      {!detail.warehouseId ? (
                        <Alert
                          type="warning"
                          showIcon
                          style={{ marginBottom: 12 }}
                          message={tr("请先在基础信息中选择履约仓库")}
                        />
                      ) : null}
                      <Button
                        type="primary"
                        style={{ marginBottom: 8 }}
                        onClick={() => openItemModal()}
                      >
                        {tr("添加明细")}
                      </Button>
                      <Table<OrderItemRow>
                        rowKey="id"
                        columns={itemColumns as never}
                        dataSource={detail.items}
                        pagination={false}
                      />
                    </>
                  ),
                },
                {
                  key: "s",
                  label: tr("物流"),
                  children: (
                    <>
                      <Button
                        type="primary"
                        style={{ marginBottom: 8 }}
                        onClick={() => openShipModal()}
                      >
                        {tr("添加物流")}
                      </Button>
                      <Table<OrderShipmentRow>
                        rowKey="id"
                        columns={shipColumns as never}
                        dataSource={detail.shipments}
                        pagination={false}
                      />
                    </>
                  ),
                },
                {
                  key: "inv",
                  label: tr("库存"),
                  children: (
                    <>
                      {detail && invEffectFailures.length > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          style={{ marginBottom: 12 }}
                          message={tr("存在失败的库存扣减或恢复记录")}
                          description={
                            <span>
                              {tr("请在异常工作台查看是否需要重新绑定\r\n                              SKU、补库存或重试扣减。")}{" "}
                              <Typography.Link
                                onClick={() =>
                                  history.push(
                                    `/orders/exceptions?orderId=${encodeURIComponent(detail.id)}`,
                                  )
                                }
                              >
                                {tr("打开异常工作台")}
                              </Typography.Link>
                            </span>
                          }
                        />
                      ) : null}
                      <Space wrap style={{ marginBottom: 12 }}>
                        {detail.inventorySummary ? (
                          <>
                            <Tag
                              color={
                                detail.inventorySummary.hasReservationSuccess
                                  ? "processing"
                                  : "default"
                              }
                            >
                              {tr("预占")}
                              {detail.inventorySummary.hasReservationSuccess
                                ? tr("：已有成功记录")
                                : tr("：尚未成功")}
                            </Tag>
                            <Tag
                              color={
                                detail.inventorySummary.hasDeductionSuccess
                                  ? "success"
                                  : "default"
                              }
                            >
                              {tr("出库")}
                              {detail.inventorySummary.hasDeductionSuccess
                                ? tr("：已有成功记录")
                                : tr("：尚未成功")}
                            </Tag>
                            <Tag
                              color={
                                detail.inventorySummary.hasReleaseSuccess
                                  ? "warning"
                                  : "default"
                              }
                            >
                              {tr("释放")}
                              {detail.inventorySummary.hasReleaseSuccess
                                ? tr("：已有成功记录")
                                : tr("：未记录")}
                            </Tag>
                            <Tag
                              color={
                                detail.inventorySummary.hasRestoreSuccess
                                  ? "processing"
                                  : "default"
                              }
                            >
                              {tr("回补")}
                              {detail.inventorySummary.hasRestoreSuccess
                                ? tr("：有过成功记录")
                                : tr("：未记录")}
                            </Tag>
                          </>
                        ) : (
                          <Tag>{tr("库存摘要不可用")}</Tag>
                        )}
                        <Popconfirm
                          title={tr("扣减绑定 SKU 的本地库存（幂等；见错误提示）")}
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, {
                                syncInventory: false,
                                warehouseId: detail.warehouseId,
                              });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(
                                  r.inventoryDeduction as Record<
                                    string,
                                    unknown
                                  >,
                                ),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error(tr((e as Error)?.message || tr("失败")));
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button
                            size="small"
                            loading={invActionLoading}
                            disabled={!detail.warehouseId}
                          >
                            {tr("应用预占 / 出库")}
                          </Button>
                        </Popconfirm>
                        <Popconfirm
                          title={tr("扣库存并触发平台出库同步队列（仍需刊登与 outbound 就绪）")}
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await deductOrderInventory(detail.id, {
                                syncInventory: true,
                                warehouseId: detail.warehouseId,
                              });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(
                                  r.inventoryDeduction as Record<
                                    string,
                                    unknown
                                  >,
                                ),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error(tr((e as Error)?.message || tr("失败")));
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button
                            size="small"
                            loading={invActionLoading}
                            disabled={!detail.warehouseId}
                          >
                            {tr("应用出库 + 推平台任务")}
                          </Button>
                        </Popconfirm>
                        <Popconfirm
                          title={tr("回滚本订单已成功扣掉的库存（需尚未被标记为「已完全对冲」等特殊状态）")}
                          onConfirm={async () => {
                            setInvActionLoading(true);
                            try {
                              const r = await restoreOrderInventory(detail.id, {
                                syncInventory: false,
                                reason: "manual_ui",
                                warehouseId: detail.warehouseId,
                              });
                              setDetail(r.order);
                              message.success(
                                summarizeInvResp(
                                  r.inventoryRestoration as Record<
                                    string,
                                    unknown
                                  >,
                                ),
                              );
                              await loadInvEffects(detail.id);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error(tr((e as Error)?.message || tr("失败")));
                            } finally {
                              setInvActionLoading(false);
                            }
                          }}
                        >
                          <Button
                            size="small"
                            danger
                            loading={invActionLoading}
                            disabled={!detail.warehouseId}
                          >
                            {tr("释放 / 回补库存")}
                          </Button>
                        </Popconfirm>
                      </Space>
                      <Space wrap style={{ marginBottom: 8 }}>
                        <Typography.Link
                          href={`/inventory/effects?orderId=${encodeURIComponent(detail.id)}`}
                        >
                          {tr("全局影响流水")}
                        </Typography.Link>
                        <Typography.Link
                          href={`/inventory/logs?orderId=${encodeURIComponent(detail.id)}`}
                        >
                          {tr("全局库存变更")}
                        </Typography.Link>
                      </Space>
                      <Typography.Paragraph
                        type="secondary"
                        style={{ fontSize: 12 }}
                      >
                        {tr("策略见「设置 → 库存 /\r\n                        订单」。平台同步失败不参与本地数据库事务。")}
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
                  key: "sku",
                  label: tr("规格匹配"),
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
        title={itemModal.row ? tr("编辑明细") : tr("新增明细")}
        open={itemModal.open}
        onCancel={() => setItemModal({ open: false })}
        destroyOnHidden
        onOk={async () => {
          const v = await itemForm.validateFields();
          if (!detail) return;
          if (itemModal.row)
            await updateOrderItem(
              detail.id,
              itemModal.row.id,
              v as Record<string, unknown>,
            );
          else await createOrderItem(detail.id, v as Record<string, unknown>);
          message.success(tr("已保存"));
          setItemModal({ open: false });
          await refreshDetail();
        }}
      >
        <Form form={itemForm} layout="vertical">
          <Form.Item
            name="productTitle"
            label={tr("标题")}
            rules={[{ required: true, message: tr("必填") }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="skuCode" label={tr("规格编码")}>
            <Input />
          </Form.Item>
          <Form.Item name="skuName" label={tr("规格名称")}>
            <Input />
          </Form.Item>
          <Form.Item
            name="quantity"
            label={tr("数量")}
            initialValue={1}
            rules={[{ required: true }]}
          >
            <InputNumber min={1} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="unitPrice" label={tr("单价")}>
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="totalPrice" label={tr("小计")}>
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={shipModal.row ? tr("编辑物流") : tr("新增物流")}
        open={shipModal.open}
        onCancel={() => setShipModal({ open: false })}
        destroyOnHidden
        onOk={async () => {
          const v = await shipForm.validateFields();
          if (!detail) return;
          if (shipModal.row)
            await updateOrderShipment(
              detail.id,
              shipModal.row.id,
              v as Record<string, unknown>,
            );
          else
            await createOrderShipment(detail.id, v as Record<string, unknown>);
          message.success(tr("已保存"));
          setShipModal({ open: false });
          await refreshDetail();
        }}
      >
        <Form form={shipForm} layout="vertical">
          <Form.Item name="carrier" label={tr("承运商")} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="trackingNo" label={tr("运单号")}>
            <Input />
          </Form.Item>
          <Form.Item name="trackingUrl" label={tr("追踪 URL")}>
            <Input />
          </Form.Item>
          <Form.Item
            name="status"
            label={tr("状态")}
            rules={[{ required: true }]}
            initialValue="pending"
          >
            <Select options={SHIP_OPTS} />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
