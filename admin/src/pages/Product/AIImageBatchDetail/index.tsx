import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import ReviewImageItemModal from '@/pages/Product/AIImageBatch/components/ReviewImageItemModal';
import { confirmApplyAiImage, confirmUndoAiImage } from '@/constants/sensitiveActions';
import {
  AI_IMAGE_APPLY_MODES,
  AI_IMAGE_REVIEW_FILTERS,
  aiImageBatchStatusTag,
  aiImageItemStatusTag,
} from '@/constants/aiProductImage';
import { aiImageWarningMessage, aiImageWarningSettingsUrl } from '@/constants/aiImageWarnings';
import {
  applyAiProductImageSelected,
  fetchAiProductImageBatchDetail,
  retryAiProductImageBatchFailed,
  undoAiProductImageBatchApplied,
  type AIProductImageBatchDetail,
  type AIProductImageItemRow,
} from '@/services/aiProductImage';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history, useParams } from '@umijs/max';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { normalizeSource } from '@/utils/urlState';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Image,
  Segmented,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

export default function AIImageBatchDetailPage() {
  useLanguage();
  const { id } = useParams<{ id: string }>();
  const { state: urlState, setState: setUrlState } = useUrlQueryState<{
    itemId?: string;
    tab?: string;
    source?: string;
    warningCode?: string;
  }>(['itemId', 'tab', 'source', 'warningCode']);
  const navSource = normalizeSource(urlState.source);
  const focusItemId = (urlState.itemId || '').trim();
  const [detail, setDetail] = useState<AIProductImageBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [statusFilter, setStatusFilter] = useState(urlState.tab || urlState.warningCode || 'all');
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [batchApplyMode, setBatchApplyMode] = useState('save_to_gallery');
  const [reviewItem, setReviewItem] = useState<AIProductImageItemRow | null>(null);
  const [reviewOpen, setReviewOpen] = useState(false);

  const reload = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const useFilter = focusItemId ? undefined : statusFilter === 'all' ? undefined : statusFilter;
      const res = await fetchAiProductImageBatchDetail(id, useFilter);
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
  }, [focusItemId, detail?.items]);

  useEffect(() => {
    if (urlState.tab) setStatusFilter(urlState.tab);
    else if (urlState.warningCode) setStatusFilter('warning');
  }, [urlState.tab, urlState.warningCode]);

  const openReview = (row: AIProductImageItemRow) => {
    setReviewItem(row);
    setReviewOpen(true);
    setUrlState({ itemId: row.id });
  };

  const closeReview = () => {
    setReviewOpen(false);
    setReviewItem(null);
    setUrlState({ itemId: undefined }, { replace: true });
  };

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
    const mode = batchApplyMode === 'replace_image' ? 'replace' : 'append';
    confirmApplyAiImage(mode, async () => {
      setActing(true);
      try {
        const res = await applyAiProductImageSelected(id, selectedKeys, batchApplyMode);
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

  const batchTag = detail ? aiImageBatchStatusTag(detail.status, detail.statusLabel) : null;

  return (
    <TmPageContainer
      title={tr("AI 图片批量复核")}
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
              <Descriptions.Item label={tr("图片数")}>{detail.imageCount}</Descriptions.Item>
              <Descriptions.Item label={tr("子项")}>
                {tr("待复核")} {detail.overview?.successCount ?? detail.successCount} {tr("/ 质量提醒")}{' '}
                {detail.overview?.warningCount ?? 0} {tr("/ 失败")} {detail.failedCount} {tr("/ 已应用")}{' '}
                {detail.appliedCount}
              </Descriptions.Item>
              <Descriptions.Item label={tr("可恢复")}>
                {detail.overview?.recoverableCount ?? 0} {tr("/ 不可恢复")} {detail.overview?.nonRecoverableCount ?? 0}
              </Descriptions.Item>
              <Descriptions.Item label={tr("接入服务状态")}>
                <Tag
                  color={
                    detail.overview?.providerReadiness?.status === 'ready'
                      ? 'success'
                      : detail.overview?.providerReadiness?.status === 'degraded'
                        ? 'warning'
                        : 'default'
                  }
                >
                  {detail.overview?.providerReadiness?.statusLabel || '—'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label={tr("创建时间")}>{formatDateTime(detail.createdAt)}</Descriptions.Item>
            </Descriptions>
            {detail.overview?.summaryMessage ? (
              <Alert
                type={detail.failedCount > 0 ? 'warning' : 'info'}
                showIcon
                style={{ marginTop: 12 }}
                message={detail.overview.summaryMessage}
                description={
                  detail.overview.nextActions?.length ? (
                    <ul style={{ margin: '8px 0 0', paddingLeft: 20 }}>
                      {detail.overview.nextActions.map((a) => (
                        <li key={a}>{a}</li>
                      ))}
                    </ul>
                  ) : undefined
                }
              />
            ) : null}
            <Space wrap style={{ marginTop: 12 }}>
              <Button
                onClick={() => {
                  const back =
                    navSource === 'config_status'
                      ? '/settings/config-status'
                      : navSource === 'taskcenter'
                        ? '/ops/task-center/failures'
                        : `/ai/image-batches${navSource ? `?source=${encodeURIComponent(navSource)}` : ''}${urlState.warningCode ? `${navSource ? '&' : '?'}warningCode=${encodeURIComponent(urlState.warningCode)}` : ''}`;
                  history.push(back);
                }}
              >
                {tr("返回列表")}
              </Button>
              <Button onClick={() => history.push('/settings/config-status')}>{tr("查看配置状态")}</Button>
              <Button onClick={() => history.push('/settings/image')}>{tr("图片 AI 设置")}</Button>
              <Button
                loading={acting}
                disabled={detail.failedCount === 0}
                onClick={async () => {
                  if (!id) return;
                  setActing(true);
                  try {
                    await retryAiProductImageBatchFailed(id);
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
              <Select
                style={{ width: 180 }}
                value={batchApplyMode}
                options={AI_IMAGE_APPLY_MODES}
                onChange={setBatchApplyMode}
              />
              <Button type="primary" loading={acting} disabled={!selectedKeys.length} onClick={onApplySelected}>
                {tr("批量应用已选图片")}
              </Button>
              <Button
                disabled={!detail.appliedCount}
                onClick={() => {
                  if (!id) return;
                  confirmUndoAiImage(async () => {
                    setActing(true);
                    try {
                      const res = await undoAiProductImageBatchApplied(id);
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
            <Alert type="warning" showIcon style={{ marginBottom: 12 }} message={tr("部分图片处理失败，可重试失败项或单独重新处理。")} />
          )}

          <Segmented
            options={AI_IMAGE_REVIEW_FILTERS.map((f) => ({ label: f.label, value: f.value }))}
            value={statusFilter}
            onChange={(v) => {
              const next = String(v);
              setStatusFilter(next);
              setUrlState(
                {
                  tab: next === 'all' ? undefined : next,
                  warningCode: next === 'warning' ? urlState.warningCode : undefined,
                  itemId: undefined,
                },
                { replace: true },
              );
            }}
            style={{ marginBottom: 12 }}
          />

          <Table<AIProductImageItemRow>
            rowKey="id"
            size="small"
            scroll={{ x: 1200 }}
            dataSource={detail.items}
            onRow={(row) => ({ style: row.id === focusItemId ? { background: 'var(--ant-color-warning-bg)' } : undefined })}
            rowSelection={{
              selectedRowKeys: selectedKeys,
              onChange: (keys) => setSelectedKeys(keys as string[]),
              getCheckboxProps: (row) => ({ disabled: !reviewableIds.includes(row.id) }),
            }}
            columns={[
              {
                title: tr("商品"),
                dataIndex: 'productTitle',
                ellipsis: true,
                render: (t, row) => <Link to={`/product/drafts/${row.productId}`}>{t || row.productId}</Link>,
              },
              { title: tr("图片类型"), dataIndex: 'imageTypeLabel', width: 90 },
              { title: tr("处理方式"), dataIndex: 'operationLabel', width: 110 },
              {
                title: tr("原图"),
                width: 80,
                render: (_, row) => <Image src={row.sourceImageUrl} width={48} height={48} style={{ objectFit: 'cover' }} />,
              },
              {
                title: tr("结果图"),
                width: 80,
                render: (_, row) =>
                  row.resultImageUrl ? (
                    <Image src={row.resultImageUrl} width={48} height={48} style={{ objectFit: 'cover' }} />
                  ) : (
                    '—'
                  ),
              },
              {
                title: tr("状态"),
                width: 100,
                render: (_, row) => {
                  const meta = aiImageItemStatusTag(row.status, row.statusLabel);
                  const isWarnOnly =
                    row.status !== 'failed' && (row.qualityWarnings?.length ?? 0) > 0;
                  return (
                    <Space direction="vertical" size={0}>
                      <Tag color={meta.color}>{meta.text}</Tag>
                      {isWarnOnly ? <Tag color="gold">{tr("质量提醒")}</Tag> : null}
                    </Space>
                  );
                },
              },
              {
                title: tr("原因 / 提醒"),
                width: 200,
                render: (_, row) => {
                  if (row.status === 'failed') {
                    const code = row.warningCode || row.errorCode;
                    return (
                      <Space direction="vertical" size={0}>
                        <Typography.Text type="danger">
                          {row.warningMessage || row.errorMessage || aiImageWarningMessage(code)}
                        </Typography.Text>
                        {code ? (
                          <TechnicalDetails label={tr("内部码")}>
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                              {code}
                            </Typography.Text>
                          </TechnicalDetails>
                        ) : null}
                      </Space>
                    );
                  }
                  if (row.qualityWarnings?.length) {
                    return (
                      <Typography.Text type="warning">{row.qualityWarnings[0].message}</Typography.Text>
                    );
                  }
                  return '—';
                },
              },
              {
                title: tr("恢复"),
                width: 120,
                render: (_, row) => {
                  if (row.status !== 'failed') return '—';
                  const code = row.warningCode || row.errorCode;
                  const url = row.settingsUrl || aiImageWarningSettingsUrl(code);
                  return (
                    <Space direction="vertical" size={0}>
                      <Typography.Text type="secondary">
                        {row.needsConfig ? tr("需配置") : row.recoverable === false ? tr("不可重试") : tr("可重试")}
                      </Typography.Text>
                      {url ? (
                        <Button type="link" size="small" onClick={() => history.push(url)}>
                          {tr("去配置")}
                        </Button>
                      ) : null}
                    </Space>
                  );
                },
              },
              {
                title: tr("操作"),
                width: 120,
                render: (_, row) => (
                  <Button type="link" size="small" onClick={() => openReview(row)}>
                    {tr("查看对比")}
                  </Button>
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

      <ReviewImageItemModal
        open={reviewOpen}
        item={reviewItem}
        onClose={closeReview}
        onDone={() => void reload()}
      />
    </TmPageContainer>
  );
}
