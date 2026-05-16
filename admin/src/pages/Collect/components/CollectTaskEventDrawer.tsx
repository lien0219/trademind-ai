import { Link } from '@umijs/renderer-react';
import {
  Collapse,
  Descriptions,
  Drawer,
  Space,
  Spin,
  Tag,
  Timeline,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import { useEffect, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import {
  fetchCollectTask,
  queryCollectTaskEvents,
  type CollectTaskEventRow,
  type CollectTaskRow,
} from '@/services/collectTasks';

export type CollectTaskEventDrawerProps = {
  taskId: string | null;
  open: boolean;
  onClose: () => void;
};

function fmtTime(s?: string | null) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}

function eventTagColor(ev: string): string | undefined {
  switch (ev) {
    case 'task.success':
      return 'success';
    case 'task.failed':
    case 'task.retry_exhausted':
      return 'error';
    case 'task.running':
      return 'processing';
    case 'task.auto_retry_scheduled':
    case 'task.auto_retry_enqueued':
    case 'task.manual_retry':
      return 'warning';
    default:
      return undefined;
  }
}

function statusTag(status?: string | null) {
  if (!status) return '—';
  const m = COLLECT_TASK_STATUS[status as keyof typeof COLLECT_TASK_STATUS];
  return <Tag color={m?.color}>{m?.text ?? status}</Tag>;
}

export function CollectTaskEventDrawer(props: CollectTaskEventDrawerProps) {
  const { taskId, open, onClose } = props;
  const [loading, setLoading] = useState(false);
  const [task, setTask] = useState<CollectTaskRow | null>(null);
  const [events, setEvents] = useState<CollectTaskEventRow[]>([]);

  useEffect(() => {
    if (!open || !taskId) {
      setTask(null);
      setEvents([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    void (async () => {
      try {
        const [tRow, ev] = await Promise.all([
          fetchCollectTask(taskId),
          queryCollectTaskEvents(taskId, { page: 1, pageSize: 100 }),
        ]);
        if (!cancelled) {
          setTask(tRow);
          setEvents(ev.list ?? []);
        }
      } catch (e) {
        if (!cancelled) {
          message.error(e instanceof Error ? e.message : '加载失败');
          setTask(null);
          setEvents([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, taskId]);

  return (
    <Drawer
      title={task ? `任务事件 · ${task.id.slice(0, 8)}…` : '任务事件'}
      width={620}
      open={open && !!taskId}
      onClose={onClose}
      destroyOnClose
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin />
        </div>
      ) : task ? (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="来源">{task.source}</Descriptions.Item>
            <Descriptions.Item label="链接">
              <Typography.Paragraph style={{ marginBottom: 0 }} copyable ellipsis={{ rows: 2 }}>
                {task.sourceUrl}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label="批次">
              {task.batchId ? (
                <Link to={`/collect/batches?batchId=${encodeURIComponent(task.batchId)}`}>{task.batchId}</Link>
              ) : (
                '—'
              )}
            </Descriptions.Item>
            <Descriptions.Item label="状态">{statusTag(task.status)}</Descriptions.Item>
            <Descriptions.Item label="草稿">
              {task.resultProductId ? (
                <Link to={`/product/drafts/${task.resultProductId}`}>{task.resultProductId}</Link>
              ) : (
                '—'
              )}
            </Descriptions.Item>
            <Descriptions.Item label="自动重试">
              {(task.retryCount ?? 0).toString()}/{task.maxRetries ?? '—'} · 下次{' '}
              <Typography.Text type="secondary">{fmtTime(task.nextRetryAt)}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="当前错误">
              {task.errorMessage ? (
                <Typography.Text type="danger">{task.errorMessage}</Typography.Text>
              ) : (
                '—'
              )}
            </Descriptions.Item>
          </Descriptions>

          <Typography.Title level={5} style={{ marginTop: 8, marginBottom: 0 }}>
            事件时间线
          </Typography.Title>

          {!events?.length ? (
            <Typography.Text type="secondary">暂无事件记录</Typography.Text>
          ) : (
            <Timeline
              items={events.map((ev) => ({
                color: eventTagColor(ev.eventType),
                children: (
                  <div>
                    <Space wrap align="center" style={{ marginBottom: 6 }}>
                      <Typography.Text strong type="secondary">
                        {fmtTime(ev.createdAt)}
                      </Typography.Text>
                      <Tag color={eventTagColor(ev.eventType)}>{ev.eventType}</Tag>
                      <Typography.Text type="secondary">
                        {(ev.fromStatus ?? '—') + ' → ' + (ev.toStatus ?? '—')}
                      </Typography.Text>
                    </Space>
                    {ev.message ? (
                      <Typography.Paragraph style={{ marginBottom: 4 }}>{ev.message}</Typography.Paragraph>
                    ) : null}
                    {(ev.retryCount != null || ev.maxRetries != null || ev.nextRetryAt) && (
                      <Typography.Paragraph type="secondary" style={{ marginBottom: 4, fontSize: 12 }}>
                        重试 {ev.retryCount ?? '—'} / {ev.maxRetries ?? '—'}
                        {ev.nextRetryAt ? ` · next ${fmtTime(ev.nextRetryAt)}` : ''}
                      </Typography.Paragraph>
                    )}
                    {ev.errorMessage ? (
                      <Typography.Text type="danger" style={{ display: 'block', marginBottom: 6 }}>
                        {ev.errorMessage}
                      </Typography.Text>
                    ) : null}
                    {ev.payload !== undefined &&
                    ev.payload !== null &&
                    typeof ev.payload === 'object' &&
                    Object.keys(ev.payload as object).length ? (
                      <Collapse
                        bordered={false}
                        size="small"
                        items={[
                          {
                            key: 'payload',
                            label: 'payload',
                            children: (
                              <pre style={{ margin: 0, maxHeight: 220, overflow: 'auto', fontSize: 12 }}>
                                {JSON.stringify(ev.payload, null, 2)}
                              </pre>
                            ),
                          },
                        ]}
                      />
                    ) : null}
                  </div>
                ),
              }))}
            />
          )}
        </Space>
      ) : (
        <Typography.Text type="secondary">无数据</Typography.Text>
      )}
    </Drawer>
  );
}
