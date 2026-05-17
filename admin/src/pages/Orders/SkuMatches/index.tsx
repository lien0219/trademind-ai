import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import { Button, Space } from 'antd';
import { history, useSearchParams } from '@umijs/max';
import { queryOrderSkuMatches, type OrderSkuMatchListRow } from '@/services/orders';
import { queryShops } from '@/services/shops';

export default function OrderSkuMatchesPage() {
  const [searchParams] = useSearchParams();
  const presetOrderId = searchParams.get('orderId')?.trim() ?? '';

  const columns: ProColumns<OrderSkuMatchListRow>[] = [
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      valueType: 'dateTime',
      width: 170,
      hideInSearch: true,
      sorter: true,
    },
    {
      title: '平台',
      dataIndex: 'platform',
      width: 90,
      valueType: 'select',
      valueEnum: {
        tiktok: { text: 'tiktok' },
        shopee: { text: 'shopee' },
        lazada: { text: 'lazada' },
        amazon: { text: 'amazon' },
        manual: { text: 'manual' },
      },
    },
    {
      title: '店铺',
      dataIndex: 'shopId',
      hideInTable: true,
      valueType: 'select',
      request: async () => {
        const r = await queryShops({ page: 1, pageSize: 500 });
        return r.list.map((s) => ({ label: `${s.shopName} (${s.platform})`, value: s.id }));
      },
    },
    {
      title: '店铺',
      dataIndex: 'shopName',
      width: 130,
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: '订单号',
      dataIndex: 'orderNo',
      width: 120,
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: '外部订单',
      dataIndex: 'externalOrderId',
      width: 120,
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: '订单 ID',
      dataIndex: 'orderId',
      hideInTable: true,
      initialValue: presetOrderId || undefined,
    },
    {
      title: '外部 SKU',
      dataIndex: 'externalSkuId',
      width: 110,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: 'SKU Code',
      dataIndex: 'skuCode',
      width: 96,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: '匹配状态',
      dataIndex: 'matchStatus',
      width: 110,
      valueType: 'select',
      valueEnum: {
        matched: { text: 'matched' },
        unmatched: { text: 'unmatched' },
        ambiguous: { text: 'ambiguous' },
        manual_bound: { text: 'manual_bound' },
        skipped: { text: 'skipped' },
      },
    },
    {
      title: '匹配类型',
      dataIndex: 'matchType',
      width: 150,
      ellipsis: true,
    },
    {
      title: '置信度',
      dataIndex: 'confidence',
      width: 72,
      hideInSearch: true,
    },
    {
      title: '商品标题',
      dataIndex: 'productTitle',
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: '本地 SKU',
      dataIndex: 'localSkuCode',
      width: 96,
      hideInSearch: true,
    },
    {
      title: '本地 SKU ID',
      dataIndex: 'productSkuId',
      hideInTable: true,
      valueType: 'text',
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, row) => (
        <Space>
          <Button
            type="link"
            size="small"
            disabled={!row.orderId}
            onClick={() => {
              if (row.orderId) history.push(`/orders?jumpOrder=${encodeURIComponent(row.orderId)}`);
            }}
          >
            查看订单
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="SKU 匹配">
      <ProTable<OrderSkuMatchListRow>
        rowKey="id"
        search={{ labelWidth: 100 }}
        request={async (params) => {
          const r = await queryOrderSkuMatches({
            page: params.current,
            pageSize: params.pageSize,
            platform: (params.platform as string) || undefined,
            shopId: (params.shopId as string) || undefined,
            matchStatus: (params.matchStatus as string) || undefined,
            matchType: (params.matchType as string) || undefined,
            orderId: (params.orderId as string) || undefined,
            productSkuId: (params.productSkuId as string) || undefined,
          });
          return { data: r.list, success: true, total: r.pagination.total };
        }}
        columns={columns}
        pagination={{ pageSize: 20 }}
      />
    </PageContainer>
  );
}
