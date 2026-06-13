import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { Button, Drawer, Popconfirm, Space, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useMemo, useRef, useState } from 'react';
import { CUSTOMER_MESSAGE_SYNC_TASK_STATUS } from '@/constants/status';
import { PLATFORM_OPTIONS, platformLabel } from '@/constants/userFriendly';
import {
  getCustomerMessageSyncTask,
  queryCustomerMessageSyncTasks,
  retryCustomerMessageSyncTask,
  type CustomerMessageSyncTaskRow,
} from '@/services/customer';

function tagFromStatus(raw: string) {
  const c = CUSTOMER_MESSAGE_SYNC_TASK_STATUS[raw as keyof typeof CUSTOMER_MESSAGE_SYNC_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function CustomerMessageSyncTasksPage() {
  const actionRef = useRef<ActionType>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<CustomerMessageSyncTaskRow | null>(null);

  const columns: ProColumns<CustomerMessageSyncTaskRow>[] = useMemo(
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
        title: '平台',
        dataIndex: 'platform',
        width: 100,
        valueType: 'select',
        fieldProps: {
          showSearch: true,
          optionFilterProp: 'label',
          options: PLATFORM_OPTIONS,
          allowClear: true,
        },
        render: (_, r) => platformLabel(r.platform),
      },
      {
        title: '模式',
        dataIndex: 'mode',
        width: 96,
        search: false,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: CUSTOMER_MESSAGE_SYNC_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: '总计',
        dataIndex: 'totalCount',
        width: 72,
        search: false,
      },
      {
        title: '成功',
        dataIndex: 'successCount',
        width: 72,
        search: false,
      },
      {
        title: '失败',
        dataIndex: 'failedCount',
        width: 72,
        search: false,
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
                const d = await getCustomerMessageSyncTask(r.id);
                setDetail(d);
                setDetailOpen(true);
              }}
            >
              查看
            </a>
            {r.status === 'failed' ? (
              <Popconfirm
                title="确认重试该同步任务？"
                onConfirm={async () => {
                  await retryCustomerMessageSyncTask(r.id);
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
    <TmPageContainer title="客服消息同步任务">
      <ProTable<CustomerMessageSyncTaskRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        headerTitle="同步记录"
        request={async (params) => {
          const res = await queryCustomerMessageSyncTasks({
            page: params.current,
            pageSize: params.pageSize,
            shopId: params.shopId as string | undefined,
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
        title={detail ? `同步任务 ${detail.id}` : '详情'}
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
              <Typography.Text type="secondary">({platformLabel(detail.platform)})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph type="danger">
                <Typography.Text strong>失败原因：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            <TechnicalDetails>
              <TaskJsonBlock title="任务输入" value={detail.input} />
              <TaskJsonBlock title="任务输出" value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>
    </TmPageContainer>
  );
}
