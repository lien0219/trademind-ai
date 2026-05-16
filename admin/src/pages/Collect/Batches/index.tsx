import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProCard, ProTable } from '@ant-design/pro-components';
import { Link, history, useLocation } from '@umijs/max';
import { Button, Drawer, Form, Input, message, Space, Tag } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { COLLECT_BATCH_STATUS, COLLECT_TASK_STATUS } from '@/constants/status';
import { CollectTaskEventDrawer } from '@/pages/Collect/components/CollectTaskEventDrawer';
import {
  createCollectBatch,
  getCollectBatch,
  queryCollectBatches,
  queryCollectBatchTasks,
  retryFailedBatchTasks,
  type CollectBatchRow,
} from '@/services/collectBatches';
import { retryCollectTask, type CollectTaskRow } from '@/services/collectTasks';

const POLL_MS = 5000;

function countDedupedLines(raw: string): number {
  const seen = new Set<string>();
  for (const line of raw.split(/\n/)) {
    const u = line.trim();
    if (!u) continue;
    seen.add(u.toLowerCase());
  }
  return seen.size;
}

function parseUrlsFromTextarea(raw: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const line of raw.split(/\n/)) {
    const u = line.trim();
    if (!u) continue;
    const k = u.toLowerCase();
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(u);
  }
  return out;
}

