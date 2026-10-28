import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import {
  DownloadOutlined,
  EyeOutlined,
  ImportOutlined,
  PlusOutlined,
  ReloadOutlined,
  RollbackOutlined,
  UploadOutlined,
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
  Statistic,
  Table,
  Tag,
  Tooltip,
  Typography,
  Upload,
  message,
  type TableColumnsType,
  type UploadFile,
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
import { platformDisplayLabel } from '@/constants/platformLabels';
import { usePermission } from '@/hooks/usePermission';
import { useUrlDrawerState, useUrlQueryState } from '@/hooks/useUrlState';
import {
  advertisingFeeIdempotencyKey,
  confirmAdvertisingFeeImport,
  createAdvertisingFeeAdjustment,
  downloadAdvertisingFeeCSVTemplate,
  getAdvertisingFee,
  previewAdvertisingFeeImport,
  queryAdvertisingFees,
  reverseAdvertisingFeeAdjustment,
  type AdvertisingAdjustment,
  type AdvertisingAllocation,
  type AdvertisingAllocationDetail,
  type AdvertisingImportPreview,
  type AdvertisingPreviewOrder,
  type AdvertisingSettlementCoverage,
  type AdvertisingSpendPreview,
  type AdvertisingValidationIssue,
} from '@/services/advertisingFees';
import { formatProfitAmount } from '@/services/profitability';
import { queryShops, type ShopListRow } from '@/services/shops';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { parsePositiveInt } from '@/utils/urlState';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import './index.less';

const QUERY_KEYS = ['page', 'pageSize', 'orderNo', 'shopId', 'currency', 'settlementCoverage', 'drawer', 'id'] as const;

const COVERAGE_META: Record<AdvertisingSettlementCoverage, { text: string; color: string }> = {
  excluded: { get text() { return tr('结算未包含'); }, color: 'success' },
  included: { get text() { return tr('结算已包含'); }, color: 'warning' },
  unknown: { get text() { return tr('覆盖未知'); }, color: 'error' },
};

const PROFIT_META = {
  confirmed: { get text() { return tr('可计入利润'); }, color: 'success' },
  mismatch: { get text() { return tr('口径不一致'); }, color: 'warning' },
  blocked: { get text() { return tr('暂不可计入'); }, color: 'error' },
} as const;

function coverageTag(value: AdvertisingSettlementCoverage) {
  const meta = COVERAGE_META[value];
  return <Tag color={meta.color}>{tr(meta.text)}</Tag>;
}

function profitTag(value: AdvertisingAllocation['profitStatus']) {
  const meta = PROFIT_META[value];
  return <Tag color={meta.color}>{tr(meta.text)}</Tag>;
}

function nativeFile(fileList: UploadFile[]) {
  const entry = fileList[0];
  return (entry?.originFileObj || entry) as File | undefined;
}

export default function AdvertisingFeesPage() {
  useLanguage();
  const { can } = usePermission();
  const canImport = can(PERMISSIONS.ADVERTISING_FEE_IMPORT);
  const canManage = can(PERMISSIONS.ADVERTISING_FEE_MANAGE);
  const actionRef = useRef<ActionType>();
  const initializedRef = useRef(false);
  const filterReadGuard = useMemo(createTenantReadGuard, []);
  const detailReadGuard = useMemo(createTenantReadGuard, []);
  const listReadGuard = useMemo(createTenantReadGuard, []);
  const previewRequestRef = useRef(0);
  const submittingRef = useRef(false);
  const adjustmentSubmittingRef = useRef(false);
  const adjustmentKeyRef = useRef<string>();
  const { state: urlState, setState: setUrlState, clearState } = useUrlQueryState<
    Record<(typeof QUERY_KEYS)[number], string | undefined>
  >(QUERY_KEYS);
  const drawer = useUrlDrawerState('advertising-fee');
  const [orderNoInput, setOrderNoInput] = useState(urlState.orderNo || '');
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [shopError, setShopError] = useState('');
  const [listError, setListError] = useState('');
  const [detail, setDetail] = useState<AdvertisingAllocationDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [importOpen, setImportOpen] = useState(false);
  const [importShopID, setImportShopID] = useState<string>();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [preview, setPreview] = useState<AdvertisingImportPreview>();
  const [previewing, setPreviewing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [adjustmentOpen, setAdjustmentOpen] = useState(false);
  const [reversingAdjustment, setReversingAdjustment] = useState<AdvertisingAdjustment>();
  const [adjusting, setAdjusting] = useState(false);
  const [adjustmentForm] = Form.useForm<{ amountMinor?: number; reason: string }>();

  const page = parsePositiveInt(urlState.page, 1);
  const pageSize = parsePositiveInt(urlState.pageSize, 20);
  const shopOptions = useMemo(
    () => shops.map((row) => ({ value: row.id, label: `${row.shopName} · ${platformDisplayLabel(row.platform)}` })),
    [shops],
  );

  useEffect(() => setOrderNoInput(urlState.orderNo || ''), [urlState.orderNo]);
  useEffect(() => {
    let active = true;
    const isCurrent = filterReadGuard.begin();
    setShops([]);
    void queryShops({ page: 1, pageSize: 100 })
      .then((result) => {
        if (!active || !isCurrent()) return;
        setShops(result.list || []);
        setShopError('');
      })
      .catch((error) => {
        if (active && isCurrent()) setShopError((error as Error)?.message || tr('店铺筛选项加载失败'));
      });
    return () => { active = false; filterReadGuard.invalidate(); };
  }, [filterReadGuard]);

  useEffect(() => {
    if (!initializedRef.current) {
      initializedRef.current = true;
      return;
    }
    void actionRef.current?.reload();
  }, [urlState.currency, urlState.orderNo, urlState.settlementCoverage, urlState.shopId]);

  useEffect(() => () => listReadGuard.invalidate(), [listReadGuard]);

  const loadDetail = useCallback(async (id: string) => {
    const isCurrent = detailReadGuard.begin();
    setDetail(undefined);
    setDetailLoading(true);
    setDetailError('');
    try {
      const result = await getAdvertisingFee(id);
      if (isCurrent()) setDetail(result);
    } catch (error) {
      if (isCurrent()) {
        setDetail(undefined);
        setDetailError((error as Error)?.message || tr('广告费用详情加载失败'));
      }
    } finally {
      if (isCurrent()) setDetailLoading(false);
    }
  }, [detailReadGuard]);

  useEffect(() => {
    if (drawer.id) void loadDetail(drawer.id);
    else {
      setDetail(undefined);
      setDetailError('');
    }
    return () => { detailReadGuard.invalidate(); };
  }, [drawer.id, loadDetail, detailReadGuard]);

  const invalidatePreview = useCallback(() => {
    previewRequestRef.current += 1;
    setPreview(undefined);
    setIdempotencyKey('');
    setPreviewing(false);
  }, []);

  const resetImport = useCallback(() => {
    setImportShopID(undefined);
    setFileList([]);
    invalidatePreview();
    submittingRef.current = false;
  }, [invalidatePreview]);

  const runPreview = useCallback(async () => {
    const file = nativeFile(fileList);
    if (!file || !importShopID || previewing) return;
    const requestID = ++previewRequestRef.current;
    setPreviewing(true);
    try {
      const result = await previewAdvertisingFeeImport(file, importShopID);
      if (requestID !== previewRequestRef.current) return;
      setPreview(result);
      setIdempotencyKey(advertisingFeeIdempotencyKey('import'));
    } catch (error) {
      if (requestID === previewRequestRef.current) message.error((error as Error)?.message || tr('广告费用校验失败'));
    } finally {
      if (requestID === previewRequestRef.current) setPreviewing(false);
    }
  }, [fileList, importShopID, previewing]);

  const confirmImport = useCallback(async () => {
    const file = nativeFile(fileList);
    if (!file || !importShopID || !preview?.valid || !idempotencyKey || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      const result = await confirmAdvertisingFeeImport(
        file,
        importShopID,
        preview.fileHash,
        preview.calculationHash,
        idempotencyKey,
      );
      message.success(result.replayed ? tr('该广告费用文件已确认，无需重复处理') : trFormat('已生成 {count} 条订单归属', { count: result.import.allocationCount }));
      setImportOpen(false);
      resetImport();
      void actionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || tr('广告费用确认失败'));
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }, [fileList, idempotencyKey, importShopID, preview, resetImport]);

  const openAdjustment = useCallback((target?: AdvertisingAdjustment) => {
    adjustmentKeyRef.current = undefined;
    setReversingAdjustment(target);
    setAdjustmentOpen(true);
  }, []);

  useEffect(() => {
    if (adjustmentOpen) adjustmentForm.resetFields();
  }, [adjustmentForm, adjustmentOpen]);

  const saveAdjustment = useCallback(async () => {
    if (!detail || adjustmentSubmittingRef.current || !canManage) return;
    adjustmentSubmittingRef.current = true;
    try {
      const values = await adjustmentForm.validateFields();
      const key = adjustmentKeyRef.current || advertisingFeeIdempotencyKey(reversingAdjustment ? 'reverse' : 'adjust');
      adjustmentKeyRef.current = key;
      setAdjusting(true);
      if (reversingAdjustment) {
        await reverseAdvertisingFeeAdjustment(detail.id, reversingAdjustment.id, { reason: values.reason.trim(), idempotencyKey: key });
        message.success(tr('广告费用调整已冲正'));
      } else {
        await createAdvertisingFeeAdjustment(detail.id, { amountMinor: values.amountMinor!, reason: values.reason.trim(), idempotencyKey: key });
        message.success(tr('广告费用调整已追加'));
      }
      setAdjustmentOpen(false);
      setReversingAdjustment(undefined);
      adjustmentKeyRef.current = undefined;
      await loadDetail(detail.id);
      void actionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || tr('广告费用调整失败'));
    } finally {
      adjustmentSubmittingRef.current = false;
      setAdjusting(false);
    }
  }, [adjustmentForm, canManage, detail, loadDetail, reversingAdjustment]);

  const queryParams = useCallback((nextPage?: number, nextPageSize?: number) => ({
    page: nextPage || page,
    pageSize: nextPageSize || pageSize,
    orderNo: urlState.orderNo,
    shopId: urlState.shopId,
    currency: urlState.currency,
    settlementCoverage: urlState.settlementCoverage as AdvertisingSettlementCoverage | undefined,
  }), [page, pageSize, urlState]);

  const columns: ProColumns<AdvertisingAllocation>[] = [
    {
      title: tr('订单'), dataIndex: 'orderNo', width: 190, fixed: 'left', ellipsis: true, copyable: true,
      render: (_, row) => <Button type="link" size="small" onClick={() => drawer.openDrawer(row.id)}>{row.orderNo}</Button>,
    },
    { title: tr('费用日期'), dataIndex: 'spendDate', width: 112 },
    { title: tr('平台 / 店铺'), width: 210, ellipsis: true, render: (_, row) => `${platformDisplayLabel(row.platform)} · ${row.shopName}` },
    { title: tr('支付时间'), dataIndex: 'paidAt', width: 175, render: (value) => formatDateTime(value as string) },
    { title: tr('初始归属'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: tr('调整合计'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.adjustmentMinor, row.currency) },
    { title: tr('当前归属'), width: 125, align: 'right', render: (_, row) => <Typography.Text strong>{formatProfitAmount(row.netAmountMinor, row.currency)}</Typography.Text> },
    { title: tr('结算覆盖'), dataIndex: 'settlementCoverage', width: 125, render: (value) => coverageTag(value as AdvertisingSettlementCoverage) },
    { title: tr('利润口径'), dataIndex: 'profitStatus', width: 125, render: (value, row) => <Tooltip title={row.reason}>{profitTag(value as AdvertisingAllocation['profitStatus'])}</Tooltip> },
    { title: tr('归属时间'), dataIndex: 'attributedAt', width: 175, render: (value) => formatDateTime(value as string) },
    { title: tr('操作'), valueType: 'option', width: 88, fixed: 'right', render: (_, row) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.id)}>{tr('查看')} </Button> },
  ];

  const spendColumns: TableColumnsType<AdvertisingSpendPreview> = [
    { title: tr('行'), dataIndex: 'line', width: 56 },
    { title: tr('费用日期'), dataIndex: 'spendDate', width: 112 },
    { title: tr('费用金额'), width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.spendMinor, row.currency) },
    { title: tr('结算覆盖'), dataIndex: 'settlementCoverage', width: 125, render: (value) => coverageTag(value) },
    { title: tr('纳入订单'), dataIndex: 'eligibleOrderCount', width: 96, align: 'right' },
    { title: tr('排除订单'), dataIndex: 'excludedOrderCount', width: 96, align: 'right' },
    { title: tr('处理'), dataIndex: 'disposition', width: 100, render: (value) => value === 'duplicate' ? <Tag>{tr('跳过重复')} </Tag> : <Tag color="processing">{tr('新增')} </Tag> },
  ];

  const orderColumns: TableColumnsType<AdvertisingPreviewOrder> = [
    { title: tr('费用日期'), dataIndex: 'spendDate', width: 112 },
    { title: tr('订单号'), dataIndex: 'orderNo', width: 180, ellipsis: true },
    { title: tr('支付时间'), dataIndex: 'paidAt', width: 175, render: (value) => value ? formatDateTime(value) : '—' },
    { title: tr('结果'), dataIndex: 'included', width: 90, render: (value) => value ? <Tag color="success">{tr('纳入')} </Tag> : <Tag>{tr('排除')} </Tag> },
    { title: tr('归属金额'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: tr('原因'), dataIndex: 'reason', width: 220, ellipsis: true, render: (value) => value || tr('符合归属条件') },
  ];

  const issueColumns: TableColumnsType<AdvertisingValidationIssue> = [
    { title: tr('行'), dataIndex: 'line', width: 56 },
    { title: tr('字段'), dataIndex: 'field', width: 180, render: (value) => value || tr('文件') },
    { title: tr('问题'), dataIndex: 'message' },
  ];

  const reversedIDs = useMemo(
    () => new Set((detail?.adjustments || []).filter((row) => row.factType === 'reversal' && row.reversesAdjustmentId).map((row) => row.reversesAdjustmentId!)),
    [detail?.adjustments],
  );
  const adjustmentColumns: TableColumnsType<AdvertisingAdjustment> = [
    { title: tr('类型'), dataIndex: 'factType', width: 96, render: (value) => value === 'reversal' ? <Tag>{tr('冲正')} </Tag> : <Tag color="processing">{tr('调整')} </Tag> },
    { title: tr('金额'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.amountMinor, row.currency) },
    { title: tr('原因'), dataIndex: 'reason', width: 240, ellipsis: true },
    { title: tr('记录时间'), dataIndex: 'createdAt', width: 175, render: (value) => formatDateTime(value) },
    {
      title: tr('操作'), width: 90, render: (_, row) => row.factType === 'adjustment' && !reversedIDs.has(row.id)
        ? <Button type="link" size="small" icon={<RollbackOutlined />} disabled={!canManage} onClick={() => openAdjustment(row)}>{tr('冲正')} </Button>
        : '—',
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.ADVERTISING_FEE_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-advertising-fee-page"
        title={tr("广告费用归属")}
        subTitle={tr("按店铺当地日将广告费用确定性归属到已支付且未取消的订单")}
        extra={<TmPageHeaderExtra>
          <Button icon={<DownloadOutlined />} onClick={downloadAdvertisingFeeCSVTemplate}>{tr('下载模板')} </Button>
          <Tooltip title={canImport ? tr('导入广告费用 CSV') : tr('当前账号无导入权限')}>
            <Button icon={<ImportOutlined />} disabled={!canImport} onClick={() => setImportOpen(true)}>{tr('导入费用')} </Button>
          </Tooltip>
        </TmPageHeaderExtra>}
      >
        <Alert type="info" showIcon message={tr("订单归属是运营估算事实")} description={tr("仅明确声明结算未包含的广告费用进入预估利润；结算已包含或覆盖未知时失败关闭。")} />
        {shopError ? <Alert type="warning" showIcon message={shopError} /> : null}
        {listError ? <ErrorAlert title={listError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>{tr('重新加载')} </Button>} /> : null}
        <TmProTable<AdvertisingAllocation>
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1560 }}
          locale={{ emptyText: listError ? tr('广告费用归属数据暂不可用') : <EmptyState compact title={tr("暂无广告费用归属")} /> }}
          filterBar={<Space className="tm-advertising-fee-toolbar" wrap>
            <Input.Search allowClear aria-label={tr("订单号")} placeholder={tr("搜索订单号")} value={orderNoInput} className="tm-advertising-fee-filter tm-advertising-fee-filter--search" onChange={(event) => setOrderNoInput(event.target.value)} onSearch={(value) => setUrlState({ orderNo: value.trim() || undefined, page: undefined }, { replace: true })} />
            <Select allowClear showSearch optionFilterProp="label" aria-label={tr("店铺")} placeholder={tr("全部店铺")} value={urlState.shopId} className="tm-advertising-fee-filter" options={shopOptions} onChange={(value) => setUrlState({ shopId: value || undefined, page: undefined }, { replace: true })} />
            <Select allowClear showSearch aria-label={tr("币种")} placeholder={tr("全部币种")} value={urlState.currency} className="tm-advertising-fee-filter" options={['CNY', 'USD', 'EUR', 'JPY', 'GBP'].map((value) => ({ value, label: value }))} onChange={(value) => setUrlState({ currency: value || undefined, page: undefined }, { replace: true })} />
            <Select allowClear aria-label={tr("结算覆盖")} placeholder={tr("全部结算口径")} value={urlState.settlementCoverage} className="tm-advertising-fee-filter" options={Object.entries(COVERAGE_META).map(([value, meta]) => ({ value, label: meta.text }))} onChange={(value) => setUrlState({ settlementCoverage: value || undefined, page: undefined }, { replace: true })} />
            <Button onClick={() => { clearState(['page', 'pageSize', 'orderNo', 'shopId', 'currency', 'settlementCoverage'], { replace: true }); setOrderNoInput(''); }}>{tr('重置')} </Button>
          </Space>}
          toolBarRender={() => []}
          pagination={{
            current: page,
            pageSize,
            showSizeChanger: true,
            onChange: (nextPage, nextPageSize) => setUrlState({ page: nextPage > 1 ? String(nextPage) : undefined, pageSize: nextPageSize !== 20 ? String(nextPageSize) : undefined }, { replace: true }),
          }}
          request={async (params) => {
            const isCurrent = listReadGuard.begin();
            try {
              const result = await queryAdvertisingFees(queryParams(params.current, params.pageSize));
              if (!isCurrent()) return { data: [], success: false, total: 0 };
              setListError('');
              return { data: result.list || [], success: true, total: result.total };
            } catch (error) {
              if (isCurrent()) setListError((error as Error)?.message || tr('广告费用归属列表加载失败'));
              return { data: [], success: false, total: 0 };
            }
          }}
        />
      </TmPageContainer>

      <Modal
        title={tr("导入广告费用")}
        open={importOpen}
        width={1040}
        destroyOnHidden
        maskClosable={!submitting}
        closable={!submitting}
        onCancel={() => { if (!submitting) { setImportOpen(false); resetImport(); } }}
        footer={<OperationToolbar className="tm-advertising-fee-actions">
          <Button disabled={submitting} onClick={() => { setImportOpen(false); resetImport(); }}>{tr('取消')} </Button>
          {preview ? <Button disabled={submitting} onClick={invalidatePreview}>{tr('重新选择')} </Button> : null}
          {!preview ? <Button type="primary" disabled={!importShopID || !nativeFile(fileList)} loading={previewing} onClick={() => void runPreview()}>{tr('校验预览')} </Button> : null}
          {preview?.valid ? <Button type="primary" icon={<ImportOutlined />} disabled={submitting} loading={submitting} onClick={() => void confirmImport()}>{tr('确认归属')} </Button> : null}
        </OperationToolbar>}
      >
        {!preview ? <div className="tm-advertising-fee-import-source">
          <Select showSearch optionFilterProp="label" aria-label={tr("费用店铺")} placeholder={tr("选择费用所属店铺")} value={importShopID} options={shopOptions} onChange={(value) => { setImportShopID(value); invalidatePreview(); }} />
          <Upload accept=".csv,text/csv" maxCount={1} fileList={fileList} beforeUpload={(file) => { setFileList([file]); invalidatePreview(); return false; }} onRemove={() => { setFileList([]); invalidatePreview(); }}>
            <Button icon={<UploadOutlined />}>{tr('选择 CSV')} </Button>
          </Upload>
          <Button type="link" icon={<DownloadOutlined />} onClick={downloadAdvertisingFeeCSVTemplate}>{tr('下载 CSV 模板')} </Button>
        </div> : <Space direction="vertical" size="middle" className="tm-advertising-fee-preview">
          <div className="tm-advertising-fee-summary">
            <Statistic title={tr("费用行")} value={preview.sourceRows} />
            <Statistic title={tr("新增费用")} value={preview.newSpends} />
            <Statistic title={tr("订单归属")} value={preview.allocationCount} />
            <Statistic title={tr("排除订单")} value={preview.rows.reduce((sum, row) => sum + row.excludedOrderCount, 0)} />
          </div>
          {preview.valid ? <Alert type="success" showIcon message={tr("广告费用校验通过")} description={`${preview.shopName} · ${preview.shopTimezone} · ${preview.policyVersion}`} /> : <Alert type="error" showIcon message={tr("广告费用未通过校验")} />}
          {preview.warnings.map((warning) => <Alert key={`${warning.line}-${warning.code}`} type="warning" showIcon message={trFormat('第 {line} 行', { line: warning.line })} description={warning.message} />)}
          {preview.issues.length ? <Table rowKey={(row) => `${row.line}-${row.field || ''}-${row.code}`} size="small" columns={issueColumns} dataSource={preview.issues} pagination={{ pageSize: 5 }} scroll={{ x: 640 }} /> : <>
            <Table rowKey={(row) => `${row.line}-${row.spendDate}-${row.currency}`} size="small" columns={spendColumns} dataSource={preview.rows} pagination={false} scroll={{ x: 780 }} />
            <Table rowKey={(row) => `${row.line}-${row.orderId}`} size="small" columns={orderColumns} dataSource={preview.orders} pagination={{ pageSize: 6 }} scroll={{ x: 920 }} />
          </>}
        </Space>}
      </Modal>

      <AppDrawer title={tr("广告费用归属详情")} open={drawer.open} onClose={drawer.closeDrawer} loading={detailLoading} width={940}>
        {detailError ? <ErrorAlert title={detailError} actionHint={drawer.id ? <Button icon={<ReloadOutlined />} onClick={() => void loadDetail(drawer.id!)}>{tr('重新加载')} </Button> : undefined} /> : null}
        {detail ? <Space direction="vertical" size="large" className="tm-advertising-fee-detail">
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
            <Descriptions.Item label={tr("订单号")}>{detail.orderNo}</Descriptions.Item>
            <Descriptions.Item label={tr("当前归属")}><Typography.Text strong>{formatProfitAmount(detail.netAmountMinor, detail.currency)}</Typography.Text></Descriptions.Item>
            <Descriptions.Item label={tr("平台 / 店铺")}>{platformDisplayLabel(detail.platform)} · {detail.shopName}</Descriptions.Item>
            <Descriptions.Item label={tr("费用日期")}>{detail.spendDate}（{detail.shopTimezone}）</Descriptions.Item>
            <Descriptions.Item label={tr("结算覆盖")}>{coverageTag(detail.settlementCoverage)}</Descriptions.Item>
            <Descriptions.Item label={tr("利润口径")}>{profitTag(detail.profitStatus)}</Descriptions.Item>
            <Descriptions.Item label={tr("初始归属")}>{formatProfitAmount(detail.amountMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label={tr("调整合计")}>{formatProfitAmount(detail.adjustmentMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label={tr("分配策略")}>{detail.policyVersion}</Descriptions.Item>
            <Descriptions.Item label={tr("确认时间")}>{formatDateTime(detail.attributedAt)}</Descriptions.Item>
          </Descriptions>
          {detail.reason ? <Alert type={detail.profitStatus === 'blocked' ? 'error' : 'warning'} showIcon message={detail.reason} /> : null}
          <OperationToolbar>
            <Button icon={<PlusOutlined />} disabled={!canManage} onClick={() => openAdjustment()}>{tr('追加调整')} </Button>
            <Button icon={<EyeOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(detail.orderId)}`)}>{tr('查看订单')} </Button>
            <Button icon={<EyeOutlined />} onClick={() => history.push(`/finance/order-profits?drawer=order-profit&id=${encodeURIComponent(detail.orderId)}`)}>{tr('查看预估利润')} </Button>
          </OperationToolbar>
          <Table rowKey="id" columns={adjustmentColumns} dataSource={detail.adjustments || []} locale={{ emptyText: tr('暂无费用调整') }} pagination={false} scroll={{ x: 760 }} />
        </Space> : !detailLoading && !detailError ? <EmptyState compact title={tr("暂无广告费用详情")} /> : null}
      </AppDrawer>

      <Modal
        title={reversingAdjustment ? tr('冲正广告费用调整') : tr('追加广告费用调整')}
        open={adjustmentOpen}
        destroyOnHidden
        maskClosable={!adjusting}
        closable={!adjusting}
        onCancel={() => { if (!adjusting) { adjustmentKeyRef.current = undefined; setAdjustmentOpen(false); setReversingAdjustment(undefined); } }}
        onOk={() => void saveAdjustment()}
        okText={reversingAdjustment ? tr('确认冲正') : tr('确认追加')}
        okButtonProps={{ disabled: !canManage }}
        confirmLoading={adjusting}
      >
        {reversingAdjustment ? <Alert type="warning" showIcon message={trFormat('将追加 {amount} 的冲正事实', { amount: formatProfitAmount(-reversingAdjustment.amountMinor, reversingAdjustment.currency) })} /> : null}
        <Form form={adjustmentForm} layout="vertical" preserve={false} className="tm-advertising-fee-adjustment-form">
          {!reversingAdjustment ? <Form.Item name="amountMinor" label={tr("调整金额（最小货币单位，可为负数）")} rules={[{ required: true, type: 'integer' }, { validator: (_, value) => value === 0 ? Promise.reject(new Error(tr('调整金额不能为 0'))) : Promise.resolve() }]}><InputNumber min={-Number.MAX_SAFE_INTEGER} max={Number.MAX_SAFE_INTEGER} precision={0} /></Form.Item> : null}
          <Form.Item name="reason" label={tr("原因")} rules={[{ required: true, whitespace: true, min: 2, message: tr('请输入至少 2 个字符的原因') }]}><Input.TextArea maxLength={500} showCount rows={3} /></Form.Item>
        </Form>
      </Modal>
    </PermissionGuard>
  );
}
