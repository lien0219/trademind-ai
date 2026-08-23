import {
  TmPageContainer,
  TechnicalDetails,
  TaskJsonBlock,
} from "@/components/ui";
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  Modal,
  Row,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from "antd";
import { formatDateTime } from "@/utils/formatTime";
import { appendSourceToUrl } from "@/utils/urlState";
import { history, useModel, useParams, useSearchParams } from "@umijs/max";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ORDER_FULFILLMENT_STATUS,
  ORDER_FULFILLMENT_RECONCILIATION_ISSUES,
  ORDER_FULFILLMENT_RECONCILIATION_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_ITEM_SKU_MATCH_STATUS,
  ORDER_PAYMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  ORDER_SYNC_SUMMARY,
} from "@/constants/status";
import {
  getOrder,
  getOrderInventoryEffects,
  getOrderSKUMatches,
  getOrderFulfillmentReconciliation,
  fulfillOrder,
  type OrderDetailDTO,
  type OrderSkuMatchRow,
  type FulfillmentReconciliation,
} from "@/services/orders";
import {
  createInventoryIdempotencyKey,
  type OrderInventoryEffectRow,
} from "@/services/inventory";
import OrderSkuMatchTab from "@/pages/Orders/SkuMatchTab";
import SalesReturnCreateModal from "@/pages/SalesReturns/CreateModal";
import { PRODUCT_COPY } from "@/constants/copywriting";
import {
  INVENTORY_DEDUCT_STATUS,
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
  inventoryTagFromMap,
} from "@/constants/inventoryLabels";
import { canWriteOrders } from "@/utils/orderPerm";
import { hasPermission, PERMISSIONS } from "@/utils/permission";

