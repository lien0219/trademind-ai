import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import {
  DownloadOutlined,
  EyeOutlined,
  FileTextOutlined,
  ImportOutlined,
  LinkOutlined,
  ReloadOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  DatePicker,
  Descriptions,
  Input,
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
import dayjs, { type Dayjs } from 'dayjs';
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
import { PLATFORM_DISPLAY_LABEL, platformDisplayLabel } from '@/constants/platformLabels';
import { usePermission } from '@/hooks/usePermission';
import { useUrlDrawerState, useUrlQueryState } from '@/hooks/useUrlState';
import { formatProfitAmount } from '@/services/profitability';
import {
  confirmSettlementImport,
  downloadSettlementCSVTemplate,
  downloadSettlementReconciliation,
  getSettlementReconciliation,
  previewSettlementImport,
  querySettlementReconciliation,
  settlementImportIdempotencyKey,
  type SettlementCSVRow,
  type SettlementImportPreview,
  type SettlementReconciliation,
  type SettlementReconciliationDetail,
  type SettlementStatus,
  type SettlementTransaction,
  type SettlementValidationIssue,
} from '@/services/settlement';
import { queryShops, type ShopListRow } from '@/services/shops';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { parsePositiveInt } from '@/utils/urlState';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import './index.less';

const { RangePicker } = DatePicker;

const QUERY_KEYS = [
  'page',
  'pageSize',
  'orderNo',
  'platform',
  'shopId',
  'currency',
  'status',
  'start',
  'end',
  'drawer',
  'id',
] as const;

const STATUS_META: Record<SettlementStatus, { text: string; color: string }> = {
  matched: { get text() { return tr('一致'); }, color: 'success' },
  pending: { get text() { return tr('待匹配'); }, color: 'processing' },
  mismatch: { get text() { return tr('不一致'); }, color: 'warning' },
  blocked: { get text() { return tr('已阻断'); }, color: 'error' },
};

const CURRENCY_OPTIONS = ['CNY', 'USD', 'EUR', 'GBP', 'JPY', 'KRW', 'SGD', 'AUD', 'CAD'].map(
  (value) => ({ value, label: value }),
);

function parseDateRange(start?: string, end?: string): [Dayjs | null, Dayjs | null] | null {
  const left = start ? dayjs(start) : null;
  const right = end ? dayjs(end) : null;
  if ((!left || left.isValid()) && (!right || right.isValid()) && (left || right)) return [left, right];
  return null;
}

function statusTag(status: SettlementStatus) {
  const meta = STATUS_META[status] || { text: status, color: 'default' };
  return <Tag color={meta.color}>{tr(meta.text)}</Tag>;
}

function nativeFile(fileList: UploadFile[]) {
  const entry = fileList[0];
  return (entry?.originFileObj || entry) as File | undefined;
}

export default function SettlementReconciliationPage() {
  useLanguage();
  const { can } = usePermission();
  const canImport = can(PERMISSIONS.SETTLEMENT_IMPORT);
  const canExport = can(PERMISSIONS.SETTLEMENT_EXPORT);
  const actionRef = useRef<ActionType>();
  const initializedRef = useRef(false);
  const filterReadGuard = useMemo(createTenantReadGuard, []);
  const detailReadGuard = useMemo(createTenantReadGuard, []);
  const listReadGuard = useMemo(createTenantReadGuard, []);
  const previewRequestRef = useRef(0);
  const submittingRef = useRef(false);
  const { state: urlState, setState: setUrlState, clearState } = useUrlQueryState<
    Record<(typeof QUERY_KEYS)[number], string | undefined>
  >(QUERY_KEYS);
  const drawer = useUrlDrawerState('settlement-reconciliation');
  const [orderNoInput, setOrderNoInput] = useState(urlState.orderNo || '');
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [filterSourceError, setFilterSourceError] = useState('');
  const [listError, setListError] = useState('');
  const [detail, setDetail] = useState<SettlementReconciliationDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [exporting, setExporting] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [importShopID, setImportShopID] = useState<string>();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [preview, setPreview] = useState<SettlementImportPreview>();
  const [previewing, setPreviewing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');

  const page = parsePositiveInt(urlState.page, 1);
  const pageSize = parsePositiveInt(urlState.pageSize, 20);
  const dateRange = useMemo(() => parseDateRange(urlState.start, urlState.end), [urlState.end, urlState.start]);
  const platformOptions = useMemo(
    () => Object.entries(PLATFORM_DISPLAY_LABEL).map(([value, label]) => ({ value, label })),
    [],
  );
  const shopOptions = useMemo(
    () => shops.map((shop) => ({ value: shop.id, label: `${shop.shopName} · ${platformDisplayLabel(shop.platform)}` })),
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
        setFilterSourceError('');
      })
      .catch((error) => {
        if (active && isCurrent()) setFilterSourceError((error as Error)?.message || tr('店铺筛选项加载失败'));
      });
    return () => {
      active = false;
      filterReadGuard.invalidate();
    };
  }, [filterReadGuard]);

  useEffect(() => {
    if (!initializedRef.current) {
      initializedRef.current = true;
      return;
    }
    void actionRef.current?.reload();
  }, [urlState.currency, urlState.end, urlState.orderNo, urlState.platform, urlState.shopId, urlState.start, urlState.status]);

  useEffect(() => () => listReadGuard.invalidate(), [listReadGuard]);

  const loadDetail = useCallback(async (id: string) => {
    const isCurrent = detailReadGuard.begin();
    setDetail(undefined);
    setDetailLoading(true);
    setDetailError('');
    try {
      const result = await getSettlementReconciliation(id);
      if (isCurrent()) setDetail(result);
    } catch (error) {
      if (isCurrent()) {
        setDetail(undefined);
        setDetailError((error as Error)?.message || tr('结算对账详情加载失败'));
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

  const queryParams = useCallback(
    (nextPage?: number, nextPageSize?: number) => ({
      page: nextPage || page,
      pageSize: nextPageSize || pageSize,
      orderNo: urlState.orderNo,
      platform: urlState.platform,
      shopId: urlState.shopId,
      currency: urlState.currency,
      status: urlState.status as SettlementStatus | undefined,
      start: urlState.start,
      end: urlState.end,
    }),
    [page, pageSize, urlState],
  );

  const runPreview = useCallback(async () => {
    const file = nativeFile(fileList);
    if (!file || !importShopID || previewing) return;
    const requestID = ++previewRequestRef.current;
    setPreviewing(true);
    try {
      const result = await previewSettlementImport(file, importShopID);
      if (requestID !== previewRequestRef.current) return;
      setPreview(result);
      setIdempotencyKey(settlementImportIdempotencyKey());
    } catch (error) {
      if (requestID !== previewRequestRef.current) return;
      message.error((error as Error)?.message || tr('账单校验失败'));
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
      const result = await confirmSettlementImport(file, importShopID, preview.fileHash, idempotencyKey);
      message.success(result.replayed ? tr('该账单已导入，无需重复处理') : trFormat('已导入 {count} 条结算交易', { count: result.import.importedRows }));
      setImportOpen(false);
      resetImport();
      void actionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || tr('账单导入失败'));
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }, [fileList, idempotencyKey, importShopID, preview, resetImport]);

  const exportCurrent = useCallback(async () => {
    if (!canExport || exporting) return;
    setExporting(true);
    try {
      const { page: _page, pageSize: _pageSize, ...filters } = queryParams();
      await downloadSettlementReconciliation(filters);
      message.success(tr('结算对账明细已导出'));
    } catch (error) {
      message.error((error as Error)?.message || tr('导出失败，请收窄筛选条件后重试'));
    } finally {
      setExporting(false);
    }
  }, [canExport, exporting, queryParams]);

  const updateFilter = useCallback(
    (key: 'platform' | 'shopId' | 'currency' | 'status', value?: string) => {
      setUrlState({ [key]: value || undefined, page: undefined }, { replace: true });
    },
    [setUrlState],
  );

  const columns: ProColumns<SettlementReconciliation>[] = [
    {
      title: tr('订单'),
      dataIndex: 'orderNo',
      width: 190,
      fixed: 'left',
      ellipsis: true,
      copyable: true,
      render: (_, row) => <Button type="link" size="small" onClick={() => drawer.openDrawer(row.id)}>{row.orderNo}</Button>,
    },
    { title: tr('平台 / 店铺'), width: 200, ellipsis: true, render: (_, row) => `${platformDisplayLabel(row.platform)} · ${row.shopName || tr('未知店铺')}` },
    { title: tr('本地订单'), width: 130, render: (_, row) => row.orderId ? <Tag color="success">{tr('已匹配')} </Tag> : <Tag>{tr('未匹配')} </Tag> },
    { title: tr('订单金额'), width: 135, align: 'right', render: (_, row) => formatProfitAmount(row.orderAmountMinor, row.currency) },
    { title: tr('账单交易总额'), width: 145, align: 'right', render: (_, row) => formatProfitAmount(row.orderGrossMinor, row.currency) },
    { title: tr('平台费用'), width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.platformFeeMinor, row.currency) },
    { title: tr('结算金额'), width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.settlementAmountMinor, row.currency) },
    { title: tr('交易数'), dataIndex: 'transactionCount', width: 86, align: 'right' },
    { title: tr('对账状态'), dataIndex: 'status', width: 112, render: (_, row) => statusTag(row.status) },
    {
      title: tr('差异说明'),
      width: 145,
      render: (_, row) => <Tooltip title={row.issues.map((issue) => issue.message).join('；')}><Typography.Text type={row.issues.length ? 'warning' : 'secondary'}>{row.issues.length ? trFormat('{count} 项', { count: row.issues.length }) : tr('无')}</Typography.Text></Tooltip>,
    },
    { title: tr('最后结算时间'), dataIndex: 'lastSettledAt', width: 175, render: (value) => formatDateTime(value as string) },
    { title: tr('操作'), valueType: 'option', width: 88, fixed: 'right', render: (_, row) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.id)}>{tr('查看')} </Button> },
  ];

  const previewColumns: TableColumnsType<SettlementCSVRow> = [
    { title: tr('行'), dataIndex: 'line', width: 60 },
    { title: tr('外部交易号'), dataIndex: 'externalTransactionId', width: 170, ellipsis: true },
    { title: tr('订单号'), dataIndex: 'orderNo', width: 160, ellipsis: true },
    { title: tr('交易总额'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.orderGrossMinor, row.currency) },
    { title: tr('平台费用'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.platformFeeMinor, row.currency) },
    { title: tr('结算金额'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.settlementAmountMinor, row.currency) },
    { title: tr('处理'), dataIndex: 'disposition', width: 90, render: (value) => value === 'duplicate' ? <Tag>{tr('跳过重复')} </Tag> : <Tag color="processing">{tr('新增')} </Tag> },
  ];

  const issueColumns: TableColumnsType<SettlementValidationIssue> = [
    { title: tr('行'), dataIndex: 'line', width: 60 },
    { title: tr('字段'), dataIndex: 'field', width: 190, render: (value) => value || tr('文件') },
    { title: tr('问题'), dataIndex: 'message' },
  ];

  const transactionColumns: TableColumnsType<SettlementTransaction> = [
    {
      title: tr('外部交易号'),
      dataIndex: 'externalTransactionId',
      width: 190,
      ellipsis: true,
      render: (value) => <Typography.Text copyable>{value}</Typography.Text>,
    },
    { title: tr('订单交易总额'), width: 145, align: 'right', render: (_, row) => formatProfitAmount(row.orderGrossMinor, row.currency) },
    { title: tr('平台费用'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.platformFeeMinor, row.currency) },
    { title: tr('结算金额'), width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.settlementAmountMinor, row.currency) },
    { title: tr('结算时间'), dataIndex: 'settledAt', width: 180, render: (value) => formatDateTime(value) },
    { title: tr('导入时间'), dataIndex: 'createdAt', width: 180, render: (value) => formatDateTime(value) },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.SETTLEMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-settlement-page"
        title={tr("平台结算对账")}
        subTitle={tr("核对平台结算交易、本地订单金额与平台费用，只有一致费用进入订单预估利润")}
        extra={<TmPageHeaderExtra>
          <Tooltip title={canImport ? tr('导入平台结算 CSV') : tr('当前账号无导入权限')}>
            <Button icon={<ImportOutlined />} disabled={!canImport} onClick={() => setImportOpen(true)}>{tr('导入账单')} </Button>
          </Tooltip>
          <Tooltip title={canExport ? tr('导出当前筛选结果，最多 5000 条') : tr('当前账号无导出权限')}>
            <Button icon={<DownloadOutlined />} disabled={!canExport} loading={exporting} onClick={() => void exportCurrent()}>{tr('导出明细')} </Button>
          </Tooltip>
        </TmPageHeaderExtra>}
      >
        <Alert type="info" showIcon message={tr("平台结算交易是不可变费用事实")} description={tr("调整和冲正以新的外部交易号导入；重复交易不会重复计费，差异账单不会进入利润计算。")} />
        {filterSourceError ? <Alert type="warning" showIcon message={filterSourceError} /> : null}
        {listError ? <ErrorAlert title={listError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>{tr('重新加载')} </Button>} /> : null}
        <TmProTable<SettlementReconciliation>
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1640 }}
          locale={{ emptyText: listError ? tr('结算对账数据暂不可用') : <EmptyState compact title={tr("暂无结算对账记录")} /> }}
          filterBar={<Space className="tm-settlement-toolbar" wrap>
            <Input.Search allowClear aria-label={tr("订单号")} placeholder={tr("搜索订单号")} value={orderNoInput} className="tm-settlement-filter tm-settlement-filter--search" onChange={(event) => setOrderNoInput(event.target.value)} onSearch={(value) => setUrlState({ orderNo: value.trim() || undefined, page: undefined }, { replace: true })} />
            <Select allowClear aria-label={tr("平台")} placeholder={tr("全部平台")} value={urlState.platform} className="tm-settlement-filter" options={platformOptions} onChange={(value) => updateFilter('platform', value)} />
            <Select allowClear showSearch optionFilterProp="label" aria-label={tr("店铺")} placeholder={tr("全部店铺")} value={urlState.shopId} className="tm-settlement-filter" options={shopOptions} onChange={(value) => updateFilter('shopId', value)} />
            <Select allowClear showSearch aria-label={tr("币种")} placeholder={tr("全部币种")} value={urlState.currency} className="tm-settlement-filter" options={CURRENCY_OPTIONS} onChange={(value) => updateFilter('currency', value)} />
            <Select allowClear aria-label={tr("对账状态")} placeholder={tr("全部状态")} value={urlState.status} className="tm-settlement-filter" options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.text }))} onChange={(value) => updateFilter('status', value)} />
            <RangePicker aria-label={tr("结算日期范围")} value={dateRange} className="tm-settlement-filter tm-settlement-filter--range" onChange={(value) => setUrlState({ start: value?.[0]?.startOf('day').toISOString(), end: value?.[1]?.endOf('day').toISOString(), page: undefined }, { replace: true })} />
            <Button onClick={() => { clearState(QUERY_KEYS.filter((key) => key !== 'drawer' && key !== 'id'), { replace: true }); setOrderNoInput(''); }}>{tr('重置')} </Button>
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
              const result = await querySettlementReconciliation(queryParams(params.current, params.pageSize));
              if (!isCurrent()) return { data: [], success: false, total: 0 };
              setListError('');
              return { data: result.list || [], success: true, total: result.total };
            } catch (error) {
              if (isCurrent()) setListError((error as Error)?.message || tr('结算对账列表加载失败'));
              return { data: [], success: false, total: 0 };
            }
          }}
        />
      </TmPageContainer>

      <Modal
        title={tr("导入平台结算账单")}
        open={importOpen}
        width={960}
        destroyOnHidden
        maskClosable={!submitting}
        closable={!submitting}
        onCancel={() => { if (!submitting) { setImportOpen(false); resetImport(); } }}
        footer={<OperationToolbar className="tm-settlement-import-actions">
          <Button onClick={() => { setImportOpen(false); resetImport(); }} disabled={submitting}>{tr('取消')} </Button>
          {preview ? <Button onClick={invalidatePreview} disabled={submitting}>{tr('重新选择')} </Button> : null}
          {!preview ? <Button type="primary" icon={<FileTextOutlined />} disabled={!importShopID || !nativeFile(fileList)} loading={previewing} onClick={() => void runPreview()}>{tr('校验预览')} </Button> : null}
          {preview?.valid ? <Button type="primary" icon={<ImportOutlined />} loading={submitting} disabled={submitting} onClick={() => void confirmImport()}>{tr('确认导入')} </Button> : null}
        </OperationToolbar>}
      >
        {!preview ? <div className="tm-settlement-import-source">
          <Select showSearch optionFilterProp="label" aria-label={tr("账单店铺")} placeholder={tr("选择账单所属店铺")} value={importShopID} options={shopOptions} onChange={(value) => { setImportShopID(value); invalidatePreview(); }} />
          <Upload accept=".csv,text/csv" maxCount={1} fileList={fileList} beforeUpload={(file) => { setFileList([file]); invalidatePreview(); return false; }} onRemove={() => { setFileList([]); invalidatePreview(); }}>
            <Button icon={<UploadOutlined />}>{tr('选择 CSV')} </Button>
          </Upload>
          <Button type="link" icon={<DownloadOutlined />} onClick={downloadSettlementCSVTemplate}>{tr('下载 CSV 模板')} </Button>
        </div> : <>
          <div className="tm-settlement-import-summary">
            <Statistic title={tr("账单行")} value={preview.sourceRows} />
            <Statistic title={tr("新增")} value={preview.newRows} />
            <Statistic title={tr("重复跳过")} value={preview.duplicateRows} />
            <Statistic title={tr("校验问题")} value={preview.issues.length} valueStyle={preview.valid ? undefined : { color: '#cf1322' }} />
          </div>
          {preview.valid ? <Alert type="success" showIcon message={tr("账单校验通过")} description={`${preview.shopName} · ${platformDisplayLabel(preview.platform)}`} /> : <Alert type="error" showIcon message={tr("账单未通过校验")} />}
          {preview.issues.length ? <Table rowKey={(row) => `${row.line}-${row.field || ''}-${row.code}`} size="small" columns={issueColumns} dataSource={preview.issues} pagination={{ pageSize: 5 }} scroll={{ x: 640 }} /> : <Table rowKey={(row) => `${row.line}-${row.externalTransactionId}`} size="small" columns={previewColumns} dataSource={preview.rows} pagination={{ pageSize: 5 }} scroll={{ x: 900 }} />}
        </>}
      </Modal>

      <AppDrawer title={tr("结算对账详情")} open={drawer.open} onClose={drawer.closeDrawer} loading={detailLoading} width={920}>
        {detailError ? <ErrorAlert title={detailError} actionHint={drawer.id ? <Button icon={<ReloadOutlined />} onClick={() => void loadDetail(drawer.id!)}>{tr('重新加载')} </Button> : undefined} /> : null}
        {detail ? <Space direction="vertical" size="large" className="tm-settlement-detail">
          <Descriptions column={{ xs: 1, sm: 2 }} bordered size="small">
            <Descriptions.Item label={tr("订单号")}>{detail.orderNo}</Descriptions.Item>
            <Descriptions.Item label={tr("对账状态")}>{statusTag(detail.status)}</Descriptions.Item>
            <Descriptions.Item label={tr("平台 / 店铺")}>{platformDisplayLabel(detail.platform)} · {detail.shopName}</Descriptions.Item>
            <Descriptions.Item label={tr("币种")}>{detail.currency || tr('多币种')}</Descriptions.Item>
            <Descriptions.Item label={tr("本地订单金额")}>{formatProfitAmount(detail.orderAmountMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label={tr("账单交易总额")}>{formatProfitAmount(detail.orderGrossMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label={tr("平台费用")}>{formatProfitAmount(detail.platformFeeMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label={tr("结算金额")}>{formatProfitAmount(detail.settlementAmountMinor, detail.currency)}</Descriptions.Item>
          </Descriptions>
          {detail.issues.length ? <Alert type={detail.status === 'blocked' ? 'error' : 'warning'} showIcon message={tr("对账差异")} description={detail.issues.map((issue) => issue.message).join('；')} /> : null}
          <OperationToolbar>
            {detail.orderId ? <Button icon={<LinkOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(detail.orderId!)}`)}>{tr('查看订单')} </Button> : null}
            {detail.orderId ? <Button icon={<LinkOutlined />} onClick={() => history.push(`/finance/order-profits?drawer=order-profit&id=${encodeURIComponent(detail.orderId!)}`)}>{tr('查看预估利润')} </Button> : null}
          </OperationToolbar>
          <Table rowKey="id" columns={transactionColumns} dataSource={detail.transactions} pagination={false} scroll={{ x: 980 }} />
        </Space> : !detailLoading && !detailError ? <EmptyState compact title={tr("暂无对账详情")} /> : null}
      </AppDrawer>
    </PermissionGuard>
  );
}
