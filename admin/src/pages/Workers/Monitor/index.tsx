import { PageContainer, ProCard, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { Col, Row, Statistic, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { useEffect, useMemo, useState } from 'react';
import { type WorkerMonitorData, type WorkerMonitorInstance, getWorkersMonitor } from '@/services/workers';

const POLL_MS = 5000;

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}

function statusTag(eff: string | undefined, raw: string) {
  const v = (eff || raw || '').toLowerCase();
  if (v === 'running') return <Tag color="success">running</Tag>;
  if (v === 'stale') return <Tag color="warning">stale</Tag>;
  if (v === 'stopped') return <Tag>stopped</Tag>;
  return <Tag>{raw}</Tag>;
}

export default function WorkersMonitorPage() {
  const [data, setData] = useState<WorkerMonitorData | null>(null);

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
        width: 100,
      },
      {
        title: 'workerId',
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
        title: 'PID',
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

  const leaseCols = (): ProColumns<WorkerMonitorData['leasedTasks']['collect'][number]>[] => [
    { title: '任务 ID', dataIndex: 'id', copyable: true, ellipsis: true },
    { title: '状态', dataIndex: 'status', width: 90 },
    { title: 'lockedBy', dataIndex: 'lockedBy', ellipsis: true },
    { title: 'lockedUntil', dataIndex: 'lockedUntil', width: 172, render: (_, r) => formatTs(r.lockedUntil || undefined) },
    { title: 'updatedAt', dataIndex: 'updatedAt', width: 172, render: (_, r) => formatTs(r.updatedAt) },
  ];

  const bt = data?.byType ?? {};

  return (
    <PageContainer header={{ title: 'Worker 监控' }}>
      <Typography.Paragraph type="secondary">
        展示队列 Worker 进程注册与任务租约（轮询 {POLL_MS / 1000}s；页面隐藏时暂停）。
      </Typography.Paragraph>

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={8} md={6}>
          <ProCard bordered>
            <Statistic title="running" value={data?.summary.running ?? 0} />
          </ProCard>
        </Col>
        <Col xs={24} sm={8} md={6}>
          <ProCard bordered>
            <Statistic title="stale" value={data?.summary.stale ?? 0} />
          </ProCard>
        </Col>
        <Col xs={24} sm={8} md={6}>
          <ProCard bordered>
            <Statistic title="stopped" value={data?.summary.stopped ?? 0} />
          </ProCard>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 16 }}>
        {(
          [
            'collect',
            'image',
            'order_sync',
            'customer_message_sync',
            'product_publish',
            'inventory_sync',
          ] as const
        ).map((k) => (
          <Col xs={24} sm={8} lg={6} key={k}>
            <ProCard title={`${k} · 实例`} bordered size="small">
              <Statistic title="running" value={bt[k]?.running ?? 0} />
              <Statistic title="stale" value={bt[k]?.stale ?? 0} />
              <Statistic title="stopped" value={bt[k]?.stopped ?? 0} />
            </ProCard>
          </Col>
        ))}
      </Row>

      <ProCard title="Worker 实例" bordered style={{ marginBottom: 16 }}>
        <ProTable<WorkerMonitorInstance>
          rowKey={(r) => r.workerInstanceId || r.workerId}
          columns={columns}
          dataSource={data?.instances ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 20 }}
        />
      </ProCard>

      <ProCard title="租约中的任务（collect）" bordered style={{ marginBottom: 16 }}>
        <ProTable
          rowKey="id"
          columns={leaseCols()}
          dataSource={data?.leasedTasks.collect ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 10 }}
        />
      </ProCard>
      <ProCard title="租约中的任务（image）" bordered style={{ marginBottom: 16 }}>
        <ProTable
          rowKey="id"
          columns={leaseCols()}
          dataSource={data?.leasedTasks.image ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 10 }}
        />
      </ProCard>
      <ProCard title="租约中的任务（order sync）" bordered style={{ marginBottom: 16 }}>
        <ProTable
          rowKey="id"
          columns={leaseCols()}
          dataSource={data?.leasedTasks.orderSync ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 10 }}
        />
      </ProCard>
      <ProCard title="租约中的任务（customer message sync）" bordered style={{ marginBottom: 16 }}>
        <ProTable
          rowKey="id"
          columns={leaseCols()}
          dataSource={data?.leasedTasks.customerMessageSync ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 10 }}
        />
      </ProCard>
      <ProCard title="租约中的任务（product publish）" bordered style={{ marginBottom: 16 }}>
        <ProTable
          rowKey="id"
          columns={leaseCols()}
          dataSource={data?.leasedTasks.productPublish ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 10 }}
        />
      </ProCard>
      <ProCard title="租约中的任务（inventory sync）" bordered>
        <ProTable
          rowKey="id"
          columns={leaseCols()}
          dataSource={data?.leasedTasks.inventorySync ?? []}
          search={false}
          options={false}
          pagination={{ pageSize: 10 }}
        />
      </ProCard>
    </PageContainer>
  );
}
