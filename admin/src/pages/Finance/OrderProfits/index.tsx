import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import {
  DownloadOutlined,
  EyeOutlined,
  LinkOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  DatePicker,
  Descriptions,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
  type TableColumnsType,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import AppDrawer from '@/components/AppDrawer';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { PLATFORM_DISPLAY_LABEL, platformDisplayLabel } from '@/constants/platformLabels';
import { usePermission } from '@/hooks/usePermission';
import { useUrlDrawerState, useUrlQueryState } from '@/hooks/useUrlState';
import { listInventoryWarehouses, type InventoryWarehouse } from '@/services/inventory';
import {
  downloadOrderProfits,
  formatMarginBps,
  formatProfitAmount,
  getOrderProfit,
  queryOrderProfits,
  type OrderProfit,
  type OrderProfitDetail,
  type ProductCostLine,
  type ProfitMoneyComponent,
  type ProfitabilityStatus,
} from '@/services/profitability';
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
  'warehouseId',
  'currency',
  'status',
  'start',
  'end',
  'drawer',
  'id',
] as const;

const STATUS_META: Record<ProfitabilityStatus, { text: string; color: string }> = {
  complete: { get text() { return tr('完整'); }, color: 'success' },
  pending: { get text() { return tr('待补充'); }, color: 'processing' },
  mismatch: { get text() { return tr('数据不一致'); }, color: 'warning' },
  blocked: { get text() { return tr('已阻断'); }, color: 'error' },
};

const COMPONENT_STATUS_META: Record<string, { text: string; color: string }> = {
  available: { get text() { return tr('已取得'); }, color: 'success' },
  missing: { get text() { return tr('缺失'); }, color: 'default' },
  pending: { get text() { return tr('待终结'); }, color: 'processing' },
  mismatch: { get text() { return tr('不一致'); }, color: 'warning' },
  blocked: { get text() { return tr('不可计算'); }, color: 'error' },
};

const COMPONENT_LABELS: Record<string, string> = {
  get revenue() { return tr('订单收入'); },
  get productCost() { return tr('商品成本'); },
  get freight() { return tr('本地运费'); },
  get refund() { return tr('成功退款'); },
  get platformFee() { return tr('平台费用'); },
  get advertisingFee() { return tr('广告费用'); },
  get warehouseFee() { return tr('仓库操作费'); },
};

const CURRENCY_OPTIONS = ['CNY', 'USD', 'EUR', 'GBP', 'JPY', 'KRW', 'SGD', 'AUD', 'CAD'].map(
  (value) => ({ value, label: value }),
);

function parseDateRange(start?: string, end?: string): [Dayjs | null, Dayjs | null] | null {
  const left = start ? dayjs(start) : null;
  const right = end ? dayjs(end) : null;
  if ((!left || left.isValid()) && (!right || right.isValid()) && (left || right)) {
    return [left, right];
  }
  return null;
}

function statusTag(status: ProfitabilityStatus) {
  const meta = STATUS_META[status] || { text: status, color: 'default' };
  return <Tag color={meta.color}>{tr(meta.text)}</Tag>;
}

function componentStatusTag(status: string) {
  const meta = COMPONENT_STATUS_META[status] || { text: status, color: 'default' };
  return <Tag color={meta.color}>{tr(meta.text)}</Tag>;
}

function componentAmount(component: ProfitMoneyComponent) {
  if (component.amountMinor != null) {
    return formatProfitAmount(component.amountMinor, component.currency);
  }
  if (component.knownAmountMinor !== 0) {
    return trFormat('已知 {amount}', { amount: formatProfitAmount(component.knownAmountMinor, component.currency) });
  }
  return '—';
}

function sourceLabel(source: string) {
  const labels: Record<string, string> = {
    order_total: tr('订单金额'),
    current_supplier_catalog: tr('当前供应商采购价'),
    current_supplier_catalog_estimate: tr('当前目录估算'),
    fulfillment_cost_snapshot: tr('履约成本快照'),
    confirmed_local_freight_quote: tr('已确认本地运费试算'),
    carrier_freight_ledger: tr('承运商运费实账'),
    refund_execution_ledger: tr('退款执行事实'),
    platform_settlement: tr('平台结算'),
    platform_settlement_ledger: tr('平台结算账单'),
    advertising_fee_ledger: tr('广告费用账'),
    warehouse_fee_ledger: tr('仓库操作费台账'),
  };
  return labels[source] ? tr(labels[source]) : source || '—';
}

