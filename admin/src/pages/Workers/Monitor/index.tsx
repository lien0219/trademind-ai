import { PageContainer, ProCard, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Col, Row, Space, Statistic, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';
import {
  TASK_CENTER_TASK_TYPE_LABEL,
  WORKER_EFFECTIVE_STATUS,
  WORKER_MONITOR_TYPE_KEYS,
  WORKER_STATUS_METRIC,
  workerTypeLabel,
} from '@/constants/taskCenter';
import { queryTaskCenterSummary, type FailuresSummary } from '@/services/taskCenter';
import {
  type LeasedTaskRow,
  type WorkerMonitorData,
  type WorkerMonitorInstance,
  type WorkerMonitorSummary,
  getWorkersMonitor,
} from '@/services/workers';

const POLL_MS = 5000;

const EMPTY_SUMMARY: WorkerMonitorSummary = { running: 0, stale: 0, stopped: 0 };

const LEASE_SECTIONS: {
  title: string;
  dataKey: keyof WorkerMonitorData['leasedTasks'];
}[] = [
  { title: '采集', dataKey: 'collect' },
  { title: 'AI 图片', dataKey: 'image' },
  { title: '订单同步', dataKey: 'orderSync' },
  { title: '客服消息同步', dataKey: 'customerMessageSync' },
  { title: '商品刊登', dataKey: 'productPublish' },
  { title: '库存同步', dataKey: 'inventorySync' },
];

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}

function statusTag(eff: string | undefined, raw: string) {
  const v = (eff || raw || '').trim().toLowerCase();
  const m = WORKER_EFFECTIVE_STATUS[v];
  if (!m) return <Tag>{raw || '—'}</Tag>;
  return <Tag color={m.color}>{m.text}</Tag>;
}

function WorkerStatusMetrics({ summary }: { summary: WorkerMonitorSummary }) {
  return (
    <Row gutter={[8, 8]}>
      {(['running', 'stale', 'stopped'] as const).map((key) => {
        const meta = WORKER_STATUS_METRIC[key];
        return (
          <Col xs={8} key={key}>
            <Statistic
              title={meta.text}
              value={summary[key] ?? 0}
              valueStyle={{ fontSize: 22, fontWeight: 600, color: meta.valueStyle }}
            />
          </Col>
        );
      })}
    </Row>
  );
}

