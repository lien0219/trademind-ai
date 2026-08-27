import {
  ArrowLeftOutlined,
  CheckCircleOutlined,
  ReloadOutlined,
  ScanOutlined,
} from "@ant-design/icons";
import { history, useParams } from "@umijs/max";
import {
  Alert,
  Button,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Progress,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import type { InputRef } from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ErrorAlert, SectionCard, TmPageContainer } from "@/components/ui";
import { usePermission } from "@/hooks/usePermission";
import {
  createFulfillmentWaveIdempotencyKey,
  getFulfillmentWave,
  verifyFulfillmentWavePack,
  type FulfillmentWave,
  type FulfillmentWaveLine,
  type FulfillmentWaveOrder,
} from "@/services/fulfillmentWaves";
import { PERMISSIONS } from "@/utils/permission";
import "./index.less";

type LineProgress = { scannedCode: string; verifiedQuantity: number };

function expectedCode(line: FulfillmentWaveLine) {
  return (line.barcode || line.skuCode || "").trim();
}

function userFacingError(error: unknown, fallback: string) {
  const text = (error as Error)?.message?.trim();
  return text && /[\u3400-\u9fff]/.test(text) ? text : fallback;
}

export default function FulfillmentPackVerificationPage() {
  const { id = "" } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canOperate = !readonly && can(PERMISSIONS.ORDER_OPERATE);
  const [wave, setWave] = useState<FulfillmentWave>();
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [scanError, setScanError] = useState("");
  const [orderScan, setOrderScan] = useState("");
  const [itemScan, setItemScan] = useState("");
  const [selectedOrder, setSelectedOrder] = useState<FulfillmentWaveOrder>();
  const [lineProgress, setLineProgress] = useState<Record<string, LineProgress>>({});
  const [carrier, setCarrier] = useState("");
  const [trackingNo, setTrackingNo] = useState("");
  const [trackingUrl, setTrackingUrl] = useState("");
  const [packageCode, setPackageCode] = useState("");
  const [actualWeightGrams, setActualWeightGrams] = useState<number>();
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const itemInputRef = useRef<InputRef>(null);

  const load = useCallback(async () => {
    if (!id) {
      setLoadError("缺少波次标识，无法加载扫描工作台。");
      setLoading(false);
      return;
    }
    setLoading(true);
    setLoadError("");
    setWave(undefined);
    setSelectedOrder(undefined);
    setOrderScan("");
    setLineProgress({});
    setScanError("");
    try {
      setWave(await getFulfillmentWave(id));
    } catch (error) {
      setLoadError(userFacingError(error, "波次加载失败，请稍后重试。"));
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const readyOrders = useMemo(
    () =>
      (wave?.orders ?? []).filter((order) =>
        ["ready_to_pack", "failed"].includes(order.status),
      ),
    [wave?.orders],
  );
  const selectedLines = useMemo(
    () =>
      (wave?.lines ?? []).filter(
        (line) => line.waveOrderId === selectedOrder?.id,
      ),
    [selectedOrder?.id, wave?.lines],
  );
  const verifiedQuantity = selectedLines.reduce(
    (sum, line) => sum + (lineProgress[line.id]?.verifiedQuantity ?? 0),
    0,
  );
  const requiredQuantity = selectedLines.reduce(
    (sum, line) => sum + line.requiredQuantity,
    0,
  );
  const allLinesVerified =
    selectedLines.length > 0 &&
    selectedLines.every(
      (line) =>
        expectedCode(line) &&
        lineProgress[line.id]?.verifiedQuantity === line.requiredQuantity,
    );
  const packageMatches =
    Boolean(trackingNo.trim()) &&
    trackingNo.trim().toLocaleLowerCase() ===
      packageCode.trim().toLocaleLowerCase();
  const canSubmit =
    canOperate &&
    Boolean(selectedOrder) &&
    Boolean(carrier.trim()) &&
    packageMatches &&
    allLinesVerified &&
    !submitting;

  const selectOrderByScan = () => {
    const scanned = orderScan.trim();
    if (!scanned) return;
    const matched = readyOrders.find(
      (order) => order.orderNo.trim().toLocaleLowerCase() === scanned.toLocaleLowerCase(),
    );
    if (!matched) {
      setScanError("订单号不属于当前波次，或订单尚未完成拣货。已阻止继续复核。");
      return;
    }
    setSelectedOrder(matched);
    setLineProgress({});
    setCarrier(matched.carrier ?? "");
    setTrackingNo(matched.trackingNo ?? "");
    setTrackingUrl(matched.trackingUrl ?? "");
    setPackageCode("");
    setActualWeightGrams(undefined);
    setScanError("");
    setItemScan("");
    window.setTimeout(() => itemInputRef.current?.focus(), 0);
  };

  const recordItemScan = () => {
    const scanned = itemScan.trim();
    if (!selectedOrder || !scanned) return;
    const matched = selectedLines.find(
      (line) =>
        expectedCode(line).toLocaleLowerCase() === scanned.toLocaleLowerCase() &&
        (lineProgress[line.id]?.verifiedQuantity ?? 0) < line.requiredQuantity,
    );
    if (!matched) {
      setScanError("商品条码与当前订单待复核商品不匹配，或该商品数量已经完成。未记录本次扫描。");
      setItemScan("");
      return;
    }
    setLineProgress((current) => ({
      ...current,
      [matched.id]: {
        scannedCode: scanned,
        verifiedQuantity: (current[matched.id]?.verifiedQuantity ?? 0) + 1,
      },
    }));
    setScanError("");
    setItemScan("");
  };

  const submit = async () => {
    if (!wave || !selectedOrder || !canSubmit || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    setScanError("");
    try {
      const next = await verifyFulfillmentWavePack(wave.id, selectedOrder.orderId, {
        expectedRevision: wave.revision,
        idempotencyKey: createFulfillmentWaveIdempotencyKey("verify-pack"),
        scannedOrderNo: orderScan.trim(),
        carrier: carrier.trim(),
        trackingNo: trackingNo.trim(),
        trackingUrl: trackingUrl.trim() || undefined,
        packageCode: packageCode.trim(),
        actualWeightGrams,
        lines: selectedLines.map((line) => ({
          lineId: line.id,
          scannedCode: lineProgress[line.id].scannedCode,
          verifiedQuantity: lineProgress[line.id].verifiedQuantity,
        })),
      });
      setWave(next);
      setSelectedOrder(undefined);
      setOrderScan("");
      setLineProgress({});
      setCarrier("");
      setTrackingNo("");
      setTrackingUrl("");
      setPackageCode("");
      setActualWeightGrams(undefined);
      message.success("扫描复核已记录，订单已进入已打包状态。");
    } catch (error) {
      setScanError(
        userFacingError(
          error,
          "扫描复核提交失败。请刷新波次，确认版本和扫描内容后重试。",
        ),
      );
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  return (
    <TmPageContainer
      className="tm-pack-verification-page"
      title="出库扫描复核"
      subTitle="本地校验订单、商品和面单，不调用真实物流、打印机、电子秤或平台接口。"
      extra={
        <Space wrap>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => history.push("/orders/fulfillment-waves")}
          >
            返回波次
          </Button>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>
            刷新
          </Button>
        </Space>
      }
    >
      {!canOperate ? (
        <Alert
          showIcon
          type="warning"
          message="当前账号为只读或缺少订单操作权限，所有扫描与提交操作已禁用。"
        />
      ) : null}
      {loadError ? (
        <ErrorAlert
          title={loadError}
          actionHint={<Button onClick={() => void load()}>重新加载</Button>}
        />
      ) : null}
      {loading ? <Spin size="large" className="tm-pack-verification-loading" /> : null}
      {!loading && wave ? (
        <>
          {!wave.packingVerificationRequired ? (
            <Alert
              showIcon
              type="info"
              message="这是扫描复核功能上线前创建的历史波次，请返回波次详情使用原有打包复核。"
            />
          ) : null}
          <SectionCard title="波次概览" description="每次成功提交只复核一个订单，波次版本随事务原子递增。">
            <Descriptions column={{ xs: 1, sm: 2, md: 4 }} size="small">
              <Descriptions.Item label="波次号">{wave.waveNo}</Descriptions.Item>
              <Descriptions.Item label="仓库">
                {[wave.warehouseCode, wave.warehouseName].filter(Boolean).join(" · ") || wave.warehouseId}
              </Descriptions.Item>
              <Descriptions.Item label="当前版本">{wave.revision}</Descriptions.Item>
              <Descriptions.Item label="待复核订单">{readyOrders.length}</Descriptions.Item>
            </Descriptions>
          </SectionCard>

          <SectionCard
            title="1. 扫描订单"
            description="仅接受当前波次中已完成拣货或待重试的订单。"
          >
            <Input
              aria-label="扫描订单号"
              autoFocus
              allowClear
              disabled={
                !canOperate ||
                !wave.packingVerificationRequired ||
                submitting ||
                Boolean(selectedOrder)
              }
              prefix={<ScanOutlined />}
              placeholder="扫描订单号后按回车"
              value={orderScan}
              onChange={(event) => setOrderScan(event.target.value)}
              onPressEnter={selectOrderByScan}
            />
            {scanError ? <Alert showIcon type="error" message={scanError} /> : null}
            {!selectedOrder && readyOrders.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有待扫描复核的订单" />
            ) : null}
          </SectionCard>

          {selectedOrder ? (
            <>
              <SectionCard
                title={`2. 复核商品 · ${selectedOrder.orderNo}`}
                description="每扫描一次商品条码计入 1 件；数量未齐或条码不符时不能提交。"
                headerExtra={<Tag color="processing">待复核</Tag>}
              >
                <Input
                  ref={itemInputRef}
                  aria-label="扫描商品条码"
                  allowClear
                  disabled={!canOperate || submitting}
                  prefix={<ScanOutlined />}
                  placeholder="连续扫描商品条码，每次扫描后按回车"
                  value={itemScan}
                  onChange={(event) => setItemScan(event.target.value)}
                  onPressEnter={recordItemScan}
                />
                <Progress
                  percent={requiredQuantity ? Math.round((verifiedQuantity / requiredQuantity) * 100) : 0}
                  format={() => `${verifiedQuantity} / ${requiredQuantity} 件`}
                  status={allLinesVerified ? "success" : "active"}
                />
                <div className="tm-pack-verification-table">
                  <Table<FulfillmentWaveLine>
                    rowKey="id"
                    pagination={false}
                    size="small"
                    scroll={{ x: 760 }}
                    dataSource={selectedLines}
                    columns={[
                      { title: "商品", dataIndex: "productTitle", width: 220, render: (value) => value || "-" },
                      { title: "SKU", dataIndex: "skuCode", width: 140, render: (value) => value || "-" },
                      { title: "应扫条码", width: 160, render: (_, line) => expectedCode(line) || "未配置" },
                      { title: "应复核", dataIndex: "requiredQuantity", width: 90 },
                      {
                        title: "已复核",
                        width: 100,
                        render: (_, line) => lineProgress[line.id]?.verifiedQuantity ?? 0,
                      },
                      {
                        title: "状态",
                        width: 100,
                        render: (_, line) =>
                          lineProgress[line.id]?.verifiedQuantity === line.requiredQuantity ? (
                            <Tag color="success">已完成</Tag>
                          ) : (
                            <Tag>待扫描</Tag>
                          ),
                      },
                    ]}
                  />
                </div>
              </SectionCard>

              <SectionCard title="3. 扫描面单并确认" description="面单条码必须与运单号一致；称重为可选的正整数克数。">
                <Form layout="vertical" className="tm-pack-verification-form">
                  <Form.Item label="承运商" required>
                    <Input aria-label="承运商" maxLength={128} disabled={!canOperate || submitting} value={carrier} onChange={(event) => setCarrier(event.target.value)} />
                  </Form.Item>
                  <Form.Item label="运单号" required>
                    <Input aria-label="运单号" maxLength={255} disabled={!canOperate || submitting} value={trackingNo} onChange={(event) => setTrackingNo(event.target.value)} />
                  </Form.Item>
                  <Form.Item label="扫描面单条码" required validateStatus={packageCode && !packageMatches ? "error" : undefined} help={packageCode && !packageMatches ? "面单条码与运单号不一致" : undefined}>
                    <Input aria-label="扫描面单条码" maxLength={255} disabled={!canOperate || submitting} prefix={<ScanOutlined />} value={packageCode} onChange={(event) => setPackageCode(event.target.value)} />
                  </Form.Item>
                  <Form.Item label="实际重量（克，可选）">
                    <InputNumber aria-label="实际重量（克，可选）" min={1} max={5_000_000} precision={0} disabled={!canOperate || submitting} value={actualWeightGrams} onChange={(value) => setActualWeightGrams(value ?? undefined)} />
                  </Form.Item>
                  <Form.Item label="物流查询地址（可选）" className="tm-pack-verification-form-wide">
                    <Input aria-label="物流查询地址（可选）" maxLength={2048} disabled={!canOperate || submitting} value={trackingUrl} onChange={(event) => setTrackingUrl(event.target.value)} />
                  </Form.Item>
                </Form>
                <Space wrap>
                  <Button
                    type="primary"
                    icon={<CheckCircleOutlined />}
                    loading={submitting}
                    disabled={!canSubmit}
                    onClick={() => void submit()}
                  >
                    确认复核并标记已打包
                  </Button>
                  <Button
                    disabled={submitting}
                    onClick={() => {
                      setSelectedOrder(undefined);
                      setOrderScan("");
                      setLineProgress({});
                      setScanError("");
                    }}
                  >
                    重置当前订单
                  </Button>
                  <Typography.Text type="secondary">
                    提交后不能修改扫描审计记录；提交前可重置并重新扫描。
                  </Typography.Text>
                </Space>
              </SectionCard>
            </>
          ) : null}
        </>
      ) : null}
    </TmPageContainer>
  );
}
