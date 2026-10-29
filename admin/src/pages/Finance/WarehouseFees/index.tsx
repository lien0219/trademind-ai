import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
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
import { financialActionRequestKey, type FinancialActionRequestKey } from '@/utils/financialActionKey';
import { createTenantReadGuard } from '@/utils/tenantAsync';
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



export default function WarehouseFeesPage() {
  useLanguage();
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
  const confirmationRequestKey = useRef<FinancialActionRequestKey>();
  const adjustmentRequestKey = useRef<FinancialActionRequestKey>();
  const detailRequest = useRef(0);
  const historyRequest = useRef(0);
  const warehouseRead = useMemo(createTenantReadGuard, []);
  const detailRead = useMemo(createTenantReadGuard, []);
  const historyRead = useMemo(createTenantReadGuard, []);
  const previewRead = useMemo(createTenantReadGuard, []);
  const candidatesRead = useMemo(createTenantReadGuard, []);
  const ledgerRead = useMemo(createTenantReadGuard, []);
  const ratesRead = useMemo(createTenantReadGuard, []);
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
    const isCurrent = warehouseRead.begin();
    setWarehouses([]);
    try {
      const result = await listInventoryWarehouses();
      if (!isCurrent()) return;
      setWarehouses(result.list || []);
      setWarehouseError('');
    } catch (error) {
      if (isCurrent()) setWarehouseError((error as Error)?.message || tr('仓库筛选项加载失败'));
    }
  }, [warehouseRead]);

  useEffect(() => {
    void loadWarehouses();
    return () => warehouseRead.invalidate();
  }, [loadWarehouses, warehouseRead]);

  const loadDetail = useCallback(async (id: string) => {
    const requestID = ++detailRequest.current;
    const isCurrent = detailRead.begin();
    setDetail(undefined);
    setDetailLoading(true);
    setDetailError('');
    try {
      const result = await getWarehouseFeeSnapshot(id);
      if (detailRequest.current === requestID && isCurrent()) setDetail(result);
    } catch (error) {
      if (detailRequest.current === requestID && isCurrent()) {
        setDetail(undefined);
        setDetailError((error as Error)?.message || tr('仓库操作费详情加载失败'));
      }
    } finally {
      if (detailRequest.current === requestID && isCurrent()) setDetailLoading(false);
    }
  }, [detailRead]);

  useEffect(() => {
    if (drawer.id) void loadDetail(drawer.id);
    else {
      detailRequest.current += 1;
      detailRead.invalidate();
      setDetail(undefined);
      setDetailError('');
      setDetailLoading(false);
    }
    return () => { detailRequest.current += 1; detailRead.invalidate(); };
  }, [detailRead, drawer.id, loadDetail]);

  const resetPreview = useCallback(() => {
    previewSession.current += 1;
    previewRead.invalidate();
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
  }, [previewRead]);

  const openPreview = useCallback(async (row: WarehouseFeeCandidate) => {
    if (!canManage || row.confirmed || !row.packVerificationId) return;
    const sessionID = ++previewSession.current;
    const isCurrent = previewRead.begin();
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
      if ((previewSession.current !== sessionID || !isCurrent())) return;
      const rates = (result.list || []).filter((rate) => rate.status === 'active');
      setCandidateRates(rates);
      setCandidateRateError('');
      if (rates.length === 1) setSelectedRateID(rates[0].id);
    } catch (error) {
      if ((previewSession.current !== sessionID || !isCurrent())) return;
      const text = (error as Error)?.message || tr('适用费率卡加载失败');
      setCandidateRateError(text);
      message.error(text);
    } finally {
      if ((previewSession.current === sessionID && isCurrent())) setPreviewing(false);
    }
  }, [canManage, previewRead]);

  const runPreview = useCallback(async () => {
    if (!candidate || !selectedRateID || previewing || previewRequestLock.current) return;
    const rate = candidateRates.find((row) => row.id === selectedRateID);
    if (!rate) return;
    const sessionID = previewSession.current;
    const isCurrent = previewRead.begin();
    previewRequestLock.current = true;
    setPreviewing(true);
    try {
      const result = await previewWarehouseFee({
        orderId: candidate.orderId,
        rateCardId: rate.id,
        rateCardRevision: rate.revision,
      });
      if ((previewSession.current !== sessionID || !isCurrent())) return;
      confirmationRequestKey.current = financialActionRequestKey(
        undefined,
        'confirm',
        [result.orderId, result.rateCardId, result.rateCardRevision, result.waveRevision, result.calculationHash],
        warehouseFeeIdempotencyKey,
      );
      setPreview(result);
    } catch (error) {
      if ((previewSession.current !== sessionID || !isCurrent())) return;
      setPreview(undefined);
      message.error((error as Error)?.message || tr('仓库操作费预览失败'));
    } finally {
      if ((previewSession.current === sessionID && isCurrent())) {
        previewRequestLock.current = false;
        setPreviewing(false);
      }
    }
  }, [candidate, candidateRates, previewRead, previewing, selectedRateID]);

  const confirmPreview = useCallback(async () => {
    if (!preview || confirming || confirmationLock.current) return;
    confirmationLock.current = true;
    setConfirming(true);
    try {
      const requestKey = financialActionRequestKey(
        confirmationRequestKey.current,
        'confirm',
        [preview.orderId, preview.rateCardId, preview.rateCardRevision, preview.waveRevision, preview.calculationHash],
        warehouseFeeIdempotencyKey,
      );
      confirmationRequestKey.current = requestKey;
      const result = await confirmWarehouseFee({
        orderId: preview.orderId,
        rateCardId: preview.rateCardId,
        rateCardRevision: preview.rateCardRevision,
        waveRevision: preview.waveRevision,
        calculationHash: preview.calculationHash,
        idempotencyKey: requestKey.key,
      });
      message.success(result.replayed ? tr('该订单费用已确认，无需重复登记') : tr('仓库操作费已确认'));
      setPreviewOpen(false);
      resetPreview();
      void candidateActionRef.current?.reload();
      void ledgerActionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || tr('仓库操作费确认失败'));
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
        message.success(tr('费率卡已生成新修订'));
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
        message.success(tr('费率卡已创建'));
      }
      setRateModalOpen(false);
      setEditingRate(undefined);
      void rateActionRef.current?.reload();
    } catch (error) {
      const text = (error as Error)?.message || tr('费率卡保存失败');
      setRateError(text);
      message.error(text);
    } finally {
      rateSubmissionLock.current = false;
      setRateSubmitting(false);
    }
  }, [canManage, editingRate, rateForm, rateSubmitting]);

  const showRateHistory = useCallback(async (row: WarehouseFeeRateCard) => {
    const requestID = ++historyRequest.current;
    const isCurrent = historyRead.begin();
    setHistoryLoading(true);
    setHistoryDetail(undefined);
    try {
      const result = await getWarehouseFeeRateCard(row.id);
      if (historyRequest.current === requestID && isCurrent()) setHistoryDetail(result);
    } catch (error) {
      if (historyRequest.current === requestID && isCurrent()) message.error((error as Error)?.message || tr('费率修订历史加载失败'));
    } finally {
      if (historyRequest.current === requestID && isCurrent()) setHistoryLoading(false);
    }
  }, [historyRead]);

  useEffect(() => () => {
    historyRead.invalidate();
    previewRead.invalidate();
    candidatesRead.invalidate();
    ledgerRead.invalidate();
    ratesRead.invalidate();
  }, [historyRead, previewRead, candidatesRead, ledgerRead, ratesRead]);

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
        const requestKey = financialActionRequestKey(
          adjustmentRequestKey.current,
          'reverse',
          [detail.id, reversingAdjustment.id, null, reason],
          warehouseFeeIdempotencyKey,
        );
        adjustmentRequestKey.current = requestKey;
        await reverseWarehouseFeeAdjustment(detail.id, reversingAdjustment.id, {
          reason,
          idempotencyKey: requestKey.key,
        });
        message.success(tr('调整事实已冲正'));
      } else {
        if (typeof values.amountMinor !== 'number') return;
        const reason = values.reason.trim();
        const requestKey = financialActionRequestKey(
          adjustmentRequestKey.current,
          'adjust',
          [detail.id, values.amountMinor, null, reason],
          warehouseFeeIdempotencyKey,
        );
        adjustmentRequestKey.current = requestKey;
        await createWarehouseFeeAdjustment(detail.id, {
          amountMinor: values.amountMinor,
          reason,
          idempotencyKey: requestKey.key,
        });
        message.success(tr('费用调整已追加'));
      }
      setAdjustmentOpen(false);
      setReversingAdjustment(undefined);
      adjustmentRequestKey.current = undefined;
      await loadDetail(detail.id);
      void ledgerActionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || tr('费用调整失败'));
    } finally {
      adjustmentSubmissionLock.current = false;
      setAdjusting(false);
    }
  }, [adjusting, adjustmentForm, canManage, detail, loadDetail, reversingAdjustment]);

  const candidateColumns: ProColumns<WarehouseFeeCandidate>[] = [
    { title: tr('订单号'), dataIndex: 'orderNo', fixed: 'left', width: 190, copyable: true, ellipsis: true },
    { title: tr('店铺'), dataIndex: 'shopName', width: 160, ellipsis: true, render: (value) => value || tr('本地订单') },
    { title: tr('仓库'), width: 190, ellipsis: true, render: (_, row) => warehouseLabel(row) },
    { title: tr('履约波次'), dataIndex: 'waveNo', width: 160, ellipsis: true },
    { title: tr('商品数量'), dataIndex: 'itemQuantity', width: 100, align: 'right' },
    { title: tr('包裹数量'), dataIndex: 'packageQuantity', width: 100, align: 'right' },
    { title: tr('包裹编码'), dataIndex: 'packageCode', width: 180, ellipsis: true, render: (value) => value || tr('未形成复核事实') },
    { title: tr('费用状态'), width: 110, render: (_, row) => row.confirmed ? <Tag color="success">{tr('已确认')} </Tag> : row.packVerificationId ? <Tag color="processing">{tr('待登记')} </Tag> : <Tag color="error">{tr('已阻断')} </Tag> },
    { title: tr('履约时间'), dataIndex: 'fulfilledAt', width: 180, render: (value) => formatDateTime(value as string) },
    {
      title: tr('操作'), valueType: 'option', width: 118, fixed: 'right',
      render: (_, row) => row.confirmed && row.snapshotId
        ? <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.snapshotId!)}>{tr('查看')} </Button>
        : <Tooltip title={!canManage ? tr('当前账号无费用登记权限') : !row.packVerificationId ? tr('缺少打包扫描复核事实') : undefined}>
          <Button type="link" size="small" icon={<CalculatorOutlined />} disabled={!canManage || !row.packVerificationId} onClick={() => void openPreview(row)}>{tr('预览')} </Button>
        </Tooltip>,
    },
  ];

  const ledgerColumns: ProColumns<WarehouseFeeSnapshot>[] = [
    { title: tr('订单号'), dataIndex: 'orderNo', fixed: 'left', width: 190, copyable: true, ellipsis: true },
    { title: tr('仓库'), width: 190, ellipsis: true, render: (_, row) => warehouseLabel(row) },
    { title: tr('费率卡'), width: 180, ellipsis: true, render: (_, row) => `${row.rateCardCode} · R${row.rateCardRevision}` },
    { title: tr('基础费用'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: tr('调整合计'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.adjustmentMinor, row.currency) },
    { title: tr('当前费用'), width: 130, align: 'right', render: (_, row) => <Typography.Text strong>{formatProfitAmount(row.netAmountMinor, row.currency)}</Typography.Text> },
    { title: tr('调整数'), dataIndex: 'adjustmentCount', width: 86, align: 'right' },
    { title: tr('确认时间'), dataIndex: 'confirmedAt', width: 180, render: (value) => formatDateTime(value as string) },
    { title: tr('操作'), valueType: 'option', fixed: 'right', width: 90, render: (_, row) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.id)}>{tr('查看')} </Button> },
  ];

  const rateColumns: ProColumns<WarehouseFeeRateCard>[] = [
    { title: tr('费率编码'), dataIndex: 'code', fixed: 'left', width: 150, copyable: true },
    { title: tr('名称'), dataIndex: 'name', width: 180, ellipsis: true },
    { title: tr('适用仓库'), width: 190, ellipsis: true, render: (_, row) => warehouseLabel(row) },
    { title: tr('出库基础费'), width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.outboundBaseFeeMinor, row.currency) },
    { title: tr('拣货单价 / 件'), width: 140, align: 'right', render: (_, row) => formatProfitAmount(row.pickingFeePerItemMinor, row.currency) },
    { title: tr('打包单价 / 包裹'), width: 150, align: 'right', render: (_, row) => formatProfitAmount(row.packingFeePerPackageMinor, row.currency) },
    { title: tr('修订'), dataIndex: 'revision', width: 80, render: (value) => `R${value}` },
    { title: tr('状态'), dataIndex: 'status', width: 90, render: (value) => value === 'active' ? <Tag color="success">{tr('启用')} </Tag> : <Tag>{tr('停用')} </Tag> },
    {
      title: tr('操作'), valueType: 'option', fixed: 'right', width: 165,
      render: (_, row) => <Space size={0}>
        <Button type="link" size="small" icon={<HistoryOutlined />} onClick={() => void showRateHistory(row)}>{tr('历史')} </Button>
        <Button type="link" size="small" disabled={!canManage} onClick={() => openRateModal(row)}>{tr('修订')} </Button>
      </Space>,
    },
  ];

  const adjustmentColumns: TableColumnsType<WarehouseFeeAdjustment> = [
    { title: tr('类型'), dataIndex: 'factType', width: 90, render: (value) => value === 'reversal' ? <Tag>{tr('冲正')} </Tag> : <Tag color="processing">{tr('调整')} </Tag> },
    { title: tr('金额'), width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: tr('原因'), dataIndex: 'reason', ellipsis: true },
    { title: tr('时间'), dataIndex: 'createdAt', width: 180, render: (value) => formatDateTime(value as string) },
    {
      title: tr('操作'), width: 90,
      render: (_, row) => {
        const reversed = detail?.adjustments.some((candidateRow) => candidateRow.reversesAdjustmentId === row.id);
        return row.factType === 'adjustment' ? <Button type="link" size="small" icon={<RollbackOutlined />} disabled={!canManage || reversed} onClick={() => openAdjustment(row)}>{reversed ? tr('已冲正') : tr('冲正')}</Button> : null;
      },
    },
  ];

  const historyColumns: TableColumnsType<WarehouseFeeRateRevision> = [
    { title: tr('修订'), dataIndex: 'revision', width: 75, render: (value) => `R${value}` },
    { title: tr('名称'), dataIndex: 'name', ellipsis: true },
    { title: tr('出库基础费'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.outboundBaseFeeMinor, row.currency) },
    { title: tr('拣货 / 件'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.pickingFeePerItemMinor, row.currency) },
    { title: tr('打包 / 包裹'), width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.packingFeePerPackageMinor, row.currency) },
    { title: tr('状态'), dataIndex: 'status', width: 80, render: (value) => value === 'active' ? tr('启用') : tr('停用') },
    { title: tr('生成时间'), dataIndex: 'createdAt', width: 180, render: (value) => formatDateTime(value as string) },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.WAREHOUSE_FEE_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-warehouse-fee-page"
        title={tr("仓库操作费台账")}
        subTitle={tr("按已完成履约事实确认出库、拣货和打包费用")}
        extra={<TmPageHeaderExtra><Button icon={<ReloadOutlined />} onClick={() => { void loadWarehouses(); void candidateActionRef.current?.reload(); void ledgerActionRef.current?.reload(); void rateActionRef.current?.reload(); }}>{tr('刷新')} </Button></TmPageHeaderExtra>}
      >
        {warehouseError ? <Alert type="warning" showIcon message={warehouseError} /> : null}
        <Tabs
          destroyOnHidden={false}
          items={[
            {
              key: 'candidates',
              label: tr('待登记费用'),
              children: <>
                {candidateError ? <ErrorAlert title={candidateError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => candidateActionRef.current?.reload()}>{tr('重新加载')} </Button>} /> : null}
                <TmProTable<WarehouseFeeCandidate>
                  rowKey="orderId"
                  actionRef={candidateActionRef}
                  columns={candidateColumns}
                  search={false}
                  cardBordered
                  scroll={{ x: 1420 }}
                  toolBarRender={() => []}
                  filterBar={<Space wrap className="tm-warehouse-fee-toolbar">
                    <Input.Search allowClear aria-label={tr("待登记订单号")} placeholder={tr("搜索订单号")} value={candidateOrderNo} onChange={(event) => setCandidateOrderNo(event.target.value)} onSearch={() => candidateActionRef.current?.reload()} />
                    <Select allowClear showSearch optionFilterProp="label" aria-label={tr("待登记仓库")} placeholder={tr("全部仓库")} value={candidateWarehouseID} options={warehouseOptions} onChange={(value) => { setCandidateWarehouseID(value); void candidateActionRef.current?.reload(); }} />
                  </Space>}
                  locale={{ emptyText: candidateError ? tr('待登记费用暂不可用') : <EmptyState compact title={tr("暂无可登记的履约订单")} /> }}
                  request={async (params) => {
                    const isCurrent = candidatesRead.begin();
                    try {
                      const result = await queryWarehouseFeeCandidates({ page: params.current, pageSize: params.pageSize, orderNo: candidateOrderNo.trim() || undefined, warehouseId: candidateWarehouseID });
                      if (!isCurrent()) return { data: [], total: 0, success: false };
                      setCandidateError('');
                      return { data: result.list || [], total: result.total, success: true };
                    } catch (error) {
                      if (isCurrent()) setCandidateError((error as Error)?.message || tr('待登记费用加载失败'));
                      return { data: [], total: 0, success: false };
                    }
                  }}
                />
              </>,
            },
            {
              key: 'ledger',
              label: tr('费用台账'),
              children: <>
                {ledgerError ? <ErrorAlert title={ledgerError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => ledgerActionRef.current?.reload()}>{tr('重新加载')} </Button>} /> : null}
                <TmProTable<WarehouseFeeSnapshot>
                  rowKey="id"
                  actionRef={ledgerActionRef}
                  columns={ledgerColumns}
                  search={false}
                  cardBordered
                  scroll={{ x: 1260 }}
                  toolBarRender={() => []}
                  filterBar={<Space wrap className="tm-warehouse-fee-toolbar">
                    <Input.Search allowClear aria-label={tr("台账订单号")} placeholder={tr("搜索订单号")} value={ledgerOrderNo} onChange={(event) => setLedgerOrderNo(event.target.value)} onSearch={() => ledgerActionRef.current?.reload()} />
                    <Select allowClear showSearch optionFilterProp="label" aria-label={tr("台账仓库")} placeholder={tr("全部仓库")} value={ledgerWarehouseID} options={warehouseOptions} onChange={(value) => { setLedgerWarehouseID(value); void ledgerActionRef.current?.reload(); }} />
                  </Space>}
                  locale={{ emptyText: ledgerError ? tr('仓库操作费台账暂不可用') : <EmptyState compact title={tr("暂无已确认费用")} /> }}
                  request={async (params) => {
                    const isCurrent = ledgerRead.begin();
                    try {
                      const result = await queryWarehouseFeeSnapshots({ page: params.current, pageSize: params.pageSize, orderNo: ledgerOrderNo.trim() || undefined, warehouseId: ledgerWarehouseID });
                      if (!isCurrent()) return { data: [], total: 0, success: false };
                      setLedgerError('');
                      return { data: result.list || [], total: result.total, success: true };
                    } catch (error) {
                      if (isCurrent()) setLedgerError((error as Error)?.message || tr('仓库操作费台账加载失败'));
                      return { data: [], total: 0, success: false };
                    }
                  }}
                />
              </>,
            },
            {
              key: 'rates',
              label: tr('费率卡'),
              children: <>
                {rateError ? <ErrorAlert title={rateError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => rateActionRef.current?.reload()}>{tr('重新加载')} </Button>} /> : null}
                <TmProTable<WarehouseFeeRateCard>
                  rowKey="id"
                  actionRef={rateActionRef}
                  columns={rateColumns}
                  search={false}
                  cardBordered
                  scroll={{ x: 1320 }}
                  toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={() => openRateModal()}>{tr('新增费率卡')} </Button>]}
                  locale={{ emptyText: rateError ? tr('费率卡暂不可用') : <EmptyState compact title={tr("暂无费率卡")} /> }}
                  request={async () => {
                    const isCurrent = ratesRead.begin();
                    try {
                      const result = await listWarehouseFeeRateCards({ includeInactive: true });
                      if (!isCurrent()) return { data: [], total: 0, success: false };
                      setRateError('');
                      return { data: result.list || [], total: result.list?.length || 0, success: true };
                    } catch (error) {
                      if (isCurrent()) setRateError((error as Error)?.message || tr('费率卡加载失败'));
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
        title={tr("仓库操作费预览")}
        open={previewOpen}
        width={760}
        destroyOnHidden
        maskClosable={!confirming}
        closable={!confirming}
        onCancel={() => { if (!confirming) { setPreviewOpen(false); resetPreview(); } }}
        footer={<OperationToolbar className="tm-warehouse-fee-modal-actions">
          <Button disabled={confirming} onClick={() => { setPreviewOpen(false); resetPreview(); }}>{tr('取消')} </Button>
          {preview ? <Button disabled={confirming} onClick={() => { confirmationRequestKey.current = undefined; setPreview(undefined); }}>{tr('更换费率')} </Button> : null}
          {!preview ? <Button type="primary" icon={<CalculatorOutlined />} disabled={!selectedRateID || candidateRates.length === 0} loading={previewing} onClick={() => void runPreview()}>{tr('生成预览')} </Button> : null}
          {preview ? <Button type="primary" disabled={!canManage || confirming} loading={confirming} onClick={() => void confirmPreview()}>{tr('确认登记')} </Button> : null}
        </OperationToolbar>}
      >
        {candidate ? <Space direction="vertical" size="middle" className="tm-warehouse-fee-modal-body">
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
            <Descriptions.Item label={tr("订单号")}>{candidate.orderNo}</Descriptions.Item>
            <Descriptions.Item label={tr("仓库")}>{warehouseLabel(candidate)}</Descriptions.Item>
            <Descriptions.Item label={tr("履约波次")}>{candidate.waveNo} · R{candidate.waveRevision}</Descriptions.Item>
            <Descriptions.Item label={tr("包裹编码")}>{candidate.packageCode}</Descriptions.Item>
            <Descriptions.Item label={tr("商品数量")}>{candidate.itemQuantity}</Descriptions.Item>
            <Descriptions.Item label={tr("包裹数量")}>{candidate.packageQuantity}</Descriptions.Item>
          </Descriptions>
          {!preview ? <Select showSearch optionFilterProp="label" aria-label={tr("适用费率卡")} placeholder={previewing ? tr('正在加载费率卡') : tr('选择适用费率卡')} loading={previewing} value={selectedRateID} options={candidateRates.map((row) => ({ value: row.id, label: rateLabel(row) }))} onChange={setSelectedRateID} /> : <>
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
              <Descriptions.Item label={tr("费率卡")}>{preview.rateCardCode} · R{preview.rateCardRevision}</Descriptions.Item>
              <Descriptions.Item label={tr("币种")}>{preview.currency}</Descriptions.Item>
              <Descriptions.Item label={tr("出库基础费")}>{formatProfitAmount(preview.outboundBaseFeeMinor, preview.currency)}</Descriptions.Item>
              <Descriptions.Item label={tr("拣货费")}>{formatProfitAmount(preview.pickingFeeMinor, preview.currency)}</Descriptions.Item>
              <Descriptions.Item label={tr("打包费")}>{formatProfitAmount(preview.packingFeeMinor, preview.currency)}</Descriptions.Item>
              <Descriptions.Item label={tr("费用合计")}><Typography.Text strong>{formatProfitAmount(preview.amountMinor, preview.currency)}</Typography.Text></Descriptions.Item>
            </Descriptions>
            <Typography.Text type="secondary" copyable={{ text: preview.calculationHash }}>{tr('计算哈希：')} {preview.calculationHash.slice(0, 16)}...</Typography.Text>
          </>}
          {candidateRateError ? <Alert type="error" showIcon message={tr("适用费率卡加载失败")} description={candidateRateError} /> : null}
          {!previewing && !candidateRateError && candidateRates.length === 0 ? <Alert type="warning" showIcon message={tr("当前仓库没有启用的费率卡")} /> : null}
        </Space> : null}
      </Modal>

      <Modal
        title={editingRate ? trFormat('修订费率卡 {code}', { code: editingRate.code }) : tr('新增费率卡')}
        open={rateModalOpen}
        destroyOnHidden
        maskClosable={!rateSubmitting}
        closable={!rateSubmitting}
        onCancel={() => { if (!rateSubmitting) { setRateModalOpen(false); setEditingRate(undefined); } }}
        onOk={() => void saveRate()}
        okText={editingRate ? tr('生成新修订') : tr('创建')}
        okButtonProps={{ disabled: !canManage }}
        confirmLoading={rateSubmitting}
      >
        <Form form={rateForm} layout="vertical" preserve={false}>
          <div className="tm-warehouse-fee-form-grid">
            <Form.Item name="warehouseId" label={tr("适用仓库")} rules={[{ required: true, message: tr('请选择仓库') }]}>
              <Select disabled={Boolean(editingRate)} showSearch optionFilterProp="label" options={warehouseOptions.filter((_, index) => warehouses[index]?.status === 'active')} />
            </Form.Item>
            <Form.Item name="code" label={tr("费率编码")} rules={[{ required: true, pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, message: tr('请输入字母、数字、下划线或中划线') }]}>
              <Input disabled={Boolean(editingRate)} maxLength={64} />
            </Form.Item>
            <Form.Item name="name" label={tr("费率名称")} rules={[{ required: true, whitespace: true, message: tr('请输入费率名称') }]}>
              <Input maxLength={160} />
            </Form.Item>
            <Form.Item name="currency" label={tr("币种")} rules={[{ required: true, message: tr('请选择币种') }]}>
              <Select options={CURRENCY_OPTIONS} />
            </Form.Item>
            <Form.Item name="outboundBaseFeeMinor" label={tr("每单出库基础费（最小货币单位）")} rules={[{ required: true, type: 'integer', min: 0 }]}>
              <InputNumber min={0} max={Number.MAX_SAFE_INTEGER} precision={0} />
            </Form.Item>
            <Form.Item name="pickingFeePerItemMinor" label={tr("每件拣货费（最小货币单位）")} rules={[{ required: true, type: 'integer', min: 0 }]}>
              <InputNumber min={0} max={Number.MAX_SAFE_INTEGER} precision={0} />
            </Form.Item>
            <Form.Item name="packingFeePerPackageMinor" label={tr("每包裹打包费（最小货币单位）")} rules={[{ required: true, type: 'integer', min: 0 }]}>
              <InputNumber min={0} max={Number.MAX_SAFE_INTEGER} precision={0} />
            </Form.Item>
            {editingRate ? <Form.Item name="status" label={tr("状态")} rules={[{ required: true }]}><Select options={[{ value: 'active', label: tr('启用') }, { value: 'inactive', label: tr('停用') }]} /></Form.Item> : null}
          </div>
        </Form>
      </Modal>

      <Modal title={historyDetail ? trFormat('费率卡 {code} 修订历史', { code: historyDetail.code }) : tr('费率修订历史')} open={historyLoading || Boolean(historyDetail)} width={900} footer={null} onCancel={() => { if (!historyLoading) { historyRequest.current += 1; setHistoryDetail(undefined); } }}>
        <Table rowKey="id" loading={historyLoading} columns={historyColumns} dataSource={historyDetail?.revisions || []} pagination={false} scroll={{ x: 820 }} />
      </Modal>

      <AppDrawer title={tr("仓库操作费详情")} open={drawer.open} onClose={drawer.closeDrawer} loading={detailLoading} width={940}>
        {detailError ? <ErrorAlert title={detailError} actionHint={drawer.id ? <Button icon={<ReloadOutlined />} onClick={() => void loadDetail(drawer.id!)}>{tr('重新加载')} </Button> : undefined} /> : null}
        {detail ? <Space direction="vertical" size="large" className="tm-warehouse-fee-detail">
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
            <Descriptions.Item label={tr("订单号")}>{detail.orderNo}</Descriptions.Item>
            <Descriptions.Item label={tr("当前费用")}><Typography.Text strong>{formatProfitAmount(detail.netAmountMinor, detail.currency)}</Typography.Text></Descriptions.Item>
            <Descriptions.Item label={tr("仓库")}>{warehouseLabel(detail)}</Descriptions.Item>
            <Descriptions.Item label={tr("履约波次")}>{detail.waveNo} · R{detail.waveRevision}</Descriptions.Item>
            <Descriptions.Item label={tr("费率卡")}>{detail.rateCardCode} · R{detail.rateCardRevision}</Descriptions.Item>
            <Descriptions.Item label={tr("确认时间")}>{formatDateTime(detail.confirmedAt)}</Descriptions.Item>
            <Descriptions.Item label={tr("出库基础费")}>{formatProfitAmount(detail.outboundBaseFeeMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label={tr("拣货费")}>{formatProfitAmount(detail.pickingFeeMinor, detail.currency)}（{detail.itemQuantity} {tr('件）')} </Descriptions.Item>
            <Descriptions.Item label={tr("打包费")}>{formatProfitAmount(detail.packingFeeMinor, detail.currency)}（{detail.packageQuantity} {tr('包裹）')} </Descriptions.Item>
            <Descriptions.Item label={tr("调整合计")}>{formatProfitAmount(detail.adjustmentMinor, detail.currency)}</Descriptions.Item>
          </Descriptions>
          <OperationToolbar>
            <Button icon={<PlusOutlined />} disabled={!canManage} onClick={() => openAdjustment()}>{tr('追加调整')} </Button>
            <Button icon={<EyeOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(detail.orderId)}`)}>{tr('查看订单')} </Button>
            <Button icon={<CalculatorOutlined />} onClick={() => history.push(`/finance/order-profits?drawer=order-profit&id=${encodeURIComponent(detail.orderId)}`)}>{tr('查看预估利润')} </Button>
          </OperationToolbar>
          <Table rowKey="id" columns={adjustmentColumns} dataSource={detail.adjustments || []} locale={{ emptyText: tr('暂无费用调整') }} pagination={false} scroll={{ x: 760 }} />
        </Space> : !detailLoading && !detailError ? <EmptyState compact title={tr("暂无费用详情")} /> : null}
      </AppDrawer>

      <Modal
        title={reversingAdjustment ? tr('冲正费用调整') : tr('追加费用调整')}
        open={adjustmentOpen}
        destroyOnHidden
        maskClosable={!adjusting}
        closable={!adjusting}
        onCancel={() => { if (!adjusting) { adjustmentRequestKey.current = undefined; setAdjustmentOpen(false); setReversingAdjustment(undefined); } }}
        onOk={() => void saveAdjustment()}
        okText={reversingAdjustment ? tr('确认冲正') : tr('确认追加')}
        okButtonProps={{ disabled: !canManage }}
        confirmLoading={adjusting}
      >
        {reversingAdjustment ? <Alert type="warning" showIcon message={trFormat('将追加 {amount} 的冲正事实', { amount: formatProfitAmount(-reversingAdjustment.amountMinor, reversingAdjustment.currency) })} /> : null}
        <Form form={adjustmentForm} layout="vertical" preserve={false} className="tm-warehouse-fee-adjustment-form">
          {!reversingAdjustment ? <Form.Item name="amountMinor" label={tr("调整金额（最小货币单位，可为负数）")} rules={[{ required: true, type: 'integer' }, { validator: (_, value) => value === 0 ? Promise.reject(new Error(tr('调整金额不能为 0'))) : Promise.resolve() }]}><InputNumber min={-Number.MAX_SAFE_INTEGER} max={Number.MAX_SAFE_INTEGER} precision={0} /></Form.Item> : null}
          <Form.Item name="reason" label={tr("原因")} rules={[{ required: true, whitespace: true, message: tr('请输入原因') }]}><Input.TextArea maxLength={500} showCount rows={3} /></Form.Item>
        </Form>
      </Modal>
    </PermissionGuard>
  );
}
