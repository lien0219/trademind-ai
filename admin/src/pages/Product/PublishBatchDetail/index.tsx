import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import {
  publishBatchStatusLabel,
  publishBatchStatusTag,
  publishCapabilityLabel,
  publishTargetStatusLabel,
} from '@/constants/publishLabels';
import {
  cancelPendingPublishBatch,
  getPublishBatch,
  retryFailedPublishBatch,
  type PublishBatchDetail,
} from '@/services/productPublish';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history, useParams } from '@umijs/max';
import { Alert, Button, Card, Descriptions, Popconfirm, Space, Table, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

const TASK_STATUS_FILTER = [
  { text: '成功', value: 'success' },
  { text: '失败', value: 'failed' },
  { text: '等待处理', value: 'pending' },
  { text: '处理中', value: 'running' },
  { text: '已取消', value: 'cancelled' },
  { text: '已跳过', value: 'skipped' },
];

function batchStatusTag(status: string, label?: string) {
  const meta = publishBatchStatusTag(status, label);
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

function taskStatusTag(status: string, label?: string) {
  const k = (status || '').trim().toLowerCase();
  if (k === 'skipped') {
    return <Tag>{publishTargetStatusLabel('skipped')}</Tag>;
  }
  const meta = publishBatchStatusTag(k, label);
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

export default function PublishBatchDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<PublishBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);

  const reload = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const res = await getPublishBatch(id);
      setDetail(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载批次失败');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const filteredItems = useMemo(() => {
    const items = detail?.items ?? [];
    if (!statusFilter.length) return items;
    return items.filter((it) => statusFilter.includes((it.status || '').toLowerCase()));
  }, [detail?.items, statusFilter]);

  const onRetryFailed = async () => {
    if (!id) return;
    setActing(true);
    try {
      await retryFailedPublishBatch(id);
      message.success('已重试失败项');
      await reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '重试失败');
    } finally {
      setActing(false);
    }
  };

  const onCancelPending = async () => {
    if (!id) return;
    setActing(true);
    try {
      const res = await cancelPendingPublishBatch(id);
      setDetail(res);
      message.success('已取消等待中的任务');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '取消失败');
    } finally {
      setActing(false);
    }
  };

  return (
    <TmPageContainer
      title="刊登批次详情"
      subTitle={detail?.name || `批次编号 ${id?.slice(0, 8)}…`}
      loading={loading}
      onBack={() => history.push('/product/publish-tasks?tab=batches')}
    >
      {detail && (
        <>
          {detail.status === 'partial_success' && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message="部分子任务失败"
              description="本批次部分商品或目标未能创建草稿，可点击下方「重试失败项」再次尝试。"
            />
          )}
          <Card style={{ marginBottom: 16 }}>
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2, md: 3 }}>
              <Descriptions.Item label="批次状态">
                {batchStatusTag(detail.status, detail.statusLabel || publishBatchStatusLabel(detail.status))}
              </Descriptions.Item>
              <Descriptions.Item label="商品数量">{detail.productCount}</Descriptions.Item>
              <Descriptions.Item label="目标数量">{detail.targetCount}</Descriptions.Item>
              <Descriptions.Item label="任务总数">{detail.taskCount}</Descriptions.Item>
              <Descriptions.Item label="成功">{detail.successCount}</Descriptions.Item>
              <Descriptions.Item label="失败">{detail.failedCount}</Descriptions.Item>
              <Descriptions.Item label="跳过">{detail.skippedCount}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(detail.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="完成时间">
                {detail.finishedAt ? formatDateTime(detail.finishedAt) : '—'}
              </Descriptions.Item>
            </Descriptions>
            <Space style={{ marginTop: 16 }} wrap>
              {detail.failedCount > 0 && (
                <Popconfirm title="只重试本批次中的失败子任务？" onConfirm={() => void onRetryFailed()}>
                  <Button type="primary" loading={acting}>
                    重试失败项
                  </Button>
                </Popconfirm>
              )}
              <Popconfirm title="只取消等待中的子任务，进行中的任务不会强杀。" onConfirm={() => void onCancelPending()}>
                <Button loading={acting}>取消等待项</Button>
              </Popconfirm>
              <Link to="/ops/task-center/failures">失败任务中心</Link>
            </Space>
          </Card>

          <Card title="子任务列表">
            <Table
              rowKey={(r) => `${r.productId}:${r.targetKey}:${r.taskId || 'skip'}`}
              size="small"
              pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (t) => `共 ${t} 条` }}
              dataSource={filteredItems}
              scroll={{ x: 1100 }}
              columns={[
                {
                  title: '商品',
                  dataIndex: 'productTitle',
                  ellipsis: true,
                  width: 160,
                  render: (t: string, r) => (
                    <Link to={`/product/drafts/${r.productId}`}>{t || '—'}</Link>
                  ),
                },
                {
                  title: '平台 / 店铺',
                  key: 'target',
                  width: 160,
                  render: (_: unknown, r) => (
                    <span>
                      {r.platformLabel}
                      {r.shopName ? ` / ${r.shopName}` : ''}
                    </span>
                  ),
                },
                {
                  title: '能力',
                  dataIndex: 'capability',
                  width: 120,
                  render: (v: string) => publishCapabilityLabel(v),
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 100,
                  filters: TASK_STATUS_FILTER,
                  filteredValue: statusFilter.length ? statusFilter : null,
                  onFilter: (value, r) => (r.status || '').toLowerCase() === String(value),
                  render: (_: unknown, r) => taskStatusTag(r.status, r.statusLabel),
                },
                {
                  title: '失败原因',
                  dataIndex: 'errorMessage',
                  ellipsis: true,
                  render: (v: string) => v || '—',
                },
                {
                  title: '操作',
                  key: 'ops',
                  width: 200,
                  render: (_: unknown, r) => (
                    <Space wrap>
                      <Link to={`/product/drafts/${r.productId}?tab=publish`}>平台配置</Link>
                      {r.taskId ? (
                        <Link to={`/product/publish-tasks?id=${r.taskId}`}>查看任务</Link>
                      ) : (
                        <Typography.Text type="secondary">—</Typography.Text>
                      )}
                    </Space>
                  ),
                },
              ]}
              onChange={(_, filters) => {
                const st = filters.status;
                if (Array.isArray(st)) {
                  setStatusFilter(st.map(String));
                } else {
                  setStatusFilter([]);
                }
              }}
            />
          </Card>

          {detail.input && (
            <TechnicalDetails label="技术详情（批次配置快照）" style={{ marginTop: 16 }}>
              <pre style={{ fontSize: 12, margin: 0, maxHeight: 240, overflow: 'auto' }}>
                {JSON.stringify(detail.input, null, 2)}
              </pre>
            </TechnicalDetails>
          )}
        </>
      )}
    </TmPageContainer>
  );
}
