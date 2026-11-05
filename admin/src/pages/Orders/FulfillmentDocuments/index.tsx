import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
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
  sourceLabel: string;
  sourceDescription: string;
}> = [
  { value: "pick_list", sourceLabel: "拣货单", sourceDescription: "按库位与 SKU 排列" },
  { value: "packing_list", sourceLabel: "配货单", sourceDescription: "按订单核对装箱明细" },
  { value: "sku_labels", sourceLabel: "规格标签", sourceDescription: "本地商品识别标签" },
  {
    value: "package_labels",
    sourceLabel: "本地包裹标签",
    sourceDescription: "仅供仓内识别，不是承运商面单",
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
  const label = DOCUMENT_TYPES.find((item) => item.value === type)?.sourceLabel;
  return label ? tr(label) : type;
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
          {tr("本地出库单据 · V")}{document.version} {tr("· 来源修订")}{" "}
          {document.sourceRevision}
        </Typography.Text>
      </div>
      <div className="fulfillment-document-print__meta">
        <strong>{snapshot.waveNo}</strong>
        <span>{warehouseLabel(snapshot)}</span>
        <span>{tr("生成：")}{formatDateTime(snapshot.generatedAt)}</span>
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
            <th>{tr("库位")}</th>
            <th>{tr("SKU / 商品")}</th>
            <th>{tr("条码")}</th>
            <th>{tr("订单")}</th>
            <th className="number">{tr("需求")}</th>
            <th className="number">{tr("已拣")}</th>
            <th className="number">{tr("缺货")}</th>
          </tr>
        </thead>
        <tbody>
          {snapshot.lines.map((line) => (
            <tr key={line.waveLineId}>
              <td>{line.locationCode || tr("未分配")}</td>
              <td>
                <strong>{line.skuCode || tr("未编码 SKU")}</strong>
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
              <strong>{tr("订单")} {order.orderNo}</strong>
              <span>{order.carrier || tr("承运商未录入")}</span>
              <span>{order.trackingNo || tr("运单号未录入")}</span>
            </div>
            <table className="fulfillment-document-print__table">
              <thead>
                <tr>
                  <th>SKU</th>
                  <th>{tr("商品 / 规格")}</th>
                  <th>{tr("条码")}</th>
                  <th className="number">{tr("装箱数量")}</th>
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
          <span className="eyebrow">{tr("本地 SKU 标签")}</span>
          <strong>{line.skuCode || tr("未编码 SKU")}</strong>
          <span>{line.skuName || line.productTitle || "—"}</span>
          <code>{line.barcode || line.skuCode || line.waveLineId}</code>
          <small>
            {line.locationCode || tr("未分配库位")} {tr("· 数量")} {line.requiredQuantity}
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
          <span className="eyebrow">{tr("本地包裹标签 · 非承运商面单")}</span>
          <strong>{order.orderNo}</strong>
          <span>{snapshot.waveNo}</span>
          <code>{order.packageCode || order.trackingNo || order.orderNo}</code>
          <small>{order.carrier || tr("承运商未录入")}</small>
          <small>{order.trackingNo || tr("运单号未录入")}</small>
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
      aria-label={trFormat("{value0}打印预览", { value0: String(documentTypeLabel(type)) })}
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
        {tr("快照校验：")}{document.snapshotHash} {tr("· 状态：")}
        {tr(STATUS_LABELS[document.snapshot.waveStatus])}
      </footer>
    </article>
  );
}

export default function FulfillmentDocumentsPage() {
  useLanguage();
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
        label: trFormat("V{value0} · 修订 {value1} · {value2}", { value0: item.version, value1: item.sourceRevision, value2: formatDateTime(item.createdAt) }),
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
      message.success(trFormat("已生成不可变单据快照 V{value0}", { value0: created.version }));
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
          ? tr("重打已登记，正在打开浏览器打印")
          : tr("打印已登记，正在打开浏览器打印"),
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
        title={tr("履约出库单据中心")}
        subTitle={tr("基于波次修订生成可追溯的本地快照；不申请运单号，不生成官方承运商面单，也不控制物理打印机。")}
        extra={
          <TmPageHeaderExtra>
            <Button
              icon={<ArrowLeftOutlined />}
              onClick={() => history.push("/orders/fulfillment-waves")}
            >
              {tr("返回波次")}
            </Button>
            <Button
              icon={<ReloadOutlined />}
              disabled={loading}
              onClick={() => void loadOverview()}
            >
              {tr("刷新")}
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {loadError ? (
          <ErrorAlert title={tr(loadError)} style={{ marginBottom: 16 }} />
        ) : null}
        {!canOperate ? (
          <Alert
            type="info"
            showIcon
            message={tr("当前账号为只读模式，可查看历史快照和打印记录，但不能生成快照或登记打印。")}
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
                title={tr("快照版本")}
                description={tr("每个版本固定保存当时的波次、订单、SKU、库位与包裹信息。")}
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
                    {tr("生成新版本")}
                  </Button>
                }
              >
                {wave?.status === "cancelled" ? (
                  <Alert
                    type="warning"
                    showIcon
                    message={tr("波次已取消，只能查看已生成的历史快照。")}
                  />
                ) : null}
                {documentOptions.length ? (
                  <Space
                    direction="vertical"
                    size="middle"
                    style={{ width: "100%" }}
                  >
                    <Select
                      aria-label={tr("单据快照版本")}
                      value={selectedDocumentId}
                      options={documentOptions}
                      onChange={setSelectedDocumentId}
                      style={{ width: "100%" }}
                    />
                    {document ? (
                      <Descriptions bordered size="small" column={1}>
                        <Descriptions.Item label={tr("波次")}>
                          {document.snapshot.waveNo}
                        </Descriptions.Item>
                        <Descriptions.Item label={tr("仓库")}>
                          {warehouseLabel(document.snapshot)}
                        </Descriptions.Item>
                        <Descriptions.Item label={tr("来源状态")}>
                          <Tag>
                            {tr(STATUS_LABELS[document.snapshot.waveStatus])}
                          </Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label={tr("生成时间")}>
                          {formatDateTime(document.createdAt)}
                        </Descriptions.Item>
                        <Descriptions.Item label={tr("快照校验")}>
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
                    title={tr("尚未生成出库单据快照")}
                    description={tr("生成后才能预览、登记打印与保留重打审计。")}
                    actionLabel={
                      canOperate && wave?.status !== "cancelled"
                        ? tr("生成首个版本")
                        : undefined
                    }
                    onAction={() => void generate()}
                  />
                )}
              </SectionCard>
            </Col>
            <Col xs={24} xl={17}>
              <SectionCard
                title={tr("打印预览")}
                description={tr("计划份数用于审计；实际打印份数仍以浏览器打印对话框为准。")}
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
                        aria-label={tr("单据类型")}
                        value={documentType}
                        options={DOCUMENT_TYPES.map((item) => ({
                          ...item,
                          label: tr(item.sourceLabel),
                          description: tr(item.sourceDescription),
                        }))}
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
                        <Typography.Text>{tr("计划份数")}</Typography.Text>
                        <InputNumber
                          aria-label={tr("计划打印份数")}
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
                        {isReprint ? tr("登记重打并打开打印") : tr("登记并打开打印")}
                      </Button>
                    </div>
                    {isReprint ? (
                      <Input.TextArea
                        aria-label={tr("重打原因")}
                        value={reason}
                        maxLength={500}
                        showCount
                        autoSize={{ minRows: 2, maxRows: 4 }}
                        placeholder={tr("必填：说明破损、卡纸、信息复核等重打原因")}
                        onChange={(event) => setReason(event.target.value)}
                      />
                    ) : null}
                    {documentType === "package_labels" ? (
                      <Alert
                        type="warning"
                        showIcon
                        message={tr("本地包裹标签只用于仓内识别，不可替代承运商官方面单或平台发货凭证。")}
                      />
                    ) : null}
                    <div className="fulfillment-document-center__preview">
                      <PrintSurface document={document} type={documentType} />
                    </div>
                  </Space>
                ) : null}
                {!documentLoading && !document ? (
                  <EmptyState compact title={tr("请选择或生成一个快照版本")} />
                ) : null}
              </SectionCard>
            </Col>
            {document ? (
              <Col span={24}>
                <SectionCard
                  title={tr("打印与重打审计")}
                  description={tr("记录操作员发起浏览器打印的事实，不声明物理打印成功。")}
                >
                  <Table<FulfillmentWaveDocumentPrintEvent>
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 10, hideOnSinglePage: true }}
                    scroll={{ x: 760 }}
                    dataSource={document.printEvents ?? []}
                    locale={{ emptyText: tr("暂无打印登记") }}
                    columns={[
                      {
                        title: tr("时间"),
                        dataIndex: "createdAt",
                        width: 180,
                        render: (value: string) => formatDateTime(value),
                      },
                      {
                        title: tr("单据"),
                        dataIndex: "documentType",
                        width: 140,
                        render: (value: FulfillmentWaveDocumentType) =>
                          documentTypeLabel(value),
                      },
                      {
                        title: tr("类型"),
                        dataIndex: "reprint",
                        width: 100,
                        render: (value: boolean) => (
                          <Tag color={value ? "warning" : "blue"}>
                            {value ? tr("重打") : tr("首打")}
                          </Tag>
                        ),
                      },
                      { title: tr("计划份数"), dataIndex: "copies", width: 100 },
                      {
                        title: tr("原因"),
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
