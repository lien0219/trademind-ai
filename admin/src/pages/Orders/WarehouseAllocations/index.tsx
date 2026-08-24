import { EyeOutlined, ReloadOutlined } from "@ant-design/icons";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import { history } from "@umijs/max";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Radio,
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
  confirmWarehouseAllocation,
  createOrderAllocationIdempotencyKey,
  batchFulfillOrders,
  createOrderBatchFulfillmentIdempotencyKey,
  getWarehouseAllocation,
  queryWarehouseAllocations,
  type BatchFulfillOrderItemPayload,
  type BatchFulfillmentItemResult,
  type BatchFulfillmentResult,
  type WarehouseAllocation,
  type WarehouseAllocationCandidate,
  type WarehouseAllocationCandidateLine,
  type WarehouseAllocationListRow,
} from "@/services/orders";
import { PERMISSIONS } from "@/utils/permission";

const STATUS_META = {
  allocated: { label: "已分仓", color: "success" },
  allocatable: { label: "可分仓", color: "processing" },
  blocked: { label: "已阻断", color: "error" },
} as const;

const BATCH_STATUS_META = {
  succeeded: { label: "履约成功", color: "success" },
  blocked: { label: "已阻断", color: "error" },
  in_progress: { label: "处理中", color: "processing" },
  failed: { label: "处理失败", color: "warning" },
} as const;

type AssignmentFilter = "all" | "allocated" | "unallocated";
type BatchShipmentDraft = Pick<
  BatchFulfillOrderItemPayload,
  "carrier" | "trackingNo" | "trackingUrl"
>;