function tagFromMap(
  raw: string,
  map: Record<string, { text: string; color: string }>,
) {
  const cfg = map[raw];
  if (!cfg) return <Tag>{raw || "—"}</Tag>;
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

function fulfillmentBlockReason(
  order: OrderDetailDTO,
  writable: boolean,
): string | undefined {
  if (!writable) return "当前账号没有订单操作权限";
  if (!order.warehouseId) return "请先绑定履约仓库";
  if (order.paymentStatus !== "paid") return "订单必须先标记为已支付";
  if (
    order.status === "cancelled" ||
    order.status === "refunded" ||
    order.status === "closed" ||
    order.status === "shipped" ||
    order.status === "delivered"
  ) {
    return "订单已取消、退款、关闭或发货，不能重复履约";
  }
  if (order.fulfillmentStatus === "fulfilled") return "订单已完成履约";
  if (
    order.items.length === 0 ||
    order.items.some((item) => !item.productSkuId || item.quantity <= 0)
  ) {
    return "请先为所有订单明细绑定本地 SKU";
  }
  return undefined;
}

export default function OrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const itemIdFocus = searchParams.get("itemId")?.trim();
  const { initialState } = useModel("@@initialState") as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const writable = canWriteOrders(
    initialState?.currentUser?.role,
    initialState?.currentUser?.permissions,
  );
  const canViewSalesReturns = hasPermission(
    initialState?.currentUser?.role,
    PERMISSIONS.SALES_RETURN_VIEW,
    initialState?.currentUser?.permissions,
  );
  const canManageSalesReturns = hasPermission(
    initialState?.currentUser?.role,
    PERMISSIONS.SALES_RETURN_MANAGE,
    initialState?.currentUser?.permissions,
  );

  const [detail, setDetail] = useState<OrderDetailDTO | null>(null);
  const [skuRows, setSkuRows] = useState<OrderSkuMatchRow[]>([]);
  const [invRows, setInvRows] = useState<OrderInventoryEffectRow[]>([]);
  const [reconciliation, setReconciliation] =
    useState<FulfillmentReconciliation | null>(null);
  const [reconciliationLoading, setReconciliationLoading] = useState(false);
  const [reconciliationError, setReconciliationError] = useState<string | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState("overview");
  const [salesReturnOpen, setSalesReturnOpen] = useState(false);
  const [fulfillModalOpen, setFulfillModalOpen] = useState(false);
  const [fulfillForm] = Form.useForm();
  const [fulfillLoading, setFulfillLoading] = useState(false);
  const [fulfillIdempotencyKey, setFulfillIdempotencyKey] = useState("");

  const fulfillmentDisabledReason = detail
    ? fulfillmentBlockReason(detail, writable)
    : "订单详情不可用";

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [d, sku, inv] = await Promise.all([
        getOrder(id),
        getOrderSKUMatches(id),
        getOrderInventoryEffects(id, { page: 1, pageSize: 100 }),
      ]);
      setDetail(d);
      setSkuRows(sku.items ?? []);
      setInvRows(inv.list ?? []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || "加载订单失败");
    } finally {
      setLoading(false);
    }
  }, [id]);

  const loadReconciliation = useCallback(async () => {
    if (!id) return;
    setReconciliationLoading(true);
    try {
      setReconciliation(await getOrderFulfillmentReconciliation(id));
      setReconciliationError(null);
    } catch (e: unknown) {
      const errorMessage = (e as Error)?.message || "加载履约对账失败";
      setReconciliationError(errorMessage);
      message.error(errorMessage);
      setReconciliation(null);
    } finally {
      setReconciliationLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (itemIdFocus) setActiveTab("sku");
  }, [itemIdFocus]);

  useEffect(() => {
    const tab = searchParams.get("tab")?.trim();
    if (tab === "inventory" || tab === "inv") setActiveTab("inv");
    else if (tab === "reconciliation" || tab === "fulfillment-reconciliation") {
      setActiveTab("reconciliation");
    }
    else if (tab === "sku") setActiveTab("sku");
    else if (tab === "exceptions") setActiveTab("exceptions");
  }, [searchParams]);

  useEffect(() => {
    if (activeTab === "reconciliation") void loadReconciliation();
  }, [activeTab, loadReconciliation]);

  const listSummary = useMemo(() => {
    if (!detail) return null;
    const matched = skuRows.filter((r) =>
      ["matched", "manual_bound"].includes(String(r.matchStatus)),
    ).length;
    const total = skuRows.length || detail.items.length;
    let skuStatus = "none";
    if (total > 0) {
      if (skuRows.some((r) => r.matchStatus === "ambiguous"))
        skuStatus = "ambiguous";
      else if (skuRows.some((r) => r.matchStatus === "unmatched"))
        skuStatus = "unmatched";
      else if (matched >= total) skuStatus = "all_matched";
      else skuStatus = "partial";
    }
    const invFailed = invRows.some((r) => r.status === "failed");
    const invOk = invRows.some(
      (r) => r.effectType === "deduct" && r.status === "success",
    );
    let invStatus = "none";
    if (skuStatus === "unmatched" || skuStatus === "ambiguous")
      invStatus = "blocked";
    else if (invOk && invFailed) invStatus = "partial";
    else if (invFailed) invStatus = "failed";
    else if (invOk) invStatus = "success";
    const syncSt =
      detail.platform === "manual" || !detail.externalOrderId
        ? "manual"
        : detail.externalOrderId
          ? "synced"
          : "unknown";
    return { skuStatus, invStatus, syncSt, matched, total };
  }, [detail, skuRows, invRows]);

  if (!id) {
    return (
      <TmPageContainer title="订单详情">
        <Alert type="error" message="缺少订单 ID" />
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer
      title={detail ? `订单 ${detail.orderNo}` : "订单详情"}
      loading={loading}
      onBack={() => {
        if (window.history.length > 1) {
          history.goBack();
          return;
        }
        history.push("/orders/list");
      }}
      extra={
        <Space wrap>
          <Button
            type="primary"
            disabled={Boolean(fulfillmentDisabledReason) || fulfillLoading}
            title={fulfillmentDisabledReason}
            onClick={() => {
              if (!detail) return;
              fulfillForm.resetFields();
              setFulfillIdempotencyKey(
                createInventoryIdempotencyKey("order-fulfillment"),
              );
              setFulfillModalOpen(true);
            }}
          >
            确认发货
          </Button>
          {canViewSalesReturns ? (
            <Button
              onClick={() =>
                history.push(
                  `/orders/sales-returns?orderId=${encodeURIComponent(id!)}`,
                )
              }
            >
              售后记录
            </Button>
          ) : null}
          {canManageSalesReturns ? (
            <Button type="primary" onClick={() => setSalesReturnOpen(true)}>
              发起售后
            </Button>
          ) : null}
          <Button
            onClick={() =>
              history.push(
                appendSourceToUrl(
                  `/orders/exceptions?orderId=${encodeURIComponent(id!)}`,
                  "order_detail",
                ),
              )
            }
          >
            异常工作台
          </Button>
          <Button
            onClick={() =>
              history.push("/ops/task-center/failures?taskType=inventory_sync")
            }
          >
            失败任务中心
          </Button>
          <Button
            onClick={() =>
              history.push(
                appendSourceToUrl(
                  `/inventory/deductions?orderId=${encodeURIComponent(id!)}`,
                  "order_detail",
                ),
              )
            }
          >
            扣减记录
          </Button>
          <Button type="link" onClick={() => history.goBack()}>
            返回列表
          </Button>
        </Space>
      }
    >
      {detail?.shopSummary?.authStatus === "unauthorized" ||
      detail?.shopSummary?.authStatus === "expired" ? (
        <Alert
          showIcon
          type="warning"
          style={{ marginBottom: 16 }}
          message="抖店/平台凭证待真实授权"
          description="未配置真实店铺凭证时，平台订单不会同步成功；请先完成店铺授权并通过连接检查。"
        />
      ) : null}

      {detail && fulfillmentDisabledReason ? (
        <Alert
          showIcon
          type="info"
          style={{ marginBottom: 16 }}
          message={fulfillmentDisabledReason}
        />
      ) : null}

      {detail &&
      listSummary &&
      listSummary.total > 0 &&
      listSummary.matched < listSummary.total ? (
        <Alert
          showIcon
          type="info"
          style={{ marginBottom: 16 }}
          message="存在未完全匹配的 SKU"
          description={
            <span>
              请先在「规格匹配」Tab 人工确认候选或绑定本地 SKU，再执行库存扣减。{" "}
              <Typography.Link onClick={() => setActiveTab("sku")}>
                前往规格匹配
              </Typography.Link>
            </span>
          }
        />
      ) : null}

      {detail ? (
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: "overview",
              label: "订单概览",
              children: (
                <Row gutter={[16, 16]}>
                  <Col span={24}>
                    <Card size="small" title="基本信息">
                      <Descriptions
                        column={{ xs: 1, sm: 2, md: 3 }}
                        size="small"
                        bordered
                      >
                        <Descriptions.Item label="订单号">
                          {detail.orderNo}
                        </Descriptions.Item>
                        <Descriptions.Item label="平台订单号">
                          {detail.externalOrderId || "—"}
                        </Descriptions.Item>
                        <Descriptions.Item label="平台">
                          {detail.platform}
                        </Descriptions.Item>
                        <Descriptions.Item label="店铺">
                          {detail.shopSummary?.shopName || "—"}
                          {detail.shopSummary?.platform
                            ? ` (${detail.shopSummary.platform})`
                            : ""}
                        </Descriptions.Item>
                        <Descriptions.Item label="履约仓库">
                          {detail.warehouseId || "—"}
                        </Descriptions.Item>
                        <Descriptions.Item label="订单状态">
                          {tagFromMap(detail.status, ORDER_STATUS)}
                        </Descriptions.Item>
                        <Descriptions.Item label="付款状态">
                          {tagFromMap(
                            detail.paymentStatus,
                            ORDER_PAYMENT_STATUS,
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="履约状态">
                          {tagFromMap(
                            detail.fulfillmentStatus,
                            ORDER_FULFILLMENT_STATUS,
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="金额">
                          {detail.currency} {detail.totalAmount}
                        </Descriptions.Item>
                        <Descriptions.Item label="下单时间">
                          {detail.orderedAt
                            ? formatDateTime(detail.orderedAt)
                            : "—"}
                        </Descriptions.Item>
                        <Descriptions.Item label="创建时间">
                          {formatDateTime(detail.createdAt)}
                        </Descriptions.Item>
                        <Descriptions.Item label="更新时间">
                          {formatDateTime(detail.updatedAt)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title="规格匹配">
                      {listSummary
                        ? tagFromMap(
                            listSummary.skuStatus,
                            ORDER_SKU_MATCH_SUMMARY,
                          )
                        : "—"}
                      <div style={{ marginTop: 8 }}>
                        {listSummary
                          ? `${listSummary.matched}/${listSummary.total} 行已匹配`
                          : ""}
                      </div>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title="库存影响">
                      {listSummary
                        ? tagFromMap(
                            listSummary.invStatus,
                            ORDER_INVENTORY_DEDUCT_SUMMARY,
                          )
                        : "—"}
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title="同步状态">
                      {listSummary
                        ? tagFromMap(listSummary.syncSt, ORDER_SYNC_SUMMARY)
                        : "—"}
                    </Card>
                  </Col>
                </Row>
              ),
            },
            {
              key: "buyer",
              label: "买家信息",
              children: (
                <Descriptions column={1} bordered size="small">
                  <Descriptions.Item label="买家">
                    {detail.customerName}
                  </Descriptions.Item>
                  <Descriptions.Item label="邮箱">
                    {detail.customerEmail || "—"}
                  </Descriptions.Item>
                  <Descriptions.Item label="电话">
                    {detail.customerPhone || "—"}
                  </Descriptions.Item>
                  <Descriptions.Item label="说明">
                    <Typography.Text type="secondary">
                      联系方式已脱敏展示；完整信息需相应权限。
                    </Typography.Text>
                  </Descriptions.Item>
                </Descriptions>
              ),
            },
            {
              key: "items",
              label: "商品明细",
              children: (
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={detail.items}
                  rowClassName={(r) =>
                    itemIdFocus && r.id === itemIdFocus
                      ? "ant-table-row-selected"
                      : ""
                  }
                  columns={[
                    {
                      title: "平台商品 ID",
                      dataIndex: "externalItemId",
                      width: 120,
                      render: (v) => v || "—",
                    },
                    {
                      title: "平台规格编号",
                      dataIndex: "externalSkuId",
                      width: 120,
                      render: (v) => v || "—",
                    },
                    {
                      title: "标题",
                      dataIndex: "productTitle",
                      ellipsis: true,
                    },
                    {
                      title: "规格",
                      dataIndex: "skuName",
                      width: 120,
                      render: (v) => v || "—",
                    },
                    { title: "数量", dataIndex: "quantity", width: 64 },
                    { title: "单价", dataIndex: "unitPrice", width: 88 },
                    {
                      title: "匹配状态",
                      width: 108,
                      render: (_, row) => {
                        const m = skuRows.find((s) => s.orderItemId === row.id);
                        return tagFromMap(
                          String(m?.matchStatus || ""),
                          ORDER_ITEM_SKU_MATCH_STATUS,
                        );
                      },
                    },
                    {
                      title: "本地规格编号",
                      width: 120,
                      render: (_, row) => {
                        const m = skuRows.find((s) => s.orderItemId === row.id);
                        return m?.localSkuCode || row.skuCode || "—";
                      },
                    },
                    {
                      title: "置信度",
                      width: 72,
                      render: (_, row) => {
                        const m = skuRows.find((s) => s.orderItemId === row.id);
                        return m?.confidence ?? "—";
                      },
                    },
                  ]}
                />
              ),
            },
            {
              key: "sku",
              label: "规格匹配",
              children: (
                <OrderSkuMatchTab
                  orderId={detail.id}
                  onRefreshOrder={load}
                  readOnly={!writable}
                  focusItemId={itemIdFocus}
                />
              ),
            },
            {
              key: "inv",
              label: "库存影响",
              children: (
                <>
                  {listSummary?.invStatus === "blocked" ? (
                    <Alert
                      showIcon
                      type="warning"
                      style={{ marginBottom: 12 }}
                      message="SKU 未就绪，暂不能扣减库存"
                      description={
                        <>
                          {INVENTORY_SKU_NOT_BOUND_MESSAGE}{" "}
                          {INVENTORY_SKU_AMBIGUOUS_MESSAGE}{" "}
                          <Typography.Link onClick={() => setActiveTab("sku")}>
                            前往规格匹配
                          </Typography.Link>
                        </>
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
                          预占
                          {detail.inventorySummary.hasReservationSuccess
                            ? "：已有成功"
                            : "：尚未成功"}
                        </Tag>
                        <Tag
                          color={
                            detail.inventorySummary.hasDeductionSuccess
                              ? "success"
                              : "default"
                          }
                        >
                          出库
                          {detail.inventorySummary.hasDeductionSuccess
                            ? "：已有成功"
                            : "：尚未成功"}
                        </Tag>
                        <Tag
                          color={
                            detail.inventorySummary.hasReleaseSuccess
                              ? "warning"
                              : "default"
                          }
                        >
                          释放
                          {detail.inventorySummary.hasReleaseSuccess
                            ? "：已有成功"
                            : "：未记录"}
                        </Tag>
                        <Tag
                          color={
                            detail.inventorySummary.hasRestoreSuccess
                              ? "processing"
                              : "default"
                          }
                        >
                          回补
                          {detail.inventorySummary.hasRestoreSuccess
                            ? "：有过成功"
                            : "：未记录"}
                        </Tag>
                      </>
                    ) : null}
                    <Typography.Link
                      href={`/inventory/deductions?orderId=${encodeURIComponent(detail.id)}`}
                    >
                      扣减记录
                    </Typography.Link>
                    <Typography.Link
                      href={`/inventory/sync-tasks?orderId=${encodeURIComponent(detail.id)}`}
                    >
                      同步任务
                    </Typography.Link>
                    <Typography.Link
                      href={`/orders/exceptions?orderId=${encodeURIComponent(detail.id)}`}
                    >
                      库存异常
                    </Typography.Link>
                  </Space>
                  <Table
                    rowKey="id"
                    size="small"
                    dataSource={invRows}
                    pagination={{ pageSize: 10 }}
                    columns={[
                      {
                        title: "类型",
                        dataIndex: "effectType",
                        width: 100,
                        render: (v) =>
                          v === "deduct"
                            ? "扣减"
                            : v === "restore"
                              ? "回滚"
                              : v,
                      },
                      {
                        title: "状态",
                        dataIndex: "status",
                        width: 88,
                        render: (v) => {
                          const cfg = inventoryTagFromMap(
                            String(v),
                            INVENTORY_DEDUCT_STATUS,
                          );
                          return <Tag color={cfg.color}>{cfg.text}</Tag>;
                        },
                      },
                      {
                        title: PRODUCT_COPY.sku,
                        dataIndex: "skuCode",
                        width: 120,
                        render: (v) => v || "—",
                      },
                      { title: "数量", dataIndex: "quantity", width: 64 },
                      {
                        title: "扣减前",
                        dataIndex: "beforeStock",
                        width: 72,
                        render: (v) => v ?? "—",
                      },
                      {
                        title: "扣减后",
                        dataIndex: "afterStock",
                        width: 72,
                        render: (v) => v ?? "—",
                      },
                      {
                        title: "原因 / 错误",
                        render: (_, r) => r.errorMessage || r.reason || "—",
                        ellipsis: true,
                      },
                      {
                        title: "时间",
                        dataIndex: "createdAt",
                        width: 156,
                        render: (v) => formatDateTime(v),
                      },
                    ]}
                  />
                </>
              ),
            },
            {
              key: "reconciliation",
              label: "履约对账",
              children: reconciliationLoading ? (
                <Alert type="info" message="正在加载履约对账" />
              ) : reconciliationError ? (
                <Alert type="error" message="履约对账加载失败" description={reconciliationError} />
              ) : reconciliation ? (
                <Space direction="vertical" size={16} style={{ width: "100%" }}>
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2, md: 3 }}>
                    <Descriptions.Item label="对账状态">
                      {tagFromMap(
                        reconciliation.reconciliationStatus,
                        ORDER_FULFILLMENT_RECONCILIATION_STATUS,
                      )}
                    </Descriptions.Item>
                    <Descriptions.Item label="订单状态">
                      {tagFromMap(reconciliation.status, ORDER_STATUS)}
                    </Descriptions.Item>
                    <Descriptions.Item label="履约状态">
                      {tagFromMap(reconciliation.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}
                    </Descriptions.Item>
                    <Descriptions.Item label="履约仓库">
                      {reconciliation.warehouseId || "—"}
                    </Descriptions.Item>
                    <Descriptions.Item label="发货单数量">
                      {reconciliation.shipmentCount}
                    </Descriptions.Item>
                    <Descriptions.Item label="库存 effect 数量">
                      {reconciliation.effectCount}
                    </Descriptions.Item>
                    <Descriptions.Item label="最后库存动作">
                      {reconciliation.lastInventoryActionAt
                        ? formatDateTime(reconciliation.lastInventoryActionAt)
                        : "—"}
                    </Descriptions.Item>
                  </Descriptions>
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={false}
                    locale={{ emptyText: "暂无库存或发货事实" }}
                    dataSource={reconciliation.timeline ?? []}
                    columns={[
                      { title: "事实", dataIndex: "type", width: 96 },
                      { title: "动作", dataIndex: "action", width: 150 },
                      { title: "状态", dataIndex: "status", width: 96, render: (v) => v || "—" },
                      { title: "数量", dataIndex: "quantity", width: 72, render: (v) => v ?? "—" },
                      { title: "时间", dataIndex: "createdAt", width: 180, render: (v) => formatDateTime(v) },
                    ]}
                  />
                  <Space wrap>
                    {(["reserve", "deduct", "release", "restore"] as const).map((key) => {
                      const summary = reconciliation[key];
                      return (
                        <Tag key={key} color={summary.expected === summary.actual ? "success" : "error"}>
                          {key}: {summary.actual}/{summary.expected}
                        </Tag>
                      );
                    })}
                    {reconciliation.issues?.length ? (
                      <Typography.Text type="danger">
                        异常：
                        {reconciliation.issues
                          .map(
                            (issue) =>
                              ORDER_FULFILLMENT_RECONCILIATION_ISSUES[issue] || issue,
                          )
                          .join("、")}
                      </Typography.Text>
                    ) : null}
                    {reconciliation.reconciliationStatus === "mismatch" ||
                    reconciliation.reconciliationStatus === "blocked" ? (
                      <Typography.Link
                        href={appendSourceToUrl(
                          `/orders/exceptions?orderId=${encodeURIComponent(detail.id)}`,
                          "order_detail",
                        )}
                      >
                        打开订单异常工作台
                      </Typography.Link>
                    ) : null}
                  </Space>
                </Space>
              ) : (
                <Alert type="warning" message="履约对账暂不可用" />
              ),
            },
            {
              key: "exceptions",
              label: "异常记录",
              children: (
                <Space direction="vertical">
                  <Typography.Paragraph type="secondary">
                    订单相关异常统一在异常工作台处理；此处提供快捷入口。
                  </Typography.Paragraph>
                  <Button
                    type="primary"
                    onClick={() =>
                      history.push(
                        appendSourceToUrl(
                          `/orders/exceptions?orderId=${encodeURIComponent(id!)}`,
                          "order_detail",
                        ),
                      )
                    }
                  >
                    打开该订单的异常工作台
                  </Button>
                </Space>
              ),
            },
            {
              key: "tech",
              label: "技术详情",
              children: (
                <TechnicalDetails>
                  <TaskJsonBlock
                    title="订单 ID"
                    value={{ id: detail.id, tenantId: detail.tenantId }}
                  />
                  <TaskJsonBlock
                    title="原始 items 数量"
                    value={{ count: detail.items.length }}
                    last
                  />
                </TechnicalDetails>
              ),
            },
          ]}
        />
      ) : (
        !loading && (
          <Alert
            type="info"
            message="未找到订单"
            description="请从订单列表重新进入，或检查是否有访问权限。"
          />
        )
      )}
      <Modal
        title="确认发货"
        open={fulfillModalOpen}
        confirmLoading={fulfillLoading}
        onCancel={() => {
          if (!fulfillLoading) setFulfillModalOpen(false);
        }}
        onOk={async () => {
          if (!detail || !fulfillIdempotencyKey || fulfillLoading) return;
          let values: Record<string, unknown>;
          try {
            values = await fulfillForm.validateFields();
          } catch {
            return;
          }
          setFulfillLoading(true);
          try {
            await fulfillOrder(detail.id, {
              idempotencyKey: fulfillIdempotencyKey,
              warehouseId: detail.warehouseId,
              carrier: String(values.carrier || "").trim(),
              trackingNo: String(values.trackingNo || "").trim(),
              trackingUrl: String(values.trackingUrl || "").trim() || undefined,
            });
            setFulfillModalOpen(false);
            message.success("已完成发货并扣减库存");
            await load();
          } catch (e: unknown) {
            message.error((e as Error)?.message || "发货失败");
          } finally {
            setFulfillLoading(false);
          }
        }}
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message="确认后会在履约仓库实际扣减库存，并将订单标记为已发货 / 已履约。"
        />
        <Typography.Paragraph type="secondary">
          履约仓库：{detail?.warehouseId || "未绑定"}
        </Typography.Paragraph>
        <Form form={fulfillForm} layout="vertical">
          <Form.Item
            name="carrier"
            label="承运商"
            rules={[{ required: true, message: "请填写承运商" }]}
          >
            <Input maxLength={128} placeholder="例如：顺丰" />
          </Form.Item>
          <Form.Item
            name="trackingNo"
            label="运单号"
            rules={[{ required: true, message: "请填写运单号" }]}
          >
            <Input maxLength={255} />
          </Form.Item>
          <Form.Item name="trackingUrl" label="追踪 URL">
            <Input maxLength={2048} type="url" />
          </Form.Item>
        </Form>
      </Modal>

      <SalesReturnCreateModal
        orderId={id}
        open={salesReturnOpen}
        onClose={() => setSalesReturnOpen(false)}
        onCreated={(salesReturnId) => {
          setSalesReturnOpen(false);
          history.push(`/orders/sales-returns/${salesReturnId}`);
        }}
      />
    </TmPageContainer>
  );
}
