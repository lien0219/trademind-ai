import { tr, trFormat } from '@/i18n';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { commonStatusLabel } from '@/constants/copywriting';
import { formatDateTime } from '@/utils/formatTime';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Col, Row, Space, Statistic, Tag, Typography } from 'antd';
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
  { get title() { return tr("采集"); }, dataKey: 'collect' },
  { get title() { return tr("AI 图片"); }, dataKey: 'image' },
  { get title() { return tr("订单同步"); }, dataKey: 'orderSync' },
  { get title() { return tr("客服消息同步"); }, dataKey: 'customerMessageSync' },
  { get title() { return tr("商品刊登"); }, dataKey: 'productPublish' },
  { get title() { return tr("库存同步"); }, dataKey: 'inventorySync' },
];


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
        title: tr("类型"),
        dataIndex: 'workerType',
        width: 120,
        render: (_, row) => workerTypeLabel(row.workerType),
      },
      {
        title: tr("进程 ID"),
        dataIndex: 'workerId',
        ellipsis: true,
        copyable: true,
      },
      {
        title: tr("主机"),
        dataIndex: 'hostname',
        width: 140,
        ellipsis: true,
      },
      {
        title: tr("系统进程号"),
        dataIndex: 'pid',
        width: 80,
      },
      {
        title: tr("状态"),
        dataIndex: 'status',
        width: 110,
        render: (_, row) => statusTag(row.effectiveStatus, row.status),
      },
      {
        title: tr("最近心跳"),
        dataIndex: 'lastHeartbeatAt',
        width: 172,
        render: (_, row) => formatDateTime(row.lastHeartbeatAt),
      },
      {
        title: tr("启动时间"),
        dataIndex: 'startedAt',
        width: 172,
        render: (_, row) => formatDateTime(row.startedAt),
      },
    ],
    [],
  );

  const leaseCols = (): ProColumns<LeasedTaskRow>[] => [
    { title: tr("任务 ID"), dataIndex: 'id', copyable: true, ellipsis: true },
    { title: tr("状态"), dataIndex: 'status', width: 90, render: (_, row) => commonStatusLabel(row.status) },
    { title: tr("锁定者"), dataIndex: 'lockedBy', ellipsis: true },
    {
      title: tr("执行截止"),
      dataIndex: 'lockedUntil',
      width: 172,
      render: (_, r) => formatDateTime(r.lockedUntil || undefined),
    },
    {
      title: tr("更新时间"),
      dataIndex: 'updatedAt',
      width: 172,
      render: (_, r) => formatDateTime(r.updatedAt),
    },
  ];

  const bt = data?.byType ?? {};
  const summary = data?.summary ?? EMPTY_SUMMARY;

  return (
    <TmPageContainer
      title={tr('后台任务监控')}
      subTitle={tr('查看采集、图片处理、订单同步、商品刊登等后台任务是否在正常运行')}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        {trFormat('展示各类型后台任务进程状态与正在执行中的任务（每 {value0} 秒刷新；页面隐藏时暂停）。', { value0: POLL_MS / 1000 })}
      </Typography.Paragraph>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} xl={14}>
          <ProCard variant="outlined" title={tr("失败任务中心快照")} size="small">
            {failSum ? (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Row gutter={[16, 8]}>
                  <Col xs={12} sm={8}>
                    <Statistic title={tr("失败(归一)")} value={failSum.totalFailed ?? 0} />
                  </Col>
                  <Col xs={12} sm={8}>
                    <Statistic title={tr("可重试")} value={failSum.retryableCount ?? 0} />
                  </Col>
                  <Col xs={24} sm={8}>
                    <Statistic
                      title={tr("重试中 / 停滞 / 执行超时")}
                      value={`${failSum.retryingTotal ?? 0}/${failSum.staleTotal ?? 0}/${failSum.leaseExpiredTotal ?? 0}`}
                    />
                  </Col>
                </Row>
                <Row gutter={[16, 8]}>
                  <Col xs={12} sm={8}>
                    <Statistic title={tr("忽略标记")} value={failSum.ignoredCount ?? 0} />
                  </Col>
                  <Col xs={12} sm={8}>
                    <Statistic title={tr("已处理标记")} value={failSum.handledCount ?? 0} />
                  </Col>
                  <Col xs={24} sm={8} style={{ display: 'flex', alignItems: 'flex-end' }}>
                    <Button type="primary" block onClick={() => history.push('/ops/task-center/failures')}>
                      {tr("打开失败任务中心")}
                    </Button>
                  </Col>
                </Row>
              </Space>
            ) : (
              <Typography.Text type="secondary">{tr("载入中...")}</Typography.Text>
            )}
          </ProCard>
        </Col>
        <Col xs={24} xl={10}>
          <ProCard variant="outlined" title={tr("实例状态汇总")} size="small">
            <WorkerStatusMetrics summary={summary} />
          </ProCard>
        </Col>
      </Row>

      <Typography.Title level={5} style={{ marginTop: 0, marginBottom: 12 }}>
        {tr("按任务类型")}
      </Typography.Title>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        {WORKER_MONITOR_TYPE_KEYS.map((k) => {
          const typeSummary = bt[k] ?? EMPTY_SUMMARY;
          const total = (typeSummary.running ?? 0) + (typeSummary.stale ?? 0) + (typeSummary.stopped ?? 0);
          return (
            <Col xs={24} sm={12} lg={8} key={k}>
              <ProCard
                variant="outlined"
                size="small"
                title={TASK_CENTER_TASK_TYPE_LABEL[k] || k}
                extra={
                  <Tag color={total > 0 ? 'processing' : 'default'}>{total} {tr("个实例")}</Tag>
                }
              >
                <WorkerStatusMetrics summary={typeSummary} />
              </ProCard>
            </Col>
          );
        })}
      </Row>

      <ProCard title={tr("后台进程列表")} variant="outlined" style={{ marginBottom: 16 }}>
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
          title={trFormat("租约中的任务 · {value0}", { value0: String(title) })}
          variant="outlined"
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
    </TmPageContainer>
  );
}
