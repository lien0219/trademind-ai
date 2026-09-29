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
  getWarehouseAllocation,
  queryWarehouseAllocations,
  type WarehouseAllocation,
  type WarehouseAllocationCandidate,
  type WarehouseAllocationCandidateLine,
  type WarehouseAllocationListRow,
} from "@/services/orders";
import {
  createFulfillmentWave,
  createFulfillmentWaveIdempotencyKey,
} from "@/services/fulfillmentWaves";
import { PERMISSIONS } from "@/utils/permission";

const STATUS_META = {
  allocated: { label: "已分仓", color: "success" },
  allocatable: { label: "可分仓", color: "processing" },
  blocked: { label: "已阻断", color: "error" },
} as const;

type AssignmentFilter = "all" | "allocated" | "unallocated";

function allocationStatusTag(
  status: WarehouseAllocationListRow["allocationStatus"],
) {
  const meta = STATUS_META[status] ?? {
    label: status || "未知",
    color: "default",
  };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

function candidateLabel(candidate: WarehouseAllocationCandidate) {
  return `${candidate.warehouseCode} · ${candidate.warehouseName}`;
}

function recommendationPolicyLabel(policy?: string) {
  if (policy === "single_warehouse_default_first_v1") {
    return "整单可满足优先，其次默认仓，再按仓库编码稳定排序";
  }
  return policy || "当前接口未返回推荐规则";
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
  const [selectedRows, setSelectedRows] = useState<
    WarehouseAllocationListRow[]
  >([]);
  const [waveOpen, setWaveOpen] = useState(false);
  const [waveRows, setWaveRows] = useState<WarehouseAllocationListRow[]>([]);
  const [waveRemark, setWaveRemark] = useState("");
  const [waveKey, setWaveKey] = useState("");
  const [waveSubmitting, setWaveSubmitting] = useState(false);
  const [waveError, setWaveError] = useState("");

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

  const openCreateWave = useCallback(() => {
    const rows = selectedRows.filter(
      (row) => row.allocationStatus === "allocated" && row.warehouseId,
    );
    if (!canOperate || rows.length === 0) {
      return;
    }
    const warehouseId = rows[0]?.warehouseId;
    if (!warehouseId || rows.some((row) => row.warehouseId !== warehouseId)) {
      message.warning("一个拣货波次只能包含同一仓库的订单，请重新选择");
      return;
    }
    if (rows.length > 50) {
      message.warning("单个拣货波次最多包含 50 个订单");
      return;
    }
    setWaveRows(rows);
    setWaveRemark("");
    setWaveKey(createFulfillmentWaveIdempotencyKey("create"));
    setWaveError("");
    setWaveOpen(true);
  }, [canOperate, selectedRows]);

  const submitCreateWave = useCallback(async () => {
    const warehouseId = waveRows[0]?.warehouseId;
    if (
      !canOperate ||
      waveSubmitting ||
      waveRows.length === 0 ||
      !waveKey ||
      !warehouseId
    ) {
      return;
    }
    setWaveSubmitting(true);
    setWaveError("");
    try {
      const wave = await createFulfillmentWave({
        idempotencyKey: waveKey,
        warehouseId,
        orderIds: waveRows.map((row) => row.id),
        remark: waveRemark.trim() || undefined,
      });
      setWaveOpen(false);
      setSelectedRows([]);
      await actionRef.current?.reload();
      message.success("拣货波次创建成功");
      history.push(
        `/orders/fulfillment-waves?drawer=fulfillment-wave&id=${encodeURIComponent(wave.id)}`,
      );
    } catch (error) {
      const errorMessage = (error as Error)?.message || "拣货波次创建失败";
      const displayMessage = /[\u3400-\u9fff]/.test(errorMessage)
        ? errorMessage
        : "拣货波次创建失败，请确认订单仍处于已付款、已分仓和完整预占状态";
      setWaveError(displayMessage);
      message.error(displayMessage);
    } finally {
      setWaveSubmitting(false);
    }
  }, [canOperate, waveKey, waveRemark, waveRows, waveSubmitting]);

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
          return (
            <Space direction="vertical" size={0}>
              <span>
                {row.recommendedWarehouseCode} · {row.recommendedWarehouseName}
              </span>
              {row.recommendedWarehouseReasons?.[0]?.message ? (
                <Typography.Text type="secondary" ellipsis>
                  {row.recommendedWarehouseReasons[0].message}
                </Typography.Text>
              ) : null}
            </Space>
          );
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
        subTitle="按整单规格可用库存计算单仓候选；人工确认并预占后，可将同仓订单组织为持久化拣货波次。"
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
          message="分仓仍按单订单单仓绑定；库存不足、规格未绑定、账本投影不一致或候选已变化时会阻断。已分仓订单必须先完成拣货与打包复核，再确认本地履约。"
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
                  (row) =>
                    row.allocationStatus === "allocated" && row.warehouseId,
                ),
              );
            },
            getCheckboxProps: (row) => ({
              disabled:
                row.allocationStatus !== "allocated" || !row.warehouseId,
            }),
          }}
          locale={{ emptyText: "暂无待处理的已付款订单" }}
          filterBar={
            <Space wrap size="small">
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
            </Space>
          }
          toolBarRender={() => [
            <Button
              key="create-wave"
              type="primary"
              disabled={!canOperate || selectedRows.length === 0}
              onClick={openCreateWave}
            >
              创建拣货波次（{selectedRows.length}）
            </Button>,
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
                <Descriptions.Item label="推荐规则">
                  {recommendationPolicyLabel(detail.recommendationPolicy)}
                </Descriptions.Item>
                {detail.recommendedWarehouseReasons?.length ? (
                  <Descriptions.Item label="推荐依据">
                    {detail.recommendedWarehouseReasons
                      .map((reason) => reason.message)
                      .join("；")}
                  </Descriptions.Item>
                ) : null}
                {detail.warehouseId ? (
                  <Descriptions.Item label="已分配仓库" span={{ xs: 1, sm: 2 }}>
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
                  action={
                    <Button
                      type="link"
                      onClick={() =>
                        drawer.id &&
                        history.push(
                          `/orders/exceptions?orderId=${encodeURIComponent(drawer.id)}`,
                        )
                      }
                    >
                      打开异常工作台
                    </Button>
                  }
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
                      {candidate.recommendationRank ? (
                        <Tag color="processing">
                          推荐排序 #{candidate.recommendationRank}
                        </Tag>
                      ) : null}
                      {candidate.isDefault ? (
                        <Tag color="blue">默认仓</Tag>
                      ) : null}
                      {candidate.eligible ? (
                        <Tag color="success">整单可分配</Tag>
                      ) : (
                        <Tag color="error">库存不足</Tag>
                      )}
                    </Space>
                    {candidate.recommendationReasons?.length ? (
                      <Typography.Text type="secondary">
                        {candidate.recommendationReasons
                          .map((reason) => reason.message)
                          .join("；")}
                      </Typography.Text>
                    ) : null}
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
          title="创建拣货波次"
          open={waveOpen}
          width={760}
          style={{ maxWidth: "calc(100vw - 24px)" }}
          destroyOnHidden
          confirmLoading={waveSubmitting}
          okText="创建波次"
          cancelText="取消"
          onCancel={() => {
            if (!waveSubmitting) setWaveOpen(false);
          }}
          onOk={() => void submitCreateWave()}
        >
          <Space direction="vertical" size="middle" style={{ width: "100%" }}>
            <Alert
              type="info"
              showIcon
              message={`已选择 ${waveRows.length} 个同仓订单`}
              description="创建时会固化订单、商品规格、需求数量和预占仓库快照，不会扣减库存或调用真实物流接口。"
            />
            {waveError ? <ErrorAlert title={waveError} /> : null}
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
              <Descriptions.Item label="仓库">
                {waveRows[0]?.warehouseCode
                  ? `${waveRows[0].warehouseCode} · ${waveRows[0].warehouseName || ""}`
                  : waveRows[0]?.warehouseId || "—"}
              </Descriptions.Item>
              <Descriptions.Item label="订单数">
                {waveRows.length}
              </Descriptions.Item>
            </Descriptions>
            <Input.TextArea
              aria-label="波次备注"
              value={waveRemark}
              maxLength={500}
              showCount
              autoSize={{ minRows: 2, maxRows: 5 }}
              placeholder="可选：拣货区域、班次或交接说明"
              onChange={(event) => setWaveRemark(event.target.value)}
            />
            <Table<WarehouseAllocationListRow>
              size="small"
              rowKey="id"
              pagination={false}
              scroll={{ x: 560, y: 280 }}
              dataSource={waveRows}
              columns={[
                { title: "订单", dataIndex: "orderNo", width: 190 },
                {
                  title: "店铺",
                  dataIndex: "shopName",
                  render: (value?: string) => value || "手工订单",
                },
                { title: "创建时间", dataIndex: "createdAt", width: 170 },
              ]}
            />
          </Space>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