export default function OrderProfitsPage() {
  useLanguage();
  const { can } = usePermission();
  const canExport = can(PERMISSIONS.ORDER_PROFIT_EXPORT);
  const actionRef = useRef<ActionType>();
  const initializedRef = useRef(false);
  const filterReadGuard = useMemo(createTenantReadGuard, []);
  const detailReadGuard = useMemo(createTenantReadGuard, []);
  const listReadGuard = useMemo(createTenantReadGuard, []);
  const { state: urlState, setState: setUrlState, clearState } = useUrlQueryState<
    Record<(typeof QUERY_KEYS)[number], string | undefined>
  >(QUERY_KEYS);
  const drawer = useUrlDrawerState('order-profit');
  const [orderNoInput, setOrderNoInput] = useState(urlState.orderNo || '');
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const [filterSourceError, setFilterSourceError] = useState('');
  const [listError, setListError] = useState('');
  const [detail, setDetail] = useState<OrderProfitDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [exporting, setExporting] = useState(false);

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
  const warehouseOptions = useMemo(
    () => warehouses.map((warehouse) => ({ value: warehouse.id, label: `${warehouse.code} · ${warehouse.name}` })),
    [warehouses],
  );

  useEffect(() => setOrderNoInput(urlState.orderNo || ''), [urlState.orderNo]);

  useEffect(() => {
    let active = true;
    const isCurrent = filterReadGuard.begin();
    setShops([]);
    setWarehouses([]);
    void Promise.all([queryShops({ page: 1, pageSize: 100 }), listInventoryWarehouses()])
      .then(([shopResult, warehouseResult]) => {
        if (!active || !isCurrent()) return;
        setShops(shopResult.list || []);
        setWarehouses(warehouseResult.list || []);
        setFilterSourceError('');
      })
      .catch((error) => {
        if (active && isCurrent()) setFilterSourceError((error as Error)?.message || tr('店铺或仓库筛选项加载失败'));
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
  }, [
    urlState.currency,
    urlState.end,
    urlState.orderNo,
    urlState.platform,
    urlState.shopId,
    urlState.start,
    urlState.status,
    urlState.warehouseId,
  ]);

  useEffect(() => () => listReadGuard.invalidate(), [listReadGuard]);

  const loadDetail = useCallback(async (orderId: string) => {
    const isCurrent = detailReadGuard.begin();
    setDetail(undefined);
    setDetailLoading(true);
    setDetailError('');
    try {
      const result = await getOrderProfit(orderId);
      if (isCurrent()) setDetail(result);
    } catch (error) {
      if (isCurrent()) {
        setDetail(undefined);
        setDetailError((error as Error)?.message || tr('预估利润详情加载失败'));
      }
    } finally {
      if (isCurrent()) setDetailLoading(false);
    }
  }, [detailReadGuard]);

  useEffect(() => {
    if (drawer.id) {
      void loadDetail(drawer.id);
    } else {
      setDetail(undefined);
      setDetailError('');
    }
    return () => { detailReadGuard.invalidate(); };
  }, [drawer.id, loadDetail, detailReadGuard]);

  const updateFilter = useCallback(
    (key: 'platform' | 'shopId' | 'warehouseId' | 'currency' | 'status', value?: string) => {
      setUrlState({ [key]: value || undefined, page: undefined }, { replace: true });
    },
    [setUrlState],
  );

  const queryParams = useCallback(
    (nextPage?: number, nextPageSize?: number) => ({
      page: nextPage || page,
      pageSize: nextPageSize || pageSize,
      orderNo: urlState.orderNo,
      platform: urlState.platform,
      shopId: urlState.shopId,
      warehouseId: urlState.warehouseId,
      currency: urlState.currency,
      status: urlState.status as ProfitabilityStatus | undefined,
      start: urlState.start,
      end: urlState.end,
    }),
    [page, pageSize, urlState],
  );

  const exportCurrent = useCallback(async () => {
    if (!canExport || exporting) return;
    setExporting(true);
    try {
      const { page: _page, pageSize: _pageSize, ...filters } = queryParams();
      await downloadOrderProfits(filters);
      message.success(tr('预估利润明细已导出'));
    } catch (error) {
      message.error((error as Error)?.message || tr('导出失败，请收窄筛选条件后重试'));
    } finally {
      setExporting(false);
    }
  }, [canExport, exporting, queryParams]);

  const columns: ProColumns<OrderProfit>[] = [
    {
      title: tr("订单"),
      dataIndex: 'orderNo',
      width: 190,
      fixed: 'left',
      ellipsis: true,
      copyable: true,
      render: (_, row) => (
        <Button type="link" size="small" onClick={() => drawer.openDrawer(row.orderId)}>
          {row.orderNo || row.orderId}
        </Button>
      ),
    },
    {
      title: tr('平台 / 店铺'),
      width: 190,
      ellipsis: true,
      render: (_, row) => `${platformDisplayLabel(row.platform)}${row.shopName ? ` · ${row.shopName}` : ''}`,
    },
    {
      title: tr('仓库'),
      width: 170,
      ellipsis: true,
      render: (_, row) => [row.warehouseCode, row.warehouseName].filter(Boolean).join(' · ') || tr('未绑定'),
    },
    {
      title: tr('订单收入'),
      width: 135,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.components.revenue.amountMinor, row.currency),
    },
    {
      title: tr('已知商品成本'),
      width: 145,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.components.productCost.knownAmountMinor, row.currency),
    },
    {
      title: tr('已知运费'),
      width: 125,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.components.freight.knownAmountMinor, row.currency),
    },
    {
      title: tr('成功退款'),
      width: 125,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.components.refund.knownAmountMinor, row.currency),
    },
    {
      title: tr('已知贡献额'),
      width: 140,
      align: 'right',
      render: (_, row) => (
        <Typography.Text type={(row.knownContributionMinor ?? 0) < 0 ? 'danger' : undefined}>
          {formatProfitAmount(row.knownContributionMinor, row.currency)}
        </Typography.Text>
      ),
    },
    {
      title: tr('预估利润'),
      width: 135,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.estimatedProfitMinor, row.currency),
    },
    {
      title: tr('完整性'),
      dataIndex: 'status',
      width: 120,
      render: (_, row) => statusTag(row.status),
    },
    {
      title: tr('费用缺口'),
      width: 120,
      render: (_, row) => (
        <Tooltip title={row.issues.map((issue) => issue.message).join('；')}>
          <Typography.Text type={row.issues.length ? 'warning' : 'secondary'}>
            {row.issues.length ? trFormat("{value0} 项", { value0: String(row.issues.length) }) : tr('无')}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: tr('下单时间'),
      dataIndex: 'orderedAt',
      width: 170,
      render: (_, row) => (row.orderedAt ? formatDateTime(row.orderedAt) : '—'),
    },
    {
      title: tr('操作'),
      valueType: 'option',
      width: 88,
      fixed: 'right',
      render: (_, row) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.orderId)}>
          {tr('查看')}
        </Button>
      ),
    },
  ];

  const costColumns: TableColumnsType<ProductCostLine> = [
    { title: tr("商品"), dataIndex: 'productTitle', width: 210, ellipsis: true },
    { title: tr('规格编码'), dataIndex: 'skuCode', width: 140, ellipsis: true, render: (value) => value || '—' },
    { title: tr('数量'), dataIndex: 'quantity', width: 72, align: 'right' },
    {
      title: tr('成本单价'),
      width: 150,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.unitCostMinor, row.currency),
    },
    {
      title: tr('商品成本'),
      width: 150,
      align: 'right',
      render: (_, row) => formatProfitAmount(row.lineCostMinor, row.currency),
    },
    { title: tr('成本依据'), dataIndex: 'source', width: 150, render: (value) => sourceLabel(value) },
    { title: tr('快照时间'), dataIndex: 'capturedAt', width: 180, render: (value) => (value ? formatDateTime(value) : '—') },
    { title: tr('来源状态'), dataIndex: 'status', width: 108, render: (value) => componentStatusTag(value) },
    { title: tr('供应商'), dataIndex: 'supplierName', width: 170, ellipsis: true, render: (value) => value || '—' },
    { title: tr('缺口说明'), dataIndex: 'reason', width: 220, ellipsis: true, render: (value) => value || '—' },
  ];

  const componentRows = detail
    ? Object.entries(detail.components).map(([key, value]) => ({ key, label: COMPONENT_LABELS[key] ? tr(COMPONENT_LABELS[key]) : key, ...value }))
    : [];
  const componentColumns: TableColumnsType<(typeof componentRows)[number]> = [
    { title: tr('费用项'), dataIndex: 'label', width: 120 },
    { title: tr('来源状态'), dataIndex: 'status', width: 108, render: (value) => componentStatusTag(value) },
    { title: tr('金额'), width: 160, align: 'right', render: (_, row) => componentAmount(row) },
    { title: tr('来源'), dataIndex: 'source', width: 190, render: (value) => sourceLabel(value) },
    { title: tr('来源时间'), dataIndex: 'sourceAt', width: 180, render: (value) => (value ? formatDateTime(value) : '—') },
    { title: tr('说明'), dataIndex: 'reason', width: 240, ellipsis: true, render: (value) => value || '—' },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.ORDER_PROFIT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-order-profit-page"
        title={tr("订单预估利润")}
        subTitle={tr("已履约订单使用履约成本快照，未履约订单使用当前目录估算；缺失费用不计为零")}
        extra={
          <TmPageHeaderExtra>
            <Tooltip title={canExport ? tr('导出当前筛选结果，最多 5000 条') : tr('当前账号无导出权限')}>
              <Button
                icon={<DownloadOutlined />}
                disabled={!canExport}
                loading={exporting}
                onClick={() => void exportCurrent()}
              >
                {tr('导出明细')}
              </Button>
            </Tooltip>
          </TmPageHeaderExtra>
        }
      >
        <Alert
          type="warning"
          showIcon
          message={tr("本页为动态预估，不是会计利润")}
          description={tr("履约快照冻结的是发货当时的本地供应商目录成本，不等于会计实际成本；历史已履约订单缺少快照时不会用当前价格回填。平台、广告和仓储费用缺失时，仅展示已知贡献额。")}
        />
        {filterSourceError ? <Alert type="warning" showIcon message={filterSourceError} /> : null}
        {listError ? (
          <ErrorAlert
            title={listError}
            actionHint={<Button icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>{tr('重新加载')} </Button>}
          />
        ) : null}
        <TmProTable<OrderProfit>
          rowKey="orderId"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1840 }}
          locale={{ emptyText: listError ? tr('预估利润数据暂不可用') : tr('暂无符合条件的订单预估利润记录') }}
          filterBar={
            <Space className="tm-order-profit-toolbar" wrap>
              <Input.Search
                allowClear
                aria-label={tr("订单号")}
                placeholder={tr("搜索订单号")}
                value={orderNoInput}
                className="tm-order-profit-filter tm-order-profit-filter--search"
                onChange={(event) => setOrderNoInput(event.target.value)}
                onSearch={(value) => setUrlState({ orderNo: value.trim() || undefined, page: undefined }, { replace: true })}
              />
              <Select
                allowClear
                aria-label={tr("平台")}
                placeholder={tr("全部平台")}
                value={urlState.platform}
                className="tm-order-profit-filter"
                options={platformOptions}
                onChange={(value) => updateFilter('platform', value)}
              />
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                aria-label={tr("店铺")}
                placeholder={tr("全部店铺")}
                value={urlState.shopId}
                className="tm-order-profit-filter"
                options={shopOptions}
                onChange={(value) => updateFilter('shopId', value)}
              />
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                aria-label={tr("仓库")}
                placeholder={tr("全部仓库")}
                value={urlState.warehouseId}
                className="tm-order-profit-filter"
                options={warehouseOptions}
                onChange={(value) => updateFilter('warehouseId', value)}
              />
              <Select
                allowClear
                showSearch
                aria-label={tr("币种")}
                placeholder={tr("全部币种")}
                value={urlState.currency}
                className="tm-order-profit-filter"
                options={CURRENCY_OPTIONS}
                onChange={(value) => updateFilter('currency', value)}
              />
              <Select
                allowClear
                aria-label={tr("完整性")}
                placeholder={tr("全部完整性")}
                value={urlState.status}
                className="tm-order-profit-filter"
                options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.text }))}
                onChange={(value) => updateFilter('status', value)}
              />
              <RangePicker
                aria-label={tr("下单日期范围")}
                value={dateRange}
                className="tm-order-profit-filter tm-order-profit-filter--range"
                onChange={(value) =>
                  setUrlState(
                    {
                      start: value?.[0]?.startOf('day').toISOString(),
                      end: value?.[1]?.endOf('day').toISOString(),
                      page: undefined,
                    },
                    { replace: true },
                  )
                }
              />
              <Button
                onClick={() => {
                  clearState(QUERY_KEYS.filter((key) => key !== 'drawer' && key !== 'id'), { replace: true });
                  setOrderNoInput('');
                }}
              >
                {tr('重置')}
              </Button>
            </Space>
          }
          toolBarRender={() => []}
          pagination={{
            current: page,
            pageSize,
            showSizeChanger: true,
            onChange: (nextPage, nextPageSize) =>
              setUrlState(
                {
                  page: nextPage > 1 ? String(nextPage) : undefined,
                  pageSize: nextPageSize !== 20 ? String(nextPageSize) : undefined,
                },
                { replace: true },
              ),
          }}
          request={async (params) => {
            const isCurrent = listReadGuard.begin();
            try {
              const result = await queryOrderProfits(queryParams(params.current, params.pageSize));
              if (!isCurrent()) return { data: [], success: false, total: 0 };
              setListError('');
              return { data: result.list || [], success: true, total: result.total || 0 };
            } catch (error) {
              if (isCurrent()) setListError((error as Error)?.message || tr('预估利润列表加载失败'));
              return { data: [], success: false, total: 0 };
            }
          }}
        />

        <AppDrawer
          title={detail?.orderNo ? trFormat("订单 {value0} · 预估利润", { value0: String(detail.orderNo) }) : tr('订单预估利润详情')}
          open={drawer.open}
          onClose={drawer.closeDrawer}
          loading={detailLoading}
          extra={
            drawer.id ? (
              <Button icon={<EyeOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(drawer.id!)}`)}>
                {tr('订单详情')}
              </Button>
            ) : null
          }
        >
          {detailError ? <ErrorAlert title={detailError} /> : null}
          {detail ? (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                <Descriptions.Item label={tr("完整性")}>{statusTag(detail.status)}</Descriptions.Item>
                <Descriptions.Item label={tr("计算版本")}>{detail.formulaVersion}</Descriptions.Item>
                <Descriptions.Item label={tr("订单收入")}>{formatProfitAmount(detail.components.revenue.amountMinor, detail.currency)}</Descriptions.Item>
                <Descriptions.Item label={tr("已知贡献额")}>{formatProfitAmount(detail.knownContributionMinor, detail.currency)}</Descriptions.Item>
                <Descriptions.Item label={tr("预估利润")}>{formatProfitAmount(detail.estimatedProfitMinor, detail.currency)}</Descriptions.Item>
                <Descriptions.Item label={tr("预估利润率")}>{formatMarginBps(detail.estimatedMarginBps)}</Descriptions.Item>
                <Descriptions.Item label={tr("计算时间")}>{formatDateTime(detail.calculatedAt)}</Descriptions.Item>
                <Descriptions.Item label={tr("下单时间")}>{detail.orderedAt ? formatDateTime(detail.orderedAt) : '—'}</Descriptions.Item>
              </Descriptions>
              {detail.issues.length ? (
                <Alert
                  type={detail.status === 'blocked' ? 'error' : detail.status === 'mismatch' ? 'warning' : 'info'}
                  showIcon
                  message={tr("费用口径尚未完整")}
                  description={detail.issues.map((issue) => `${COMPONENT_LABELS[issue.component] || issue.component}：${issue.message}`).join('；')}
                />
              ) : null}
              <Table
                rowKey="key"
                size="small"
                pagination={false}
                scroll={{ x: 1000 }}
                columns={componentColumns}
                dataSource={componentRows}
              />
              <Typography.Title level={5}>{tr('商品成本明细')} </Typography.Title>
              <Table<ProductCostLine>
                rowKey="orderItemId"
                size="small"
                pagination={false}
                scroll={{ x: 1510 }}
                columns={costColumns}
                dataSource={detail.productCostLines || []}
                locale={{ emptyText: tr('暂无订单商品明细') }}
              />
              <Space wrap>
                <Button icon={<LinkOutlined />} onClick={() => history.push(`/orders/${encodeURIComponent(detail.orderId)}`)}>
                  {tr('订单')}
                </Button>
                {detail.related.fulfillmentWaveId ? (
                  <Button
                    icon={<LinkOutlined />}
                    onClick={() => history.push(`/orders/fulfillment-waves?drawer=fulfillment-wave&id=${encodeURIComponent(detail.related.fulfillmentWaveId!)}`)}
                  >
                    {tr('履约波次')}
                  </Button>
                ) : null}
                {detail.productCostLines
                  .filter((line) => line.supplierId)
                  .filter((line, index, rows) => rows.findIndex((candidate) => candidate.supplierId === line.supplierId) === index)
                  .map((line) => (
                    <Button
                      key={line.supplierId}
                      icon={<LinkOutlined />}
                      onClick={() => history.push(`/procurement/suppliers?keyword=${encodeURIComponent(line.supplierName || '')}`)}
                    >
                      {line.supplierName || tr('供应商')}
                    </Button>
                  ))}
                {detail.related.refundExecutionIds.map((id, index) => (
                  <Button key={id} icon={<LinkOutlined />} onClick={() => history.push(`/orders/refund-executions/${encodeURIComponent(id)}`)}>
                    {tr('退款执行')} {index + 1}
                  </Button>
                ))}
                {detail.related.advertisingFeeAllocationId ? (
                  <Button
                    icon={<LinkOutlined />}
                    onClick={() => history.push(`/finance/advertising-fees?drawer=advertising-fee&id=${encodeURIComponent(detail.related.advertisingFeeAllocationId!)}`)}
                  >
                    {tr('广告费用归属')}
                  </Button>
                ) : null}
                {detail.related.warehouseFeeSnapshotId ? (
                  <Button
                    icon={<LinkOutlined />}
                    onClick={() => history.push(`/finance/warehouse-fees?drawer=warehouse-fee&id=${encodeURIComponent(detail.related.warehouseFeeSnapshotId!)}`)}
                  >
                    {tr('仓库操作费')}
                  </Button>
                ) : null}
                {detail.related.freightFeeChargeIds.map((id, index) => (
                  <Button
                    key={id}
                    icon={<LinkOutlined />}
                    onClick={() => history.push(`/finance/freight-fees?drawer=freight-fee&id=${encodeURIComponent(id)}`)}
                  >
                    {tr('承运商运费账单')} {index + 1}
                  </Button>
                ))}
                {detail.related.freightShipmentCount > 0 && detail.related.freightBilledShipmentCount < detail.related.freightShipmentCount ? (
                  <Typography.Text type="warning">
                    {tr('运费账单已覆盖')} {detail.related.freightBilledShipmentCount}/{detail.related.freightShipmentCount} {tr('个包裹；当前利润仍使用本地估价。')}
                  </Typography.Text>
                ) : null}
              </Space>
            </Space>
          ) : null}
        </AppDrawer>
      </TmPageContainer>
    </PermissionGuard>
  );
}
