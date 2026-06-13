import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { Tag } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useMemo, useRef } from 'react';
import { useLocation } from '@umijs/renderer-react';
import type { InventoryChangeLogRow } from '@/services/inventory';
import { queryGlobalInventoryLogs } from '@/services/inventory';

function renderDelta(delta: number) {
  const color = delta > 0 ? 'green' : delta < 0 ? 'red' : 'default';
  const prefix = delta > 0 ? '+' : '';
  return <Tag color={color}>{`${prefix}${delta}`}</Tag>;
}

export default function InventoryLogsPage() {
  const actionRef = useRef<ActionType>();
  const { search } = useLocation();

  const initialSearch = useMemo(() => {
    const q = new URLSearchParams(search);
    const productId = (q.get('productId') || '').trim();
    const productSkuId = (q.get('productSkuId') || '').trim();
    const orderId = (q.get('orderId') || '').trim();
    const out: Record<string, string> = {};
    if (productId) out.productId = productId;
    if (productSkuId) out.productSkuId = productSkuId;
    if (orderId) out.orderId = orderId;
    return out;
  }, [search]);

  const columns: ProColumns<InventoryChangeLogRow>[] = useMemo(
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
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      { title: '商品 ID', dataIndex: 'productId', hideInTable: true },
      { title: 'SKU ID', dataIndex: 'productSkuId', hideInTable: true },
      { title: '订单 ID', dataIndex: 'orderId', hideInTable: true },
      {
        title: '变更类型',
        dataIndex: 'changeType',
        width: 132,
        ellipsis: true,
      },
      {
        title: '变更前',
        dataIndex: 'beforeStock',
        width: 88,
        search: false,
      },
      {
        title: '变更后',
        dataIndex: 'afterStock',
        width: 88,
        search: false,
      },
      {
        title: '变化',
        dataIndex: 'delta',
        width: 88,
        search: false,
        render: (_, r) => renderDelta(r.delta),
      },
      {
        title: '原因',
        dataIndex: 'reason',
        width: 140,
        ellipsis: true,
        search: false,
        render: (_, r) => r.reason || '—',
      },
      {
        title: '备注',
        dataIndex: 'remark',
        ellipsis: true,
        search: false,
        render: (_, r) => r.remark || '—',
      },
      {
        title: '关联订单',
        dataIndex: 'refOrderId',
        width: 140,
        ellipsis: true,
        copyable: true,
        search: false,
        render: (_, r) => r.refOrderId || '—',
      },
      {
        title: '关联订单行',
        dataIndex: 'refOrderItemId',
        width: 140,
        ellipsis: true,
        copyable: true,
        search: false,
        render: (_, r) => r.refOrderItemId || '—',
      },
    ],
    [],
  );

  return (
    <TmPageContainer title="库存流水" subTitle="查看本地库存变更记录，便于核对扣减、恢复与手动调整。">
      <ProTable<InventoryChangeLogRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        form={{ initialValues: initialSearch }}
        search={{ defaultCollapsed: false }}
        pagination={{ pageSize: 20 }}
        request={async (params) => {
          const res = await queryGlobalInventoryLogs({
            page: params.current,
            pageSize: params.pageSize,
            productId: (params.productId as string)?.trim() || undefined,
            productSkuId: (params.productSkuId as string)?.trim() || undefined,
            orderId: (params.orderId as string)?.trim() || undefined,
            changeType: (params.changeType as string)?.trim() || undefined,
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />
    </TmPageContainer>
  );
}
