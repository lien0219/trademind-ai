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
  matched: { text: '一致', color: 'success' },
  pending: { text: '待匹配', color: 'processing' },
  mismatch: { text: '不一致', color: 'warning' },
  blocked: { text: '已阻断', color: 'error' },
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
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

function nativeFile(fileList: UploadFile[]) {
  const entry = fileList[0];
  return (entry?.originFileObj || entry) as File | undefined;
}

export default function SettlementReconciliationPage() {
  const { can } = usePermission();
  const canImport = can(PERMISSIONS.SETTLEMENT_IMPORT);
  const canExport = can(PERMISSIONS.SETTLEMENT_EXPORT);
  const actionRef = useRef<ActionType>();
  const initializedRef = useRef(false);
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
    void queryShops({ page: 1, pageSize: 100 })
      .then((result) => {
        if (!active) return;
        setShops(result.list || []);
        setFilterSourceError('');
      })
      .catch((error) => {
        if (active) setFilterSourceError((error as Error)?.message || '店铺筛选项加载失败');
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!initializedRef.current) {
      initializedRef.current = true;
      return;
    }
    void actionRef.current?.reload();
  }, [urlState.currency, urlState.end, urlState.orderNo, urlState.platform, urlState.shopId, urlState.start, urlState.status]);

  const loadDetail = useCallback(async (id: string) => {
    setDetailLoading(true);
    setDetailError('');
    try {
      setDetail(await getSettlementReconciliation(id));
    } catch (error) {
      setDetail(undefined);
      setDetailError((error as Error)?.message || '结算对账详情加载失败');
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (drawer.id) void loadDetail(drawer.id);
    else {
      setDetail(undefined);
      setDetailError('');
    }
  }, [drawer.id, loadDetail]);

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
      message.error((error as Error)?.message || '账单校验失败');
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
      message.success(result.replayed ? '该账单已导入，无需重复处理' : `已导入 ${result.import.importedRows} 条结算交易`);
      setImportOpen(false);
      resetImport();
      void actionRef.current?.reload();
    } catch (error) {
      message.error((error as Error)?.message || '账单导入失败');
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
      message.success('结算对账明细已导出');
    } catch (error) {
      message.error((error as Error)?.message || '导出失败，请收窄筛选条件后重试');
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
      title: '订单',
      dataIndex: 'orderNo',
      width: 190,
      fixed: 'left',
      ellipsis: true,
      copyable: true,
      render: (_, row) => <Button type="link" size="small" onClick={() => drawer.openDrawer(row.id)}>{row.orderNo}</Button>,
    },
    { title: '平台 / 店铺', width: 200, ellipsis: true, render: (_, row) => `${platformDisplayLabel(row.platform)} · ${row.shopName || '未知店铺'}` },
    { title: '本地订单', width: 130, render: (_, row) => row.orderId ? <Tag color="success">已匹配</Tag> : <Tag>未匹配</Tag> },
    { title: '订单金额', width: 135, align: 'right', render: (_, row) => formatProfitAmount(row.orderAmountMinor, row.currency) },
    { title: '账单交易总额', width: 145, align: 'right', render: (_, row) => formatProfitAmount(row.orderGrossMinor, row.currency) },
    { title: '平台费用', width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.platformFeeMinor, row.currency) },
    { title: '结算金额', width: 130, align: 'right', render: (_, row) => formatProfitAmount(row.settlementAmountMinor, row.currency) },
    { title: '交易数', dataIndex: 'transactionCount', width: 86, align: 'right' },
    { title: '对账状态', dataIndex: 'status', width: 112, render: (_, row) => statusTag(row.status) },
    {
      title: '差异说明',
      width: 145,
      render: (_, row) => <Tooltip title={row.issues.map((issue) => issue.message).join('；')}><Typography.Text type={row.issues.length ? 'warning' : 'secondary'}>{row.issues.length ? `${row.issues.length} 项` : '无'}</Typography.Text></Tooltip>,
    },
    { title: '最后结算时间', dataIndex: 'lastSettledAt', width: 175, render: (value) => formatDateTime(value as string) },
    { title: '操作', valueType: 'option', width: 88, fixed: 'right', render: (_, row) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.id)}>查看</Button> },
  ];

  const previewColumns: TableColumnsType<SettlementCSVRow> = [
    { title: '行', dataIndex: 'line', width: 60 },
    { title: '外部交易号', dataIndex: 'externalTransactionId', width: 170, ellipsis: true },
    { title: '订单号', dataIndex: 'orderNo', width: 160, ellipsis: true },
    { title: '交易总额', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.orderGrossMinor, row.currency) },
    { title: '平台费用', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.platformFeeMinor, row.currency) },
    { title: '结算金额', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.settlementAmountMinor, row.currency) },
    { title: '处理', dataIndex: 'disposition', width: 90, render: (value) => value === 'duplicate' ? <Tag>跳过重复</Tag> : <Tag color="processing">新增</Tag> },
  ];

  const issueColumns: TableColumnsType<SettlementValidationIssue> = [
    { title: '行', dataIndex: 'line', width: 60 },
    { title: '字段', dataIndex: 'field', width: 190, render: (value) => value || '文件' },
    { title: '问题', dataIndex: 'message' },
  ];

  const transactionColumns: TableColumnsType<SettlementTransaction> = [
    {
      title: '外部交易号',
      dataIndex: 'externalTransactionId',
      width: 190,
      ellipsis: true,
      render: (value) => <Typography.Text copyable>{value}</Typography.Text>,
    },
    { title: '订单交易总额', width: 145, align: 'right', render: (_, row) => formatProfitAmount(row.orderGrossMinor, row.currency) },
    { title: '平台费用', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.platformFeeMinor, row.currency) },
    { title: '结算金额', width: 125, align: 'right', render: (_, row) => formatProfitAmount(row.settlementAmountMinor, row.currency) },
    { title: '结算时间', dataIndex: 'settledAt', width: 180, render: (value) => formatDateTime(value) },
    { title: '导入时间', dataIndex: 'createdAt', width: 180, render: (value) => formatDateTime(value) },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.SETTLEMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-settlement-page"
        title="平台结算对账"
        subTitle="核对平台结算交易、本地订单金额与平台费用，只有一致费用进入订单预估利润"
        extra={<TmPageHeaderExtra>
          <Tooltip title={canImport ? '导入平台结算 CSV' : '当前账号无导入权限'}>
            <Button icon={<ImportOutlined />} disabled={!canImport} onClick={() => setImportOpen(true)}>导入账单</Button>
          </Tooltip>
          <Tooltip title={canExport ? '导出当前筛选结果，最多 5000 条' : '当前账号无导出权限'}>
            <Button icon={<DownloadOutlined />} disabled={!canExport} loading={exporting} onClick={() => void exportCurrent()}>导出明细</Button>
          </Tooltip>
        </TmPageHeaderExtra>}
      >
        <Alert type="info" showIcon message="平台结算交易是不可变费用事实" description="调整和冲正以新的外部交易号导入；重复交易不会重复计费，差异账单不会进入利润计算。" />
        {filterSourceError ? <Alert type="warning" showIcon message={filterSourceError} /> : null}
        {listError ? <ErrorAlert title={listError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>重新加载</Button>} /> : null}
        <TmProTable<SettlementReconciliation>
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1640 }}
          locale={{ emptyText: listError ? '结算对账数据暂不可用' : <EmptyState compact title="暂无结算对账记录" /> }}
          filterBar={<Space className="tm-settlement-toolbar" wrap>
            <Input.Search allowClear aria-label="订单号" placeholder="搜索订单号" value={orderNoInput} className="tm-settlement-filter tm-settlement-filter--search" onChange={(event) => setOrderNoInput(event.target.value)} onSearch={(value) => setUrlState({ orderNo: value.trim() || undefined, page: undefined }, { replace: true })} />
            <Select allowClear aria-label="平台" placeholder="全部平台" value={urlState.platform} className="tm-settlement-filter" options={platformOptions} onChange={(value) => updateFilter('platform', value)} />
            <Select allowClear showSearch optionFilterProp="label" aria-label="店铺" placeholder="全部店铺" value={urlState.shopId} className="tm-settlement-filter" options={shopOptions} onChange={(value) => updateFilter('shopId', value)} />
            <Select allowClear showSearch aria-label="币种" placeholder="全部币种" value={urlState.currency} className="tm-settlement-filter" options={CURRENCY_OPTIONS} onChange={(value) => updateFilter('currency', value)} />
            <Select allowClear aria-label="对账状态" placeholder="全部状态" value={urlState.status} className="tm-settlement-filter" options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.text }))} onChange={(value) => updateFilter('status', value)} />
            <RangePicker aria-label="结算日期范围" value={dateRange} className="tm-settlement-filter tm-settlement-filter--range" onChange={(value) => setUrlState({ start: value?.[0]?.startOf('day').toISOString(), end: value?.[1]?.endOf('day').toISOString(), page: undefined }, { replace: true })} />
            <Button onClick={() => { clearState(QUERY_KEYS.filter((key) => key !== 'drawer' && key !== 'id'), { replace: true }); setOrderNoInput(''); }}>重置</Button>
          </Space>}
          toolBarRender={() => []}
          pagination={{
            current: page,
            pageSize,
            showSizeChanger: true,
            onChange: (nextPage, nextPageSize) => setUrlState({ page: nextPage > 1 ? String(nextPage) : undefined, pageSize: nextPageSize !== 20 ? String(nextPageSize) : undefined }, { replace: true }),
          }}
          request={async (params) => {
            try {
              const result = await querySettlementReconciliation(queryParams(params.current, params.pageSize));
              setListError('');
              return { data: result.list || [], success: true, total: result.total };
            } catch (error) {
              setListError((error as Error)?.message || '结算对账列表加载失败');
              return { data: [], success: false, total: 0 };
            }
          }}
        />
      </TmPageContainer>

      <Modal
        title="导入平台结算账单"
        open={importOpen}
        width={960}
        destroyOnHidden
        maskClosable={!submitting}
        closable={!submitting}
        onCancel={() => { if (!submitting) { setImportOpen(false); resetImport(); } }}
        footer={<OperationToolbar className="tm-settlement-import-actions">
          <Button onClick={() => { setImportOpen(false); resetImport(); }} disabled={submitting}>取消</Button>
          {preview ? <Button onClick={invalidatePreview} disabled={submitting}>重新选择</Button> : null}
          {!preview ? <Button type="primary" icon={<FileTextOutlined />} disabled={!importShopID || !nativeFile(fileList)} loading={previewing} onClick={() => void runPreview()}>校验预览</Button> : null}
          {preview?.valid ? <Button type="primary" icon={<ImportOutlined />} loading={submitting} disabled={submitting} onClick={() => void confirmImport()}>确认导入</Button> : null}
        </OperationToolbar>}
      >
        {!preview ? <div className="tm-settlement-import-source">
          <Select showSearch optionFilterProp="label" aria-label="账单店铺" placeholder="选择账单所属店铺" value={importShopID} options={shopOptions} onChange={(value) => { setImportShopID(value); invalidatePreview(); }} />
          <Upload accept=".csv,text/csv" maxCount={1} fileList={fileList} beforeUpload={(file) => { setFileList([file]); invalidatePreview(); return false; }} onRemove={() => { setFileList([]); invalidatePreview(); }}>
            <Button icon={<UploadOutlined />}>选择 CSV</Button>
          </Upload>
          <Button type="link" icon={<DownloadOutlined />} onClick={downloadSettlementCSVTemplate}>下载 CSV 模板</Button>
        </div> : <>
          <div className="tm-settlement-import-summary">
            <Statistic title="账单行" value={preview.sourceRows} />
            <Statistic title="新增" value={preview.newRows} />
            <Statistic title="重复跳过" value={preview.duplicateRows} />
            <Statistic title="校验问题" value={preview.issues.length} valueStyle={preview.valid ? undefined : { color: '#cf1322' }} />
          </div>
          {preview.valid ? <Alert type="success" showIcon message="账单校验通过" description={`${preview.shopName} · ${platformDisplayLabel(preview.platform)}`} /> : <Alert type="error" showIcon message="账单未通过校验" />}
          {preview.issues.length ? <Table rowKey={(row) => `${row.line}-${row.field || ''}-${row.code}`} size="small" columns={issueColumns} dataSource={preview.issues} pagination={{ pageSize: 5 }} scroll={{ x: 640 }} /> : <Table rowKey={(row) => `${row.line}-${row.externalTransactionId}`} size="small" columns={previewColumns} dataSource={preview.rows} pagination={{ pageSize: 5 }} scroll={{ x: 900 }} />}
        </>}
      </Modal>

      <AppDrawer title="结算对账详情" open={drawer.open} onClose={drawer.closeDrawer} loading={detailLoading} width={920}>
        {detailError ? <ErrorAlert title={detailError} actionHint={drawer.id ? <Button icon={<ReloadOutlined />} onClick={() => void loadDetail(drawer.id!)}>重新加载</Button> : undefined} /> : null}
        {detail ? <Space direction="vertical" size="large" className="tm-settlement-detail">
          <Descriptions column={{ xs: 1, sm: 2 }} bordered size="small">
            <Descriptions.Item label="订单号">{detail.orderNo}</Descriptions.Item>
            <Descriptions.Item label="对账状态">{statusTag(detail.status)}</Descriptions.Item>
            <Descriptions.Item label="平台 / 店铺">{platformDisplayLabel(detail.platform)} · {detail.shopName}</Descriptions.Item>
            <Descriptions.Item label="币种">{detail.currency || '多币种'}</Descriptions.Item>
            <Descriptions.Item label="本地订单金额">{formatProfitAmount(detail.orderAmountMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label="账单交易总额">{formatProfitAmount(detail.orderGrossMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label="平台费用">{formatProfitAmount(detail.platformFeeMinor, detail.currency)}</Descriptions.Item>
            <Descriptions.Item label="结算金额">{formatProfitAmount(detail.settlementAmountMinor, detail.currency)}</Descriptions.Item>
          </Descriptions>
          {detail.issues.length ? <Alert type={detail.status === 'blocked' ? 'error' : 'warning'} showIcon message="对账差异" description={detail.issues.map((issue) => issue.message).join('；')} /> : null}
          <OperationToolbar>
            {detail.orderId ? <Button icon={<LinkOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(detail.orderId!)}`)}>查看订单</Button> : null}
            {detail.orderId ? <Button icon={<LinkOutlined />} onClick={() => history.push(`/finance/order-profits?drawer=order-profit&id=${encodeURIComponent(detail.orderId!)}`)}>查看预估利润</Button> : null}
          </OperationToolbar>
          <Table rowKey="id" columns={transactionColumns} dataSource={detail.transactions} pagination={false} scroll={{ x: 980 }} />
        </Space> : !detailLoading && !detailError ? <EmptyState compact title="暂无对账详情" /> : null}
      </AppDrawer>
    </PermissionGuard>
  );
}
