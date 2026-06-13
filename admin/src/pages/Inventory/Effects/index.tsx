import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useMemo, useRef } from 'react';
import { useLocation } from '@umijs/renderer-react';
import type { OrderInventoryEffectRow } from '@/services/inventory';
import { queryGlobalInventoryEffects } from '@/services/inventory';

export default function InventoryEffectsPage() {
  const actionRef = useRef<ActionType>();
  const { search } = useLocation();

  const initialSearch = useMemo(() => {
    const q = new URLSearchParams(search);
    const orderId = (q.get('orderId') || '').trim();
    const productSkuId = (q.get('productSkuId') || '').trim();
    const out: Record<string, string> = {};
    if (orderId) out.orderId = orderId;
    if (productSkuId) out.productSkuId = productSkuId;
    return out;
  }, [search]);

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
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 160,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      { title: '订单 ID', dataIndex: 'orderId', ellipsis: true, copyable: true, width: 120 },
      { title: '订单号', dataIndex: 'orderNo', search: false, width: 120, ellipsis: true },
      { title: '影响类型', dataIndex: 'effectType', width: 120 },
      { title: '状态', dataIndex: 'status', width: 100 },
      { title: '数量', dataIndex: 'quantity', width: 72, search: false },
      { title: '商品规格 ID', dataIndex: 'productSkuId', width: 120, ellipsis: true, copyable: true },
      {
        title: '错误',
        dataIndex: 'errorMessage',
        search: false,
        ellipsis: true,
        render: (_, r) => r.errorMessage || '—',
      },
    ],
    [],
  );

  return (
    <TmPageContainer title="订单库存影响" subTitle="查看订单对本地库存的扣减、恢复与跳过记录。">
      <ProTable<OrderInventoryEffectRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        form={{ initialValues: initialSearch }}
        search={{ labelWidth: 112, defaultCollapsed: false }}
        pagination={{ pageSize: 20 }}
        request={async (params) => {
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
        }}
      />
    </TmPageContainer>
  );
}
