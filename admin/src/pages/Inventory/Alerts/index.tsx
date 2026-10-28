import { tr, trFormat } from '@/i18n';
import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import {
  Button,
  Checkbox,
  Descriptions,
  Form,
  InputNumber,
  Modal,
  Popover,
  Radio,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import { history } from '@umijs/max';
import { Link } from '@umijs/renderer-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import { usePermission } from '@/hooks/usePermission';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { parsePositiveInt } from '@/utils/urlState';
import { PERMISSIONS } from '@/utils/permission';

const ALERT_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'alertType',
  'stockStatus',
  'platform',
  'shopId',
  'source',
] as const;
import { INVENTORY_SKU_NOT_BOUND_MESSAGE } from '@/constants/inventoryLabels';
import {
  INVENTORY_SYNC_BATCHES_LABEL,
  INVENTORY_SYNC_TASKS_LABEL,
} from '@/constants/userFriendly';
import { confirmInventoryManualAdjust, confirmInventorySync } from '@/constants/sensitiveActions';
import {
  adjustSkuStock,
  batchUpdateStockSettings,
  createInventoryIdempotencyKey,
  createInventorySyncBatch,
  listInventoryWarehouses,
  listSkuWarehouseBalances,
  previewBatchStockSettings,
  queryInventoryAlerts,
  syncPublicationSkuInventory,
  type BatchStockSettingsPreviewPayload,
  type InventoryAlertRow,
  type InventoryWarehouse,
  type WarehouseBalance,
} from '@/services/inventory';
import { updateProductSkuStockSettings } from '@/services/products';

const BATCH_STOCK_DEFAULT_MAX = 500;

function alertsStockBatchNeedsConfirmAll(p: BatchStockSettingsPreviewPayload): boolean {
  if (p.productSkuIds?.length) return false;
  if ((p.productId ?? '').trim()) return false;
  if ((p.platform ?? '').trim()) return false;
  if ((p.shopId ?? '').trim()) return false;
  if ((p.keyword ?? '').trim()) return false;
  if ((p.stockStatus ?? '').trim()) return false;
  if (p.onlyPublished) return false;
  if (p.alertTypes?.length) return false;
  if (!p.includeNormal) return false;
  return true;
}

const ALERT_TYPE_LABEL: Record<string, string> = {
  out_of_stock: '售罄',
  low_stock: '低库存',
  below_safety_stock: '低于安全线',
  platform_stock_mismatch: '平台不一致',
  platform_stock_unknown: '平台库存未知',
  inventory_sync_failed: '同步失败',
};

const STOCK_STATUS_LABEL: Record<string, { source: string; color: string }> = {
  normal: { source: "正常", color: 'green' },
  low_stock: { source: "低库存", color: 'orange' },
  out_of_stock: { source: "售罄", color: 'red' },
  below_safety_stock: { source: "低于安全线", color: 'gold' },
};

const PLATFORM_ST_LABEL: Record<string, { source: string; color: string }> = {
  platform_stock_synced: { source: "一致", color: 'green' },
  platform_stock_mismatch: { source: "不一致", color: 'orange' },
  platform_stock_unknown: { source: "未知", color: 'default' },
};

function stockStatusTag(raw: string) {
  const m = STOCK_STATUS_LABEL[raw];
  if (!m) return <Tag>{raw}</Tag>;
  return <Tag color={m.color}>{tr(m.source)}</Tag>;
}

