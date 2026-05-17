import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { Button, Drawer, Popconfirm, Space, Tag, Typography, message } from 'antd';
import dayjs from 'dayjs';
import { useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import {
  getInventorySyncTask,
  queryInventorySyncTasks,
  retryInventorySyncTask,
  type InventorySyncTaskDTO,
} from '@/services/inventory';

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function InventorySyncTasksPage() {
  const actionRef = useRef<ActionType>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<InventorySyncTaskDTO | null>(null);

  const columns: ProColumns<InventorySyncTaskDTO>[] = useMemo(
    () => [
      {
        title: '创建时间范围',
        dataIndex: 'createdRange',
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
        render: (_, r) => dayjs(r.createdAt).format('YYYY-MM-DD HH:mm'),
      },
      {
        title: '商品 ID',
        dataIndex: 'productId',
        hideInTable: true,
      },
      {
        title: 'SKU ID',
        dataIndex: 'productSkuId',
        hideInTable: true,
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        hideInTable: true,
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.shopName || '—',
      },
      {
        title: '商品标题',
        dataIndex: 'productTitle',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.productTitle || '—',
      },
      {
        title: 'SKU 编码',
        dataIndex: 'skuCode',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuCode || '—',
      },
      {
        title: 'platform',
        dataIndex: 'platform',
        width: 100,
      },
      {
        title: '目标库存',
        dataIndex: 'targetStock',
        width: 92,
        search: false,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: COLLECT_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: '开始',
        dataIndex: 'startedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.startedAt ? dayjs(r.startedAt).format('YYYY-MM-DD HH:mm:ss') : '—'),
      },
      {
        title: '结束',
        dataIndex: 'finishedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.finishedAt ? dayjs(r.finishedAt).format('YYYY-MM-DD HH:mm:ss') : '—'),
      },
      {
        title: '错误',
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        render: (_, r) => r.errorMessage || '—',
      },
      {
        title: '操作',
        valueType: 'option',
        width: 140,
        render: (_, r) => (
          <Space>
            <a
              onClick={async () => {
                const d = await getInventorySyncTask(r.id);
                setDetail(d);
                setDetailOpen(true);
              }}
            >
              查看
            </a>
            {r.status === 'failed' ? (
              <Popconfirm
                title="确认重试该库存同步任务？"
                onConfirm={async () => {
                  await retryInventorySyncTask(r.id);
                  message.success('已提交重试');
                  actionRef.current?.reload();
                }}
              >
                <Button type="link" size="small" style={{ padding: 0 }}>
                  重试
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
        ),
      },
    ],
    [],
  );

  return (
    <PageContainer title="库存同步任务">
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        TikTok / Shopee / Lazada / Amazon 的真实库存写入仍为 planned / 接入中；仅 mock 或通过后续版本开放的平台可端到端演示。
      </Typography.Paragraph>
      <ProTable<InventorySyncTaskDTO>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        headerTitle="任务列表"
        request={async (params) => {
          const res = await queryInventorySyncTasks({
            page: params.current,
            pageSize: params.pageSize,
            shopId: params.shopId as string | undefined,
            productId: params.productId as string | undefined,
            productSkuId: params.productSkuId as string | undefined,
            platform: params.platform as string | undefined,
            status: params.status as string | undefined,
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />

      <Drawer
        width={560}
        title={detail ? `库存同步 ${detail.id}` : '详情'}
        open={detailOpen}
        destroyOnClose
        onClose={() => {
          setDetailOpen(false);
          setDetail(null);
        }}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Typography.Text strong>状态：</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>店铺：</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>商品：</Typography.Text> {detail.productTitle || detail.productId}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>SKU：</Typography.Text> {detail.skuCode || detail.productSkuId || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>目标库存：</Typography.Text> {detail.targetStock}
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text type="secondary">taskId</Typography.Text>
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph>
                <Typography.Text strong>错误：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            <Typography.Paragraph>
              <Typography.Text strong>input（摘要）</Typography.Text>
              <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 220 }}>
                {JSON.stringify(detail.input ?? {}, null, 2)}
              </pre>
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>output（摘要）</Typography.Text>
              <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 220 }}>
                {JSON.stringify(detail.output ?? {}, null, 2)}
              </pre>
            </Typography.Paragraph>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
