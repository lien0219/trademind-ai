import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import InventorySyncDisabledBanner from '@/components/inventory/InventorySyncDisabledBanner';
import {
  INVENTORY_BIND_STATUS,
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
  INVENTORY_STOCK_STATUS,
  INVENTORY_SYNC_STATUS,
  inventoryTagFromMap,
} from '@/constants/inventoryLabels';
import { INVENTORY_COPY, PRODUCT_COPY } from '@/constants/copywriting';
import { queryInventoryCenter, type InventoryCenterRow } from '@/services/inventory';
import { Button, Empty, Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history, useLocation } from '@umijs/max';
import { useEffect, useMemo, useRef } from 'react';

function tagFrom(raw: string, map: Record<string, { text: string; color: string }>) {
  const cfg = inventoryTagFromMap(raw, map);
  return <Tag color={cfg.color}>{cfg.text}</Tag>;
}

export default function InventoryCenterPage() {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const location = useLocation();

  const skuIdFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('skuId')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);

  useEffect(() => {
    if (!skuIdFromUrl) return;
    formRef.current?.setFieldsValue?.({ productSkuId: skuIdFromUrl });
    actionRef.current?.reload?.();
  }, [skuIdFromUrl]);

  const columns: ProColumns<InventoryCenterRow>[] = useMemo(
    () => [
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: '商品标题 / 规格编码 / 名称' },
      },
      { title: '规格 ID', dataIndex: 'productSkuId', hideInTable: true },
      { title: '店铺 ID', dataIndex: 'shopId', hideInTable: true },
      { title: '平台', dataIndex: 'platform', hideInTable: true },
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
      { title: '本地库存', dataIndex: 'stock', width: 88, search: false },
      { title: '可用库存', dataIndex: 'availableStock', width: 88, search: false },
      { title: '预警阈值', dataIndex: 'warningStock', width: 88, search: false },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        width: 100,
        search: false,
        render: (_, r) => tagFrom(r.stockStatus, INVENTORY_STOCK_STATUS),
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
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>查看商品</Link>
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
    [],
  );

  return (
    <TmPageContainer
      title="库存中心"
      subTitle="查看本地库存、SKU 绑定与平台同步状态；不自动同步、不自动补货。"
    >
      <InventorySyncDisabledBanner />
      <Typography.Paragraph type="secondary">
        {INVENTORY_SKU_NOT_BOUND_MESSAGE}{' '}
        {INVENTORY_SKU_AMBIGUOUS_MESSAGE}
      </Typography.Paragraph>
      <ProTable<InventoryCenterRow>
        rowKey="productSkuId"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 100, defaultCollapsed: false }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        locale={{
          emptyText: (
            <Empty description="暂无库存数据">
              <Space direction="vertical">
                <Typography.Text type="secondary">可先创建商品草稿并维护 SKU 库存。</Typography.Text>
                <Button type="primary" onClick={() => history.push('/product/drafts')}>
                  前往商品草稿
                </Button>
              </Space>
            </Empty>
          ),
        }}
        request={async (params) => {
          try {
            const res = await queryInventoryCenter({
              keyword: (params.keyword as string) || undefined,
              productSkuId: (params.productSkuId as string) || skuIdFromUrl,
              shopId: (params.shopId as string) || undefined,
              platform: (params.platform as string) || undefined,
              stockStatus: (params.stockStatus as string) || undefined,
              skuBindStatus: (params.skuBindStatus as string) || undefined,
              syncStatus: (params.syncStatus as string) || undefined,
              hasException: params.hasException === 'true' || params.hasException === true,
              page: params.current,
              pageSize: params.pageSize,
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
