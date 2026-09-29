import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import InventorySyncDisabledBanner from '@/components/inventory/InventorySyncDisabledBanner';
import {
  INVENTORY_BIND_STATUS,
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
  INVENTORY_RECONCILIATION_STATUS,
  INVENTORY_STOCK_STATUS,
  INVENTORY_SYNC_STATUS,
  inventoryTagFromMap,
} from '@/constants/inventoryLabels';
import { INVENTORY_COPY, PRODUCT_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import {
  listInventoryWarehouses,
  queryInventoryCenter,
  type InventoryCenterRow,
} from '@/services/inventory';
import { Alert, Button, Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import { Link } from '@umijs/max';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { parsePositiveInt } from '@/utils/urlState';

const INVENTORY_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'stockStatus',
  'syncStatus',
  'skuBindStatus',
  'platform',
  'shopId',
  'warehouseId',
  'hasException',
  'productSkuId',
  'source',
  'skuId',
] as const;

function tagFrom(raw: string, map: Record<string, { text: string; color: string }>) {
  const cfg = inventoryTagFromMap(raw, map);
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

export default function InventoryCenterPage() {
  const emptyLocale = useListEmptyLocale('inventoryCenter', { permissionScoped: true });
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof INVENTORY_QUERY_KEYS)[number], string | undefined>>(
      INVENTORY_QUERY_KEYS,
    );
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const [warehouseOptions, setWarehouseOptions] = useState<{ label: string; value: string }[]>([]);
  const [warehouseOptionsError, setWarehouseOptionsError] = useState('');
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });

  const skuIdFromUrl = useMemo(() => {
    return urlState.productSkuId || urlState.skuId;
  }, [urlState.productSkuId, urlState.skuId]);

  const loadWarehouseOptions = useCallback(async () => {
    try {
      const response = await listInventoryWarehouses();
      setWarehouseOptions(
        (response.list ?? [])
          .filter((warehouse) => warehouse.status === 'active')
          .map((warehouse) => ({
            value: warehouse.id,
            label: `${warehouse.name}（${warehouse.code}）${warehouse.isDefault ? ' · 默认' : ''}`,
          })),
      );
      setWarehouseOptionsError('');
    } catch (error) {
      setWarehouseOptions([]);
      setWarehouseOptionsError((error as Error)?.message || '仓库列表加载失败');
    }
  }, []);

  useEffect(() => {
    void loadWarehouseOptions();
  }, [loadWarehouseOptions]);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      stockStatus: urlState.stockStatus,
      syncStatus: urlState.syncStatus,
      skuBindStatus: urlState.skuBindStatus,
      platform: urlState.platform,
      shopId: urlState.shopId,
      warehouseId: urlState.warehouseId,
      hasException: urlState.hasException,
      productSkuId: skuIdFromUrl,
    });
  }, [
    skuIdFromUrl,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.shopId,
    urlState.warehouseId,
    urlState.hasException,
    urlState.skuBindStatus,
    urlState.stockStatus,
    urlState.syncStatus,
  ]);

  useEffect(() => {
    if (!skuIdFromUrl) return;
    actionRef.current?.reload?.();
  }, [skuIdFromUrl]);

  const columns: ProColumns<InventoryCenterRow>[] = useMemo(
    () => [
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: '商品标题 / 规格编码 / 名称', ...keywordFieldProps },
      },
      { title: '规格 ID', dataIndex: 'productSkuId', hideInTable: true },
      { title: '店铺 ID', dataIndex: 'shopId', hideInTable: true },
      { title: '平台', dataIndex: 'platform', hideInTable: true },
      {
        title: '仓库范围',
        dataIndex: 'warehouseId',
        hideInTable: true,
        valueType: 'select',
        fieldProps: {
          allowClear: true,
          options: warehouseOptions,
          placeholder: '全部仓库',
          showSearch: true,
          optionFilterProp: 'label',
        },
      },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(INVENTORY_STOCK_STATUS).map(([k, v]) => [k, { text: v.text }])),
      },
      {
        title: INVENTORY_COPY.skuBinding,
        dataIndex: 'skuBindStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(INVENTORY_BIND_STATUS).map(([k, v]) => [k, { text: v.text }])),
      },
      {
        title: '同步状态',
        dataIndex: 'syncStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(INVENTORY_SYNC_STATUS).map(([k, v]) => [k, { text: v.text }])),
      },
      {
        title: '仅有异常',
        dataIndex: 'hasException',
        hideInTable: true,
        valueType: 'select',
        valueEnum: { true: { text: '是' }, false: { text: '否' } },
      },
      {
        title: '库存范围',
        dataIndex: 'inventoryScope',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, row) => row.inventoryScope === 'warehouse'
          ? `${row.warehouseName || '指定仓库'}${row.warehouseCode ? `（${row.warehouseCode}）` : ''}`
          : '全部仓库',
      },
      {
        title: '商品',
        dataIndex: 'productTitle',
        width: 180,
        search: false,
        ellipsis: true,
        render: (_, r) => (
          <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{r.productTitle || '—'}</Link>
        ),
      },
      {
        title: PRODUCT_COPY.sku,
        dataIndex: 'skuCode',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuCode || '—',
      },
      {
        title: '规格',
        dataIndex: 'skuName',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuName || '—',
      },
      { title: '兼容投影', dataIndex: 'projectionStock', width: 96, search: false },
      { title: '在手', dataIndex: 'onHandStock', width: 72, search: false },
      { title: '预占', dataIndex: 'reservedStock', width: 72, search: false },
      { title: '残次', dataIndex: 'damagedStock', width: 72, search: false },
      { title: '在途', dataIndex: 'inTransitStock', width: 72, search: false },
      { title: '可售', dataIndex: 'sellableStock', width: 72, search: false },
      { title: '可用', dataIndex: 'availableStock', width: 72, search: false },
      { title: '预警阈值', dataIndex: 'warningStock', width: 88, search: false },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        width: 100,
        search: false,
        render: (_, r) => tagFrom(r.stockStatus, INVENTORY_STOCK_STATUS),
      },
      {
        title: '投影对账',
        dataIndex: 'reconciliationStatus',
        width: 108,
        search: false,
        render: (_, row) => tagFrom(row.reconciliationStatus, INVENTORY_RECONCILIATION_STATUS),
      },
      {
        title: INVENTORY_COPY.skuBinding,
        dataIndex: 'skuBindStatus',
        width: 96,
        search: false,
        render: (_, r) => tagFrom(r.skuBindStatus, INVENTORY_BIND_STATUS),
      },
      {
        title: '平台同步',
        dataIndex: 'platformSyncStatus',
        width: 96,
        search: false,
        render: (_, r) => tagFrom(r.platformSyncStatus, INVENTORY_SYNC_STATUS),
      },
      {
        title: '最近扣减',
        dataIndex: 'lastDeductAt',
        width: 156,
        search: false,
        render: (_, r) => (r.lastDeductAt ? formatDateTime(r.lastDeductAt) : '—'),
      },
      {
        title: '最近同步',
        dataIndex: 'lastSyncAt',
        width: 156,
        search: false,
        render: (_, r) => (r.lastSyncAt ? formatDateTime(r.lastSyncAt) : '—'),
      },
      {
        title: '异常',
        dataIndex: 'exceptionCount',
        width: 72,
        search: false,
        render: (_, r) =>
          r.exceptionCount > 0 ? <Tag color="red">{r.exceptionCount}</Tag> : <Tag>0</Tag>,
      },
      {
        title: '操作',
        valueType: 'option',
        width: 280,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size="small">
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>分仓明细</Link>
            <Link to={`/inventory/deductions?productSkuId=${encodeURIComponent(r.productSkuId)}`}>
              扣减记录
            </Link>
            <Link to={`/inventory/sync-tasks?productSkuId=${encodeURIComponent(r.productSkuId)}`}>
              同步任务
            </Link>
            {r.exceptionCount > 0 ? (
              <Link to={`/ops/task-center/failures?taskType=inventory_sync`}>失败任务</Link>
            ) : null}
          </Space>
        ),
      },
    ],
    [keywordFieldProps, warehouseOptions],
  );

  return (
    <TmPageContainer
      title="库存中心"
      subTitle="按仓库查看可用库存与兼容投影对账；不自动同步、不自动补货。"
    >
      <InventorySyncDisabledBanner />
      <Alert
        type="info"
        showIcon
        message="库存口径说明"
        description="可售库存为在手扣除残次后的数量；可用库存还会扣除预占。兼容投影只用于迁移期对账和既有平台同步流程，不代表当前可用库存。"
        style={{ marginBottom: 16 }}
      />
      {warehouseOptionsError ? (
        <Alert
          type="warning"
          showIcon
          message="仓库筛选暂不可用"
          description={warehouseOptionsError}
          action={<Button onClick={() => void loadWarehouseOptions()}>重试</Button>}
          style={{ marginBottom: 16 }}
        />
      ) : null}
      <KeywordSafetyHint visible={showSensitiveHint} />
      <Typography.Paragraph type="secondary">
        {INVENTORY_SKU_NOT_BOUND_MESSAGE}{' '}
        {INVENTORY_SKU_AMBIGUOUS_MESSAGE}
      </Typography.Paragraph>
      <ProTable<InventoryCenterRow>
        rowKey="productSkuId"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        scroll={{ x: 2200 }}
        search={{ labelWidth: 100, defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(INVENTORY_QUERY_KEYS, { replace: true });
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
        locale={emptyLocale}
        request={async (params) => {
          try {
            const qp = {
              keyword: prepareKeyword(params.keyword),
              productSkuId:
                (params.productSkuId as string | undefined)?.trim() || skuIdFromUrl,
              shopId: (params.shopId as string | undefined)?.trim(),
              platform: (params.platform as string | undefined)?.trim(),
              warehouseId: (params.warehouseId as string | undefined)?.trim(),
              stockStatus: (params.stockStatus as string | undefined)?.trim(),
              skuBindStatus: (params.skuBindStatus as string | undefined)?.trim(),
              syncStatus: (params.syncStatus as string | undefined)?.trim(),
              page: params.current ?? tablePage,
              pageSize: params.pageSize ?? tablePageSize,
            };
            setUrlState(
              {
                page: Number(qp.page) > 1 ? qp.page : undefined,
                pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
                keyword: qp.keyword,
                productSkuId: qp.productSkuId,
                shopId: qp.shopId,
                platform: qp.platform,
                warehouseId: qp.warehouseId,
                stockStatus: qp.stockStatus,
                skuBindStatus: qp.skuBindStatus,
                syncStatus: qp.syncStatus,
                hasException: params.hasException === 'true' ? 'true' : undefined,
                source: urlState.source,
              },
              { replace: true },
            );
            const res = await queryInventoryCenter({
              keyword: qp.keyword,
              productSkuId: qp.productSkuId,
              shopId: qp.shopId,
              platform: qp.platform,
              warehouseId: qp.warehouseId,
              stockStatus: qp.stockStatus,
              skuBindStatus: qp.skuBindStatus,
              syncStatus: qp.syncStatus,
              hasException: params.hasException === 'true' || params.hasException === true,
              page: qp.page,
              pageSize: qp.pageSize,
            });
            return { data: res.list ?? [], success: true, total: res.pagination?.total ?? 0 };
          } catch (e: unknown) {
            message.error((e as Error)?.message || '加载失败');
            return { data: [], success: false, total: 0 };
          }
        }}
      />
    </TmPageContainer>
  );
}
