import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import ReviewImageItemModal from '@/pages/Product/AIImageBatch/components/ReviewImageItemModal';
import {
  AI_IMAGE_APPLY_MODES,
  AI_IMAGE_REVIEW_FILTERS,
  aiImageBatchStatusTag,
  aiImageItemStatusTag,
} from '@/constants/aiProductImage';
import {
  applyAiProductImageSelected,
  fetchAiProductImageBatchDetail,
  retryAiProductImageBatchFailed,
  undoAiProductImageBatchApplied,
  type AIProductImageBatchDetail,
  type AIProductImageItemRow,
} from '@/services/aiProductImage';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history, useParams, useSearchParams } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Image,
  Modal,
  Popconfirm,
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
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const focusItemId = (searchParams.get('itemId') || '').trim();
  const [detail, setDetail] = useState<AIProductImageBatchDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [statusFilter, setStatusFilter] = useState('all');
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
      message.error((e as Error)?.message || '加载失败');
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
      title: '批量应用已选图片',
      content:
        '将把选中的 AI 图片结果应用到商品。原图不会被删除，替换操作可以在安全条件下撤销。',
      onOk: async () => {
        setActing(true);
        try {
          const res = await applyAiProductImageSelected(id, selectedKeys, batchApplyMode);
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

  const batchTag = detail ? aiImageBatchStatusTag(detail.status, detail.statusLabel) : null;

  return (
    <TmPageContainer
      title="AI 图片批量复核"
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
              <Descriptions.Item label="图片数">{detail.imageCount}</Descriptions.Item>
              <Descriptions.Item label="子项">
                待复核 {detail.successCount} / 失败 {detail.failedCount} / 已应用 {detail.appliedCount}
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
                    await retryAiProductImageBatchFailed(id);
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
              <Select
                style={{ width: 180 }}
                value={batchApplyMode}
                options={AI_IMAGE_APPLY_MODES}
                onChange={setBatchApplyMode}
              />
              <Button type="primary" loading={acting} disabled={!selectedKeys.length} onClick={onApplySelected}>
                批量应用已选图片
              </Button>
              <Popconfirm
                title="撤销本批次已应用结果？"
                description="若商品图片后续被人工修改，对应项撤销会失败。"
                onConfirm={async () => {
                  if (!id) return;
                  setActing(true);
                  try {
                    const res = await undoAiProductImageBatchApplied(id);
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
            <Alert type="warning" showIcon style={{ marginBottom: 12 }} message="部分图片处理失败，可重试失败项或单独重新处理。" />
          )}

          <Segmented
            options={AI_IMAGE_REVIEW_FILTERS.map((f) => ({ label: f.label, value: f.value }))}
            value={statusFilter}
            onChange={(v) => setStatusFilter(String(v))}
            style={{ marginBottom: 12 }}
          />

          <Table<AIProductImageItemRow>
            rowKey="id"
            size="small"
            scroll={{ x: 1200 }}
            dataSource={detail.items}
            onRow={(row) => ({ style: row.id === focusItemId ? { background: '#fffbe6' } : undefined })}
            rowSelection={{
              selectedRowKeys: selectedKeys,
              onChange: (keys) => setSelectedKeys(keys as string[]),
              getCheckboxProps: (row) => ({ disabled: !reviewableIds.includes(row.id) }),
            }}
            columns={[
              {
                title: '商品',
                dataIndex: 'productTitle',
                ellipsis: true,
                render: (t, row) => <Link to={`/product/drafts/${row.productId}`}>{t || row.productId}</Link>,
              },
              { title: '图片类型', dataIndex: 'imageTypeLabel', width: 90 },
              { title: '处理方式', dataIndex: 'operationLabel', width: 110 },
              {
                title: '原图',
                width: 80,
                render: (_, row) => <Image src={row.sourceImageUrl} width={48} height={48} style={{ objectFit: 'cover' }} />,
              },
              {
                title: '结果图',
                width: 80,
                render: (_, row) =>
                  row.resultImageUrl ? (
                    <Image src={row.resultImageUrl} width={48} height={48} style={{ objectFit: 'cover' }} />
                  ) : (
                    '—'
                  ),
              },
              {
                title: '状态',
                width: 100,
                render: (_, row) => {
                  const meta = aiImageItemStatusTag(row.status, row.statusLabel);
                  return <Tag color={meta.color}>{meta.text}</Tag>;
                },
              },
              {
                title: '质量提醒',
                width: 140,
                render: (_, row) =>
                  row.qualityWarnings?.length ? (
                    <Typography.Text type="warning">{row.qualityWarnings[0].message}</Typography.Text>
                  ) : (
                    '—'
                  ),
              },
              {
                title: '操作',
                width: 120,
                render: (_, row) => (
                  <Button type="link" size="small" onClick={() => { setReviewItem(row); setReviewOpen(true); }}>
                    查看对比
                  </Button>
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

      <ReviewImageItemModal
        open={reviewOpen}
        item={reviewItem}
        onClose={() => setReviewOpen(false)}
        onDone={() => void reload()}
      />
    </TmPageContainer>
  );
}