export default function InventoryAlertsPage() {
  const emptyLocale = useListEmptyLocale('inventoryAlerts', { permissionScoped: true });
  const { can, readonly } = usePermission();
  const canAdjustInventory = can(PERMISSIONS.INVENTORY_OPERATE) && !readonly;
  const actionRef = useRef<ActionType>();
  const searchFormRef = useRef<ProFormInstance>();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof ALERT_QUERY_KEYS)[number], string | undefined>>(ALERT_QUERY_KEYS);
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef: searchFormRef,
    actionRef,
    setTablePage,
  });
  const [selectedSkuIds, setSelectedSkuIds] = useState<string[]>([]);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkIncludeLocalAlerts, setBulkIncludeLocalAlerts] = useState(false);
  const [bulkSubmitting, setBulkSubmitting] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [active, setActive] = useState<InventoryAlertRow | null>(null);
  const [adjustSubmitting, setAdjustSubmitting] = useState(false);
  const [adjustLoading, setAdjustLoading] = useState(false);
  const [adjustWarehouses, setAdjustWarehouses] = useState<InventoryWarehouse[]>([]);
  const [adjustBalances, setAdjustBalances] = useState<WarehouseBalance[]>([]);
  const [adjustIdempotencyKey, setAdjustIdempotencyKey] = useState('');
  const adjustLoadSequence = useRef(0);
  const [settingsSubmitting, setSettingsSubmitting] = useState(false);
  const [adjustForm] = Form.useForm<{ warehouseId: string; stock: number }>();
  const [settingsForm] = Form.useForm<{ warningStock: number; safetyStock: number }>();
  const [batchStockOpen, setBatchStockOpen] = useState(false);
  const [batchStockScope, setBatchStockScope] = useState<'selected' | 'filter'>('selected');
  const [batchMatched, setBatchMatched] = useState<number | null>(null);
  const [batchPreviewLoading, setBatchPreviewLoading] = useState(false);
  const [batchStockSubmitting, setBatchStockSubmitting] = useState(false);
  const [batchStockForm] = Form.useForm<{ warningStock: number; safetyStock: number }>();

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    searchFormRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      alertType: urlState.alertType,
      stockStatus: urlState.stockStatus,
      platform: urlState.platform,
      shopId: urlState.shopId,
    });
  }, [
    urlState.alertType,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.shopId,
    urlState.stockStatus,
  ]);

  const buildStockBatchPayload = (): BatchStockSettingsPreviewPayload => {
    const fv = searchFormRef.current?.getFieldsValue?.() ?? {};
    const alertType = typeof fv.alertType === 'string' ? fv.alertType.trim() : '';
    const alertTypes = alertType ? [alertType] : [];
    const base = {
      keyword: (fv.keyword as string)?.trim() || undefined,
      platform: (fv.platform as string)?.trim().toLowerCase() || undefined,
      shopId: (fv.shopId as string)?.trim() || undefined,
      stockStatus: (fv.stockStatus as string)?.trim() || undefined,
      onlyPublished: Boolean(fv.onlyPublished),
      includeNormal: Boolean(fv.includeNormal),
      alertTypes: alertTypes.length ? alertTypes : undefined,
    };
    if (batchStockScope === 'selected' && selectedSkuIds.length > 0) {
      return { ...base, productSkuIds: selectedSkuIds };
    }
    return base;
  };

  const runBatchPreview = async () => {
    setBatchPreviewLoading(true);
    try {
      const raw = buildStockBatchPayload();
      const res = await previewBatchStockSettings({
        ...raw,
        page: 1,
        pageSize: 10,
      });
      setBatchMatched(res.matchedCount);
    } catch (e) {
      setBatchMatched(null);
      message.error((e as Error)?.message || tr('预览失败'));
    } finally {
      setBatchPreviewLoading(false);
    }
  };

  useEffect(() => {
    if (!batchStockOpen) return;
    void runBatchPreview();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [batchStockOpen, batchStockScope]);

  const columns: ProColumns<InventoryAlertRow>[] = useMemo(
    () => [
      {
        title: tr("关键词"),
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: tr("标题 / 规格编码 / 名称"), ...keywordFieldProps },
      },
      {
        title: tr("平台"),
        dataIndex: 'platform',
        hideInTable: true,
        fieldProps: { placeholder: 'tiktok / shopee / lazada / amazon' },
      },
      {
        title: tr("店铺 ID"),
        dataIndex: 'shopId',
        hideInTable: true,
      },
      {
        title: tr("预警类型"),
        dataIndex: 'alertType',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          out_of_stock: { text: tr("售罄") },
          low_stock: { text: tr("低库存") },
          below_safety_stock: { text: tr("低于安全线") },
          platform_stock_mismatch: { text: tr("平台不一致") },
          platform_stock_unknown: { text: tr("平台未知") },
          inventory_sync_failed: { text: tr("同步失败") },
        },
      },
      {
        title: tr("库存状态"),
        dataIndex: 'stockStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          normal: { text: tr("正常") },
          low_stock: { text: tr("低库存") },
          out_of_stock: { text: tr("售罄") },
          below_safety_stock: { text: tr("低于安全线") },
        },
      },
      {
        title: tr("仅已刊登"),
        dataIndex: 'onlyPublished',
        hideInTable: true,
        renderFormItem: () => <Switch checkedChildren={tr("是")} unCheckedChildren={tr("否")} />,
      },
      {
        title: tr("含正常"),
        dataIndex: 'includeNormal',
        hideInTable: true,
        renderFormItem: () => <Switch checkedChildren={tr("是")} unCheckedChildren={tr("否")} />,
      },
      {
        title: tr("商品"),
        dataIndex: 'productTitle',
        width: 180,
        search: false,
        ellipsis: true,
        render: (_, r) => (
          <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{r.productTitle || '—'}</Link>
        ),
      },
      {
        title: tr("规格编号"),
        dataIndex: 'skuCode',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => (
          <Typography.Text>
            {r.skuCode || '—'}
            <Typography.Paragraph type="secondary" style={{ margin: 0, fontSize: 12 }} ellipsis>
              {r.skuName || r.productSkuId}
            </Typography.Paragraph>
          </Typography.Text>
        ),
      },
      {
        title: tr("本地库存"),
        dataIndex: 'stock',
        width: 88,
        search: false,
      },
      {
        title: tr("预警线"),
        dataIndex: 'warningStock',
        width: 72,
        search: false,
      },
      {
        title: tr("安全线"),
        dataIndex: 'safetyStock',
        width: 72,
        search: false,
      },
      {
        title: tr("库存状态"),
        dataIndex: 'stockStatus',
        width: 108,
        search: false,
        render: (_, r) => stockStatusTag(r.stockStatus),
      },
      {
        title: tr("预警"),
        dataIndex: 'alertTypes',
        width: 220,
        search: false,
        render: (_, r) => (
          <Space wrap size={[0, 4]}>
            {(r.alertTypes ?? []).map((t) => (
              <Tag
                key={t}
                color={t.includes('failed') ? 'red' : t.includes('mismatch') ? 'orange' : undefined}
              >
                        {tr(ALERT_TYPE_LABEL[t] || t)}
              </Tag>
            ))}
          </Space>
        ),
      },
      {
        title: tr("平台库存"),
        dataIndex: 'platformStocks',
        width: 260,
        search: false,
        render: (_, r) => {
          if (!r.platformStocks?.length) return <Typography.Text type="secondary">—</Typography.Text>;
          return (
            <Popover
              title={tr("刊登规格明细")}
              content={
                <div style={{ maxWidth: 420 }}>
                  {r.platformStocks.map((p) => {
                    const st = PLATFORM_ST_LABEL[p.platformStockStatus] || {
                      source: p.platformStockStatus,
                      color: 'default',
                    };
                    return (
                      <Descriptions key={p.publicationSkuId} size="small" column={1} style={{ marginBottom: 12 }}>
                        <Descriptions.Item label={tr("店铺")}>{p.shopName || p.shopId}</Descriptions.Item>
                        <Descriptions.Item label={tr("平台")}>{p.platform}</Descriptions.Item>
                        <Descriptions.Item label={tr("平台库存")}>
                          {typeof p.platformStock === 'number' ? p.platformStock : '—'}
                        </Descriptions.Item>
                        <Descriptions.Item label={tr("状态")}>
                          <Tag color={st.color}>{tr(st.source)}</Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label={tr("最近任务")}>{p.lastSyncStatus || '—'}</Descriptions.Item>
                      </Descriptions>
                    );
                  })}
                </div>
              }
            >
              <Space wrap size={[0, 4]}>
                {r.platformStocks.slice(0, 3).map((p) => {
                  const st = PLATFORM_ST_LABEL[p.platformStockStatus] || {
                    source: p.platformStockStatus,
                    color: 'default',
                  };
                  return (
                    <Tag key={p.publicationSkuId} color={st.color}>
                      {p.platform}:{typeof p.platformStock === 'number' ? p.platformStock : '?'}
                    </Tag>
                  );
                })}
                {r.platformStocks.length > 3 ? <Tag>+{r.platformStocks.length - 3}</Tag> : null}
              </Space>
            </Popover>
          );
        },
      },
      {
        title: tr("最近同步"),
        dataIndex: 'lastSyncStatus',
        width: 108,
        search: false,
        render: (_, r) => {
          if (!r.lastSyncStatus) return '—';
          const fail = r.lastSyncStatus === 'failed';
          return <Tag color={fail ? 'red' : 'default'}>{r.lastSyncStatus}</Tag>;
        },
      },
      {
        title: tr("最近库存变更"),
        dataIndex: 'lastInventoryChangeAt',
        width: 156,
        search: false,
        render: (_, r) =>
          r.lastInventoryChangeAt ? formatDateTime(r.lastInventoryChangeAt) : '—',
      },
      {
        title: tr("操作"),
        valueType: 'option' as const,
        width: 280,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size="small">
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{tr("商品")}</Link>
            <Button
              type="link"
              size="small"
              disabled={!canAdjustInventory}
              style={{ padding: 0 }}
              onClick={() => {
                setActive(r);
                setAdjustOpen(true);
                setAdjustIdempotencyKey(createInventoryIdempotencyKey('manual-adjust'));
                setAdjustLoading(true);
                const sequence = ++adjustLoadSequence.current;
                void Promise.all([listInventoryWarehouses(), listSkuWarehouseBalances(r.productId, r.productSkuId)])
                  .then(([warehouseResult, balanceResult]) => {
                    if (sequence !== adjustLoadSequence.current) return;
                    const warehouses = (warehouseResult.list ?? []).filter((item) => item.status === 'active');
                    const balances = balanceResult.list ?? [];
                    setAdjustWarehouses(warehouses);
                    setAdjustBalances(balances);
                    const selected = warehouses.find((item) => item.isDefault) ?? warehouses[0];
                    const current = balances.find((item) => item.warehouseId === selected?.id);
                    adjustForm.setFieldsValue({
                      warehouseId: selected?.id,
                      stock: current?.onHand ?? (balances.length === 0 && selected?.isDefault ? r.stock : 0),
                    });
                  })
                  .catch((error: unknown) => {
                    if (sequence !== adjustLoadSequence.current) return;
                    message.error((error as Error)?.message || tr('仓库库存加载失败'));
                    setAdjustWarehouses([]);
                    setAdjustBalances([]);
                  })
                  .finally(() => {
                    if (sequence === adjustLoadSequence.current) setAdjustLoading(false);
                  });
              }}
            >
              {tr("调整库存")}
            </Button>
            <Button
              type="link"
              size="small"
              style={{ padding: 0 }}
              onClick={() => {
                setActive(r);
                settingsForm.setFieldsValue({ warningStock: r.warningStock, safetyStock: r.safetyStock });
                setSettingsOpen(true);
              }}
            >
              {tr("预警线")}
            </Button>
            <Link
              to={`/inventory/logs?productId=${encodeURIComponent(r.productId)}&productSkuId=${encodeURIComponent(r.productSkuId)}`}
            >
              {tr("流水")}
            </Link>
            <Link to="/inventory/sync-tasks">{tr("同步任务")}</Link>
          </Space>
        ),
      },
    ],
    [adjustForm, canAdjustInventory, settingsForm, keywordFieldProps],
  );

  return (
    <TmPageContainer title={tr("库存预警")} subTitle={tr("查看低库存、缺货与平台库存不一致等预警，可按需创建同步任务。")}>
      <KeywordSafetyHint visible={showSensitiveHint} />
      <Typography.Paragraph type="secondary">
        {tr("本页仅查询库存并提供提醒，不会自动修改平台库存。平台库存推送请从")}{' '}
        <Link to="/inventory/sync-tasks">{INVENTORY_SYNC_TASKS_LABEL}</Link>
        {tr("发起；也可勾选规格行后批量创建同步批次：")}{' '}
        <Link to="/inventory/sync-batches">{INVENTORY_SYNC_BATCHES_LABEL}</Link>。{INVENTORY_SKU_NOT_BOUND_MESSAGE}
      </Typography.Paragraph>
      <ProTable<InventoryAlertRow>
        rowKey={(r) => r.productSkuId}
        actionRef={actionRef}
        formRef={searchFormRef}
        columns={columns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 100 }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(ALERT_QUERY_KEYS, { replace: true });
        }}
        pagination={{
          current: tablePage,
          pageSize: tablePageSize,
          showSizeChanger: true,
          onChange: (page, pageSize) => {
            setTablePage(page);
            setTablePageSize(pageSize);
            setUrlState({
              page: page > 1 ? page : undefined,
              pageSize: pageSize !== 20 ? pageSize : undefined,
            });
          },
        }}
        rowSelection={{
          selectedRowKeys: selectedSkuIds,
          onChange: (keys) => setSelectedSkuIds(keys.map(String)),
        }}
        tableAlertRender={false}
        toolBarRender={() => [
          <Button
            key="batch-stock"
            onClick={() => {
              const scope = selectedSkuIds.length ? 'selected' : 'filter';
              setBatchStockScope(scope);
              batchStockForm.setFieldsValue({ warningStock: 10, safetyStock: 2 });
              setBatchStockOpen(true);
            }}
          >
            {tr("批量设置预警线")}
          </Button>,
          <Button
            key="bulk-sync"
            type="primary"
            disabled={selectedSkuIds.length === 0}
            onClick={() => {
              setBulkIncludeLocalAlerts(false);
              setBulkOpen(true);
            }}
          >
            {tr("批量同步库存")}
          </Button>,
        ]}
        expandable={{
          expandedRowRender: (r) =>
            r.platformStocks?.length ? (
              <Space direction="vertical" style={{ width: '100%' }}>
                {r.platformStocks.map((p) => {
                  const st = PLATFORM_ST_LABEL[p.platformStockStatus] || {
                    source: p.platformStockStatus,
                    color: 'default',
                  };
                  const runnable =
                    (p.platform || '').toLowerCase() === 'tiktok' ||
                    (p.platform || '').toLowerCase() === 'shopee' ||
                    (p.platform || '').toLowerCase() === 'lazada' ||
                    (p.platform || '').toLowerCase() === 'amazon';
                  return (
                    <Space key={p.publicationSkuId} wrap align="start">
                      <Typography.Text>
                        [{p.platform}] {p.shopName || p.shopId} — {tr("平台库存")}{' '}
                        {typeof p.platformStock === 'number' ? p.platformStock : '—'}{' '}
                        <Tag color={st.color}>{tr(st.source)}</Tag>
                      </Typography.Text>
                      <Button
                        type="link"
                        size="small"
                        disabled={!runnable}
                        onClick={() => {
                          const stock = r.stock;
                          const targetLabel = `[${p.platform}] ${p.shopName || p.shopId}`;
                          confirmInventorySync(targetLabel, runnable, async () => {
                            try {
                              await syncPublicationSkuInventory(p.publicationSkuId, {
                                stock,
                                fromInventoryAlert: true,
                              });
                              message.success(tr("已创建同步任务"));
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || tr('失败'));
                            }
                          });
                        }}
                      >
                        {tr("同步库存")}
                      </Button>
                    </Space>
                  );
                })}
              </Space>
            ) : (
              <Typography.Text type="secondary">{tr("无刊登映射")}</Typography.Text>
            ),
        }}
        locale={emptyLocale}
        request={async (params, sort, filter) => {
          void sort;
          void filter;
          try {
            const qp = {
              keyword: prepareKeyword(params.keyword),
              platform: (params.platform as string | undefined)?.trim(),
              shopId: (params.shopId as string | undefined)?.trim(),
              alertType: (params.alertType as string | undefined)?.trim(),
              stockStatus: (params.stockStatus as string | undefined)?.trim(),
              page: params.current ?? tablePage,
              pageSize: params.pageSize ?? tablePageSize,
            };
            setUrlState(
              {
                page: Number(qp.page) > 1 ? qp.page : undefined,
                pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
                keyword: qp.keyword,
                alertType: qp.alertType,
                stockStatus: qp.stockStatus,
                platform: qp.platform,
                shopId: qp.shopId,
                source: urlState.source,
              },
              { replace: true },
            );
            const res = await queryInventoryAlerts({
              keyword: qp.keyword,
              platform: qp.platform,
              shopId: qp.shopId,
              alertType: qp.alertType,
              stockStatus: qp.stockStatus,
              onlyPublished: Boolean(params.onlyPublished),
              includeNormal: Boolean(params.includeNormal),
              page: qp.page,
              pageSize: qp.pageSize,
            });
            return {
              data: res.list ?? [],
              success: true,
              total: res.pagination?.total ?? 0,
            };
          } catch (e: unknown) {
            message.error((e as Error)?.message || tr('加载失败'));
            return { data: [], success: false, total: 0 };
          }
        }}
      />

      <Modal
        title={tr("批量同步库存")}
        open={bulkOpen}
        onCancel={() => setBulkOpen(false)}
        okText={tr("创建批次")}
        confirmLoading={bulkSubmitting}
        onOk={() => {
          if (selectedSkuIds.length === 0) return Promise.reject();
          return new Promise<void>((resolve, reject) => {
            confirmInventorySync(trFormat("选中的 {value0} 个规格", { value0: selectedSkuIds.length }), true, async () => {
              const fv = searchFormRef.current?.getFieldsValue?.() ?? {};
              const platformRaw = typeof fv.platform === 'string' ? fv.platform.trim().toLowerCase() : '';
              const shopRaw = typeof fv.shopId === 'string' ? fv.shopId.trim() : '';
              const alertTypes = bulkIncludeLocalAlerts
                ? [
                    'platform_stock_mismatch',
                    'inventory_sync_failed',
                    'low_stock',
                    'out_of_stock',
                    'below_safety_stock',
                  ]
                : ['platform_stock_mismatch', 'inventory_sync_failed'];
              setBulkSubmitting(true);
              try {
                const batch = await createInventorySyncBatch({
                  source: 'inventory_alert',
                  platform: platformRaw || undefined,
                  shopId: shopRaw || undefined,
                  productSkuIds: selectedSkuIds,
                  onlyAlerts: true,
                  alertTypes,
                  onlyPublished: true,
                });
                message.success(trFormat("已创建批次 {value0}（跳过 {value1}）", { value0: batch.batchNo, value1: batch.skippedCount }));
                setBulkOpen(false);
                setSelectedSkuIds([]);
                actionRef.current?.reload();
                history.push(`/inventory/sync-tasks?batchId=${encodeURIComponent(batch.id)}`);
                resolve();
              } catch (e: unknown) {
                message.error((e as Error)?.message || tr('创建失败'));
                reject(e);
              } finally {
                setBulkSubmitting(false);
              }
            });
          });
        }}
      >
        <Typography.Paragraph>
          {tr("将为选中的")} <Typography.Text strong>{selectedSkuIds.length}</Typography.Text> {tr("个规格创建库存同步批次。\r\n          默认仅包含「平台库存不一致」「同步失败」类预警对应的刊登规格；不包含单纯的本地低库存/售罄，除非你勾选下方选项。")}
        </Typography.Paragraph>
        <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
          {tr("列表筛选中的平台 / 店铺会一并传给后端以收窄刊登映射（若留空则由服务端按规格聚合）。")}
        </Typography.Paragraph>
        <Checkbox checked={bulkIncludeLocalAlerts} onChange={(e) => setBulkIncludeLocalAlerts(e.target.checked)}>
          {tr("同时包含本地低库存 / 售罄 / 低于安全线预警（将把对应刊登规格推送到平台队列）")}
        </Checkbox>
      </Modal>

      <Modal
        title={active ? `${tr("调整库存")} · ${active.skuCode}` : tr("调整库存")}
        open={adjustOpen}
        onCancel={() => {
          adjustLoadSequence.current += 1;
          setAdjustOpen(false);
          setAdjustLoading(false);
          setAdjustWarehouses([]);
          setAdjustBalances([]);
          setAdjustIdempotencyKey('');
          adjustForm.resetFields();
        }}
        okButtonProps={{
          loading: adjustSubmitting,
          disabled: !canAdjustInventory || adjustLoading || adjustWarehouses.length === 0 || !adjustIdempotencyKey,
        }}
        onOk={() => {
          if (!active) return Promise.reject();
          return adjustForm.validateFields().then((v) =>
            new Promise<void>((resolve, reject) => {
              confirmInventoryManualAdjust(
                async () => {
                  setAdjustSubmitting(true);
                  try {
                    await adjustSkuStock(active.productId, active.productSkuId, {
                      warehouseId: v.warehouseId,
                      stock: v.stock,
                      idempotencyKey: adjustIdempotencyKey,
                      reason: 'manual_adjust',
                      remark: 'from_inventory_alerts',
                    });
                    message.success(tr("已更新"));
                    adjustLoadSequence.current += 1;
                    setAdjustOpen(false);
                    setAdjustWarehouses([]);
                    setAdjustBalances([]);
                    setAdjustIdempotencyKey('');
                    adjustForm.resetFields();
                    actionRef.current?.reload();
                    resolve();
                  } catch (e: unknown) {
                              message.error((e as Error)?.message || tr('失败'));
                    reject(e);
                  } finally {
                    setAdjustSubmitting(false);
                  }
                },
                resolve,
              );
            }),
          );
        }}
      >
        <Typography.Paragraph type="secondary">
          {tr("调整值只覆盖所选仓库的在手库存；尚未迁移的历史库存会先进入默认仓，没有默认仓时进入待分配仓。")}
        </Typography.Paragraph>
        <Form form={adjustForm} layout="vertical">
          <Form.Item name="warehouseId" label={tr("仓库")} rules={[{ required: true, message: tr("请选择仓库") }]}>
            <Select
              loading={adjustLoading}
              placeholder={tr("请选择启用的仓库")}
              options={adjustWarehouses.map((item) => ({
                value: item.id,
                label: trFormat("{value0}（{value1}）{value2}", { value0: item.name, value1: item.code, value2: item.isDefault ? tr(" · 默认") : '' }),
              }))}
              onChange={(warehouseId) => {
                const balance = adjustBalances.find((item) => item.warehouseId === warehouseId);
                const warehouse = adjustWarehouses.find((item) => item.id === warehouseId);
                adjustForm.setFieldValue('stock', balance?.onHand ?? (adjustBalances.length === 0 && warehouse?.isDefault ? active?.stock ?? 0 : 0));
              }}
            />
          </Form.Item>
          <Form.Item name="stock" label={tr("库存（≥0）")} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={active ? `${tr("预警线")} · ${active.skuCode}` : tr("预警线")}
        open={settingsOpen}
        onCancel={() => setSettingsOpen(false)}
        okButtonProps={{ loading: settingsSubmitting }}
        onOk={async () => {
          if (!active) return;
          const v = await settingsForm.validateFields();
          if (v.safetyStock > v.warningStock) {
            message.error(tr("安全线不能大于预警线"));
            return;
          }
          setSettingsSubmitting(true);
          try {
            await updateProductSkuStockSettings(active.productId, active.productSkuId, {
              warningStock: v.warningStock,
              safetyStock: v.safetyStock,
            });
            message.success(tr("已保存"));
            setSettingsOpen(false);
            actionRef.current?.reload();
          } catch (e: unknown) {
            message.error((e as Error)?.message || tr('失败'));
          } finally {
            setSettingsSubmitting(false);
          }
        }}
      >
        <Form form={settingsForm} layout="vertical">
          <Form.Item name="warningStock" label={tr("预警库存线")} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="safetyStock" label={tr("安全库存线")} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={tr("批量设置预警线")}
        open={batchStockOpen}
        onCancel={() => {
          setBatchStockOpen(false);
          setBatchMatched(null);
        }}
        okText={tr("应用")}
        confirmLoading={batchStockSubmitting}
        onOk={() => {
          return batchStockForm
            .validateFields()
            .then((v) => {
              if (v.safetyStock > v.warningStock) {
                message.error(tr("安全线不能大于预警线"));
                return Promise.reject(new Error('validation'));
              }
              if (batchStockScope === 'selected' && selectedSkuIds.length === 0) {
                message.error(tr("请先勾选规格，或改用「当前筛选结果」"));
                return Promise.reject(new Error('validation'));
              }
              const payload = buildStockBatchPayload();
              const needAll = alertsStockBatchNeedsConfirmAll(payload);
              return new Promise<void>((resolve, reject) => {
                Modal.confirm({
                  title: tr("确认仅修改预警线？"),
                  width: 520,
                  content: (
                    <div>
                      <Typography.Paragraph>
                        {trFormat("将修改 {value0} 个规格的预警线和安全线；不会调整本地实际库存、写入库存流水或创建平台同步任务。", { value0: batchMatched ?? '—' })}
                      </Typography.Paragraph>
                      {needAll ? (
                        <Typography.Paragraph type="warning">
                          {tr("当前为「含正常规格的全表筛选」，将按全表确认提交。")}
                        </Typography.Paragraph>
                      ) : null}
                      {(batchMatched ?? 0) > BATCH_STOCK_DEFAULT_MAX ? (
                        <Typography.Paragraph type="warning">
                          {trFormat("匹配数量超过默认单次上限 {value0}，提交时需额外确认大批量操作。", { value0: BATCH_STOCK_DEFAULT_MAX })}
                        </Typography.Paragraph>
                      ) : null}
                    </div>
                  ),
                  okText: tr("确认应用"),
                  onOk: async () => {
                    setBatchStockSubmitting(true);
                    try {
                      const raw = buildStockBatchPayload();
                      await batchUpdateStockSettings({
                        ...raw,
                        warningStock: v.warningStock,
                        safetyStock: v.safetyStock,
                        confirm: true,
                        confirmLarge: (batchMatched ?? 0) > BATCH_STOCK_DEFAULT_MAX,
                        confirmAll: needAll,
                      });
                      message.success(tr("已批量更新预警线"));
                      setBatchStockOpen(false);
                      setBatchMatched(null);
                      setSelectedSkuIds([]);
                      actionRef.current?.reload();
                      resolve();
                    } catch (e) {
                      message.error((e as Error)?.message || tr('失败'));
                      reject(e);
                    } finally {
                      setBatchStockSubmitting(false);
                    }
                  },
                });
              });
            })
            .catch((e: unknown) => {
              if ((e as Error)?.message === 'validation') return;
              throw e;
            });
        }}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {tr("仅更新 product_skus.warning_stock / safety_stock，并重新计算 stock_status。")}
        </Typography.Paragraph>
        <Form form={batchStockForm} layout="vertical" initialValues={{ warningStock: 10, safetyStock: 2 }}>
          <Form.Item label={tr("应用范围")}>
            <Radio.Group
              value={batchStockScope}
              onChange={(e) => setBatchStockScope(e.target.value as 'selected' | 'filter')}
            >
              <Radio value="selected" disabled={selectedSkuIds.length === 0}>
                {trFormat("仅选中行（{value0}）", { value0: selectedSkuIds.length })}
              </Radio>
              <Radio value="filter">{tr("当前筛选结果（含列表搜索条件）")}</Radio>
            </Radio.Group>
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
            {tr("影响数量：")}{batchPreviewLoading ? tr("计算中…") : batchMatched !== null ? trFormat("{value0} 个规格", { value0: String(batchMatched) }) : '—'}
          </Typography.Paragraph>
          <Form.Item name="warningStock" label={tr("预警库存线")} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="safetyStock" label={tr("安全库存线")} rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Button type="link" size="small" onClick={() => void runBatchPreview()} loading={batchPreviewLoading}>
            {tr("刷新匹配数")}
          </Button>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
