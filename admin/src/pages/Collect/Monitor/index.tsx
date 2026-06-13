import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import type { ProColumns } from '@ant-design/pro-components';
import { Link } from '@umijs/renderer-react';
import { Badge, Button, Card, Col, Progress, Row, Space, Statistic, Tag, Tooltip, Typography } from 'antd';
import { useEffect, useState, type ReactNode } from 'react';
import { COLLECT_BATCH_STATUS, COLLECT_TASK_STATUS } from '@/constants/status';
import { CollectTaskEventDrawer } from '@/pages/Collect/components/CollectTaskEventDrawer';
import { type CollectMonitorData, getCollectMonitor } from '@/services/collectMonitor';

const POLL_MS = 5000;


function sumTasks(t: CollectMonitorData['tasks']) {
  const retr = t.retryingCount ?? t.retrying;
  return t.pending + retr + t.running + t.success + t.failed + t.cancelled;
}

function sumBatches(b: CollectMonitorData['batches']) {
  return b.running + b.partialSuccess + b.success + b.failed + b.cancelled;
}

export default function CollectMonitorPage() {
  const [data, setData] = useState<CollectMonitorData | null>(null);
  const [eventDrawerOpen, setEventDrawerOpen] = useState(false);
  const [eventDrawerTaskId, setEventDrawerTaskId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setInterval> | undefined;

    const tick = async () => {
      try {
        const d = await getCollectMonitor();
        if (!cancelled) setData(d);
      } catch {
        // polling errors ignored; last successful snapshot stays visible
      }
    };

    const arm = () => {
      if (timer) clearInterval(timer);
      timer = undefined;
      if (typeof document !== 'undefined' && document.visibilityState !== 'hidden') {
        timer = setInterval(tick, POLL_MS);
      }
    };

    void tick();
    arm();

    const onVis = () => {
      if (typeof document !== 'undefined' && document.visibilityState !== 'hidden') {
        void tick();
      }
      arm();
    };

    document.addEventListener('visibilitychange', onVis);
    return () => {
      cancelled = true;
      if (timer) clearInterval(timer);
      document.removeEventListener('visibilitychange', onVis);
    };
  }, []);

  const failureColumns: ProColumns<CollectMonitorData['recentFailures'][number]>[] = [
    {
      title: '时间',
      dataIndex: 'updatedAt',
      width: 172,
      render: (_, row) => formatDateTime(row.updatedAt),
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 80,
    },
    {
      title: '链接',
      dataIndex: 'sourceUrl',
      ellipsis: true,
      copyable: true,
    },
    {
      title: '批次',
      dataIndex: 'batchId',
      width: 120,
      ellipsis: true,
      render: (_, row) =>
        row.batchId ? (
          <Link to={`/collect/batches?batchId=${encodeURIComponent(row.batchId)}`}>{row.batchId.slice(0, 8)}…</Link>
        ) : (
          '—'
        ),
    },
    {
      title: '错误原因',
      dataIndex: 'errorMessage',
      ellipsis: true,
      render: (_, row) => (
        <Tooltip title={row.errorMessage}>
          <Typography.Text ellipsis style={{ maxWidth: 320 }}>
            {row.errorMessage || '—'}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      render: (_, row) => {
        const actions: ReactNode[] = [
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
        if (row.batchId) {
          actions.push(
            <Link key="batch" to={`/collect/batches?batchId=${encodeURIComponent(row.batchId)}`}>
              跳转批次
            </Link>,
          );
        }
        return actions;
      },
    },
  ];

  const retryingColumns: ProColumns<CollectMonitorData['recentRetrying'][number]>[] = [
    {
      title: '时间',
      dataIndex: 'updatedAt',
      width: 172,
      render: (_, row) => formatDateTime(row.updatedAt),
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 80,
    },
    {
      title: '链接',
      dataIndex: 'sourceUrl',
      ellipsis: true,
      copyable: true,
    },
    {
      title: '重试',
      width: 88,
      render: (_, row) => `${row.retryCount}/${row.maxRetries}`,
    },
    {
      title: '下次重试',
      dataIndex: 'nextRetryAt',
      width: 172,
      render: (_, row) => formatDateTime(row.nextRetryAt),
    },
    {
      title: '错误摘要',
      dataIndex: 'errorMessage',
      ellipsis: true,
      render: (_, row) => (
        <Tooltip title={row.errorMessage}>
          <Typography.Text ellipsis style={{ maxWidth: 280 }}>
            {row.errorMessage || '—'}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 160,
      render: (_, row) => {
        const actions: ReactNode[] = [
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
        if (row.batchId) {
          actions.push(
            <Link key="batch" to={`/collect/batches?batchId=${encodeURIComponent(row.batchId)}`}>
              跳转批次
            </Link>,
          );
        }
        return actions;
      },
    },
  ];

  const q = data?.queue;
  const r = data?.retry;
  const w = data?.worker;
  const col = data?.collector;
  const tasks = data?.tasks;
  const batches = data?.batches;
  const taskTotal = tasks ? sumTasks(tasks) : 0;
  const batchTotal = batches ? sumBatches(batches) : 0;

  return (
    <TmPageContainer title="采集监控" subTitle="查看采集任务排队情况、后台进程与采集服务状态（约每 5 秒刷新）">
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card size="small" variant="outlined">
            <Statistic title="排队任务数" value={q?.depth ?? '—'} suffix={q && !q.redisAvailable ? <Tag color="warning">队列不可用</Tag> : null} />
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
              {q?.name ?? '—'} · {q?.redisAvailable ? '任务队列正常' : '任务队列不可用'}
            </Typography.Paragraph>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card size="small" variant="outlined">
            <Statistic title="同时执行数" value={w?.concurrency ?? '—'} />
            <div style={{ marginTop: 8 }}>
              <Space size="small">
                <Tag color={w?.enabled ? 'blue' : 'default'}>{w?.enabled ? '队列已启用' : '队列未启用'}</Tag>
                <Tag color={w?.running ? 'success' : 'default'}>{w?.running ? '运行中' : '未运行'}</Tag>
              </Space>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card size="small" variant="outlined">
            <Statistic
              title="任务 pending / running / failed"
              value={`${((tasks?.pending ?? 0) + (tasks?.retryingCount ?? tasks?.retrying ?? 0)).toString()} / ${(tasks?.running ?? 0).toString()} / ${(tasks?.failed ?? 0).toString()}`}
            />
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              排队数含等待重试
            </Typography.Text>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card size="small" variant="outlined">
            <div style={{ marginBottom: 4 }}>采集服务</div>
            <Badge status={col?.reachable ? 'success' : 'error'} text={col?.reachable ? '可达' : '不可达'} />
            <Typography.Paragraph ellipsis type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
              {col?.baseUrl ?? '—'}
            </Typography.Paragraph>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} md={12}>
          <Card size="small" variant="outlined" title="自动重试">
            <Space direction="vertical" size="small" style={{ width: '100%' }}>
              <Space wrap>
                <Tag color={r?.enabled ? 'success' : 'default'}>{r?.enabled ? '已开启' : '已关闭'}</Tag>
                <Tag>默认最大次数 {r?.maxRetries ?? '—'}</Tag>
                <Tag>
                  阶梯基准 {r?.baseDelaySeconds ?? '—'}s / 上限 {r?.maxDelaySeconds ?? '—'}s
                </Tag>
              </Space>
              <Typography.Text>
                已到点待入队：<strong>{r?.nextRetryDueCount ?? 0}</strong>
                {r?.oldestRetryingSeconds != null ? (
                  <>
                    {' '}
                    · 等待重试状态最久约 <strong>{r.oldestRetryingSeconds}s</strong>
                  </>
                ) : null}
              </Typography.Text>
            </Space>
          </Card>
        </Col>
      </Row>
      {q?.oldestPendingSeconds != null && (
        <ProCard title="队列积压提示" variant="outlined" style={{ marginBottom: 16 }} size="small">
          <Typography.Text>
            最早 pending / retrying 任务已等待约 <strong>{q.oldestPendingSeconds}s</strong>
          </Typography.Text>
        </ProCard>
      )}

      <ProCard title="任务状态分布" variant="outlined" style={{ marginBottom: 16 }} size="small">
        {!tasks || taskTotal === 0 ? (
          <Typography.Text type="secondary">暂无任务记录</Typography.Text>
        ) : (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            {(
              [
                ['pending', tasks.pending, 'processing'],
                ['retrying', tasks.retrying, 'processing'],
                ['running', tasks.running, 'processing'],
                ['success', tasks.success, 'success'],
                ['failed', tasks.failed, 'error'],
                ['cancelled', tasks.cancelled, 'default'],
              ] as const
            ).map(([key, n, tone]) => {
              const meta = COLLECT_TASK_STATUS[key as keyof typeof COLLECT_TASK_STATUS];
              const pct = Math.round((n / taskTotal) * 1000) / 10;
              return (
                <div key={key}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <Tag color={tone === 'processing' ? 'processing' : tone === 'success' ? 'success' : tone === 'error' ? 'error' : 'default'}>
                      {meta?.text ?? key}
                    </Tag>
                    <span>
                      {n} ({pct}%)
                    </span>
                  </div>
                  <Progress percent={pct} size="small" status={key === 'failed' ? 'exception' : 'normal'} showInfo={false} />
                </div>
              );
            })}
          </Space>
        )}
      </ProCard>

      <ProCard title="批次状态分布" variant="outlined" style={{ marginBottom: 16 }} size="small">
        {!batches || batchTotal === 0 ? (
          <Typography.Text type="secondary">暂无批次记录</Typography.Text>
        ) : (
          <Space wrap size="middle">
            {(
              [
                ['running', batches.running],
                ['partial_success', batches.partialSuccess],
                ['success', batches.success],
                ['failed', batches.failed],
                ['cancelled', batches.cancelled],
              ] as const
            ).map(([key, n]) => {
              const meta = COLLECT_BATCH_STATUS[key as keyof typeof COLLECT_BATCH_STATUS];
              return (
                <Tag key={key} color={meta?.color}>
                  {meta?.text ?? key}: {n}
                </Tag>
              );
            })}
          </Space>
        )}
      </ProCard>

      <ProCard title="最近等待重试（10 条）" variant="outlined" style={{ marginBottom: 16 }} size="small">
        <ProTable<CollectMonitorData['recentRetrying'][number]>
          rowKey="id"
          search={false}
          options={false}
          pagination={false}
          columns={retryingColumns}
          dataSource={data?.recentRetrying ?? []}
          loading={!data}
        />
      </ProCard>

      <ProCard title="最近失败任务（10 条）" variant="outlined" size="small">
        <ProTable<CollectMonitorData['recentFailures'][number]>
          rowKey="id"
          search={false}
          options={false}
          pagination={false}
          columns={failureColumns}
          dataSource={data?.recentFailures ?? []}
          loading={!data}
        />
      </ProCard>

      <CollectTaskEventDrawer
        taskId={eventDrawerTaskId}
        open={eventDrawerOpen}
        onClose={() => {
          setEventDrawerOpen(false);
          setEventDrawerTaskId(null);
        }}
      />
    </TmPageContainer>
  );
}
