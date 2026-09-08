import {
  CalculatorOutlined,
  EyeOutlined,
  HistoryOutlined,
  PlusOutlined,
  ReloadOutlined,
  RollbackOutlined,
} from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  message,
  type TableColumnsType,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import AppDrawer from '@/components/AppDrawer';
import PermissionGuard from '@/components/PermissionGuard';
import {
  EmptyState,
  ErrorAlert,
  OperationToolbar,
  TmPageContainer,
  TmPageHeaderExtra,
  TmProTable,
} from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { useUrlDrawerState } from '@/hooks/useUrlState';
import { listInventoryWarehouses, type InventoryWarehouse } from '@/services/inventory';
import { formatProfitAmount } from '@/services/profitability';
import {
  confirmWarehouseFee,
  createWarehouseFeeAdjustment,
  createWarehouseFeeRateCard,
  getWarehouseFeeRateCard,
  getWarehouseFeeSnapshot,
  listWarehouseFeeRateCards,
  previewWarehouseFee,
  queryWarehouseFeeCandidates,
  queryWarehouseFeeSnapshots,
  reverseWarehouseFeeAdjustment,
  updateWarehouseFeeRateCard,
  warehouseFeeIdempotencyKey,
  type WarehouseFeeAdjustment,
  type WarehouseFeeCandidate,
  type WarehouseFeePreview,
  type WarehouseFeeRateCard,
  type WarehouseFeeRateCardDetail,
  type WarehouseFeeRateRevision,
  type WarehouseFeeSnapshot,
  type WarehouseFeeSnapshotDetail,
} from '@/services/warehouseFees';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import './index.less';

type RateFormValues = {
  warehouseId: string;
  code: string;
  name: string;
  currency: string;
  outboundBaseFeeMinor: number;
  pickingFeePerItemMinor: number;
  packingFeePerPackageMinor: number;
  status: 'active' | 'inactive';
};

type AdjustmentFormValues = {
  amountMinor?: number;
  reason: string;
};

const CURRENCY_OPTIONS = ['CNY', 'USD', 'EUR', 'GBP', 'JPY', 'KRW', 'SGD', 'AUD', 'CAD'].map(
  (value) => ({ value, label: value }),
);

function warehouseLabel(row: { warehouseCode?: string; warehouseName?: string; warehouseId: string }) {
  return [row.warehouseCode, row.warehouseName].filter(Boolean).join(' · ') || row.warehouseId;
}

function rateLabel(row: WarehouseFeeRateCard) {
  return `${row.code} · ${row.name} · ${row.currency}`;
}

type StableRequestKey = {
  signature: string;
  key: string;
};

function stableRequestKey(current: StableRequestKey | undefined, action: string, signature: string) {
  if (current?.signature === signature) return current;
  return { signature, key: warehouseFeeIdempotencyKey(action) };
}