export default function CollectBatchesPage() {
  const location = useLocation();
  const actionRef = useRef<ActionType>();
  const taskActionRef = useRef<ActionType>();
  const [form] = Form.useForm<{ source: string; urls: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [polling, setPolling] = useState(POLL_MS);
  const [taskPolling, setTaskPolling] = useState<number | undefined>(undefined);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [activeBatch, setActiveBatch] = useState<CollectBatchRow | null>(null);
  const [eventDrawerOpen, setEventDrawerOpen] = useState(false);
  const [eventDrawerTaskId, setEventDrawerTaskId] = useState<string | null>(null);
  const urlsWatch = Form.useWatch('urls', form) as string | undefined;

  const displayCount = useMemo(() => countDedupedLines(urlsWatch ?? ''), [urlsWatch]);

  useEffect(() => {
    const sync = () => setPolling(document.visibilityState === 'hidden' ? undefined : POLL_MS);
    sync();
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, []);

  useEffect(() => {
    const sync = () =>
      setTaskPolling(document.visibilityState === 'hidden' || !drawerOpen ? undefined : POLL_MS);
    sync();
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, [drawerOpen]);

  const openDrawer = useCallback((row: CollectBatchRow) => {
    setActiveBatch(row);
    setDrawerOpen(true);
  }, []);

  const closeDrawer = () => {
    setDrawerOpen(false);
    setActiveBatch(null);
  };

  useEffect(() => {
    const q = new URLSearchParams(location.search || '');
    const bid = q.get('batchId')?.trim();
    if (!bid) return;
    let cancelled = false;
    void (async () => {
      try {
        const row = await getCollectBatch(bid);
        if (cancelled) return;
        openDrawer(row);
      } catch {
        message.error('批次不存在或暂不可用');
      } finally {
        if (!cancelled) {
          q.delete('batchId');
          const qs = q.toString();
          history.replace(`${location.pathname}${qs ? `?${qs}` : ''}`);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [location.search, location.pathname, openDrawer]);

  const batchColumns: ProColumns<CollectBatchRow>[] = [
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.createdAt),
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 88,
      valueType: 'select',
      valueEnum: { '1688': { text: '1688' } },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 112,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(COLLECT_BATCH_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => {
        const m = COLLECT_BATCH_STATUS[row.status as keyof typeof COLLECT_BATCH_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: '总数',
      dataIndex: 'totalCount',
      width: 64,
      search: false,
    },
    {
      title: '排队',
      dataIndex: 'pendingCount',
      width: 64,
      search: false,
    },
    {
      title: '执行中',
      dataIndex: 'runningCount',
      width: 76,
      search: false,
    },
    {
      title: '成功',
      dataIndex: 'successCount',
      width: 64,
      search: false,
    },
    {
      title: '失败',
      dataIndex: 'failedCount',
      width: 64,
      search: false,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      search: false,
      render: (_, row) => [
        <Button key="view" type="link" size="small" onClick={() => openDrawer(row)}>
          查看任务
        </Button>,
        <Button
          key="retry"
          type="link"
          size="small"
          disabled={row.failedCount <= 0}
          onClick={async () => {
            if (row.failedCount <= 0) return;
            try {
              const r = await retryFailedBatchTasks(row.id);
              message.success(`已重新入队 ${r.retried} 条失败任务`);
              actionRef.current?.reload();
            } catch (e) {
              message.error(e instanceof Error ? e.message : '重试失败');
            }
          }}
        >
          重试失败
        </Button>,
      ],
    },
  ];

  const taskColumns: ProColumns<CollectTaskRow>[] = [
    {
      title: '链接',
      dataIndex: 'sourceUrl',
      ellipsis: true,
      copyable: true,
      search: false,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      valueType: 'select',
      valueEnum: {
        pending: { text: '排队' },
        running: { text: '执行中' },
        retrying: { text: '等待重试' },
        success: { text: '成功' },
        failed: { text: '失败' },
        cancelled: { text: '取消' },
      },
      render: (_, row) => {
        const m = COLLECT_TASK_STATUS[row.status as keyof typeof COLLECT_TASK_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: '重试',
      width: 100,
      search: false,
      render: (_, row) => (
        <span>
          {row.retryCount ?? 0}/{row.maxRetries ?? '—'}
        </span>
      ),
    },
    {
      title: '下次重试',
      dataIndex: 'nextRetryAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.nextRetryAt),
    },
    {
      title: '商品草稿',
      dataIndex: 'resultProductId',
      width: 220,
      search: false,
      render: (_, row) =>
        row.resultProductId ? (
          <Link to={`/product/drafts/${row.resultProductId}`}>{row.resultProductId}</Link>
        ) : (
          '—'
        ),
    },
    {
      title: '错误',
      dataIndex: 'errorMessage',
      ellipsis: true,
      search: false,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 160,
      search: false,
      render: (_, row) => {
        const actions = [
          <Button
            key="ev"
            type="link"
            size="small"
            onClick={() => {
              setEventDrawerTaskId(row.id);
              setEventDrawerOpen(true);
            }}
          >
            事件
          </Button>,
        ];
        if (row.status === 'failed') {
          actions.push(
            <Button
              key="r1"
              type="link"
              size="small"
              onClick={async () => {
                try {
                  await retryCollectTask(row.id);
                  message.success('已重新入队');
                  taskActionRef.current?.reload();
                  actionRef.current?.reload();
                } catch (e) {
                  message.error(e instanceof Error ? e.message : '重试失败');
                }
              }}
            >
              重试
            </Button>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer title="批量采集">
      <ProCard bordered style={{ marginBottom: 16 }} bodyStyle={{ paddingBottom: 8 }}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{ source: '1688', urls: '' }}
          onFinish={async (vals) => {
            const urls = parseUrlsFromTextarea(vals.urls ?? '');
            if (urls.length === 0) {
              message.warning('请至少填写一条链接');
              return;
            }
            setSubmitting(true);
            try {
              const res = await createCollectBatch({ source: vals.source?.trim() || '1688', urls });
              message.success(`批量采集任务已提交，共 ${res.taskCount} 条，正在后台处理`);
              form.setFieldsValue({ urls: '' });
              actionRef.current?.reload();
            } catch (e) {
              message.error(e instanceof Error ? e.message : '提交失败');
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Form.Item label="来源" name="source" rules={[{ required: true }]}>
              <Input style={{ maxWidth: 200 }} placeholder="1688" />
            </Form.Item>
            <Form.Item
              label={
                <span>
                  商品链接（每行一条） <Tag>当前 {displayCount} 条</Tag>
                </span>
              }
              name="urls"
              rules={[{ required: true, message: '请填写链接' }]}
            >
              <Input.TextArea
                rows={12}
                placeholder={
                  'https://detail.1688.com/offer/...\nhttps://detail.1688.com/offer/...'
                }
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" loading={submitting}>
                提交批量采集
              </Button>
            </Form.Item>
          </Space>
        </Form>
      </ProCard>

      <ProTable<CollectBatchRow>
        rowKey="id"
        actionRef={actionRef}
        columns={batchColumns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        polling={polling}
        headerTitle="批次列表"
        request={async (params) => {
          const res = await queryCollectBatches({
            page: params.current,
            pageSize: params.pageSize,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />

      <Drawer
        title={activeBatch ? `批次 · ${activeBatch.source} · ${activeBatch.id.slice(0, 8)}…` : '批次任务'}
        placement="right"
        width={960}
        open={drawerOpen}
        onClose={closeDrawer}
        destroyOnClose
      >
        {activeBatch && (
          <>
            <ProCard bordered size="small" style={{ marginBottom: 16 }} bodyStyle={{ padding: '12px 16px' }}>
              <Space wrap size="middle">
                <Tag>总数 {activeBatch.totalCount}</Tag>
                <Tag>排队 {activeBatch.pendingCount}</Tag>
                <Tag>执行中 {activeBatch.runningCount}</Tag>
                <Tag color="success">成功 {activeBatch.successCount}</Tag>
                <Tag color="error">失败 {activeBatch.failedCount}</Tag>
                <Tag>取消 {activeBatch.cancelledCount}</Tag>
              </Space>
            </ProCard>

            <ProTable<CollectTaskRow>
              rowKey="id"
              actionRef={taskActionRef}
              columns={taskColumns}
              search={{ filterType: 'light' }}
              pagination={{ defaultPageSize: 20, showSizeChanger: true }}
              options={{ reload: true }}
              polling={taskPolling}
              headerTitle={false}
              toolBarRender={() => []}
              request={async (params) => {
                const res = await queryCollectBatchTasks(activeBatch.id, {
                  page: params.current,
                  pageSize: params.pageSize,
                  status: params.status as string | undefined,
                });
                try {
                  const snap = await getCollectBatch(activeBatch.id);
                  const bid = activeBatch.id;
                  setActiveBatch((cur) => (cur && cur.id === bid ? { ...snap } : cur));
                } catch {
                  // ignore stale header refresh
                }
                return {
                  data: res.list,
                  success: true,
                  total: res.pagination.total,
                };
              }}
            />
          </>
        )}
      </Drawer>

      <CollectTaskEventDrawer
        taskId={eventDrawerTaskId}
        open={eventDrawerOpen}
        onClose={() => {
          setEventDrawerOpen(false);
          setEventDrawerTaskId(null);
        }}
      />
    </PageContainer>
  );
}

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}
