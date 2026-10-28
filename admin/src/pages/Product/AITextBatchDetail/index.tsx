import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import ReviewItemModal from '@/pages/Product/AITextBatch/components/ReviewItemModal';
import {
  AI_TEXT_REVIEW_FILTERS,
  aiTextBatchStatusTag,
  aiTextItemStatusTag,
} from '@/constants/aiProductText';
import { confirmApplyAiText, confirmUndoAiText } from '@/constants/sensitiveActions';
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
import { useUrlQueryState } from '@/hooks/useUrlState';
import { normalizeSource } from '@/utils/urlState';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

export default function AITextBatchDetailPage() {
  useLanguage();
  const { id } = useParams<{ id: string }>();
  const { state: urlState, setState: setUrlState } = useUrlQueryState<{
    itemId?: string;
    tab?: string;
    source?: string;
  }>(['itemId', 'tab', 'source']);
  const navSource = normalizeSource(urlState.source);
  const focusItemId = (urlState.itemId || '').trim();
  const [detail, setDetail] = useState<AIProductTextBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [statusFilter, setStatusFilter] = useState(urlState.tab || 'all');
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [reviewItem, setReviewItem] = useState<AIProductTextItemRow | null>(null);
  const [reviewOpen, setReviewOpen] = useState(false);

  const reload = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const useFilter = focusItemId ? undefined : statusFilter === 'all' ? undefined : statusFilter;
      const res = await fetchAiProductTextBatchDetail(id, useFilter);
      setDetail(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载失败"));
    } finally {
      setLoading(false);
    }
  }, [id, statusFilter, focusItemId]);

  useEffect(() => {
    void reload();
    const timer = setInterval(() => {
      if (detail?.status === 'running') void reload();
    }, 5000);
    return () => clearInterval(timer);
  }, [reload, detail?.status]);

  useEffect(() => {
    if (!focusItemId || !detail?.items?.length) return;
    const target = detail.items.find((it) => it.id === focusItemId);
    if (!target) return;
    setReviewItem(target);
    setReviewOpen(true);
    if (target.status === 'failed' || target.status === 'conflict') {
      setStatusFilter('all');
    }
  }, [focusItemId, detail?.items]);

  const reviewableIds = useMemo(
    () =>
      (detail?.items ?? [])
        .filter((it) => it.status === 'pending_review' || it.status === 'success')
        .map((it) => it.id),
    [detail?.items],
  );

  const onApplySelected = () => {
    if (!id || !selectedKeys.length) {
      message.warning(tr("请选择待复核结果"));
      return;
    }
    confirmApplyAiText(tr("选中文案"), async () => {
      setActing(true);
      try {
        const res = await applyAiProductTextSelected(id, selectedKeys);
        message.success(trFormat("成功 {value0}，冲突 {value1}，失败 {value2}", { value0: res.successCount, value1: res.conflictCount, value2: res.failedCount }));
        setSelectedKeys([]);
        await reload();
      } catch (e: unknown) {
        message.error((e as Error)?.message || tr("批量应用失败"));
      } finally {
        setActing(false);
      }
    });
  };

  useEffect(() => {
    if (urlState.tab) setStatusFilter(urlState.tab);
  }, [urlState.tab]);

  const openReview = (row: AIProductTextItemRow) => {
    setReviewItem(row);
    setReviewOpen(true);
    setUrlState({ itemId: row.id });
  };

  const closeReview = () => {
    setReviewOpen(false);
    setReviewItem(null);
    setUrlState({ itemId: undefined }, { replace: true });
  };

  const batchTag = detail ? aiTextBatchStatusTag(detail.status, detail.statusLabel) : null;

  return (
    <TmPageContainer
      title={tr("AI 商品文案批量复核")}
      subTitle={detail ? trFormat("批次 {value0}", { value0: String(detail.batchNo) }) : undefined}
      loading={loading && !detail}
    >
      {detail && (
        <>
          <Card size="small" style={{ marginBottom: 16 }}>
            <Descriptions column={{ xs: 1, sm: 2, md: 4 }} size="small">
              <Descriptions.Item label={tr("状态")}>
                {batchTag ? <Tag color={batchTag.color}>{batchTag.text}</Tag> : detail.status}
              </Descriptions.Item>
              <Descriptions.Item label={tr("商品数")}>{detail.productCount}</Descriptions.Item>
              <Descriptions.Item label={tr("子项数")}>{detail.itemCount}</Descriptions.Item>
              <Descriptions.Item label={tr("待复核/成功")}>
                {detail.successCount} {tr("/ 失败")} {detail.failedCount} {tr("/ 已应用")} {detail.appliedCount}
              </Descriptions.Item>
              <Descriptions.Item label={tr("创建时间")}>{formatDateTime(detail.createdAt)}</Descriptions.Item>
            </Descriptions>
            <Space wrap style={{ marginTop: 12 }}>
              <Button
                onClick={() => {
                  const back =
                    navSource === 'ai_workbench'
                      ? '/ai/operation-workbench'
                      : `/ai/text-batches${navSource ? `?source=${encodeURIComponent(navSource)}` : ''}`;
                  history.push(back);
                }}
              >
                {navSource === 'ai_workbench' ? tr("返回 AI 工作台") : tr("返回批次列表")}
              </Button>
              <Button
                loading={acting}
                disabled={detail.failedCount === 0}
                onClick={async () => {
                  if (!id) return;
                  setActing(true);
                  try {
                    await retryAiProductTextBatchFailed(id);
                    message.success(tr("已重试失败项"));
                    await reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || tr("重试失败"));
                  } finally {
                    setActing(false);
                  }
                }}
              >
                {tr("重试失败项")}
              </Button>
              <Button
                loading={acting}
                type="primary"
                disabled={!selectedKeys.length}
                onClick={onApplySelected}
              >
                {tr("批量应用已选结果")}
              </Button>
              <Button
                disabled={!detail.appliedCount}
                onClick={() => {
                  if (!id) return;
                  confirmUndoAiText(tr("本批次文案"), async () => {
                    setActing(true);
                    try {
                      const res = await undoAiProductTextBatchApplied(id);
                      message.success(trFormat("撤销成功 {value0}，冲突 {value1}", { value0: res.successCount, value1: res.conflictCount }));
                      await reload();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || tr("撤销失败"));
                    } finally {
                      setActing(false);
                    }
                  });
                }}
              >
                {tr("批量撤销本批次应用")}
              </Button>
            </Space>
          </Card>

          {detail.status === 'partial_success' && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message={tr("部分子项生成失败，可重试失败项或单独重新生成。")}
            />
          )}

          <Segmented
            options={AI_TEXT_REVIEW_FILTERS.map((f) => ({ label: f.label, value: f.value }))}
            value={statusFilter}
            onChange={(v) => {
              const next = String(v);
              setStatusFilter(next);
              setUrlState({ tab: next === 'all' ? undefined : next, itemId: undefined }, { replace: true });
            }}
            style={{ marginBottom: 12 }}
          />

          <Table<AIProductTextItemRow>
            rowKey="id"
            size="small"
            scroll={{ x: 1100 }}
            dataSource={detail.items}
            onRow={(row) => ({
              style: row.id === focusItemId ? { background: 'var(--ant-color-warning-bg)' } : undefined,
            })}
            rowSelection={{
              selectedRowKeys: selectedKeys,
              onChange: (keys) => setSelectedKeys(keys as string[]),
              getCheckboxProps: (row) => ({
                disabled: !reviewableIds.includes(row.id),
              }),
            }}
            columns={[
              {
                title: tr("商品"),
                dataIndex: 'productTitle',
                ellipsis: true,
                render: (t, row) => (
                  <Link to={`/product/drafts/${row.productId}`}>{t || row.productId}</Link>
                ),
              },
              { title: tr("类型"), dataIndex: 'operationLabel', width: 100 },
              {
                title: tr("状态"),
                dataIndex: 'statusLabel',
                width: 110,
                render: (_, row) => {
                  const meta = aiTextItemStatusTag(row.status, row.statusLabel);
                  return <Tag color={meta.color}>{meta.text}</Tag>;
                },
              },
              {
                title: tr("当前内容"),
                dataIndex: 'currentContent',
                ellipsis: true,
                width: 160,
              },
              {
                title: tr("AI 建议"),
                dataIndex: 'generatedText',
                ellipsis: true,
                width: 180,
              },
              {
                title: tr("质量提醒"),
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
                title: tr("操作"),
                width: 200,
                render: (_, row) => (
                  <Space wrap size="small">
                    <Button type="link" size="small" onClick={() => openReview(row)}>
                      {tr("查看对比")}
                    </Button>
                    {(row.status === 'pending_review' || row.status === 'success') && (
                      <Button type="link" size="small" onClick={() => openReview(row)}>
                        {tr("应用")}
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
          />

          {detail.output ? (
            <TechnicalDetails label={tr("技术详情")}>
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
        onClose={closeReview}
        onApply={async (text) => {
          if (!reviewItem) return;
          const label = reviewItem.operationLabel || tr("AI 文案");
          confirmApplyAiText(label, async () => {
            setActing(true);
            try {
              if (text !== reviewItem.prepareApplyText) {
                await updateAiProductTextEditedText(reviewItem.id, text);
              }
              await applyAiProductTextItem(reviewItem.id, text);
              message.success(tr("已应用"));
              closeReview();
              await reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || tr("应用失败"));
            } finally {
              setActing(false);
            }
          });
        }}
        onRegenerate={async () => {
          if (!reviewItem) return;
          setActing(true);
          try {
            const updated = await regenerateAiProductTextItem(reviewItem.id);
            setReviewItem(updated);
            message.success(tr("已重新生成"));
            await reload();
          } catch (e: unknown) {
                message.error((e as Error)?.message || tr("重新生成失败"));
          } finally {
            setActing(false);
          }
        }}
        onReject={async () => {
          if (!reviewItem) return;
          setActing(true);
          try {
            await rejectAiProductTextItem(reviewItem.id);
            message.success(tr("已放弃该建议"));
            setReviewOpen(false);
            await reload();
          } catch (e: unknown) {
              message.error((e as Error)?.message || tr("操作失败"));
          } finally {
            setActing(false);
          }
        }}
      />
    </TmPageContainer>
  );
}
