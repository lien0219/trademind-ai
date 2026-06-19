import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import { publishBatchStatusLabel, publishCapabilityLabel } from '@/constants/publishLabels';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import {
  cancelPendingPublishBatch,
  getPublishBatch,
  retryFailedPublishBatch,
  type PublishBatchDetail,
} from '@/services/productPublish';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history, useParams } from '@umijs/max';
import { Button, Card, Descriptions, Popconfirm, Space, Table, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';

function statusTag(status: string, label?: string) {
  const c = COLLECT_TASK_STATUS[status as keyof typeof COLLECT_TASK_STATUS];
  const text = label || c?.text || status;
  return <Tag color={c?.color}>{text}</Tag>;
}

export default function PublishBatchDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<PublishBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);

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
          <Card style={{ marginBottom: 16 }}>
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2, md: 3 }}>
              <Descriptions.Item label="批次状态">
                {statusTag(detail.status, detail.statusLabel || publishBatchStatusLabel(detail.status))}
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
              pagination={{ pageSize: 20, showSizeChanger: true }}
              dataSource={detail.items ?? []}
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
                  render: (_: unknown, r) => statusTag(r.status, r.statusLabel),
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
                  width: 120,
                  render: (_: unknown, r) => (
                    <Space>
                      {r.taskId ? (
                        <Link to={`/product/publish-tasks?id=${r.taskId}`}>查看任务</Link>
                      ) : (
                        <Typography.Text type="secondary">—</Typography.Text>
                      )}
                    </Space>
                  ),
                },
              ]}
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
