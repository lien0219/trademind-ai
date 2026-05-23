import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { Button, Drawer, Popconfirm, Space, Spin, Table, Tabs, Tag, Typography, message } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { history } from '@umijs/max';
import { useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import {
  INVENTORY_SYNC_BATCH_MAX_SIZE_LABEL,
  platformLabel,
} from '@/constants/userFriendly';
import {
  getInventorySyncBatch,
  queryInventorySyncBatchTasks,
  queryInventorySyncBatches,
  retryInventorySyncBatchFailed,
  type InventorySyncBatchDTO,
  type InventorySyncTaskDTO,
} from '@/services/inventory';

const SOURCE_LABEL: Record<string, string> = {
  manual: '手动',
  inventory_alert: '库存预警',
  product_detail: '商品详情',
  failed_retry: '失败重试',
  order_deduct: '订单扣减',
  system: '系统',
};

const BATCH_STATUS_META: Record<string, { color: string; text: string }> = {
  pending: { color: 'default', text: '待执行' },
  running: { color: 'processing', text: '执行中' },
  success: { color: 'success', text: '成功' },
  partial_success: { color: 'warning', text: '部分成功' },
  failed: { color: 'error', text: '失败' },
  cancelled: { color: 'default', text: '已取消' },
};

function batchStatusTag(raw: string) {
  const m = BATCH_STATUS_META[raw];
  if (!m) return <Tag>{raw}</Tag>;
  return <Tag color={m.color}>{m.text}</Tag>;
}

function taskTagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

function summarizeJson(val: unknown, max = 1200): string {
  try {
    const s = JSON.stringify(val ?? {}, null, 2);
    return s.length > max ? `${s.slice(0, max)}\n…` : s;
  } catch {
    return '';
  }
}

export default function InventorySyncBatchesPage() {
  const actionRef = useRef<ActionType>();
  const tasksActionRef = useRef<ActionType>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerLoading, setDrawerLoading] = useState(false);
  const [drawerBatch, setDrawerBatch] = useState<InventorySyncBatchDTO | null>(null);

  const columns: ProColumns<InventorySyncBatchDTO>[] = useMemo(
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
        title: '批次号',
        dataIndex: 'batchNo',
        width: 148,
        ellipsis: true,
        search: false,
      },
      {
        title: '来源',
        dataIndex: 'source',
        width: 108,
        search: false,
        render: (_, r) => SOURCE_LABEL[r.source] || r.source,
      },
      {
        title: '来源筛选',
        dataIndex: 'source',
        hideInTable: true,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(SOURCE_LABEL).map(([k, v]) => [k, { text: v }])),
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 96,
        render: (_, r) => platformLabel(r.platform),
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        hideInTable: true,
      },
      {
        title: '商品 ID',
        dataIndex: 'productId',
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
        title: '状态',
        dataIndex: 'status',
        width: 112,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(BATCH_STATUS_META).map(([k, v]) => [k, { text: v.text }]),
        ),
        render: (_, r) => batchStatusTag(r.status),
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
        title: '跳过',
        dataIndex: 'skippedCount',
        width: 72,
        search: false,
      },
      {
        title: '操作',
        valueType: 'option',
        width: 220,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size="small">
            <a
              onClick={async () => {
                setDrawerOpen(true);
                setDrawerLoading(true);
                setDrawerBatch(null);
                try {
                  const b = await getInventorySyncBatch(r.id, { recentTasks: 15 });
                  setDrawerBatch(b);
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '加载失败');
                  setDrawerOpen(false);
                } finally {
                  setDrawerLoading(false);
                }
              }}
            >
              详情
            </a>
            <a
              onClick={() => {
                history.push(`/inventory/sync-tasks?batchId=${encodeURIComponent(r.id)}`);
              }}
            >
              任务列表
            </a>
            <Popconfirm
              title="对该批次内失败任务发起重试？"
              disabled={r.failedCount <= 0}
              onConfirm={async () => {
                try {
                  await retryInventorySyncBatchFailed(r.id);
                  message.success('已提交重试');
                  actionRef.current?.reload();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              <Button type="link" size="small" style={{ padding: 0 }} disabled={r.failedCount <= 0}>
                重试失败
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [],
  );

  const taskColumns: ProColumns<InventorySyncTaskDTO>[] = useMemo(
    () => [
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 156,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      { title: '平台', dataIndex: 'platform', width: 88, render: (_, r) => platformLabel(r.platform) },
      { title: '店铺', dataIndex: 'shopName', width: 120, ellipsis: true, render: (_, r) => r.shopName || '—' },
      { title: 'SKU', dataIndex: 'skuCode', width: 112, ellipsis: true, render: (_, r) => r.skuCode || '—' },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        search: false,
        render: (_, r) => taskTagFromStatus(r.status),
      },
      {
        title: '错误',
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        render: (_, r) => r.errorMessage || '—',
      },
    ],
    [],
  );

  return (
    <PageContainer title="库存同步批次">
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        批量创建的库存同步任务会归入批次；Worker 完成后按任务聚合回写批次统计。单次创建上限见{' '}
        <Typography.Link href="/settings/inventory">设置 · 库存 / 订单</Typography.Link>
        中的「{INVENTORY_SYNC_BATCH_MAX_SIZE_LABEL}」。
      </Typography.Paragraph>
      <ProTable<InventorySyncBatchDTO>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1400 }}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        headerTitle="批次列表"
        request={async (params) => {
          try {
            const res = await queryInventorySyncBatches({
              page: params.current,
              pageSize: params.pageSize,
              source: params.source as string | undefined,
              status: params.status as string | undefined,
              platform: params.platform as string | undefined,
              shopId: params.shopId as string | undefined,
              productId: params.productId as string | undefined,
              start: typeof params.start === 'string' ? params.start : undefined,
              end: typeof params.end === 'string' ? params.end : undefined,
            });
            return { data: res.items ?? [], total: res.pagination.total, success: true };
          } catch (e: unknown) {
            message.error((e as Error)?.message || '加载失败');
            return { data: [], total: 0, success: false };
          }
        }}
      />

      <Drawer
        width={720}
        title={drawerBatch ? `批次 ${drawerBatch.batchNo}` : '批次详情'}
        open={drawerOpen}
        destroyOnHidden
        onClose={() => {
          setDrawerOpen(false);
          setDrawerBatch(null);
        }}
      >
        <Spin spinning={drawerLoading}>
          {drawerBatch ? (
            <Tabs
              items={[
                {
                  key: 'summary',
                  label: '概要',
                  children: (
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Space wrap>
                        {batchStatusTag(drawerBatch.status)}
                        <Tag>{SOURCE_LABEL[drawerBatch.source] || drawerBatch.source}</Tag>
                        {drawerBatch.platform ? <Tag>{platformLabel(drawerBatch.platform)}</Tag> : null}
                      </Space>
                      <Typography.Paragraph style={{ marginBottom: 0 }}>
                        <Typography.Text strong>计数：</Typography.Text> 总计 {drawerBatch.totalCount} · 待处理{' '}
                        {drawerBatch.pendingCount} · 运行中 {drawerBatch.runningCount} · 成功 {drawerBatch.successCount} · 失败{' '}
                        {drawerBatch.failedCount} · 跳过 {drawerBatch.skippedCount}
                      </Typography.Paragraph>
                      {drawerBatch.skippedReason ? (
                        <Typography.Paragraph>
                          <Typography.Text strong>跳过摘要：</Typography.Text> {drawerBatch.skippedReason}
                        </Typography.Paragraph>
                      ) : null}
                      <Typography.Paragraph style={{ marginBottom: 0 }}>
                        <Typography.Text strong>input（摘要）</Typography.Text>
                      </Typography.Paragraph>
                      <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200 }}>
                        {summarizeJson(drawerBatch.input)}
                      </pre>
                      <Typography.Paragraph style={{ marginBottom: 0 }}>
                        <Typography.Text strong>output（摘要）</Typography.Text>
                      </Typography.Paragraph>
                      <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200 }}>
                        {summarizeJson(drawerBatch.output)}
                      </pre>
                      <Typography.Paragraph style={{ marginBottom: 0 }}>
                        <Typography.Text strong>最近任务（最多 15 条）</Typography.Text>
                      </Typography.Paragraph>
                      <Table<InventorySyncTaskDTO>
                        size="small"
                        pagination={false}
                        rowKey="id"
                        dataSource={drawerBatch.recentTasks ?? []}
                        columns={[
                          {
                            title: '状态',
                            dataIndex: 'status',
                            width: 96,
                            render: (v: string) => taskTagFromStatus(v),
                          },
                          {
                            title: '平台',
                            dataIndex: 'platform',
                            width: 88,
                            render: (_: unknown, row: InventorySyncTaskDTO) => platformLabel(row.platform),
                          },
                          {
                            title: 'SKU',
                            dataIndex: 'skuCode',
                            ellipsis: true,
                            render: (_: unknown, row: InventorySyncTaskDTO) => row.skuCode || row.productSkuId || '—',
                          },
                          {
                            title: '错误',
                            dataIndex: 'errorMessage',
                            ellipsis: true,
                            render: (t: string | undefined) => t || '—',
                          },
                        ]}
                      />
                      <Space wrap>
                        <Button
                          type="primary"
                          onClick={() =>
                            history.push(`/inventory/sync-tasks?batchId=${encodeURIComponent(drawerBatch.id)}`)
                          }
                        >
                          打开任务列表（按批次筛选）
                        </Button>
                        <Popconfirm
                          title="对该批次内失败任务发起重试？"
                          disabled={drawerBatch.failedCount <= 0}
                          onConfirm={async () => {
                            try {
                              await retryInventorySyncBatchFailed(drawerBatch.id);
                              message.success('已提交重试');
                              const b = await getInventorySyncBatch(drawerBatch.id, { recentTasks: 15 });
                              setDrawerBatch(b);
                              actionRef.current?.reload();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '失败');
                            }
                          }}
                        >
                          <Button disabled={drawerBatch.failedCount <= 0}>重试失败项</Button>
                        </Popconfirm>
                      </Space>
                    </Space>
                  ),
                },
                {
                  key: 'tasks',
                  label: '任务分页',
                  children: (
                    <ProTable<InventorySyncTaskDTO>
                      rowKey="id"
                      actionRef={tasksActionRef}
                      search={false}
                      options={false}
                      pagination={{ pageSize: 15, showSizeChanger: true }}
                      columns={taskColumns}
                      request={async (p) => {
                        const res = await queryInventorySyncBatchTasks(drawerBatch.id, {
                          page: p.current,
                          pageSize: p.pageSize,
                        });
                        return { data: res.list, total: res.pagination.total, success: true };
                      }}
                    />
                  ),
                },
              ]}
            />
          ) : null}
        </Spin>
      </Drawer>
    </PageContainer>
  );
}
