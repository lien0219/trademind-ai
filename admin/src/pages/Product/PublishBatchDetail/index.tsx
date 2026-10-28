import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import PublishBoundaryBanner from '@/components/platform/PublishBoundaryBanner';
import DouyinE2EPrecheckBanner from '@/components/platform/DouyinE2EPrecheckBanner';
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
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import { Link, history, useParams, useSearchParams } from '@umijs/max';
import { normalizeSource } from '@/utils/urlState';
import { Alert, Button, Card, Descriptions, Popconfirm, Space, Table, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

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
  const language = useLanguage();
  const { id } = useParams<{ id: string }>();
  const { can } = usePermission();
  const canRetry = can(PERMISSIONS.TASK_RETRY);
  const canCancel = can(PERMISSIONS.PUBLISH_CREATE_DRAFT);
  const [searchParams] = useSearchParams();
  const navSource = normalizeSource(searchParams.get('source') || undefined);
  const [detail, setDetail] = useState<PublishBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const taskStatusFilter = useMemo(() => [
    { text: tr("成功"), value: 'success' },
    { text: tr("失败"), value: 'failed' },
    { text: tr("等待处理"), value: 'pending' },
    { text: tr("处理中"), value: 'running' },
    { text: tr("已取消"), value: 'cancelled' },
    { text: tr("已跳过"), value: 'skipped' },
  ], [language]);

  const reload = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const res = await getPublishBatch(id);
      setDetail(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载批次失败"));
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
    if (!canRetry) {
      message.error(tr("当前账号无任务重试权限"));
      return;
    }
    if (!id) return;
    setActing(true);
    try {
      await retryFailedPublishBatch(id);
      message.success(tr("已重试失败项"));
      await reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("重试失败"));
    } finally {
      setActing(false);
    }
  };

  const onCancelPending = async () => {
    if (!canCancel) {
      message.error(tr("当前账号无刊登草稿写权限"));
      return;
    }
    if (!id) return;
    setActing(true);
    try {
      const res = await cancelPendingPublishBatch(id);
      setDetail(res);
      message.success(tr("已取消等待中的任务"));
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("取消失败"));
    } finally {
      setActing(false);
    }
  };

  return (
    <TmPageContainer
      title={tr("刊登批次详情")}
      subTitle={detail?.name || trFormat("批次编号 {value0}…", { value0: String(id?.slice(0, 8)) })}
      loading={loading}
      onBack={() => {
        const qs = new URLSearchParams();
        qs.set('tab', 'batches');
        if (navSource) qs.set('source', navSource);
        history.push(`/product/publish-tasks?${qs.toString()}`);
      }}
    >
      {detail && (
        <>
          <DouyinE2EPrecheckBanner blockedByCredentials compact />
          <PublishBoundaryBanner capability="real_draft_create" blockedByCredentials />
          {detail.status === 'partial_success' && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message={tr("部分子任务失败")}
              description={tr("本批次部分商品或目标未能创建草稿，可点击下方「重试失败项」再次尝试。")}
            />
          )}
          <Card style={{ marginBottom: 16 }}>
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2, md: 3 }}>
              <Descriptions.Item label={tr("批次状态")}>
                {batchStatusTag(detail.status, detail.statusLabel || publishBatchStatusLabel(detail.status))}
              </Descriptions.Item>
              <Descriptions.Item label={tr("商品数量")}>{detail.productCount}</Descriptions.Item>
              <Descriptions.Item label={tr("目标数量")}>{detail.targetCount}</Descriptions.Item>
              <Descriptions.Item label={tr("任务总数")}>{detail.taskCount}</Descriptions.Item>
              <Descriptions.Item label={tr("成功")}>{detail.successCount}</Descriptions.Item>
              <Descriptions.Item label={tr("失败")}>{detail.failedCount}</Descriptions.Item>
              <Descriptions.Item label={tr("跳过")}>{detail.skippedCount}</Descriptions.Item>
              <Descriptions.Item label={tr("创建时间")}>{formatDateTime(detail.createdAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("完成时间")}>
                {detail.finishedAt ? formatDateTime(detail.finishedAt) : '—'}
              </Descriptions.Item>
            </Descriptions>
            <Space style={{ marginTop: 16 }} wrap>
              {detail.failedCount > 0 && (canRetry ? (
                <Popconfirm title={tr("只重试本批次中的失败子任务？")} onConfirm={() => void onRetryFailed()}>
                  <Button type="primary" loading={acting}>{tr("重试失败项")}</Button>
                </Popconfirm>
              ) : (
                <Button type="primary" disabled>{tr("重试失败项")}</Button>
              ))}
              {canCancel ? (
                <Popconfirm title={tr("只取消等待中的子任务，进行中的任务不会强杀。")} onConfirm={() => void onCancelPending()}>
                  <Button loading={acting}>{tr("取消等待项")}</Button>
                </Popconfirm>
              ) : (
                <Button disabled>{tr("取消等待项")}</Button>
              )}
              <Link to="/ops/task-center/failures">{tr("失败任务中心")}</Link>
            </Space>
          </Card>

          <Card title={tr("子任务列表")}>
            <Table
              rowKey={(r) => `${r.productId}:${r.targetKey}:${r.taskId || 'skip'}`}
              size="small"
              pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (t) => trFormat("共 {value0} 条", { value0: t }) }}
              dataSource={filteredItems}
              scroll={{ x: 1100 }}
              columns={[
                {
                  title: tr("商品"),
                  dataIndex: 'productTitle',
                  ellipsis: true,
                  width: 160,
                  render: (t: string, r) => (
                    <Link to={`/product/drafts/${r.productId}`}>{t || '—'}</Link>
                  ),
                },
                {
                  title: tr("平台 / 店铺"),
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
                  title: tr("能力"),
                  dataIndex: 'capability',
                  width: 120,
                  render: (v: string) => publishCapabilityLabel(v),
                },
                {
                  title: tr("状态"),
                  dataIndex: 'status',
                  width: 100,
                  filters: taskStatusFilter,
                  filteredValue: statusFilter.length ? statusFilter : null,
                  onFilter: (value, r) => (r.status || '').toLowerCase() === String(value),
                  render: (_: unknown, r) => taskStatusTag(r.status, r.statusLabel),
                },
                {
                  title: tr("失败原因"),
                  dataIndex: 'errorMessage',
                  ellipsis: true,
                  render: (v: string) => v || '—',
                },
                {
                  title: tr("操作"),
                  key: 'ops',
                  width: 200,
                  render: (_: unknown, r) => (
                    <Space wrap>
                      <Link to={`/product/drafts/${r.productId}?tab=publish`}>{tr("平台配置")}</Link>
                      {r.taskId ? (
                        <Link to={`/product/publish-tasks?id=${r.taskId}`}>{tr("查看任务")}</Link>
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
            <TechnicalDetails label={tr("技术详情（批次配置快照）")} style={{ marginTop: 16 }}>
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