export default function WarehouseFeesPage() {
  const { can } = usePermission();
  const canManage = can(PERMISSIONS.WAREHOUSE_FEE_MANAGE);
  const candidateActionRef = useRef<ActionType>();
  const ledgerActionRef = useRef<ActionType>();
  const rateActionRef = useRef<ActionType>();
  const confirmationLock = useRef(false);
  const previewRequestLock = useRef(false);
  const rateSubmissionLock = useRef(false);
  const adjustmentSubmissionLock = useRef(false);
  const previewSession = useRef(0);
  const confirmationRequestKey = useRef<StableRequestKey>();
  const adjustmentRequestKey = useRef<StableRequestKey>();
  const detailRequest = useRef(0);
  const historyRequest = useRef(0);
  const drawer = useUrlDrawerState('warehouse-fee');
  const [rateForm] = Form.useForm<RateFormValues>();
  const [adjustmentForm] = Form.useForm<AdjustmentFormValues>();
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const [warehouseError, setWarehouseError] = useState('');
  const [candidateOrderNo, setCandidateOrderNo] = useState('');
  const [candidateWarehouseID, setCandidateWarehouseID] = useState<string>();
  const [candidateError, setCandidateError] = useState('');
  const [ledgerOrderNo, setLedgerOrderNo] = useState('');
  const [ledgerWarehouseID, setLedgerWarehouseID] = useState<string>();
  const [ledgerError, setLedgerError] = useState('');
  const [previewOpen, setPreviewOpen] = useState(false);
  const [candidate, setCandidate] = useState<WarehouseFeeCandidate>();
  const [candidateRates, setCandidateRates] = useState<WarehouseFeeRateCard[]>([]);
  const [candidateRateError, setCandidateRateError] = useState('');
  const [selectedRateID, setSelectedRateID] = useState<string>();
  const [preview, setPreview] = useState<WarehouseFeePreview>();
  const [previewing, setPreviewing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [rateModalOpen, setRateModalOpen] = useState(false);
  const [editingRate, setEditingRate] = useState<WarehouseFeeRateCard>();
  const [rateSubmitting, setRateSubmitting] = useState(false);
  const [rateError, setRateError] = useState('');
  const [historyDetail, setHistoryDetail] = useState<WarehouseFeeRateCardDetail>();
  const [historyLoading, setHistoryLoading] = useState(false);
  const [detail, setDetail] = useState<WarehouseFeeSnapshotDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [adjustmentOpen, setAdjustmentOpen] = useState(false);
  const [reversingAdjustment, setReversingAdjustment] = useState<WarehouseFeeAdjustment>();
  const [adjusting, setAdjusting] = useState(false);

  const warehouseOptions = useMemo(
    () => warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` })),
    [warehouses],
  );

  useEffect(() => {
    if (!rateModalOpen) return;
    if (editingRate) {
      rateForm.setFieldsValue({
        warehouseId: editingRate.warehouseId,
        code: editingRate.code,
        name: editingRate.name,
        currency: editingRate.currency,
        outboundBaseFeeMinor: editingRate.outboundBaseFeeMinor,
        pickingFeePerItemMinor: editingRate.pickingFeePerItemMinor,
        packingFeePerPackageMinor: editingRate.packingFeePerPackageMinor,
        status: editingRate.status,
      });
      return;
    }
    rateForm.resetFields();
    rateForm.setFieldsValue({
      currency: 'CNY',
      status: 'active',
      outboundBaseFeeMinor: 0,
      pickingFeePerItemMinor: 0,
      packingFeePerPackageMinor: 0,
    });
  }, [editingRate, rateForm, rateModalOpen]);

  useEffect(() => {
    if (adjustmentOpen) adjustmentForm.resetFields();
  }, [adjustmentForm, adjustmentOpen, reversingAdjustment]);

  const loadWarehouses = useCallback(async () => {
    try {
      const result = await listInventoryWarehouses();
      setWarehouses(result.list || []);
      setWarehouseError('');
    } catch (error) {
      setWarehouseError((error as Error)?.message || '仓库筛选项加载失败');
    }
  }, []);

  useEffect(() => {
    void loadWarehouses();
  }, [loadWarehouses]);

  const loadDetail = useCallback(async (id: string) => {
    const requestID = ++detailRequest.current;
    setDetailLoading(true);
    setDetailError('');
    try {
      const result = await getWarehouseFeeSnapshot(id);
      if (detailRequest.current === requestID) setDetail(result);
    } catch (error) {
      if (detailRequest.current === requestID) {
        setDetail(undefined);
        setDetailError((error as Error)?.message || '仓库操作费详情加载失败');
      }
    } finally {
      if (detailRequest.current === requestID) setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (drawer.id) void loadDetail(drawer.id);
    else {
      detailRequest.current += 1;
      setDetail(undefined);
      setDetailError('');
      setDetailLoading(false);
    }
  }, [drawer.id, loadDetail]);

  const resetPreview = useCallback(() => {
    previewSession.current += 1;
    confirmationLock.current = false;
    previewRequestLock.current = false;
    confirmationRequestKey.current = undefined;
    setCandidate(undefined);
    setCandidateRates([]);
    setCandidateRateError('');
    setSelectedRateID(undefined);
    setPreview(undefined);
    setPreviewing(false);
    setConfirming(false);
  }, []);

  const openPreview = useCallback(async (row: WarehouseFeeCandidate) => {
    if (!canManage || row.confirmed || !row.packVerificationId) return;
    const sessionID = ++previewSession.current;
    previewRequestLock.current = false;
    confirmationRequestKey.current = undefined;
    setCandidate(row);
    setPreview(undefined);
    setSelectedRateID(undefined);
    setCandidateRates([]);
    setCandidateRateError('');
    setPreviewOpen(true);
    setPreviewing(true);
    try {
      const result = await listWarehouseFeeRateCards({ warehouseId: row.warehouseId });
      if (previewSession.current !== sessionID) return;
      const rates = (result.list || []).filter((rate) => rate.status === 'active');
      setCandidateRates(rates);
      setCandidateRateError('');
      if (rates.length === 1) setSelectedRateID(rates[0].id);
    } catch (error) {
      if (previewSession.current !== sessionID) return;
      const text = (error as Error)?.message || '适用费率卡加载失败';
      setCandidateRateError(text);
      message.error(text);
    } finally {
      if (previewSession.current === sessionID) setPreviewing(false);
    }
  }, [canManage]);

  const runPreview = useCallback(async () => {
    if (!candidate || !selectedRateID || previewing || previewRequestLock.current) return;
    const rate = candidateRates.find((row) => row.id === selectedRateID);
    if (!rate) return;
    const sessionID = previewSession.current;
    previewRequestLock.current = true;
    setPreviewing(true);
    try {
      const result = await previewWarehouseFee({
        orderId: candidate.orderId,
        rateCardId: rate.id,
        rateCardRevision: rate.revision,
      });
      if (previewSession.current !== sessionID) return;
      confirmationRequestKey.current = stableRequestKey(
        undefined,
        'confirm',
        `${result.orderId}:${result.rateCardId}:${result.rateCardRevision}:${result.waveRevision}:${result.calculationHash}`,
      );
      setPreview(result);
    } catch (error) {
      if (previewSession.current !== sessionID) return;
      setPreview(undefined);
      message.error((error as Error)?.message || '仓库操作费预览失败');
    } finally {
      if (previewSession.current === sessionID) {
        previewRequestLock.current = false;
        setPreviewing(false);
      }
    }
  }, [candidate, candidateRates, previewing, selectedRateID]);

  const confirmPreview = useCallback(async () => {
    if (!preview || confirming || confirmationLock.current) return;
    confirmationLock.current = true;
    setConfirming(true);
    try {
      const signature = `${preview.orderId}:${preview.rateCardId}:${preview.rateCardRevision}:${preview.waveRevision}:${preview.calculationHash}`;
      const requestKey = stableRequestKey(confirmationRequestKey.current, 'confirm', signature);
      confirmationRequestKey.current = requestKey;
      const result = await confirmWarehouseFee({
        orderId: preview.orderId,
        rateCardId: preview.rateCardId,
        rateCardRevision: preview.rateCardRevision,
        waveRevision: preview.waveRevision,
        calculationHash: preview.calculationHash,
        idempotencyKey: requestKey.key,
      });
      message.success(result.replayed ? '该订单费用已确认，无需重复登记' : '仓库操作费已确认');
      setPreviewOpen(false);
      resetPreview();
      void candidateActionRef.current?.reload();
      void ledgerActionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || '仓库操作费确认失败');
    } finally {
      confirmationLock.current = false;
      setConfirming(false);
    }
  }, [confirming, preview, resetPreview]);

  const openRateModal = useCallback((row?: WarehouseFeeRateCard) => {
    setEditingRate(row);
    setRateError('');
    setRateModalOpen(true);
  }, []);

  const saveRate = useCallback(async () => {
    if (!canManage || rateSubmitting || rateSubmissionLock.current) return;
    rateSubmissionLock.current = true;
    let values: RateFormValues;
    try {
      values = await rateForm.validateFields();
    } catch {
      rateSubmissionLock.current = false;
      return;
    }
    setRateSubmitting(true);
    setRateError('');
    try {
      if (editingRate) {
        await updateWarehouseFeeRateCard(editingRate.id, {
          expectedRevision: editingRate.revision,
          name: values.name.trim(),
          currency: values.currency,
          outboundBaseFeeMinor: values.outboundBaseFeeMinor,
          pickingFeePerItemMinor: values.pickingFeePerItemMinor,
          packingFeePerPackageMinor: values.packingFeePerPackageMinor,
          status: values.status,
        });
        message.success('费率卡已生成新修订');
      } else {
        await createWarehouseFeeRateCard({
          warehouseId: values.warehouseId,
          code: values.code.trim().toUpperCase(),
          name: values.name.trim(),
          currency: values.currency,
          outboundBaseFeeMinor: values.outboundBaseFeeMinor,
          pickingFeePerItemMinor: values.pickingFeePerItemMinor,
          packingFeePerPackageMinor: values.packingFeePerPackageMinor,
        });
        message.success('费率卡已创建');
      }
      setRateModalOpen(false);
      setEditingRate(undefined);
      void rateActionRef.current?.reload();
    } catch (error) {
      const text = (error as Error)?.message || '费率卡保存失败';
      setRateError(text);
      message.error(text);
    } finally {
      rateSubmissionLock.current = false;
      setRateSubmitting(false);
    }
  }, [canManage, editingRate, rateForm, rateSubmitting]);

  const showRateHistory = useCallback(async (row: WarehouseFeeRateCard) => {
    const requestID = ++historyRequest.current;
    setHistoryLoading(true);
    setHistoryDetail(undefined);
    try {
      const result = await getWarehouseFeeRateCard(row.id);
      if (historyRequest.current === requestID) setHistoryDetail(result);
    } catch (error) {
      if (historyRequest.current === requestID) message.error((error as Error)?.message || '费率修订历史加载失败');
    } finally {
      if (historyRequest.current === requestID) setHistoryLoading(false);
    }
  }, []);

  const openAdjustment = useCallback((row?: WarehouseFeeAdjustment) => {
    adjustmentRequestKey.current = undefined;
    setReversingAdjustment(row);
    setAdjustmentOpen(true);
  }, []);

  const saveAdjustment = useCallback(async () => {
    if (!detail || !canManage || adjusting || adjustmentSubmissionLock.current) return;
    adjustmentSubmissionLock.current = true;
    let values: AdjustmentFormValues;
    try {
      values = await adjustmentForm.validateFields();
    } catch {
      adjustmentSubmissionLock.current = false;
      return;
    }
    setAdjusting(true);
    try {
      if (reversingAdjustment) {
        const reason = values.reason.trim();
        const requestKey = stableRequestKey(
          adjustmentRequestKey.current,
          'reverse',
          `${detail.id}:${reversingAdjustment.id}:${reason}`,
        );
        adjustmentRequestKey.current = requestKey;
        await reverseWarehouseFeeAdjustment(detail.id, reversingAdjustment.id, {
          reason,
          idempotencyKey: requestKey.key,
        });
        message.success('调整事实已冲正');
      } else {
        if (typeof values.amountMinor !== 'number') return;
        const reason = values.reason.trim();
        const requestKey = stableRequestKey(
          adjustmentRequestKey.current,
          'adjust',
          `${detail.id}:${values.amountMinor}:${reason}`,
        );
        adjustmentRequestKey.current = requestKey;
        await createWarehouseFeeAdjustment(detail.id, {
          amountMinor: values.amountMinor,
          reason,
          idempotencyKey: requestKey.key,
        });
        message.success('费用调整已追加');
      }
      setAdjustmentOpen(false);
      setReversingAdjustment(undefined);
      adjustmentRequestKey.current = undefined;
      await loadDetail(detail.id);
      void ledgerActionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || '费用调整失败');
    } finally {
      adjustmentSubmissionLock.current = false;
      setAdjusting(false);
    }
  }, [adjusting, adjustmentForm, canManage, detail, loadDetail, reversingAdjustment]);

  const candidateColumns: ProColumns<WarehouseFeeCandidate>[] = [
    { title: '订单号', dataIndex: 'orderNo', fixed: 'left', width: 190, copyable: true, ellipsis: true },
    { title: '店铺', dataIndex: 'shopName', width: 160, ellipsis: true, render: (value) => value || '本地订单' },
    { title: '仓库', width: 190, ellipsis: true, render: (_, row) => warehouseLabel(row) },
    { title: '履约波次', dataIndex: 'waveNo', width: 160, ellipsis: true },
    { title: '商品数量', dataIndex: 'itemQuantity', width: 100, align: 'right' },
    { title: '包裹数量', dataIndex: 'packageQuantity', width: 100, align: 'right' },
    { title: '包裹编码', dataIndex: 'packageCode', width: 180, ellipsis: true, render: (value) => value || '未形成复核事实' },
    { title: '费用状态', width: 110, render: (_, row) => row.confirmed ? <Tag color="success">已确认</Tag> : row.packVerificationId ? <Tag color="processing">待登记</Tag> : <Tag color="error">已阻断</Tag> },
    { title: '履约时间', dataIndex: 'fulfilledAt', width: 180, render: (value) => formatDateTime(value as string) },
    {
      title: '操作', valueType: 'option', width: 118, fixed: 'right',
      render: (_, row) => row.confirmed && row.snapshotId
        ? <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.snapshotId!)}>查看</Button>
        : <Tooltip title={!canManage ? '当前账号无费用登记权限' : !row.packVerificationId ? '缺少打包扫描复核事实' : undefined}>
          <Button type="link" size="small" icon={<CalculatorOutlined />} disabled={!canManage || !row.packVerificationId} onClick={() => void openPreview(row)}>预览</Button>
        </Tooltip>,
    },
  ];

  const ledgerColumns: ProColumns<WarehouseFeeSnapshot>[] = [
    { title: '订单号', dataIndex: 'orderNo', fixed: 'left', width: 190, copyable: true, ellipsis: true },
    { title: '仓库', width: 190, ellipsis: true, render: (_, row) => warehouseLabel(row) },
    { title: '费率卡', width: 180, ellipsis: true, render: (_, row) => `${row.rateCardCode} · R${row.rateCardRevision}` },
    { title: '基础费用', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: '调整合计', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.adjustmentMinor, row.currency) },
    { title: '当前费用', width: 130, align: 'right', render: (_, row) => <Typography.Text strong>{formatProfitAmount(row.netAmountMinor, row.currency)}</Typography.Text> },
    { title: '调整数', dataIndex: 'adjustmentCount', width: 86, align: 'right' },
    { title: '确认时间', dataIndex: 'confirmedAt', width: 180, render: (value) => formatDateTime(value as string) },
    { title: '操作', valueType: 'option', fixed: 'right', width: 90, render: (_, row) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.id)}>查看</Button> },
  ];

  const rateColumns: ProColumns<WarehouseFeeRateCard>[] = [
    { title: '费率编码', dataIndex: 'code', fixed: 'left', width: 150, copyable: true },
    { title: '名称', dataIndex: 'name', width: 180, ellipsis: true },
    { title: '适用仓库', width: 190, ellipsis: true, render: (_, row) => warehouseLabel(row) },
    { title: '出库基础费', width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.outboundBaseFeeMinor, row.currency) },
    { title: '拣货单价 / 件', width: 140, align: 'right', render: (_, row) => formatProfitAmount(row.pickingFeePerItemMinor, row.currency) },
    { title: '打包单价 / 包裹', width: 150, align: 'right', render: (_, row) => formatProfitAmount(row.packingFeePerPackageMinor, row.currency) },
    { title: '修订', dataIndex: 'revision', width: 80, render: (value) => `R${value}` },
    { title: '状态', dataIndex: 'status', width: 90, render: (value) => value === 'active' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag> },
    {
      title: '操作', valueType: 'option', fixed: 'right', width: 165,
      render: (_, row) => <Space size={0}>
        <Button type="link" size="small" icon={<HistoryOutlined />} onClick={() => void showRateHistory(row)}>历史</Button>
        <Button type="link" size="small" disabled={!canManage} onClick={() => openRateModal(row)}>修订</Button>
      </Space>,
    },
  ];

  const adjustmentColumns: TableColumnsType<WarehouseFeeAdjustment> = [
    { title: '类型', dataIndex: 'factType', width: 90, render: (value) => value === 'reversal' ? <Tag>冲正</Tag> : <Tag color="processing">调整</Tag> },
    { title: '金额', width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: '原因', dataIndex: 'reason', ellipsis: true },
    { title: '时间', dataIndex: 'createdAt', width: 180, render: (value) => formatDateTime(value as string) },
    {
      title: '操作', width: 90,
      render: (_, row) => {
        const reversed = detail?.adjustments.some((candidateRow) => candidateRow.reversesAdjustmentId === row.id);
        return row.factType === 'adjustment' ? <Button type="link" size="small" icon={<RollbackOutlined />} disabled={!canManage || reversed} onClick={() => openAdjustment(row)}>{reversed ? '已冲正' : '冲正'}</Button> : null;
      },
    },
  ];

  const historyColumns: TableColumnsType<WarehouseFeeRateRevision> = [
    { title: '修订', dataIndex: 'revision', width: 75, render: (value) => `R${value}` },
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '出库基础费', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.outboundBaseFeeMinor, row.currency) },
    { title: '拣货 / 件', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.pickingFeePerItemMinor, row.currency) },
    { title: '打包 / 包裹', width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.packingFeePerPackageMinor, row.currency) },
    { title: '状态', dataIndex: 'status', width: 80, render: (value) => value === 'active' ? '启用' : '停用' },
    { title: '生成时间', dataIndex: 'createdAt', width: 180, render: (value) => formatDateTime(value as string) },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.WAREHOUSE_FEE_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-warehouse-fee-page"
        title="仓库操作费台账"
        subTitle="按已完成履约事实确认出库、拣货和打包费用"
        extra={<TmPageHeaderExtra><Button icon={<ReloadOutlined />} onClick={() => { void loadWarehouses(); void candidateActionRef.current?.reload(); void ledgerActionRef.current?.reload(); void rateActionRef.current?.reload(); }}>刷新</Button></TmPageHeaderExtra>}
      >
        {warehouseError ? <Alert type="warning" showIcon message={warehouseError} /> : null}
        <Tabs
          destroyOnHidden={false}
          items={[
            {
              key: 'candidates',
              label: '待登记费用',
              children: <>
                {candidateError ? <ErrorAlert title={candidateError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => candidateActionRef.current?.reload()}>重新加载</Button>} /> : null}
                <TmProTable<WarehouseFeeCandidate>
                  rowKey="orderId"
                  actionRef={candidateActionRef}
                  columns={candidateColumns}
                  search={false}
                  cardBordered
                  scroll={{ x: 1420 }}
                  toolBarRender={() => []}
                  filterBar={<Space wrap className="tm-warehouse-fee-toolbar">
                    <Input.Search allowClear aria-label="待登记订单号" placeholder="搜索订单号" value={candidateOrderNo} onChange={(event) => setCandidateOrderNo(event.target.value)} onSearch={() => candidateActionRef.current?.reload()} />
                    <Select allowClear showSearch optionFilterProp="label" aria-label="待登记仓库" placeholder="全部仓库" value={candidateWarehouseID} options={warehouseOptions} onChange={(value) => { setCandidateWarehouseID(value); void candidateActionRef.current?.reload(); }} />
                  </Space>}
                  locale={{ emptyText: candidateError ? '待登记费用暂不可用' : <EmptyState compact title="暂无可登记的履约订单" /> }}
                  request={async (params) => {
                    try {
                      const result = await queryWarehouseFeeCandidates({ page: params.current, pageSize: params.pageSize, orderNo: candidateOrderNo.trim() || undefined, warehouseId: candidateWarehouseID });
                      setCandidateError('');
                      return { data: result.list || [], total: result.total, success: true };
                    } catch (error) {
                      setCandidateError((error as Error)?.message || '待登记费用加载失败');
                      return { data: [], total: 0, success: false };
                    }
                  }}
                />
              </>,
            },
            {
              key: 'ledger',
              label: '费用台账',
              children: <>
                {ledgerError ? <ErrorAlert title={ledgerError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => ledgerActionRef.current?.reload()}>重新加载</Button>} /> : null}
                <TmProTable<WarehouseFeeSnapshot>
                  rowKey="id"
                  actionRef={ledgerActionRef}
                  columns={ledgerColumns}
                  search={false}
                  cardBordered
                  scroll={{ x: 1260 }}
                  toolBarRender={() => []}
                  filterBar={<Space wrap className="tm-warehouse-fee-toolbar">
                    <Input.Search allowClear aria-label="台账订单号" placeholder="搜索订单号" value={ledgerOrderNo} onChange={(event) => setLedgerOrderNo(event.target.value)} onSearch={() => ledgerActionRef.current?.reload()} />
                    <Select allowClear showSearch optionFilterProp="label" aria-label="台账仓库" placeholder="全部仓库" value={ledgerWarehouseID} options={warehouseOptions} onChange={(value) => { setLedgerWarehouseID(value); void ledgerActionRef.current?.reload(); }} />
                  </Space>}
                  locale={{ emptyText: ledgerError ? '仓库操作费台账暂不可用' : <EmptyState compact title="暂无已确认费用" /> }}
                  request={async (params) => {
                    try {
                      const result = await queryWarehouseFeeSnapshots({ page: params.current, pageSize: params.pageSize, orderNo: ledgerOrderNo.trim() || undefined, warehouseId: ledgerWarehouseID });
                      setLedgerError('');
                      return { data: result.list || [], total: result.total, success: true };
                    } catch (error) {
                      setLedgerError((error as Error)?.message || '仓库操作费台账加载失败');
                      return { data: [], total: 0, success: false };
                    }
                  }}
                />
              </>,
            },
            {
              key: 'rates',
              label: '费率卡',
              children: <>
                {rateError ? <ErrorAlert title={rateError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => rateActionRef.current?.reload()}>重新加载</Button>} /> : null}
                <TmProTable<WarehouseFeeRateCard>
                  rowKey="id"
                  actionRef={rateActionRef}
                  columns={rateColumns}
                  search={false}
                  cardBordered
                  scroll={{ x: 1320 }}
                  toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={() => openRateModal()}>新增费率卡</Button>]}
                  locale={{ emptyText: rateError ? '费率卡暂不可用' : <EmptyState compact title="暂无费率卡" /> }}
                  request={async () => {
                    try {
                      const result = await listWarehouseFeeRateCards({ includeInactive: true });
                      setRateError('');
                      return { data: result.list || [], total: result.list?.length || 0, success: true };
                    } catch (error) {
                      setRateError((error as Error)?.message || '费率卡加载失败');
                      return { data: [], total: 0, success: false };
                    }
                  }}
                  pagination={false}
                />
              </>,
            },
          ]}
        />
      </TmPageContainer>

      <Modal
        title="仓库操作费预览"
        open={previewOpen}
        width={760}
        destroyOnHidden
        maskClosable={!confirming}
        closable={!confirming}
        onCancel={() => { if (!confirming) { setPreviewOpen(false); resetPreview(); } }}
        footer={<OperationToolbar className="tm-warehouse-fee-modal-actions">
          <Button disabled={confirming} onClick={() => { setPreviewOpen(false); resetPreview(); }}>取消</Button>
          {preview ? <Button disabled={confirming} onClick={() => { confirmationRequestKey.current = undefined; setPreview(undefined); }}>更换费率</Button> : null}
          {!preview ? <Button type="primary" icon={<CalculatorOutlined />} disabled={!selectedRateID || candidateRates.length === 0} loading={previewing} onClick={() => void runPreview()}>生成预览</Button> : null}
          {preview ? <Button type="primary" disabled={!canManage || confirming} loading={confirming} onClick={() => void confirmPreview()}>确认登记</Button> : null}
        </OperationToolbar>}
      >
        {candidate ? <Space direction="vertical" size="middle" className="tm-warehouse-fee-modal-body">
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
            <Descriptions.Item label="订单号">{candidate.orderNo}</Descriptions.Item>
            <Descriptions.Item label="仓库">{warehouseLabel(candidate)}</Descriptions.Item>
            <Descriptions.Item label="履约波次">{candidate.waveNo} · R{candidate.waveRevision}</Descriptions.Item>
            <Descriptions.Item label="包裹编码">{candidate.packageCode}</Descriptions.Item>
            <Descriptions.Item label="商品数量">{candidate.itemQuantity}</Descriptions.Item>
            <Descriptions.Item label="包裹数量">{candidate.packageQuantity}</Descriptions.Item>
          </Descriptions>
          {!preview ? <Select showSearch optionFilterProp="label" aria-label="适用费率卡" placeholder={previewing ? '正在加载费率卡' : '选择适用费率卡'} loading={previewing} value={selectedRateID} options={candidateRates.map((row) => ({ value: row.id, label: rateLabel(row) }))} onChange={setSelectedRateID} /> : <>
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
              <Descriptions.Item label="费率卡">{preview.rateCardCode} · R{preview.rateCardRevision}</Descriptions.Item>
              <Descriptions.Item label="币种">{preview.currency}</Descriptions.Item>
              <Descriptions.Item label="出库基础费">{formatProfitAmount(preview.outboundBaseFeeMinor, preview.currency)}</Descriptions.Item>
              <Descriptions.Item label="拣货费">{formatProfitAmount(preview.pickingFeeMinor, preview.currency)}</Descriptions.Item>
              <Descriptions.Item label="打包费">{formatProfitAmount(preview.packingFeeMinor, preview.currency)}</Descriptions.Item>
              <Descriptions.Item label="费用合计"><Typography.Text strong>{formatProfitAmount(preview.amountMinor, preview.currency)}</Typography.Text></Descriptions.Item>
            </Descriptions>
            <Typography.Text type="secondary" copyable={{ text: preview.calculationHash }}>计算哈希：{preview.calculationHash.slice(0, 16)}...</Typography.Text>
          </>}
          {candidateRateError ? <Alert type="error" showIcon message="适用费率卡加载失败" description={candidateRateError} /> : null}
          {!previewing && !candidateRateError && candidateRates.length === 0 ? <Alert type="warning" showIcon message="当前仓库没有启用的费率卡" /> : null}
        </Space> : null}
      </Modal>

      <Modal
        title={editingRate ? `修订费率卡 ${editingRate.code}` : '新增费率卡'}
        open={rateModalOpen}
        destroyOnHidden
        maskClosable={!rateSubmitting}
        closable={!rateSubmitting}
        onCancel={() => { if (!rateSubmitting) { setRateModalOpen(false); setEditingRate(undefined); } }}
        onOk={() => void saveRate()}
        okText={editingRate ? '生成新修订' : '创建'}
        okButtonProps={{ disabled: !canManage }}
        confirmLoading={rateSubmitting}
      >
        <Form form={rateForm} layout="vertical" preserve={false}>
          <div className="tm-warehouse-fee-form-grid">
            <Form.Item name="warehouseId" label="适用仓库" rules={[{ required: true, message: '请选择仓库' }]}>
              <Select disabled={Boolean(editingRate)} showSearch optionFilterProp="label" options={warehouseOptions.filter((_, index) => warehouses[index]?.status === 'active')} />
            </Form.Item>
            <Form.Item name="code" label="费率编码" rules={[{ required: true, pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, message: '请输入字母、数字、下划线或中划线' }]}>
              <Input disabled={Boolean(editingRate)} maxLength={64} />
            </Form.Item>
            <Form.Item name="name" label="费率名称" rules={[{ required: true, whitespace: true, message: '请输入费率名称' }]}>
              <Input maxLength={160} />
            </Form.Item>
            <Form.Item name="currency" label="币种" rules={[{ required: true, message: '请选择币种' }]}>
              <Select options={CURRENCY_OPTIONS} />
            </Form.Item>
            <Form.Item name="outboundBaseFeeMinor" label="每单出库基础费（最小货币单位）" rules={[{ required: true, type: 'integer', min: 0 }]}>
              <InputNumber min={0} max={Number.MAX_SAFE_INTEGER} precision={0} />
            </Form.Item>
            <Form.Item name="pickingFeePerItemMinor" label="每件拣货费（最小货币单位）" rules={[{ required: true, type: 'integer', min: 0 }]}>
              <InputNumber min={0} max={Number.MAX_SAFE_INTEGER} precision={0} />
            </Form.Item>
            <Form.Item name="packingFeePerPackageMinor" label="每包裹打包费（最小货币单位）" rules={[{ required: true, type: 'integer', min: 0 }]}>
              <InputNumber min={0} max={Number.MAX_SAFE_INTEGER} precision={0} />
            </Form.Item>
            {editingRate ? <Form.Item name="status" label="状态" rules={[{ required: true }]}><Select options={[{ value: 'active', label: '启用' }, { value: 'inactive', label: '停用' }]} /></Form.Item> : null}
          </div>
        </Form>
      </Modal>

      <Modal title={historyDetail ? `${historyDetail.code} 修订历史` : '费率修订历史'} open={historyLoading || Boolean(historyDetail)} width={900} footer={null} onCancel={() => { if (!historyLoading) { historyRequest.current += 1; setHistoryDetail(undefined); } }}>
        <Table rowKey="id" loading={historyLoading} columns={historyColumns} dataSource={historyDetail?.revisions || []} pagination={false} scroll={{ x: 820 }} />
      </Modal>

      <AppDrawer title="仓库操作费详情" open={drawer.open} onClose={drawer.closeDrawer} loading={detailLoading} width={940}>
        {detailError ? <ErrorAlert title={detailError} actionHint={drawer.id ? <Button icon={<ReloadOutlined />} onClick={() => void loadDetail(drawer.id!)}>重新加载</Button> : undefined} /> : null}
        {detail ? <Space direction="vertical" size="large" className="tm-warehouse-fee-detail">
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
            <Descriptions.Item label="订单号">{detail.orderNo}</Descriptions.Item>
            <Descriptions.Item label="当前费用"><Typography.Text strong>{formatProfitAmount(detail.netAmountMinor, detail.currency)}</Typography.Text></Descriptions.Item>
            <Descriptions.Item label="仓库">{warehouseLabel(detail)}</Descriptions.Item>
            <Descriptions.Item label="履约波次">{detail.waveNo} · R{detail.waveRevision}</Descriptions.Item>
            <Descriptions.Item label="费率卡">{detail.rateCardCode} · R{detail.rateCardRevision}</Descriptions.Item>
            <Descriptions.Item label="确认时间">{formatDateTime(detail.confirmedAt)}</Descriptions.Item>
            <Descriptions.Item label="出库基础费">{formatProfitAmount(detail.outboundBaseFeeMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label="拣货费">{formatProfitAmount(detail.pickingFeeMinor, detail.currency)}（{detail.itemQuantity} 件）</Descriptions.Item>
            <Descriptions.Item label="打包费">{formatProfitAmount(detail.packingFeeMinor, detail.currency)}（{detail.packageQuantity} 包裹）</Descriptions.Item>
            <Descriptions.Item label="调整合计">{formatProfitAmount(detail.adjustmentMinor, detail.currency)}</Descriptions.Item>
          </Descriptions>
          <OperationToolbar>
            <Button icon={<PlusOutlined />} disabled={!canManage} onClick={() => openAdjustment()}>追加调整</Button>
            <Button icon={<EyeOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(detail.orderId)}`)}>查看订单</Button>
            <Button icon={<CalculatorOutlined />} onClick={() => history.push(`/finance/order-profits?drawer=order-profit&id=${encodeURIComponent(detail.orderId)}`)}>查看预估利润</Button>
          </OperationToolbar>
          <Table rowKey="id" columns={adjustmentColumns} dataSource={detail.adjustments || []} locale={{ emptyText: '暂无费用调整' }} pagination={false} scroll={{ x: 760 }} />
        </Space> : !detailLoading && !detailError ? <EmptyState compact title="暂无费用详情" /> : null}
      </AppDrawer>

      <Modal
        title={reversingAdjustment ? '冲正费用调整' : '追加费用调整'}
        open={adjustmentOpen}
        destroyOnHidden
        maskClosable={!adjusting}
        closable={!adjusting}
        onCancel={() => { if (!adjusting) { adjustmentRequestKey.current = undefined; setAdjustmentOpen(false); setReversingAdjustment(undefined); } }}
        onOk={() => void saveAdjustment()}
        okText={reversingAdjustment ? '确认冲正' : '确认追加'}
        okButtonProps={{ disabled: !canManage }}
        confirmLoading={adjusting}
      >
        {reversingAdjustment ? <Alert type="warning" showIcon message={`将追加 ${formatProfitAmount(-reversingAdjustment.amountMinor, reversingAdjustment.currency)} 的冲正事实`} /> : null}
        <Form form={adjustmentForm} layout="vertical" preserve={false} className="tm-warehouse-fee-adjustment-form">
          {!reversingAdjustment ? <Form.Item name="amountMinor" label="调整金额（最小货币单位，可为负数）" rules={[{ required: true, type: 'integer' }, { validator: (_, value) => value === 0 ? Promise.reject(new Error('调整金额不能为 0')) : Promise.resolve() }]}><InputNumber min={-Number.MAX_SAFE_INTEGER} max={Number.MAX_SAFE_INTEGER} precision={0} /></Form.Item> : null}
          <Form.Item name="reason" label="原因" rules={[{ required: true, whitespace: true, message: '请输入原因' }]}><Input.TextArea maxLength={500} showCount rows={3} /></Form.Item>
        </Form>
      </Modal>
    </PermissionGuard>
  );
}
