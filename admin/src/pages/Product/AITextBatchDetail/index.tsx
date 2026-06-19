import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import ReviewItemModal from '@/pages/Product/AITextBatch/components/ReviewItemModal';
import {
  AI_TEXT_REVIEW_FILTERS,
  aiTextBatchStatusTag,
  aiTextItemStatusTag,
} from '@/constants/aiProductText';
import {
  applyAiProductTextItem,
  applyAiProductTextSelected,
  fetchAiProductTextBatchDetail,
  rejectAiProductTextItem,
  regenerateAiProductTextItem,
  retryAiProductTextBatchFailed,
  undoAiProductTextBatchApplied,
  updateAiProductTextEditedText,
  type AIProductTextBatchDetail,
  type AIProductTextItemRow,
} from '@/services/aiProductText';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history, useParams } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Modal,
  Popconfirm,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

export default function AITextBatchDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<AIProductTextBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [statusFilter, setStatusFilter] = useState('all');
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [reviewItem, setReviewItem] = useState<AIProductTextItemRow | null>(null);
  const [reviewOpen, setReviewOpen] = useState(false);

  const reload = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const sf = statusFilter === 'all' ? undefined : statusFilter;
      const res = await fetchAiProductTextBatchDetail(id, sf);
      setDetail(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [id, statusFilter]);

  useEffect(() => {
    void reload();
    const timer = setInterval(() => {
      if (detail?.status === 'running') void reload();
    }, 5000);
    return () => clearInterval(timer);
  }, [reload, detail?.status]);

  const reviewableIds = useMemo(
    () =>
      (detail?.items ?? [])
        .filter((it) => it.status === 'pending_review' || it.status === 'success')
        .map((it) => it.id),
    [detail?.items],
  );

  const onApplySelected = () => {
    if (!id || !selectedKeys.length) {
      message.warning('请选择待复核结果');
      return;
    }
    Modal.confirm({
      title: '批量应用已选结果',
      content:
        '将把选中的 AI 文案应用到商品。应用前会再次检查商品是否被人工修改，发现冲突的商品不会被覆盖。',
      onOk: async () => {
        setActing(true);
        try {
          const res = await applyAiProductTextSelected(id, selectedKeys);
          message.success(`成功 ${res.successCount}，冲突 ${res.conflictCount}，失败 ${res.failedCount}`);
          setSelectedKeys([]);
          await reload();
        } catch (e: unknown) {
          message.error((e as Error)?.message || '批量应用失败');
        } finally {
          setActing(false);
        }
      },
    });
  };

  const openReview = (row: AIProductTextItemRow) => {
    setReviewItem(row);
    setReviewOpen(true);
  };

  const batchTag = detail ? aiTextBatchStatusTag(detail.status, detail.statusLabel) : null;

  return (
    <TmPageContainer
      title="AI 商品文案批量复核"
      subTitle={detail ? `批次 ${detail.batchNo}` : undefined}
      loading={loading && !detail}
    >
      {detail && (
        <>
          <Card size="small" style={{ marginBottom: 16 }}>
            <Descriptions column={{ xs: 1, sm: 2, md: 4 }} size="small">
              <Descriptions.Item label="状态">
                {batchTag ? <Tag color={batchTag.color}>{batchTag.text}</Tag> : detail.status}
              </Descriptions.Item>
              <Descriptions.Item label="商品数">{detail.productCount}</Descriptions.Item>
              <Descriptions.Item label="子项数">{detail.itemCount}</Descriptions.Item>
              <Descriptions.Item label="待复核/成功">
                {detail.successCount} / 失败 {detail.failedCount} / 已应用 {detail.appliedCount}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(detail.createdAt)}</Descriptions.Item>
            </Descriptions>
            <Space wrap style={{ marginTop: 12 }}>
              <Button onClick={() => history.push('/product/drafts')}>返回商品列表</Button>
              <Button
                loading={acting}
                disabled={detail.failedCount === 0}
                onClick={async () => {
                  if (!id) return;
                  setActing(true);
                  try {
                    await retryAiProductTextBatchFailed(id);
                    message.success('已重试失败项');
                    await reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '重试失败');
                  } finally {
                    setActing(false);
                  }
                }}
              >
                重试失败项
              </Button>
              <Button
                loading={acting}
                type="primary"
                disabled={!selectedKeys.length}
                onClick={onApplySelected}
              >
                批量应用已选结果
              </Button>
              <Popconfirm
                title="撤销本批次已应用结果？"
                description="若商品后续被人工修改，对应项撤销会失败。"
                onConfirm={async () => {
                  if (!id) return;
                  setActing(true);
                  try {
                    const res = await undoAiProductTextBatchApplied(id);
                    message.success(`撤销成功 ${res.successCount}，冲突 ${res.conflictCount}`);
                    await reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '撤销失败');
                  } finally {
                    setActing(false);
                  }
                }}
              >
                <Button disabled={!detail.appliedCount}>批量撤销本批次应用</Button>
              </Popconfirm>
            </Space>
          </Card>

          {detail.status === 'partial_success' && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="部分子项生成失败，可重试失败项或单独重新生成。"
            />
          )}

          <Segmented
            options={AI_TEXT_REVIEW_FILTERS.map((f) => ({ label: f.label, value: f.value }))}
            value={statusFilter}
            onChange={(v) => setStatusFilter(String(v))}
            style={{ marginBottom: 12 }}
          />

          <Table<AIProductTextItemRow>
            rowKey="id"
            size="small"
            scroll={{ x: 1100 }}
            dataSource={detail.items}
            rowSelection={{
              selectedRowKeys: selectedKeys,
              onChange: (keys) => setSelectedKeys(keys as string[]),
              getCheckboxProps: (row) => ({
                disabled: !reviewableIds.includes(row.id),
              }),
            }}
            columns={[
              {
                title: '商品',
                dataIndex: 'productTitle',
                ellipsis: true,
                render: (t, row) => (
                  <Link to={`/product/drafts/${row.productId}`}>{t || row.productId}</Link>
                ),
              },
              { title: '类型', dataIndex: 'operationLabel', width: 100 },
              {
                title: '状态',
                dataIndex: 'statusLabel',
                width: 110,
                render: (_, row) => {
                  const meta = aiTextItemStatusTag(row.status, row.statusLabel);
                  return <Tag color={meta.color}>{meta.text}</Tag>;
                },
              },
              {
                title: '当前内容',
                dataIndex: 'currentContent',
                ellipsis: true,
                width: 160,
              },
              {
                title: 'AI 建议',
                dataIndex: 'generatedText',
                ellipsis: true,
                width: 180,
              },
              {
                title: '质量提醒',
                dataIndex: 'qualityWarnings',
                width: 140,
                render: (w: AIProductTextItemRow['qualityWarnings']) =>
                  w?.length ? (
                    <Typography.Text type="warning">{w[0].message}</Typography.Text>
                  ) : (
                    '—'
                  ),
              },
              {
                title: '操作',
                width: 200,
                render: (_, row) => (
                  <Space wrap size="small">
                    <Button type="link" size="small" onClick={() => openReview(row)}>
                      查看对比
                    </Button>
                    {(row.status === 'pending_review' || row.status === 'success') && (
                      <Button type="link" size="small" onClick={() => openReview(row)}>
                        应用
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
          />

          {detail.output ? (
            <TechnicalDetails label="技术详情">
              <pre style={{ fontSize: 12, margin: 0, maxHeight: 240, overflow: 'auto' }}>
                {JSON.stringify(detail.output, null, 2)}
              </pre>
            </TechnicalDetails>
          ) : null}
        </>
      )}

      <ReviewItemModal
        open={reviewOpen}
        item={reviewItem}
        loading={acting}
        onClose={() => {
          setReviewOpen(false);
          setReviewItem(null);
        }}
        onApply={async (text) => {
          if (!reviewItem) return;
          setActing(true);
          try {
            if (text !== reviewItem.prepareApplyText) {
              await updateAiProductTextEditedText(reviewItem.id, text);
            }
            await applyAiProductTextItem(reviewItem.id, text);
            message.success('已应用');
            setReviewOpen(false);
            await reload();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '应用失败');
          } finally {
            setActing(false);
          }
        }}
        onRegenerate={async () => {
          if (!reviewItem) return;
          setActing(true);
          try {
            const updated = await regenerateAiProductTextItem(reviewItem.id);
            setReviewItem(updated);
            message.success('已重新生成');
            await reload();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '重新生成失败');
          } finally {
            setActing(false);
          }
        }}
        onReject={async () => {
          if (!reviewItem) return;
          setActing(true);
          try {
            await rejectAiProductTextItem(reviewItem.id);
            message.success('已放弃该建议');
            setReviewOpen(false);
            await reload();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '操作失败');
          } finally {
            setActing(false);
          }
        }}
      />
    </TmPageContainer>
  );
}
