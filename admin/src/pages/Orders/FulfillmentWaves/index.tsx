import {
  CheckCircleOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileTextOutlined,
  InboxOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  ScanOutlined,
  StopOutlined,
} from "@ant-design/icons";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import { history } from "@umijs/max";
import {
  Alert,
  Button,
  Descriptions,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import AppDrawer from "@/components/AppDrawer";
import PermissionGuard from "@/components/PermissionGuard";
import { ErrorAlert, TmPageContainer, TmProTable } from "@/components/ui";
import { usePermission } from "@/hooks/usePermission";
import { useUrlDrawerState } from "@/hooks/useUrlState";
import {
  cancelFulfillmentWave,
  completeFulfillmentWave,
  createFulfillmentWaveIdempotencyKey,
  downloadFulfillmentWavePickCSV,
  getFulfillmentWave,
  packFulfillmentWaveOrder,
  queryFulfillmentWaves,
  recordFulfillmentWavePicks,
  startFulfillmentWave,
  type FulfillmentWave,
  type FulfillmentWaveLine,
  type FulfillmentWaveOrder,
  type FulfillmentWaveStatus,
} from "@/services/fulfillmentWaves";
import { PERMISSIONS } from "@/utils/permission";

const WAVE_STATUS_META: Record<
  FulfillmentWaveStatus,
  { label: string; color: string }
> = {
  draft: { label: "待开始", color: "default" },
  picking: { label: "拣货中", color: "processing" },
  packing: { label: "待打包", color: "blue" },
  completing: { label: "完成处理中", color: "gold" },
  partial: { label: "部分完成", color: "warning" },
  completed: { label: "已完成", color: "success" },
  cancelled: { label: "已取消", color: "default" },
};

const ORDER_STATUS_META: Record<string, { label: string; color: string }> = {
  pending: { label: "待拣货", color: "default" },
  blocked: { label: "缺货阻断", color: "error" },
  ready_to_pack: { label: "待打包复核", color: "processing" },
  packed: { label: "已打包", color: "blue" },
  fulfilled: { label: "已履约", color: "success" },
  failed: { label: "履约失败", color: "warning" },
};

type PickDraftValue = {
  pickedQuantity: number;
  scannedBarcode: string;
  scannedLocationCode: string;
};
type PickDraft = Record<string, PickDraftValue>;
type PackDraft = { carrier: string; trackingNo: string; trackingUrl: string };

function waveStatusTag(status: FulfillmentWaveStatus) {
  const meta = WAVE_STATUS_META[status] ?? {
    label: "未知状态",
    color: "default",
  };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

function orderStatusTag(status: string) {
  const meta = ORDER_STATUS_META[status] ?? {
    label: "未知状态",
    color: "default",
  };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

function userFacingError(error: unknown, fallback: string) {
  const text = (error as Error)?.message?.trim();
  return text && /[\u3400-\u9fff]/.test(text) ? text : fallback;
}

function warehouseLabel(wave: FulfillmentWave) {
  return (
    [wave.warehouseCode, wave.warehouseName].filter(Boolean).join(" · ") ||
    wave.warehouseId
  );
}

function orderByID(wave?: FulfillmentWave) {
  return new Map((wave?.orders ?? []).map((order) => [order.orderId, order]));
}

export default function FulfillmentWavesPage() {
  const { can, readonly } = usePermission();
  const canOperate = !readonly && can(PERMISSIONS.ORDER_OPERATE);
  const actionRef = useRef<ActionType>();
  const drawer = useUrlDrawerState("fulfillment-wave");
  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [status, setStatus] = useState<FulfillmentWaveStatus | undefined>();
  const [listError, setListError] = useState("");
  const [detail, setDetail] = useState<FulfillmentWave>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");
  const [submitting, setSubmitting] = useState("");
  const [pickOpen, setPickOpen] = useState(false);
  const [pickDraft, setPickDraft] = useState<PickDraft>({});
  const [packOrder, setPackOrder] = useState<FulfillmentWaveOrder>();
  const [packDraft, setPackDraft] = useState<PackDraft>({
    carrier: "",
    trackingNo: "",
    trackingUrl: "",
  });

  const loadDetail = useCallback(async (id: string) => {
    setDetailLoading(true);
    setDetailError("");
    try {
      setDetail(await getFulfillmentWave(id));
    } catch (error) {
      setDetail(undefined);
      setDetailError(userFacingError(error, "波次详情加载失败，请稍后重试"));
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (drawer.id) {
      void loadDetail(drawer.id);
    } else {
      setDetail(undefined);
      setDetailError("");
    }
  }, [drawer.id, loadDetail]);

  useEffect(() => {
    void actionRef.current?.reloadAndRest?.();
  }, [keyword, status]);

  const refresh = useCallback(async () => {
    if (drawer.id) await loadDetail(drawer.id);
    await actionRef.current?.reload();
  }, [drawer.id, loadDetail]);

  const runRevisionAction = useCallback(
    async (
      name: string,
      action: (wave: FulfillmentWave) => Promise<unknown>,
      success: string,
    ) => {
      if (!detail || submitting || !canOperate) return;
      setSubmitting(name);
      setDetailError("");
      try {
        await action(detail);
        message.success(success);
        await refresh();
      } catch (error) {
        const text = userFacingError(
          error,
          `${success.replace("成功", "")}失败，波次可能已变化，请刷新后重试`,
        );
        setDetailError(text);
        message.error(text);
      } finally {
        setSubmitting("");
      }
    },
    [canOperate, detail, refresh, submitting],
  );

  const requestStart = useCallback(() => {
    if (!detail) return;
    Modal.confirm({
      title: "开始拣货？",
      content: `将按 ${warehouseLabel(detail)} 的库存预占快照开始波次 ${detail.waveNo}。`,
      okText: "开始拣货",
      cancelText: "取消",
      onOk: () =>
        runRevisionAction(
          "start",
          (wave) =>
            startFulfillmentWave(wave.id, {
              expectedRevision: wave.revision,
              idempotencyKey: createFulfillmentWaveIdempotencyKey("start"),
            }),
          "开始拣货成功",
        ),
    });
  }, [detail, runRevisionAction]);

  const editablePickLines = useMemo(() => {
    const orders = orderByID(detail);
    return (detail?.lines ?? []).filter((line) => {
      const orderStatus = orders.get(line.orderId)?.status;
      return orderStatus !== "packed" && orderStatus !== "fulfilled";
    });
  }, [detail]);

  const openPick = useCallback(() => {
    if (!detail || editablePickLines.length === 0) return;
    setPickDraft(
      Object.fromEntries(
        editablePickLines.map((line) => [
          line.id,
          {
            pickedQuantity: line.pickedQuantity,
            scannedBarcode: "",
            scannedLocationCode: "",
          },
        ]),
      ),
    );
    setPickOpen(true);
  }, [detail, editablePickLines]);

  const submitPick = useCallback(async () => {
    if (!detail || submitting || !canOperate || editablePickLines.length === 0)
      return;
    setSubmitting("pick");
    try {
      await recordFulfillmentWavePicks(detail.id, {
        expectedRevision: detail.revision,
        idempotencyKey: createFulfillmentWaveIdempotencyKey("pick"),
        lines: editablePickLines.map((line) => {
          const draft = pickDraft[line.id] ?? {
            pickedQuantity: 0,
            scannedBarcode: "",
            scannedLocationCode: "",
          };
          const pickedQuantity = draft.pickedQuantity;
          return {
            lineId: line.id,
            pickedQuantity,
            shortageQuantity: line.requiredQuantity - pickedQuantity,
            scannedBarcode: draft.scannedBarcode.trim() || undefined,
            scannedLocationCode: draft.scannedLocationCode.trim() || undefined,
          };
        }),
      });
      setPickOpen(false);
      message.success("拣货结果已保存");
      await refresh();
    } catch (error) {
      const text = userFacingError(
        error,
        "拣货结果保存失败，波次可能已变化，请刷新后重试",
      );
      setDetailError(text);
      message.error(text);
    } finally {
      setSubmitting("");
    }
  }, [canOperate, detail, editablePickLines, pickDraft, refresh, submitting]);

  const openPack = useCallback((order: FulfillmentWaveOrder) => {
    setPackOrder(order);
    setPackDraft({
      carrier: order.carrier ?? "",
      trackingNo: order.trackingNo ?? "",
      trackingUrl: order.trackingUrl ?? "",
    });
  }, []);

  const submitPack = useCallback(async () => {
    if (!detail || !packOrder || submitting || !canOperate) return;
    const carrier = packDraft.carrier.trim();
    const trackingNo = packDraft.trackingNo.trim();
    if (!carrier || !trackingNo) {
      message.warning("请填写承运商和运单号");
      return;
    }
    setSubmitting("pack");
    try {
      await packFulfillmentWaveOrder(detail.id, packOrder.orderId, {
        expectedRevision: detail.revision,
        idempotencyKey: createFulfillmentWaveIdempotencyKey("pack"),
        carrier,
        trackingNo,
        trackingUrl: packDraft.trackingUrl.trim() || undefined,
      });
      setPackOrder(undefined);
      message.success("打包复核已保存");
      await refresh();
    } catch (error) {
      const text = userFacingError(
        error,
        "打包复核保存失败，波次可能已变化，请刷新后重试",
      );
      setDetailError(text);
      message.error(text);
    } finally {
      setSubmitting("");
    }
  }, [canOperate, detail, packDraft, packOrder, refresh, submitting]);

  const requestComplete = useCallback(() => {
    if (!detail) return;
    const packedCount = (detail.orders ?? []).filter(
      (order) => order.status === "packed",
    ).length;
    Modal.confirm({
      title: "确认完成已打包订单？",
      content: `本次将处理 ${packedCount} 单：在本地扣减库存、创建发货记录并更新订单状态。不会调用真实物流或平台接口，失败项不会自动重试。`,
      okText: "确认完成",
      cancelText: "取消",
      onOk: () =>
        runRevisionAction(
          "complete",
          (wave) =>
            completeFulfillmentWave(wave.id, {
              expectedRevision: wave.revision,
              idempotencyKey: createFulfillmentWaveIdempotencyKey("complete"),
            }),
          "波次处理成功",
        ),
    });
  }, [detail, runRevisionAction]);

  const requestCancel = useCallback(() => {
    if (!detail) return;
    Modal.confirm({
      title: "取消该拣货波次？",
      content:
        "取消只释放订单的波次占用，不会自动释放已预占库存。库存预占需在订单库存流程中人工处理。",
      okText: "确认取消",
      okButtonProps: { danger: true },
      cancelText: "返回",
      onOk: () =>
        runRevisionAction(
          "cancel",
          (wave) =>
            cancelFulfillmentWave(wave.id, {
              expectedRevision: wave.revision,
              idempotencyKey: createFulfillmentWaveIdempotencyKey("cancel"),
            }),
          "波次取消成功",
        ),
    });
  }, [detail, runRevisionAction]);

  const columns: ProColumns<FulfillmentWave>[] = [
    {
      title: "波次",
      dataIndex: "waveNo",
      width: 190,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => drawer.openDrawer(row.id)}
        >
          {row.waveNo}
        </Button>
      ),
    },
    { title: "仓库", width: 190, render: (_, row) => warehouseLabel(row) },
    {
      title: "状态",
      dataIndex: "status",
      width: 120,
      render: (_, row) => waveStatusTag(row.status),
    },
    { title: "订单", dataIndex: "orderCount", width: 80 },
    {
      title: "拣货进度",
      width: 150,
      render: (_, row) => `${row.pickedQuantity} / ${row.requiredQuantity}`,
    },
    {
      title: "异常",
      width: 130,
      render: (_, row) =>
        row.shortageQuantity || row.failedCount ? (
          <Typography.Text type="danger">
            缺货 {row.shortageQuantity} · 失败 {row.failedCount}
          </Typography.Text>
        ) : (
          "—"
        ),
    },
    {
      title: "创建时间",
      dataIndex: "createdAt",
      valueType: "dateTime",
      width: 170,
    },
    {
      title: "操作",
      valueType: "option",
      width: 90,
      fixed: "right",
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          icon={<EyeOutlined />}
          onClick={() => drawer.openDrawer(row.id)}
        >
          查看
        </Button>
      ),
    },
  ];

  const lineColumns = [
    {
      title: "订单",
      width: 180,
      render: (_: unknown, line: FulfillmentWaveLine) =>
        orderByID(detail).get(line.orderId)?.orderNo ?? line.orderId,
    },
    {
      title: "商品 / 规格",
      render: (_: unknown, line: FulfillmentWaveLine) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{line.productTitle || "未命名商品"}</Typography.Text>
          <Typography.Text type="secondary">
            {line.skuCode || line.skuName || line.productSkuId}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: "库位 / 条码",
      width: 180,
      render: (_: unknown, line: FulfillmentWaveLine) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{line.locationCode || "未配置库位"}</Typography.Text>
          <Typography.Text type="secondary">{line.barcode || "未配置条码"}</Typography.Text>
        </Space>
      ),
    },
    { title: "需求", dataIndex: "requiredQuantity", width: 72 },
    { title: "已拣", dataIndex: "pickedQuantity", width: 72 },
    {
      title: "缺货",
      dataIndex: "shortageQuantity",
      width: 72,
      render: (value: number) =>
        value > 0 ? (
          <Typography.Text type="danger">{value}</Typography.Text>
        ) : (
          0
        ),
    },
  ];

  const packedCount = (detail?.orders ?? []).filter(
    (order) => order.status === "packed",
  ).length;
  const cancellable =
    detail &&
    !["completed", "cancelled", "completing"].includes(detail.status) &&
    detail.fulfilledCount === 0;
  const pickable =
    detail &&
    ["picking", "packing", "partial"].includes(detail.status) &&
    editablePickLines.length > 0;

  return (
    <PermissionGuard require={PERMISSIONS.ORDER_VIEW} showForbiddenPage>
      <TmPageContainer
        title="拣货波次"
        subTitle="先按预占快照拣货，再逐单打包复核；只有人工确认完成后才写入本地出库和发货事实。"
      >
        {!canOperate ? (
          <Alert
            type="info"
            showIcon
            message="当前账号为只读模式，可查看和导出波次，但不能执行拣货、打包或完成操作。"
            style={{ marginBottom: 16 }}
          />
        ) : null}
        <Alert
          type="warning"
          showIcon
          message="当前版本仅处理单仓本地履约；不调用真实物流或平台写接口，不自动重试，不自动释放预占，也不支持跨仓拆单。"
          style={{ marginBottom: 16 }}
        />
        {listError ? <ErrorAlert title={listError} /> : null}
        <TmProTable<FulfillmentWave>
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1160 }}
          locale={{
            emptyText: "暂无拣货波次，可从履约分仓页选择已分仓订单创建",
          }}
          toolBarRender={() => [
            <Space key="filters" wrap size="small" style={{ width: "100%" }}>
              <Input.Search
                aria-label="搜索波次"
                allowClear
                value={keywordInput}
                placeholder="波次号或订单号"
                style={{ width: "min(260px, 100%)" }}
                onChange={(event) => setKeywordInput(event.target.value)}
                onSearch={(value) => setKeyword(value.trim())}
              />
              <Select<FulfillmentWaveStatus>
                aria-label="波次状态"
                allowClear
                value={status}
                placeholder="全部状态"
                style={{ width: 150 }}
                options={Object.entries(WAVE_STATUS_META).map(
                  ([value, meta]) => ({
                    value: value as FulfillmentWaveStatus,
                    label: meta.label,
                  }),
                )}
                onChange={setStatus}
              />
              <Button
                onClick={() => history.push("/orders/warehouse-allocations")}
              >
                前往履约分仓
              </Button>
            </Space>,
          ]}
          request={async (params) => {
            try {
              const result = await queryFulfillmentWaves({
                page: params.current,
                pageSize: params.pageSize,
                keyword: keyword || undefined,
                status,
              });
              setListError("");
              return {
                data: result.list ?? [],
                success: true,
                total: result.pagination?.total ?? 0,
              };
            } catch (error) {
              const text = userFacingError(
                error,
                "波次列表加载失败，请稍后重试",
              );
              setListError(text);
              return { data: [], success: false, total: 0 };
            }
          }}
        />

        <AppDrawer
          title={detail?.waveNo ? `拣货波次 ${detail.waveNo}` : "拣货波次详情"}
          open={drawer.open}
          onClose={drawer.closeDrawer}
          loading={detailLoading}
          extra={
            <Button
              icon={<ReloadOutlined />}
              disabled={!drawer.id || detailLoading || Boolean(submitting)}
              onClick={() => drawer.id && void loadDetail(drawer.id)}
            >
              刷新
            </Button>
          }
        >
          {detailError ? <ErrorAlert title={detailError} /> : null}
          {detail ? (
            <Space direction="vertical" size="middle" style={{ width: "100%" }}>
              <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                <Descriptions.Item label="状态">
                  {waveStatusTag(detail.status)}
                </Descriptions.Item>
                <Descriptions.Item label="仓库">
                  {warehouseLabel(detail)}
                </Descriptions.Item>
                <Descriptions.Item label="订单 / 规格">
                  {detail.orderCount} / {detail.lineCount}
                </Descriptions.Item>
                <Descriptions.Item label="拣货数量">
                  {detail.pickedQuantity} / {detail.requiredQuantity}
                </Descriptions.Item>
                <Descriptions.Item label="缺货数量">
                  {detail.shortageQuantity}
                </Descriptions.Item>
                <Descriptions.Item label="完成 / 失败">
                  {detail.fulfilledCount} / {detail.failedCount}
                </Descriptions.Item>
                {detail.remark ? (
                  <Descriptions.Item label="备注" span={{ xs: 1, sm: 2 }}>
                    {detail.remark}
                  </Descriptions.Item>
                ) : null}
              </Descriptions>
              {detail.status === "completing" ? (
                <Alert
                  type="warning"
                  showIcon
                  message="当前完成请求正在处理或等待人工重试，请勿使用其他幂等键重复提交。"
                />
              ) : null}
              <Space wrap>
                <Button
                  icon={<FileTextOutlined />}
                  onClick={() =>
                    history.push(
                      `/orders/fulfillment-waves/${encodeURIComponent(detail.id)}/documents`,
                    )
                  }
                >
                  出库单据中心
                </Button>
                <Button
                  icon={<DownloadOutlined />}
                  onClick={() => downloadFulfillmentWavePickCSV(detail)}
                >
                  导出拣货单
                </Button>
                {detail.status === "draft" ? (
                  <Button
                    type="primary"
                    icon={<PlayCircleOutlined />}
                    loading={submitting === "start"}
                    disabled={!canOperate || Boolean(submitting)}
                    onClick={requestStart}
                  >
                    开始拣货
                  </Button>
                ) : null}
                {pickable ? (
                  <Button
                    icon={<InboxOutlined />}
                    disabled={!canOperate || Boolean(submitting)}
                    onClick={openPick}
                  >
                    录入拣货结果
                  </Button>
                ) : null}
                {detail.packingVerificationRequired &&
                ["packing", "partial"].includes(detail.status) ? (
                  <Button
                    icon={<ScanOutlined />}
                    disabled={!canOperate || Boolean(submitting)}
                    onClick={() =>
                      history.push(
                        `/orders/fulfillment-waves/${encodeURIComponent(detail.id)}/verify-pack`,
                      )
                    }
                  >
                    进入出库扫描复核
                  </Button>
                ) : null}
                {packedCount > 0 &&
                ["packing", "partial"].includes(detail.status) ? (
                  <Button
                    type="primary"
                    icon={<CheckCircleOutlined />}
                    loading={submitting === "complete"}
                    disabled={!canOperate || Boolean(submitting)}
                    onClick={requestComplete}
                  >
                    完成已打包订单（{packedCount}）
                  </Button>
                ) : null}
                {cancellable ? (
                  <Button
                    danger
                    icon={<StopOutlined />}
                    loading={submitting === "cancel"}
                    disabled={!canOperate || Boolean(submitting)}
                    onClick={requestCancel}
                  >
                    取消波次
                  </Button>
                ) : null}
              </Space>

              <Typography.Title level={5} style={{ margin: 0 }}>
                订单打包复核
              </Typography.Title>
              <Table<FulfillmentWaveOrder>
                rowKey="id"
                size="small"
                pagination={false}
                scroll={{ x: 780 }}
                dataSource={detail.orders ?? []}
                columns={[
                  { title: "订单", dataIndex: "orderNo", width: 190 },
                  {
                    title: "状态",
                    dataIndex: "status",
                    width: 130,
                    render: (value: string) => orderStatusTag(value),
                  },
                  {
                    title: "承运商",
                    dataIndex: "carrier",
                    width: 130,
                    render: (value?: string) => value || "—",
                  },
                  {
                    title: "运单号",
                    dataIndex: "trackingNo",
                    width: 180,
                    render: (value?: string) => value || "—",
                  },
                  {
                    title: "称重",
                    dataIndex: "actualWeightGrams",
                    width: 110,
                    render: (value?: number) =>
                      typeof value === "number" ? `${value} 克` : "—",
                  },
                  {
                    title: "结果",
                    dataIndex: "failureReason",
                    ellipsis: true,
                    render: (value?: string) =>
                      value ? (
                        <Typography.Text type="danger">
                          履约失败，请核对订单与库存后人工重试
                        </Typography.Text>
                      ) : (
                        "—"
                      ),
                  },
                  {
                    title: "操作",
                    width: 100,
                    fixed: "right",
                    render: (_: unknown, order: FulfillmentWaveOrder) => {
                      if (
                        detail.packingVerificationRequired &&
                        ["ready_to_pack", "failed"].includes(order.status)
                      ) {
                        return (
                          <Button
                            type="link"
                            size="small"
                            disabled={!canOperate || Boolean(submitting)}
                            onClick={() =>
                              history.push(
                                `/orders/fulfillment-waves/${encodeURIComponent(detail.id)}/verify-pack`,
                              )
                            }
                          >
                            扫描复核
                          </Button>
                        );
                      }
                      if (
                        !detail.packingVerificationRequired &&
                        ["ready_to_pack", "failed", "packed"].includes(
                          order.status,
                        )
                      ) {
                        return (
                        <Button
                          type="link"
                          size="small"
                          disabled={!canOperate || Boolean(submitting)}
                          onClick={() => openPack(order)}
                        >
                          {order.status === "packed" ? "修改复核" : "打包复核"}
                        </Button>
                        );
                      }
                      return "—";
                    },
                  },
                ]}
              />

              <Typography.Title level={5} style={{ margin: 0 }}>
                拣货明细
              </Typography.Title>
              <Table<FulfillmentWaveLine>
                rowKey="id"
                size="small"
                pagination={false}
                scroll={{ x: 820 }}
                dataSource={detail.lines ?? []}
                columns={lineColumns}
              />
            </Space>
          ) : null}
        </AppDrawer>

        <Modal
          title="录入拣货结果"
          open={pickOpen}
          width={900}
          style={{ maxWidth: "calc(100vw - 24px)" }}
          destroyOnHidden
          okText="保存拣货结果"
          cancelText="取消"
          confirmLoading={submitting === "pick"}
          onCancel={() => !submitting && setPickOpen(false)}
          onOk={() => void submitPick()}
        >
          <Alert
            type="info"
            showIcon
            message="逐条填写实际拣到数量；系统自动计算缺货数量。缺货订单不能进入打包复核。"
            style={{ marginBottom: 12 }}
          />
          <Table<FulfillmentWaveLine>
            rowKey="id"
            size="small"
            pagination={false}
            scroll={{ x: 700 }}
            dataSource={editablePickLines}
            columns={[
              {
                title: "订单",
                width: 180,
                render: (_: unknown, line) =>
                  orderByID(detail).get(line.orderId)?.orderNo ?? line.orderId,
              },
              {
                title: "商品规格",
                render: (_: unknown, line) =>
                  line.skuCode || line.skuName || line.productSkuId,
              },
              { title: "需求", dataIndex: "requiredQuantity", width: 80 },
              {
                title: "实际拣到",
                width: 130,
                render: (_: unknown, line) => (
                  <InputNumber
                    aria-label={`${line.skuCode || line.productSkuId} 实际拣到数量`}
                    min={0}
                    max={line.requiredQuantity}
                    precision={0}
                    value={pickDraft[line.id]?.pickedQuantity ?? 0}
                    onChange={(value) =>
                      setPickDraft((current) => ({
                        ...current,
                        [line.id]: {
                          ...(current[line.id] ?? {
                            pickedQuantity: 0,
                            scannedBarcode: "",
                            scannedLocationCode: "",
                          }),
                          pickedQuantity: Number(value ?? 0),
                        },
                      }))
                    }
                  />
                ),
              },
              {
                title: "缺货",
                width: 80,
                render: (_: unknown, line) =>
                  line.requiredQuantity - (pickDraft[line.id]?.pickedQuantity ?? 0),
              },
              {
                title: "扫描条码",
                width: 170,
                render: (_: unknown, line) => (
                  <Input
                    aria-label={`${line.skuCode || line.productSkuId} 扫描条码`}
                    maxLength={128}
                    placeholder={line.barcode || "未配置条码"}
                    value={pickDraft[line.id]?.scannedBarcode ?? ""}
                    onChange={(event) =>
                      setPickDraft((current) => ({
                        ...current,
                        [line.id]: {
                          ...(current[line.id] ?? {
                            pickedQuantity: 0,
                            scannedBarcode: "",
                            scannedLocationCode: "",
                          }),
                          scannedBarcode: event.target.value,
                        },
                      }))
                    }
                  />
                ),
              },
              {
                title: "扫描库位",
                width: 150,
                render: (_: unknown, line) => (
                  <Input
                    aria-label={`${line.skuCode || line.productSkuId} 扫描库位`}
                    maxLength={64}
                    placeholder={line.locationCode || "未配置库位"}
                    value={pickDraft[line.id]?.scannedLocationCode ?? ""}
                    onChange={(event) =>
                      setPickDraft((current) => ({
                        ...current,
                        [line.id]: {
                          ...(current[line.id] ?? {
                            pickedQuantity: 0,
                            scannedBarcode: "",
                            scannedLocationCode: "",
                          }),
                          scannedLocationCode: event.target.value,
                        },
                      }))
                    }
                  />
                ),
              },
            ]}
          />
        </Modal>

        <Modal
          title={packOrder ? `打包复核 · ${packOrder.orderNo}` : "打包复核"}
          open={Boolean(packOrder)}
          destroyOnHidden
          okText="保存复核"
          cancelText="取消"
          confirmLoading={submitting === "pack"}
          onCancel={() => !submitting && setPackOrder(undefined)}
          onOk={() => void submitPack()}
        >
          <Space direction="vertical" size="middle" style={{ width: "100%" }}>
            <Alert
              type="info"
              showIcon
              message="这里只记录人工承运商和运单信息；保存不会立即扣库存或调用真实物流接口。"
            />
            <Input
              aria-label="承运商"
              maxLength={128}
              placeholder="承运商"
              value={packDraft.carrier}
              onChange={(event) =>
                setPackDraft((current) => ({
                  ...current,
                  carrier: event.target.value,
                }))
              }
            />
            <Input
              aria-label="运单号"
              maxLength={255}
              placeholder="运单号"
              value={packDraft.trackingNo}
              onChange={(event) =>
                setPackDraft((current) => ({
                  ...current,
                  trackingNo: event.target.value,
                }))
              }
            />
            <Input
              aria-label="物流轨迹链接"
              maxLength={2048}
              placeholder="物流轨迹链接（可选，http(s)）"
              value={packDraft.trackingUrl}
              onChange={(event) =>
                setPackDraft((current) => ({
                  ...current,
                  trackingUrl: event.target.value,
                }))
              }
            />
          </Space>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