export default function WorkersMonitorPage() {
  const [data, setData] = useState<WorkerMonitorData | null>(null);
  const [failSum, setFailSum] = useState<FailuresSummary | null>(null);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setInterval> | undefined;

    const tick = async () => {
      try {
        const d = await getWorkersMonitor();
        if (!cancelled) setData(d);
      } catch {
        /* keep last snapshot */
      }
      try {
        const s = await queryTaskCenterSummary();
        if (!cancelled) setFailSum(s);
      } catch {
        /* ignore */
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

  const columns: ProColumns<WorkerMonitorInstance>[] = useMemo(
    () => [
      {
        title: '类型',
        dataIndex: 'workerType',
        width: 120,
        render: (_, row) => workerTypeLabel(row.workerType),
      },
      {
        title: '进程 ID',
        dataIndex: 'workerId',
        ellipsis: true,
        copyable: true,
      },
      {
        title: '主机',
        dataIndex: 'hostname',
        width: 140,
        ellipsis: true,
      },
      {
        title: '系统进程号',
        dataIndex: 'pid',
        width: 80,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 110,
        render: (_, row) => statusTag(row.effectiveStatus, row.status),
      },
      {
        title: '最近心跳',
        dataIndex: 'lastHeartbeatAt',
        width: 172,
        render: (_, row) => formatTs(row.lastHeartbeatAt),
      },
      {
        title: '启动时间',
        dataIndex: 'startedAt',
        width: 172,
        render: (_, row) => formatTs(row.startedAt),
      },
    ],
    [],
  );

  const leaseCols = (): ProColumns<LeasedTaskRow>[] => [
    { title: '任务 ID', dataIndex: 'id', copyable: true, ellipsis: true },
    { title: '状态', dataIndex: 'status', width: 90 },
    { title: '锁定者', dataIndex: 'lockedBy', ellipsis: true },
    {
      title: '执行截止',
      dataIndex: 'lockedUntil',
      width: 172,
      render: (_, r) => formatTs(r.lockedUntil || undefined),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      width: 172,
      render: (_, r) => formatTs(r.updatedAt),
    },
  ];

  const bt = data?.byType ?? {};
  const summary = data?.summary ?? EMPTY_SUMMARY;

  return (
    <PageContainer header={{ title: '后台任务监控', subTitle: '查看采集、图片、订单同步等后台进程是否在正常运行' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        展示各类型后台任务进程状态与正在执行中的任务（每 {POLL_MS / 1000} 秒刷新；页面隐藏时暂停）。
      </Typography.Paragraph>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} xl={14}>
          <ProCard bordered title="失败任务中心快照" size="small">
            {failSum ? (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Row gutter={[16, 8]}>
                  <Col xs={12} sm={8}>
                    <Statistic title="失败(归一)" value={failSum.totalFailed ?? 0} />
                  </Col>
                  <Col xs={12} sm={8}>
                    <Statistic title="可重试" value={failSum.retryableCount ?? 0} />
                  </Col>
                  <Col xs={24} sm={8}>
                    <Statistic
                      title="重试中 / 停滞 / 执行超时"
                      value={`${failSum.retryingTotal ?? 0}/${failSum.staleTotal ?? 0}/${failSum.leaseExpiredTotal ?? 0}`}
                    />
                  </Col>
                </Row>
                <Row gutter={[16, 8]}>
                  <Col xs={12} sm={8}>
                    <Statistic title="忽略标记" value={failSum.ignoredCount ?? 0} />
                  </Col>
                  <Col xs={12} sm={8}>
                    <Statistic title="已处理标记" value={failSum.handledCount ?? 0} />
                  </Col>
                  <Col xs={24} sm={8} style={{ display: 'flex', alignItems: 'flex-end' }}>
                    <Button type="primary" block onClick={() => history.push('/ops/task-center/failures')}>
                      打开失败任务中心
                    </Button>
                  </Col>
                </Row>
              </Space>
            ) : (
              <Typography.Text type="secondary">载入中...</Typography.Text>
            )}
          </ProCard>
        </Col>
        <Col xs={24} xl={10}>
          <ProCard bordered title="实例状态汇总" size="small">
            <WorkerStatusMetrics summary={summary} />
          </ProCard>
        </Col>
      </Row>

      <Typography.Title level={5} style={{ marginTop: 0, marginBottom: 12 }}>
        按任务类型
      </Typography.Title>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        {WORKER_MONITOR_TYPE_KEYS.map((k) => {
          const typeSummary = bt[k] ?? EMPTY_SUMMARY;
          const total = (typeSummary.running ?? 0) + (typeSummary.stale ?? 0) + (typeSummary.stopped ?? 0);
          return (
            <Col xs={24} sm={12} lg={8} key={k}>
              <ProCard
                bordered
                size="small"
                title={TASK_CENTER_TASK_TYPE_LABEL[k] || k}
                extra={
                  <Tag color={total > 0 ? 'processing' : 'default'}>{total} 个实例</Tag>
                }
              >
                <WorkerStatusMetrics summary={typeSummary} />
              </ProCard>
            </Col>
          );
        })}
      </Row>

      <ProCard title="后台进程列表" bordered style={{ marginBottom: 16 }}>
        <ProTable<WorkerMonitorInstance>
          rowKey={(r) => r.workerInstanceId || r.workerId}
          columns={columns}
          dataSource={data?.instances ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 20 }}
        />
      </ProCard>

      {LEASE_SECTIONS.map(({ title, dataKey }) => (
        <ProCard
          key={dataKey}
          title={`租约中的任务 · ${title}`}
          bordered
          style={{ marginBottom: 16 }}
          size="small"
        >
          <ProTable<LeasedTaskRow>
            rowKey="id"
            columns={leaseCols()}
            dataSource={data?.leasedTasks[dataKey] ?? []}
            search={false}
            options={false}
            pagination={{ pageSize: 10 }}
          />
        </ProCard>
      ))}
    </PageContainer>
  );
}
