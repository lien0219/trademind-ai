import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { Button, Drawer, Popconfirm, Space, Tabs, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { Link, useLocation } from '@umijs/max';
import { useEffect, useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { publishBatchStatusLabel } from '@/constants/publishLabels';
import { platformLabel } from '@/constants/userFriendly';
import {
  getProductPublishTask,
  queryProductPublishTasks,
  queryPublishBatches,
  retryProductPublishTask,
  type ProductPublishTaskDTO,
  type PublishBatchListItem,
} from '@/services/productPublish';

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function ProductPublishTasksPage() {
  const location = useLocation();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const statusFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('status')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);
  const tabFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('tab')?.trim() || 'tasks';
    } catch {
      return 'tasks';
    }
  }, [location.search]);
  const [activeTab, setActiveTab] = useState(tabFromUrl);
  const batchActionRef = useRef<ActionType>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<ProductPublishTaskDTO | null>(null);

  useEffect(() => {
    if (!statusFromUrl) return;
    formRef.current?.setFieldsValue?.({ status: statusFromUrl });
    actionRef.current?.reload?.();
  }, [statusFromUrl]);

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
        title: '平台',
        dataIndex: 'platform',
        width: 100,
        render: (_, r) => platformLabel(r.platform),
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

  useEffect(() => {
    setActiveTab(tabFromUrl);
  }, [tabFromUrl]);

  const batchColumns: ProColumns<PublishBatchListItem>[] = useMemo(
    () => [
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '批次名称',
        dataIndex: 'name',
        ellipsis: true,
        search: false,
        render: (_, r) => r.name || `批次 ${r.id.slice(0, 8)}`,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 110,
        search: false,
        render: (_, r) => (
          <Tag>{r.statusLabel || publishBatchStatusLabel(r.status)}</Tag>
        ),
      },
      { title: '商品数', dataIndex: 'productCount', width: 80, search: false },
      { title: '目标数', dataIndex: 'targetCount', width: 80, search: false },
      { title: '任务数', dataIndex: 'taskCount', width: 80, search: false },
      { title: '成功', dataIndex: 'successCount', width: 72, search: false },
      { title: '失败', dataIndex: 'failedCount', width: 72, search: false },
      {
        title: '操作',
        valueType: 'option',
        width: 100,
        render: (_, r) => <Link to={`/product/publish-batches/${r.id}`}>查看</Link>,
      },
    ],
    [],
  );

  return (
    <TmPageContainer title="商品刊登任务" subTitle="查看刊登子任务与批量刊登批次进度。">
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'tasks',
            label: '子任务',
            children: (
              <ProTable<ProductPublishTaskDTO>
                rowKey="id"
                actionRef={actionRef}
                formRef={formRef}
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
            ),
          },
          {
            key: 'batches',
            label: '刊登批次',
            children: (
              <ProTable<PublishBatchListItem>
                rowKey="id"
                actionRef={batchActionRef}
                columns={batchColumns}
                search={false}
                pagination={{ pageSize: 20, showSizeChanger: true }}
                headerTitle="批量刊登批次"
                request={async (params) => {
                  const res = await queryPublishBatches({
                    page: params.current,
                    pageSize: params.pageSize,
                  });
                  return { data: res.list, total: res.pagination.total, success: true };
                }}
              />
            ),
          },
        ]}
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
            {detail.errorMessage ? (
              <Typography.Paragraph>
                <Typography.Text strong>失败原因：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            {detail.platformProductId ? (
              <Typography.Paragraph copyable={{ text: detail.platformProductId }}>
                <Typography.Text strong>平台商品编号：</Typography.Text> {detail.platformProductId}
              </Typography.Paragraph>
            ) : null}
            {detail.retryable != null ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>可以重试：</Typography.Text> {detail.retryable ? '是' : '否'}
              </Typography.Paragraph>
            ) : null}
            <TechnicalDetails>
              {detail.requestId ? (
                <Typography.Paragraph copyable={{ text: detail.requestId }} style={{ marginBottom: 8 }}>
                  <Typography.Text strong>请求编号：</Typography.Text> {detail.requestId}
                </Typography.Paragraph>
              ) : null}
              <Typography.Paragraph copyable={{ text: detail.id }} style={{ marginBottom: 8 }}>
                <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
              </Typography.Paragraph>
              <TaskJsonBlock title="平台提交内容" value={detail.platformPayload} />
              <TaskJsonBlock title="平台返回结果" value={detail.platformResult ?? detail.output} />
              <TaskJsonBlock title="任务输入" value={detail.input} />
              <TaskJsonBlock title="任务输出" value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>
    </TmPageContainer>
  );
}