function allocationStatusTag(
  status: WarehouseAllocationListRow["allocationStatus"],
) {
  const meta = STATUS_META[status] ?? {
    label: status || "未知",
    color: "default",
  };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

function batchStatusTag(status: BatchFulfillmentItemResult["status"]) {
  const meta = BATCH_STATUS_META[status] ?? {
    label: status || "未知",
    color: "default",
  };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

function candidateLabel(candidate: WarehouseAllocationCandidate) {
  return `${candidate.warehouseCode} · ${candidate.warehouseName}`;
}

export default function WarehouseAllocationsPage() {
  const { can, readonly } = usePermission();
  const canOperate = !readonly && can(PERMISSIONS.ORDER_OPERATE);
  const actionRef = useRef<ActionType>();
  const drawer = useUrlDrawerState("warehouse-allocation");
  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [assignment, setAssignment] = useState<AssignmentFilter>("unallocated");
  const [listError, setListError] = useState("");
  const [detail, setDetail] = useState<WarehouseAllocation>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");
  const [selectedWarehouseId, setSelectedWarehouseId] = useState<string>();
  const [idempotencyKey, setIdempotencyKey] = useState(() =>
    createOrderAllocationIdempotencyKey(),
  );
  const [confirming, setConfirming] = useState(false);
  const [selectedRows, setSelectedRows] = useState<WarehouseAllocationListRow[]>(
    [],
  );
  const [batchOpen, setBatchOpen] = useState(false);
  const [batchRows, setBatchRows] = useState<WarehouseAllocationListRow[]>([]);
  const [batchDrafts, setBatchDrafts] = useState<Record<string, BatchShipmentDraft>>(
    {},
  );
  const [batchKey, setBatchKey] = useState("");
  const [batchSubmitting, setBatchSubmitting] = useState(false);
  const [batchError, setBatchError] = useState("");
  const [batchResult, setBatchResult] = useState<BatchFulfillmentResult>();

  const loadDetail = useCallback(async (orderId: string) => {
    setDetailLoading(true);
    setDetailError("");
    try {
      const next = await getWarehouseAllocation(orderId);
      setDetail(next);
      const recommended = next.candidates.find(
        (candidate) =>
          candidate.warehouseId === next.recommendedWarehouseId &&
          candidate.eligible,
      );
      setSelectedWarehouseId(recommended?.warehouseId);
      setIdempotencyKey(createOrderAllocationIdempotencyKey());
    } catch (error) {
      setDetail(undefined);
      setSelectedWarehouseId(undefined);
      setDetailError((error as Error)?.message || "分仓候选加载失败");
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (drawer.id) {
      void loadDetail(drawer.id);
      return;
    }
    setDetail(undefined);
    setDetailError("");
    setSelectedWarehouseId(undefined);
  }, [drawer.id, loadDetail]);

  useEffect(() => {
    void actionRef.current?.reloadAndRest?.();
    setSelectedRows([]);
  }, [assignment, keyword]);

  const openBatchFulfillment = useCallback(() => {
    const rows = selectedRows.filter(
      (row) => row.allocationStatus === "allocated" && row.warehouseId,
    );
    if (!canOperate || rows.length === 0) {
      return;
    }
    setBatchRows(rows);
    setBatchDrafts(
      Object.fromEntries(
        rows.map((row) => [
          row.id,
          { carrier: "", trackingNo: "", trackingUrl: "" },
        ]),
      ),
    );
    setBatchKey(createOrderBatchFulfillmentIdempotencyKey());
    setBatchError("");
    setBatchResult(undefined);
    setBatchOpen(true);
  }, [canOperate, selectedRows]);

  const updateBatchDraft = useCallback(
    (orderId: string, field: keyof BatchShipmentDraft, value: string) => {
      setBatchDrafts((current) => ({
        ...current,
        [orderId]: { ...current[orderId], [field]: value },
      }));
    },
    [],
  );

  const submitBatchFulfillment = useCallback(async () => {
    if (!canOperate || batchSubmitting || batchRows.length === 0 || !batchKey) {
      return;
    }
    const missing = batchRows.find((row) => {
      const draft = batchDrafts[row.id];
      return !draft?.carrier.trim() || !draft?.trackingNo.trim();
    });
    if (missing) {
      setBatchError(`请补充订单 ${missing.orderNo} 的承运商和运单号`);
      return;
    }
    setBatchSubmitting(true);
    setBatchError("");
    try {
      const result = await batchFulfillOrders({
        batchIdempotencyKey: batchKey,
        items: batchRows.map((row) => ({
          orderId: row.id,
          warehouseId: row.warehouseId,
          carrier: batchDrafts[row.id]?.carrier.trim() ?? "",
          trackingNo: batchDrafts[row.id]?.trackingNo.trim() ?? "",
          trackingUrl: batchDrafts[row.id]?.trackingUrl?.trim() || undefined,
        })),
      });
      setBatchResult(result);
      setSelectedRows([]);
      await actionRef.current?.reload();
      if (result.summary.succeeded > 0) {
        message.success(`批量履约已处理 ${result.summary.succeeded} 单`);
      }
      if (result.summary.blocked + result.summary.failed + result.summary.inProgress > 0) {
        message.warning("部分订单未完成，请根据异常结果处理后重试");
      }
    } catch (error) {
      const errorMessage = (error as Error)?.message || "批量履约失败";
      setBatchError(errorMessage);
      message.error(errorMessage);
    } finally {
      setBatchSubmitting(false);
    }
  }, [
    batchDrafts,
    batchKey,
    batchRows,
    batchSubmitting,
    canOperate,
  ]);

  const selectedCandidate = useMemo(
    () =>
      detail?.candidates.find(
        (candidate) => candidate.warehouseId === selectedWarehouseId,
      ),
    [detail?.candidates, selectedWarehouseId],
  );

  const confirmSelected = useCallback(async () => {
    if (
      !drawer.id ||
      !selectedCandidate ||
      !selectedCandidate.eligible ||
      confirming ||
      !canOperate
    ) {
      return;
    }
    setConfirming(true);
    try {
      await confirmWarehouseAllocation(drawer.id, {
        warehouseId: selectedCandidate.warehouseId,
        expectedRevision: selectedCandidate.revision,
        idempotencyKey,
      });
      message.success("分仓确认成功，整单库存已预占");
      await Promise.all([loadDetail(drawer.id), actionRef.current?.reload()]);
    } catch (error) {
      const errorMessage = (error as Error)?.message || "分仓确认失败";
      message.error(errorMessage);
      setDetailError(`${errorMessage}。候选可能已变化，请刷新后重试。`);
      setIdempotencyKey(createOrderAllocationIdempotencyKey());
    } finally {
      setConfirming(false);
    }
  }, [
    canOperate,
    confirming,
    drawer.id,
    idempotencyKey,
    loadDetail,
    selectedCandidate,
  ]);

  const requestConfirmation = useCallback(() => {
    if (!selectedCandidate || confirming) return;
    Modal.confirm({
      title: "确认整单分配到该仓库？",
      content: `订单将绑定到 ${candidateLabel(selectedCandidate)}，并在同一事务中预占全部 SKU 库存。`,
      okText: "确认分仓并预占",
      cancelText: "取消",
      onOk: confirmSelected,
    });
  }, [confirmSelected, confirming, selectedCandidate]);

  const columns: ProColumns<WarehouseAllocationListRow>[] = [
    {
      title: "订单",
      dataIndex: "orderNo",
      width: 190,
      ellipsis: true,
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Typography.Link onClick={() => history.push(`/orders/${row.id}`)}>
            {row.orderNo}
          </Typography.Link>
          <Typography.Text type="secondary">
            {row.shopName || row.platform}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: "订单状态",
      dataIndex: "status",
      width: 125,
      render: (_, row) => (
        <Space direction="vertical" size={2}>
          <Tag>{row.status}</Tag>
          <Typography.Text type="secondary">
            {row.paymentStatus}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: "分仓状态",
      dataIndex: "allocationStatus",
      width: 110,
      render: (_, row) => allocationStatusTag(row.allocationStatus),
    },
    {
      title: "仓库建议",
      dataIndex: "recommendedWarehouseName",
      width: 220,
      render: (_, row) => {
        if (row.allocationStatus === "allocated") {
          return row.warehouseName
            ? `${row.warehouseCode} · ${row.warehouseName}`
            : "已绑定仓库";
        }
        if (row.recommendedWarehouseId) {
          return `${row.recommendedWarehouseCode} · ${row.recommendedWarehouseName}`;
        }
        return <Typography.Text type="secondary">暂无可用建议</Typography.Text>;
      },
    },
    {
      title: "候选",
      dataIndex: "eligibleCandidateCount",
      width: 105,
      render: (_, row) =>
        `${row.eligibleCandidateCount} / ${row.candidateCount}`,
    },
    {
      title: "阻断原因",
      dataIndex: "blocks",
      ellipsis: true,
      render: (_, row) =>
        row.blocks?.map((block) => block.message).join("；") || "—",
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
      width: 96,
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
      title: "商品规格",
      dataIndex: "skuCode",
      render: (_: unknown, line: WarehouseAllocationCandidateLine) =>
        line.skuCode || line.productSkuId,
    },
    { title: "需求", dataIndex: "required", width: 72 },
    { title: "可用", dataIndex: "available", width: 72 },
    {
      title: "缺口",
      dataIndex: "shortage",
      width: 72,
      render: (value: number) =>
        value > 0 ? (
          <Typography.Text type="danger">{value}</Typography.Text>
        ) : (
          0
        ),
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.ORDER_VIEW} showForbiddenPage>
      <TmPageContainer
        title="履约分仓"
        subTitle="按整单 SKU 可用库存计算单仓候选；人工确认后原子绑定并预占，已分仓订单可批量履约。"
      >
        {!canOperate ? (
          <Alert
            type="info"
            showIcon
            message="当前账号为只读模式，可查看分仓候选，但不能确认分仓或预占库存。"
            style={{ marginBottom: 16 }}
          />
        ) : null}
        <Alert
          type="warning"
          showIcon
          message="分仓仍按单订单单仓绑定；已分仓订单可批量履约。库存不足、SKU 未绑定、账本投影不一致或候选已变化时会阻断。"
          style={{ marginBottom: 16 }}
        />
        {listError ? <ErrorAlert title={listError} /> : null}
        <TmProTable<WarehouseAllocationListRow>
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1200 }}
          rowSelection={{
            selectedRowKeys: selectedRows.map((row) => row.id),
            onChange: (_keys, rows) => {
              setSelectedRows(
                rows.filter(
                  (row) => row.allocationStatus === "allocated" && row.warehouseId,
                ),
              );
            },
            getCheckboxProps: (row) => ({
              disabled: row.allocationStatus !== "allocated" || !row.warehouseId,
            }),
          }}
          locale={{ emptyText: "暂无待处理的已付款订单" }}
          toolBarRender={() => [
            <Space
              key="filters-and-actions"
              wrap
              size="small"
              style={{ width: "100%" }}
            >
              <Input.Search
                aria-label="搜索订单"
                allowClear
                value={keywordInput}
                placeholder="订单号、客户或平台订单号"
                style={{ width: "min(260px, 100%)" }}
                onChange={(event) => setKeywordInput(event.target.value)}
                onSearch={(value) => {
                  setKeyword(value.trim());
                }}
              />
              <Select<AssignmentFilter>
                aria-label="仓库分配状态"
                value={assignment}
                style={{ width: 140 }}
                options={[
                  { value: "unallocated", label: "未绑定仓库" },
                  { value: "allocated", label: "已绑定仓库" },
                  { value: "all", label: "全部订单" },
                ]}
                onChange={(value) => {
                  setAssignment(value);
                }}
              />
              <Button
                type="primary"
                disabled={!canOperate || selectedRows.length === 0}
                onClick={openBatchFulfillment}
              >
                批量履约（{selectedRows.length}）
              </Button>
            </Space>,
          ]}
          request={async (params) => {
            try {
              const result = await queryWarehouseAllocations({
                page: params.current,
                pageSize: params.pageSize,
                keyword: keyword || undefined,
                assignment,
              });
              setListError("");
              return {
                data: result.list ?? [],
                success: true,
                total: result.pagination?.total ?? 0,
              };
            } catch (error) {
              const errorMessage =
                (error as Error)?.message || "分仓列表加载失败";
              setListError(errorMessage);
              message.error(errorMessage);
              return { data: [], success: false, total: 0 };
            }
          }}
        />

        <AppDrawer
          title="分仓候选详情"
          open={drawer.open}
          onClose={drawer.closeDrawer}
          loading={detailLoading}
          extra={
            <Button
              icon={<ReloadOutlined />}
              disabled={!drawer.id || detailLoading}
              onClick={() => drawer.id && void loadDetail(drawer.id)}
            >
              刷新候选
            </Button>
          }
        >
          {detailError ? <ErrorAlert title={detailError} /> : null}
          {detail ? (
            <Space direction="vertical" size="middle" style={{ width: "100%" }}>
              <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                <Descriptions.Item label="分仓状态">
                  {allocationStatusTag(detail.status)}
                </Descriptions.Item>
                <Descriptions.Item label="可用候选">
                  {detail.eligibleCandidateCount} / {detail.candidateCount}
                </Descriptions.Item>
                {detail.warehouseId ? (
                  <Descriptions.Item label="已分配仓库" span={2}>
                    {detail.warehouseCode} · {detail.warehouseName}
                  </Descriptions.Item>
                ) : null}
              </Descriptions>
              {detail.blocks?.length ? (
                <Alert
                  type="error"
                  showIcon
                  message="当前订单不可确认分仓"
                  description={detail.blocks
                    .map((block) => block.message)
                    .join("；")}
                />
              ) : null}
              {detail.candidates.map((candidate) => (
                <Card key={candidate.warehouseId} size="small">
                  <Space
                    direction="vertical"
                    size="small"
                    style={{ width: "100%" }}
                  >
                    <Radio
                      checked={selectedWarehouseId === candidate.warehouseId}
                      disabled={
                        !canOperate ||
                        detail.status !== "allocatable" ||
                        !candidate.eligible
                      }
                      onChange={() =>
                        setSelectedWarehouseId(candidate.warehouseId)
                      }
                    >
                      <Typography.Text strong>
                        {candidateLabel(candidate)}
                      </Typography.Text>
                    </Radio>
                    <Space wrap>
                      {candidate.isDefault ? (
                        <Tag color="blue">默认仓</Tag>
                      ) : null}
                      {candidate.eligible ? (
                        <Tag color="success">整单可分配</Tag>
                      ) : (
                        <Tag color="error">库存不足</Tag>
                      )}
                    </Space>
                    <Table<WarehouseAllocationCandidateLine>
                      size="small"
                      rowKey="productSkuId"
                      pagination={false}
                      columns={lineColumns}
                      dataSource={candidate.lines}
                      scroll={{ x: 520 }}
                    />
                  </Space>
                </Card>
              ))}
              <Space wrap>
                <Button
                  type="primary"
                  loading={confirming}
                  disabled={
                    !canOperate ||
                    detail.status !== "allocatable" ||
                    !selectedCandidate?.eligible
                  }
                  onClick={requestConfirmation}
                >
                  确认分仓并预占库存
                </Button>
                {!canOperate ? (
                  <Typography.Text type="secondary">
                    只读账号不可执行写操作
                  </Typography.Text>
                ) : null}
              </Space>
            </Space>
          ) : null}
        </AppDrawer>

        <Modal
          title="批量履约"
          open={batchOpen}
          width={960}
          style={{ maxWidth: "calc(100vw - 24px)" }}
          destroyOnHidden
          confirmLoading={batchSubmitting}
          okText={batchResult ? "关闭" : "提交批量履约"}
          cancelText="取消"
          onCancel={() => {
            if (!batchSubmitting) {
              setBatchOpen(false);
            }
          }}
          onOk={() => {
            if (batchResult) {
              setBatchOpen(false);
              return;
            }
            void submitBatchFulfillment();
          }}
        >
          {batchResult ? (
            <Space direction="vertical" size="middle" style={{ width: "100%" }}>
              <Alert
                type={
                  batchResult.summary.blocked +
                    batchResult.summary.failed +
                    batchResult.summary.inProgress >
                  0
                    ? "warning"
                    : "success"
                }
                showIcon
                message={`已处理 ${batchResult.summary.requested} 单：成功 ${batchResult.summary.succeeded}，阻断 ${batchResult.summary.blocked}，处理中 ${batchResult.summary.inProgress}，失败 ${batchResult.summary.failed}`}
                description="成功订单已在本地完成扣减、发货单和履约状态更新；异常项不会自动重试。"
              />
              <Table<BatchFulfillmentItemResult>
                size="small"
                rowKey="orderId"
                pagination={false}
                scroll={{ x: 720 }}
                dataSource={batchResult.items}
                columns={[
                  { title: "订单", dataIndex: "orderNo", width: 180 },
                  {
                    title: "仓库",
                    dataIndex: "warehouseId",
                    width: 160,
                    render: (value: string | undefined) => value || "—",
                  },
                  {
                    title: "结果",
                    dataIndex: "status",
                    width: 100,
                    render: (value: BatchFulfillmentItemResult["status"]) =>
                      batchStatusTag(value),
                  },
                  { title: "异常说明", dataIndex: "error", ellipsis: true },
                ]}
              />
              <Typography.Text strong>拣配汇总</Typography.Text>
              <Table
                size="small"
                rowKey={(row) => `${row.warehouseId}-${row.productSkuId}`}
                pagination={false}
                scroll={{ x: 720 }}
                locale={{ emptyText: "没有成功订单可生成拣配汇总" }}
                dataSource={batchResult.pickList}
                columns={[
                  {
                    title: "仓库",
                    render: (_: unknown, row) =>
                      row.warehouseCode
                        ? `${row.warehouseCode} · ${row.warehouseName || ""}`
                        : row.warehouseId,
                    width: 190,
                  },
                  {
                    title: "商品规格",
                    render: (_: unknown, row) =>
                      row.skuCode || row.skuName || row.productTitle || row.productSkuId,
                    width: 220,
                  },
                  { title: "数量", dataIndex: "quantity", width: 80 },
                  { title: "订单数", dataIndex: "orderCount", width: 80 },
                ]}
              />
            </Space>
          ) : (
            <Space direction="vertical" size="middle" style={{ width: "100%" }}>
              <Alert
                type="info"
                showIcon
                message={`本页已选 ${batchRows.length} 个已分仓订单`}
                description="请为每个订单填写人工承运商和运单号。提交只写入本地履约事实，不调用真实平台或物流接口。"
              />
              {batchError ? <ErrorAlert title={batchError} /> : null}
              <Table<WarehouseAllocationListRow>
                size="small"
                rowKey="id"
                pagination={false}
                scroll={{ x: 760 }}
                dataSource={batchRows}
                columns={[
                  { title: "订单", dataIndex: "orderNo", width: 180 },
                  {
                    title: "仓库",
                    render: (_: unknown, row) =>
                      row.warehouseCode
                        ? `${row.warehouseCode} · ${row.warehouseName || ""}`
                        : row.warehouseId || "—",
                    width: 190,
                  },
                  {
                    title: "承运商 *",
                    width: 180,
                    render: (_: unknown, row) => (
                      <Input
                        aria-label={`${row.orderNo} 承运商`}
                        value={batchDrafts[row.id]?.carrier}
                        maxLength={128}
                        placeholder="如：顺丰"
                        onChange={(event) =>
                          updateBatchDraft(row.id, "carrier", event.target.value)
                        }
                      />
                    ),
                  },
                  {
                    title: "运单号 *",
                    width: 210,
                    render: (_: unknown, row) => (
                      <Input
                        aria-label={`${row.orderNo} 运单号`}
                        value={batchDrafts[row.id]?.trackingNo}
                        maxLength={255}
                        placeholder="填写运单号"
                        onChange={(event) =>
                          updateBatchDraft(row.id, "trackingNo", event.target.value)
                        }
                      />
                    ),
                  },
                  {
                    title: "轨迹链接",
                    width: 240,
                    render: (_: unknown, row) => (
                      <Input
                        aria-label={`${row.orderNo} 轨迹链接`}
                        value={batchDrafts[row.id]?.trackingUrl}
                        maxLength={2048}
                        placeholder="可选，http(s)"
                        onChange={(event) =>
                          updateBatchDraft(row.id, "trackingUrl", event.target.value)
                        }
                      />
                    ),
                  },
                ]}
              />
            </Space>
          )}
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
