import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { Button, Drawer, Popconfirm, Space, Tag, Typography, Alert, message, Table } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { history, useLocation } from '@umijs/max';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PAGE_COPY } from '@/constants/copywriting';
import { ORDER_SYNC_TASK_STATUS } from '@/constants/status';
import {
  getOrderSyncTask,
  queryOrderSyncTasks,
  retryOrderSyncTask,
  type OrderSyncTaskDTO,
  type OrderSyncTaskOutput,
} from '@/services/orderSync';

function parseOutput(raw: unknown): OrderSyncTaskOutput | null {
  if (!raw || typeof raw !== 'object') return null;
  return raw as OrderSyncTaskOutput;
}

function tagFromStatus(raw: string) {
  const c = ORDER_SYNC_TASK_STATUS[raw as keyof typeof ORDER_SYNC_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function OrderSyncTasksPage() {
  const actionRef = useRef<ActionType>();
  const location = useLocation();
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<OrderSyncTaskDTO | null>(null);

  const openDetail = useCallback(async (id: string) => {
    const d = await getOrderSyncTask(id);
    setDetail(d);
    setDetailOpen(true);
  }, []);

  useEffect(() => {
    const id = new URLSearchParams(location.search || '').get('id')?.trim();
    if (!id) return;
    void openDetail(id);
  }, [location.search, openDetail]);

  const columns: ProColumns<OrderSyncTaskDTO>[] = useMemo(
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
        valueEnum: {
          douyin_shop: { text: '抖店' },
          tiktok: { text: 'TikTok' },
          shopee: { text: 'Shopee' },
          lazada: { text: 'Lazada' },
          amazon: { text: 'Amazon' },
          mock: { text: '模拟' },
        },
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
        valueEnum: ORDER_SYNC_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: '合计',
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
              onClick={() => {
                void openDetail(r.id);
              }}
            >
              查看
            </a>
            {r.status === 'failed' || r.status === 'partial_success' ? (
              <Popconfirm
                title={
                  r.status === 'partial_success'
                    ? '重试将仅拉取失败页，不会重复拉取已成功页。确认？'
                    : '确认重试该同步任务？'
                }
                onConfirm={async () => {
                  await retryOrderSyncTask(r.id);
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

  const output = parseOutput(detail?.output);

  return (
    <TmPageContainer title={PAGE_COPY.orderSyncTasks.title} subTitle={PAGE_COPY.orderSyncTasks.description}>
      <Alert
        showIcon
        type="info"
        style={{ marginBottom: 16 }}
        message="抖店订单同步说明"
        description="须先在「设置 → 平台接入设置 → 抖店」开启「开启订单同步」，并在「店铺管理」完成店铺授权。未授权或授权过期时不能同步；失败任务可在本页重试或到「失败任务中心」查看。"
      />
      <ProTable<OrderSyncTaskDTO>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        headerTitle="同步记录"
        request={async (params) => {
          const res = await queryOrderSyncTasks({
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
            {detail.status === 'partial_success' ? (
              <Alert
                showIcon
                type="warning"
                message="部分成功：部分页已同步，部分页失败"
                description="已成功页的数据已写入本地订单；失败页可在下方列表查看原因，点击「重试失败页」仅重拉失败页。"
              />
            ) : null}
            <div>
              <Typography.Text strong>状态：</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Space wrap>
              <Typography.Link
                onClick={() =>
                  history.push(`/ops/task-center/failures?taskType=order_sync&keyword=${encodeURIComponent(detail.id)}`)
                }
              >
                在失败任务中心查看
              </Typography.Link>
              <Typography.Link
                onClick={() =>
                  history.push(`/orders/exceptions?exceptionType=order_sync_partial_failed`)
                }
              >
                订单异常工作台
              </Typography.Link>
            </Space>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>店铺：</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph type="danger">
                <Typography.Text strong>失败原因：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            {output ? (
              <>
                <Typography.Paragraph style={{ marginBottom: 8 }}>
                  <Typography.Text strong>分页：</Typography.Text> 总页 {output.totalPages ?? 0} · 成功{' '}
                  {output.successPages ?? 0} · 失败 {output.failedPages ?? 0}
                  {output.retryPagesOnly ? ' · 上次为重试失败页模式' : ''}
                </Typography.Paragraph>
                <Typography.Paragraph style={{ marginBottom: 8 }}>
                  <Typography.Text strong>拉取订单：</Typography.Text> {output.totalFetched ?? 0} 条 · 新建{' '}
                  {output.createdOrders ?? 0} · 更新 {output.updatedOrders ?? 0}
                </Typography.Paragraph>
                <Typography.Paragraph style={{ marginBottom: 8 }}>
                  <Typography.Text strong>规格匹配：</Typography.Text> 已匹配 {output.matchedItems ?? 0} · 未匹配{' '}
                  {output.unmatchedItems ?? 0}
                  {output.deductedStockItems ? ` · 扣库 ${output.deductedStockItems} 行` : ''}
                </Typography.Paragraph>
                {(output.pageErrors?.length ?? 0) > 0 ? (
                  <>
                    <Typography.Text strong>失败页列表</Typography.Text>
                    <Table
                      size="small"
                      style={{ marginTop: 8 }}
                      pagination={false}
                      rowKey={(r) => String(r.page)}
                      dataSource={output.pageErrors}
                      columns={[
                        { title: '页码', dataIndex: 'page', width: 64 },
                        {
                          title: 'cursor / nextPage',
                          key: 'cursor',
                          width: 120,
                          render: (_, r) => r.cursor || r.nextPage || '—',
                        },
                        { title: '错误码', dataIndex: 'errorCode', width: 100, render: (v) => v || '—' },
                        { title: '错误信息', dataIndex: 'error', ellipsis: true },
                        {
                          title: '可重试',
                          width: 72,
                          render: () => <Tag color="success">是</Tag>,
                        },
                      ]}
                    />
                    {detail.status === 'partial_success' || detail.status === 'failed' ? (
                      <Popconfirm
                        title="仅重试失败页（不重复拉取已成功页）"
                        onConfirm={async () => {
                          await retryOrderSyncTask(detail.id);
                          message.success('已提交重试失败页');
                          await openDetail(detail.id);
                          actionRef.current?.reload();
                        }}
                      >
                        <Button type="primary" size="small" style={{ marginTop: 8 }}>
                          重试失败页
                        </Button>
                      </Popconfirm>
                    ) : null}
                  </>
                ) : null}
              </>
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
