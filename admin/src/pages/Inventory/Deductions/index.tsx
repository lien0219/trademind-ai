import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import {
  INVENTORY_DEDUCT_SOURCE,
  INVENTORY_DEDUCT_STATUS,
  inventoryTagFromMap,
} from '@/constants/inventoryLabels';
import { queryGlobalInventoryEffects, type OrderInventoryEffectRow } from '@/services/inventory';
import { Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { Link, useLocation } from '@umijs/max';
import { useMemo, useRef } from 'react';

function effectTypeLabel(raw: string) {
  return INVENTORY_DEDUCT_SOURCE[raw] || raw || '—';
}

export default function InventoryDeductionsPage() {
  const actionRef = useRef<ActionType>();
  const location = useLocation();

  const initialSearch = useMemo(() => {
    const q = new URLSearchParams(location.search || '');
    const out: Record<string, string> = {};
    const orderId = (q.get('orderId') || '').trim();
    const productSkuId = (q.get('productSkuId') || '').trim();
    if (orderId) out.orderId = orderId;
    if (productSkuId) out.productSkuId = productSkuId;
    return out;
  }, [location.search]);

  const columns: ProColumns<OrderInventoryEffectRow>[] = useMemo(
    () => [
      {
        title: '时间范围',
        dataIndex: 'timeRange',
        hideInTable: true,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: '扣减时间',
        dataIndex: 'createdAt',
        width: 160,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '来源订单',
        dataIndex: 'orderId',
        width: 140,
        render: (_, r) =>
          r.orderId ? (
            <Link to={`/orders/${r.orderId}?tab=inventory`}>{r.orderNo || r.orderId.slice(0, 8)}</Link>
          ) : (
            '—'
          ),
      },
      {
        title: '商品',
        dataIndex: 'productTitle',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) =>
          r.productId ? (
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{r.productTitle || '—'}</Link>
          ) : (
            r.productTitle || '—'
          ),
      },
      {
        title: 'SKU',
        dataIndex: 'skuCode',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuCode || '—',
      },
      { title: '扣减数量', dataIndex: 'quantity', width: 88, search: false },
      { title: '扣减前', dataIndex: 'beforeStock', width: 80, search: false, render: (_, r) => r.beforeStock ?? '—' },
      { title: '扣减后', dataIndex: 'afterStock', width: 80, search: false, render: (_, r) => r.afterStock ?? '—' },
      {
        title: '扣减状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(INVENTORY_DEDUCT_STATUS).map(([k, v]) => [k, { text: v.text }]),
        ),
        render: (_, r) => {
          const cfg = inventoryTagFromMap(r.status, INVENTORY_DEDUCT_STATUS);
          return <Tag color={cfg.color}>{cfg.text}</Tag>;
        },
      },
      {
        title: '来源',
        dataIndex: 'effectType',
        width: 120,
        valueType: 'select',
        valueEnum: {
          deduct: { text: '订单同步扣减' },
          restore: { text: '系统回滚' },
        },
        render: (_, r) => effectTypeLabel(r.effectType),
      },
      {
        title: '失败原因',
        dataIndex: 'errorMessage',
        search: false,
        ellipsis: true,
        render: (_, r) => r.errorMessage || r.reason || '—',
      },
      {
        title: '操作',
        valueType: 'option',
        width: 200,
        render: (_, r) => (
          <Space wrap size="small">
            {r.orderId ? <Link to={`/orders/${r.orderId}?tab=inventory`}>订单详情</Link> : null}
            {r.status === 'failed' ? (
              <Link to={`/ops/task-center/failures?taskType=inventory_sync`}>失败任务</Link>
            ) : null}
            {r.productSkuId ? (
              <Link to={`/inventory?skuId=${encodeURIComponent(r.productSkuId)}`}>库存中心</Link>
            ) : null}
          </Space>
        ),
      },
    ],
    [],
  );

  return (
    <TmPageContainer title="库存扣减记录" subTitle="订单扣减必须可追溯到订单详情；失败记录进入失败任务中心。">
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        来源包括订单同步扣减、人工修正与系统回滚。不允许静默扣减失败。
      </Typography.Paragraph>
      <ProTable<OrderInventoryEffectRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        form={{ initialValues: initialSearch }}
        search={{ labelWidth: 112, defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        request={async (params) => {
          try {
            const res = await queryGlobalInventoryEffects({
              page: params.current,
              pageSize: params.pageSize,
              orderId: (params.orderId as string)?.trim() || undefined,
              productSkuId: (params.productSkuId as string)?.trim() || undefined,
              effectType: params.effectType as string | undefined,
              status: params.status as string | undefined,
              start: typeof params.start === 'string' ? params.start : undefined,
              end: typeof params.end === 'string' ? params.end : undefined,
            });
            return { data: res.list, total: res.pagination.total, success: true };
          } catch (e: unknown) {
            message.error((e as Error)?.message || '加载失败');
            return { data: [], success: false, total: 0 };
          }
        }}
      />
    </TmPageContainer>
  );
}
