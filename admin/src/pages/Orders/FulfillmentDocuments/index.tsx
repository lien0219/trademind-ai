import {
  ArrowLeftOutlined,
  FileAddOutlined,
  PrinterOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { history, useParams } from "@umijs/max";
import {
  Alert,
  Button,
  Col,
  Descriptions,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import PermissionGuard from "@/components/PermissionGuard";
import {
  EmptyState,
  ErrorAlert,
  SectionCard,
  TmPageContainer,
  TmPageHeaderExtra,
} from "@/components/ui";
import { usePermission } from "@/hooks/usePermission";
import {
  createFulfillmentWaveIdempotencyKey,
  generateFulfillmentWaveDocument,
  getFulfillmentWave,
  getFulfillmentWaveDocument,
  queryFulfillmentWaveDocuments,
  recordFulfillmentWaveDocumentPrint,
  type FulfillmentWave,
  type FulfillmentWaveDocumentDetail,
  type FulfillmentWaveDocumentPrintEvent,
  type FulfillmentWaveDocumentSnapshot,
  type FulfillmentWaveDocumentType,
  type FulfillmentWaveStatus,
} from "@/services/fulfillmentWaves";
import { formatDateTime } from "@/utils/formatTime";
import { PERMISSIONS } from "@/utils/permission";
import "./index.less";

const DOCUMENT_TYPES: Array<{
  value: FulfillmentWaveDocumentType;
  label: string;
  description: string;
}> = [
  { value: "pick_list", label: "拣货单", description: "按库位与 SKU 排列" },
  { value: "packing_list", label: "配货单", description: "按订单核对装箱明细" },
  { value: "sku_labels", label: "规格标签", description: "本地商品识别标签" },
  {
    value: "package_labels",
    label: "本地包裹标签",
    description: "仅供仓内识别，不是承运商面单",
  },
];

const STATUS_LABELS: Record<FulfillmentWaveStatus, string> = {
  draft: "待开始",
  picking: "拣货中",
  packing: "待打包",
  completing: "完成处理中",
  partial: "部分完成",
  completed: "已完成",
  cancelled: "已取消",
};

function documentTypeLabel(type: FulfillmentWaveDocumentType) {
  return DOCUMENT_TYPES.find((item) => item.value === type)?.label ?? type;
}

function warehouseLabel(snapshot: FulfillmentWaveDocumentSnapshot) {
  return (
    [snapshot.warehouseCode, snapshot.warehouseName]
      .filter(Boolean)
      .join(" · ") || snapshot.warehouseId
  );
}

function userFacingError(error: unknown, fallback: string) {
  const text = (error as Error)?.message?.trim();
  return text && /[\u3400-\u9fff]/.test(text) ? text : fallback;
}

function PrintHeader({
  document,
  type,
}: {
  document: FulfillmentWaveDocumentDetail;
  type: FulfillmentWaveDocumentType;
}) {
  const { snapshot } = document;
  return (
    <header className="fulfillment-document-print__header">
      <div>
        <Typography.Title level={2}>{documentTypeLabel(type)}</Typography.Title>
        <Typography.Text type="secondary">
          本地出库单据 · V{document.version} · 来源修订{" "}
          {document.sourceRevision}
        </Typography.Text>
      </div>
      <div className="fulfillment-document-print__meta">
        <strong>{snapshot.waveNo}</strong>
        <span>{warehouseLabel(snapshot)}</span>
        <span>生成：{formatDateTime(snapshot.generatedAt)}</span>
      </div>
    </header>
  );
}

function PickList({ snapshot }: { snapshot: FulfillmentWaveDocumentSnapshot }) {
  return (
    <div className="fulfillment-document-print__table-wrap">
      <table className="fulfillment-document-print__table">
        <thead>
          <tr>
            <th>库位</th>
            <th>SKU / 商品</th>
            <th>条码</th>
            <th>订单</th>
            <th className="number">需求</th>
            <th className="number">已拣</th>
            <th className="number">缺货</th>
          </tr>
        </thead>
        <tbody>
          {snapshot.lines.map((line) => (
            <tr key={line.waveLineId}>
              <td>{line.locationCode || "未分配"}</td>
              <td>
                <strong>{line.skuCode || "未编码 SKU"}</strong>
                <small>{line.skuName || line.productTitle || "—"}</small>
              </td>
              <td>{line.barcode || "—"}</td>
              <td>{line.orderNo}</td>
              <td className="number">{line.requiredQuantity}</td>
              <td className="number">{line.pickedQuantity}</td>
              <td className="number">{line.shortageQuantity}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PackingList({
  snapshot,
}: {
  snapshot: FulfillmentWaveDocumentSnapshot;
}) {
  return (
    <div className="fulfillment-document-print__orders">
      {snapshot.orders.map((order) => {
        const lines = snapshot.lines.filter(
          (line) => line.orderId === order.orderId,
        );
        return (
          <section
            key={order.waveOrderId}
            className="fulfillment-document-print__order"
          >
            <div className="fulfillment-document-print__order-head">
              <strong>订单 {order.orderNo}</strong>
              <span>{order.carrier || "承运商未录入"}</span>
              <span>{order.trackingNo || "运单号未录入"}</span>
            </div>
            <table className="fulfillment-document-print__table">
              <thead>
                <tr>
                  <th>SKU</th>
                  <th>商品 / 规格</th>
                  <th>条码</th>
                  <th className="number">装箱数量</th>
                </tr>
              </thead>
              <tbody>
                {lines.map((line) => (
                  <tr key={line.waveLineId}>
                    <td>{line.skuCode || "—"}</td>
                    <td>{line.skuName || line.productTitle || "—"}</td>
                    <td>{line.barcode || "—"}</td>
                    <td className="number">
                      {line.requiredQuantity - line.shortageQuantity}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        );
      })}
    </div>
  );
}

function SKULabels({
  snapshot,
}: {
  snapshot: FulfillmentWaveDocumentSnapshot;
}) {
  return (
    <div className="fulfillment-document-print__labels">
      {snapshot.lines.map((line) => (
        <article
          key={line.waveLineId}
          className="fulfillment-document-print__label"
        >
          <span className="eyebrow">本地 SKU 标签</span>
          <strong>{line.skuCode || "未编码 SKU"}</strong>
          <span>{line.skuName || line.productTitle || "—"}</span>
          <code>{line.barcode || line.skuCode || line.waveLineId}</code>
          <small>
            {line.locationCode || "未分配库位"} · 数量 {line.requiredQuantity}
          </small>
        </article>
      ))}
    </div>
  );
}

function PackageLabels({
  snapshot,
}: {
  snapshot: FulfillmentWaveDocumentSnapshot;
}) {
  return (
    <div className="fulfillment-document-print__labels">
      {snapshot.orders.map((order) => (
        <article
          key={order.waveOrderId}
          className="fulfillment-document-print__label package"
        >
          <span className="eyebrow">本地包裹标签 · 非承运商面单</span>
          <strong>{order.orderNo}</strong>
          <span>{snapshot.waveNo}</span>
          <code>{order.packageCode || order.trackingNo || order.orderNo}</code>
          <small>{order.carrier || "承运商未录入"}</small>
          <small>{order.trackingNo || "运单号未录入"}</small>
        </article>
      ))}
    </div>
  );
}

function PrintSurface({
  document,
  type,
}: {
  document: FulfillmentWaveDocumentDetail;
  type: FulfillmentWaveDocumentType;
}) {
  return (
    <article
      className="fulfillment-document-print"
      aria-label={`${documentTypeLabel(type)}打印预览`}
    >
      <PrintHeader document={document} type={type} />
      {type === "pick_list" ? <PickList snapshot={document.snapshot} /> : null}
      {type === "packing_list" ? (
        <PackingList snapshot={document.snapshot} />
      ) : null}
      {type === "sku_labels" ? (
        <SKULabels snapshot={document.snapshot} />
      ) : null}
      {type === "package_labels" ? (
        <PackageLabels snapshot={document.snapshot} />
      ) : null}
      <footer>
        快照校验：{document.snapshotHash} · 状态：
        {STATUS_LABELS[document.snapshot.waveStatus]}
      </footer>
    </article>
  );
}

export default function FulfillmentDocumentsPage() {
  const { id = "" } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canOperate = !readonly && can(PERMISSIONS.ORDER_OPERATE);
  const [wave, setWave] = useState<FulfillmentWave>();
  const [documentOptions, setDocumentOptions] = useState<
    Array<{ label: string; value: string }>
  >([]);
  const [selectedDocumentId, setSelectedDocumentId] = useState("");
  const [document, setDocument] = useState<FulfillmentWaveDocumentDetail>();
  const [documentType, setDocumentType] =
    useState<FulfillmentWaveDocumentType>("pick_list");
  const [copies, setCopies] = useState(1);
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(true);
  const [documentLoading, setDocumentLoading] = useState(false);
  const [submitting, setSubmitting] = useState<"generate" | "print" | "">("");
  const [loadError, setLoadError] = useState("");

  const loadOverview = useCallback(async () => {
    if (!id) {
      setLoadError("缺少波次标识，无法加载单据中心。");
      setLoading(false);
      return;
    }
    setLoading(true);
    setLoadError("");
    try {
      const [waveResult, listResult] = await Promise.all([
        getFulfillmentWave(id),
        queryFulfillmentWaveDocuments(id, { page: 1, pageSize: 100 }),
      ]);
      setWave(waveResult);
      const options = listResult.list.map((item) => ({
        value: item.id,
        label: `V${item.version} · 修订 ${item.sourceRevision} · ${formatDateTime(item.createdAt)}`,
      }));
      setDocumentOptions(options);
      setSelectedDocumentId((current) =>
        options.some((item) => item.value === current)
          ? current
          : (options[0]?.value ?? ""),
      );
    } catch (error) {
      setLoadError(
        userFacingError(error, "出库单据中心加载失败，请稍后重试。"),
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

  useEffect(() => {
    if (!id || !selectedDocumentId) {
      setDocument(undefined);
      return;
    }
    let active = true;
    setDocumentLoading(true);
    setLoadError("");
    void getFulfillmentWaveDocument(id, selectedDocumentId)
      .then((result) => {
        if (active) setDocument(result);
      })
      .catch((error) => {
        if (active) {
          setDocument(undefined);
          setLoadError(
            userFacingError(error, "单据快照加载失败，请稍后重试。"),
          );
        }
      })
      .finally(() => {
        if (active) setDocumentLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id, selectedDocumentId]);

  useEffect(() => {
    setReason("");
  }, [documentType, selectedDocumentId]);

  const priorPrints = useMemo(
    () =>
      (document?.printEvents ?? []).filter(
        (event) => event.documentType === documentType,
      ),
    [document, documentType],
  );
  const isReprint = priorPrints.length > 0;

  const generate = useCallback(async () => {
    if (
      !id ||
      !wave ||
      !canOperate ||
      submitting ||
      wave.status === "cancelled"
    )
      return;
    setSubmitting("generate");
    setLoadError("");
    try {
      const created = await generateFulfillmentWaveDocument(id, {
        expectedRevision: wave.revision,
        idempotencyKey:
          createFulfillmentWaveIdempotencyKey("document-generate"),
      });
      message.success(`已生成不可变单据快照 V${created.version}`);
      setDocument(created);
      setSelectedDocumentId(created.id);
      await loadOverview();
    } catch (error) {
      setLoadError(
        userFacingError(error, "生成单据快照失败，请刷新波次后重试。"),
      );
    } finally {
      setSubmitting("");
    }
  }, [canOperate, id, loadOverview, submitting, wave]);

  const print = useCallback(async () => {
    if (!id || !document || !canOperate || submitting) return;
    if (isReprint && reason.trim().length < 2) {
      setLoadError("重打必须填写至少 2 个字符的原因。");
      return;
    }
    setSubmitting("print");
    setLoadError("");
    try {
      await recordFulfillmentWaveDocumentPrint(id, document.id, {
        documentType,
        copies,
        reason: reason.trim() || undefined,
        idempotencyKey: createFulfillmentWaveIdempotencyKey("document-print"),
      });
      const refreshed = await getFulfillmentWaveDocument(id, document.id);
      setDocument(refreshed);
      message.success(
        isReprint
          ? "重打已登记，正在打开浏览器打印"
          : "打印已登记，正在打开浏览器打印",
      );
      window.setTimeout(() => window.print(), 0);
    } catch (error) {
      setLoadError(
        userFacingError(error, "登记打印失败，尚未打开浏览器打印。"),
      );
    } finally {
      setSubmitting("");
    }
  }, [
    canOperate,
    copies,
    document,
    documentType,
    id,
    isReprint,
    reason,
    submitting,
  ]);

  return (
    <PermissionGuard require={PERMISSIONS.ORDER_VIEW} showForbiddenPage>
      <TmPageContainer
        title="履约出库单据中心"
        subTitle="基于波次修订生成可追溯的本地快照；不申请运单号，不生成官方承运商面单，也不控制物理打印机。"
        extra={
          <TmPageHeaderExtra>
            <Button
              icon={<ArrowLeftOutlined />}
              onClick={() => history.push("/orders/fulfillment-waves")}
            >
              返回波次
            </Button>
            <Button
              icon={<ReloadOutlined />}
              disabled={loading}
              onClick={() => void loadOverview()}
            >
              刷新
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {loadError ? (
          <ErrorAlert title={loadError} style={{ marginBottom: 16 }} />
        ) : null}
        {!canOperate ? (
          <Alert
            type="info"
            showIcon
            message="当前账号为只读模式，可查看历史快照和打印记录，但不能生成快照或登记打印。"
            style={{ marginBottom: 16 }}
          />
        ) : null}
        {loading ? (
          <div className="fulfillment-document-center__loading">
            <Spin />
          </div>
        ) : (
          <Row gutter={[16, 16]} align="top">
            <Col xs={24} xl={7}>
              <SectionCard
                title="快照版本"
                description="每个版本固定保存当时的波次、订单、SKU、库位与包裹信息。"
                headerExtra={
                  <Button
                    type="primary"
                    icon={<FileAddOutlined />}
                    loading={submitting === "generate"}
                    disabled={
                      !canOperate ||
                      Boolean(submitting) ||
                      !wave ||
                      wave.status === "cancelled"
                    }
                    onClick={() => void generate()}
                  >
                    生成新版本
                  </Button>
                }
              >
                {wave?.status === "cancelled" ? (
                  <Alert
                    type="warning"
                    showIcon
                    message="波次已取消，只能查看已生成的历史快照。"
                  />
                ) : null}
                {documentOptions.length ? (
                  <Space
                    direction="vertical"
                    size="middle"
                    style={{ width: "100%" }}
                  >
                    <Select
                      aria-label="单据快照版本"
                      value={selectedDocumentId}
                      options={documentOptions}
                      onChange={setSelectedDocumentId}
                      style={{ width: "100%" }}
                    />
                    {document ? (
                      <Descriptions bordered size="small" column={1}>
                        <Descriptions.Item label="波次">
                          {document.snapshot.waveNo}
                        </Descriptions.Item>
                        <Descriptions.Item label="仓库">
                          {warehouseLabel(document.snapshot)}
                        </Descriptions.Item>
                        <Descriptions.Item label="来源状态">
                          <Tag>
                            {STATUS_LABELS[document.snapshot.waveStatus]}
                          </Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label="生成时间">
                          {formatDateTime(document.createdAt)}
                        </Descriptions.Item>
                        <Descriptions.Item label="快照校验">
                          <Typography.Text
                            copyable
                            ellipsis={{ tooltip: document.snapshotHash }}
                          >
                            {document.snapshotHash}
                          </Typography.Text>
                        </Descriptions.Item>
                      </Descriptions>
                    ) : null}
                  </Space>
                ) : (
                  <EmptyState
                    compact
                    title="尚未生成出库单据快照"
                    description="生成后才能预览、登记打印与保留重打审计。"
                    actionLabel={
                      canOperate && wave?.status !== "cancelled"
                        ? "生成首个版本"
                        : undefined
                    }
                    onAction={() => void generate()}
                  />
                )}
              </SectionCard>
            </Col>
            <Col xs={24} xl={17}>
              <SectionCard
                title="打印预览"
                description="计划份数用于审计；实际打印份数仍以浏览器打印对话框为准。"
              >
                {documentLoading ? (
                  <div className="fulfillment-document-center__loading">
                    <Spin />
                  </div>
                ) : null}
                {!documentLoading && document ? (
                  <Space
                    direction="vertical"
                    size="middle"
                    style={{ width: "100%" }}
                  >
                    <div className="fulfillment-document-center__controls">
                      <Select<FulfillmentWaveDocumentType>
                        aria-label="单据类型"
                        value={documentType}
                        options={DOCUMENT_TYPES}
                        optionRender={(option) => (
                          <div>
                            <strong>{option.data.label}</strong>
                            <div className="fulfillment-document-center__option-desc">
                              {option.data.description}
                            </div>
                          </div>
                        )}
                        onChange={setDocumentType}
                        style={{ minWidth: 220 }}
                      />
                      <div className="fulfillment-document-center__copies">
                        <Typography.Text>计划份数</Typography.Text>
                        <InputNumber
                          aria-label="计划打印份数"
                          min={1}
                          max={100}
                          precision={0}
                          value={copies}
                          onChange={(value) => setCopies(value ?? 1)}
                        />
                      </div>
                      <Button
                        type="primary"
                        icon={<PrinterOutlined />}
                        loading={submitting === "print"}
                        disabled={
                          !canOperate ||
                          Boolean(submitting) ||
                          wave?.status === "cancelled"
                        }
                        onClick={() => void print()}
                      >
                        {isReprint ? "登记重打并打开打印" : "登记并打开打印"}
                      </Button>
                    </div>
                    {isReprint ? (
                      <Input.TextArea
                        aria-label="重打原因"
                        value={reason}
                        maxLength={500}
                        showCount
                        autoSize={{ minRows: 2, maxRows: 4 }}
                        placeholder="必填：说明破损、卡纸、信息复核等重打原因"
                        onChange={(event) => setReason(event.target.value)}
                      />
                    ) : null}
                    {documentType === "package_labels" ? (
                      <Alert
                        type="warning"
                        showIcon
                        message="本地包裹标签只用于仓内识别，不可替代承运商官方面单或平台发货凭证。"
                      />
                    ) : null}
                    <div className="fulfillment-document-center__preview">
                      <PrintSurface document={document} type={documentType} />
                    </div>
                  </Space>
                ) : null}
                {!documentLoading && !document ? (
                  <EmptyState compact title="请选择或生成一个快照版本" />
                ) : null}
              </SectionCard>
            </Col>
            {document ? (
              <Col span={24}>
                <SectionCard
                  title="打印与重打审计"
                  description="记录操作员发起浏览器打印的事实，不声明物理打印成功。"
                >
                  <Table<FulfillmentWaveDocumentPrintEvent>
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 10, hideOnSinglePage: true }}
                    scroll={{ x: 760 }}
                    dataSource={document.printEvents ?? []}
                    locale={{ emptyText: "暂无打印登记" }}
                    columns={[
                      {
                        title: "时间",
                        dataIndex: "createdAt",
                        width: 180,
                        render: (value: string) => formatDateTime(value),
                      },
                      {
                        title: "单据",
                        dataIndex: "documentType",
                        width: 140,
                        render: (value: FulfillmentWaveDocumentType) =>
                          documentTypeLabel(value),
                      },
                      {
                        title: "类型",
                        dataIndex: "reprint",
                        width: 100,
                        render: (value: boolean) => (
                          <Tag color={value ? "warning" : "blue"}>
                            {value ? "重打" : "首打"}
                          </Tag>
                        ),
                      },
                      { title: "计划份数", dataIndex: "copies", width: 100 },
                      {
                        title: "原因",
                        dataIndex: "reason",
                        render: (value?: string) => value || "—",
                      },
                    ]}
                  />
                </SectionCard>
              </Col>
            ) : null}
          </Row>
        )}
      </TmPageContainer>
    </PermissionGuard>
  );
}
