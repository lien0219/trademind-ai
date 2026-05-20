import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { Button, Drawer, Popconfirm, Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import {
  getProductPublishTask,
  queryProductPublishTasks,
  retryProductPublishTask,
  type ProductPublishTaskDTO,
} from '@/services/productPublish';

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function ProductPublishTasksPage() {
  const actionRef = useRef<ActionType>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<ProductPublishTaskDTO | null>(null);

  const columns: ProColumns<ProductPublishTaskDTO>[] = useMemo(
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
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '商品 ID',
        dataIndex: 'productId',
        hideInTable: true,
        valueType: 'text',
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'text',
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
        title: '商品',
        dataIndex: 'productTitle',
        width: 160,
        search: false,
        ellipsis: true,
        render: (_, r) => r.productTitle || '—',
      },
      {
        title: 'platform',
        dataIndex: 'platform',
        width: 100,
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
        render: (_, r) => (r.startedAt ? formatDateTime(r.startedAt) : '—'),
      },
      {
        title: '结束',
        dataIndex: 'finishedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.finishedAt ? formatDateTime(r.finishedAt) : '—'),
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
                const d = await getProductPublishTask(r.id);
                setDetail(d);
                setDetailOpen(true);
              }}
            >
              查看
            </a>
            {r.status === 'failed' ? (
              <Popconfirm
                title="确认重试该刊登任务？"
                onConfirm={async () => {
                  await retryProductPublishTask(r.id);
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
    <PageContainer title="商品刊登任务">
      <ProTable<ProductPublishTaskDTO>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        headerTitle="刊登记录"
        request={async (params) => {
          const res = await queryProductPublishTasks({
            page: params.current,
            pageSize: params.pageSize,
            shopId: params.shopId as string | undefined,
            productId: params.productId as string | undefined,
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
        title={detail ? `刊登任务 ${detail.id}` : '详情'}
        open={detailOpen}
        destroyOnHidden
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
              <Typography.Text strong>商品：</Typography.Text>{' '}
              {detail.productTitle || detail.productId}
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
              <Typography.Text strong>input</Typography.Text>
              <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200 }}>
                {JSON.stringify(detail.input ?? {}, null, 2)}
              </pre>
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>output</Typography.Text>
              <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200 }}>
                {JSON.stringify(detail.output ?? {}, null, 2)}
              </pre>
            </Typography.Paragraph>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
