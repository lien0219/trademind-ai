import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { TmPageContainer } from '@/components/ui';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { aiImageBatchStatusTag } from '@/constants/aiProductImage';
import { fetchAiProductImageBatches, type AIProductImageBatchRow } from '@/services/aiProductImage';
import { AI_IMAGE_WARNING_LABEL, AI_IMAGE_WARNING_CODES } from '@/constants/aiImageWarnings';
import { formatDateTime } from '@/utils/formatTime';
import { normalizeSource, parsePositiveInt } from '@/utils/urlState';
import { Link, history } from '@umijs/max';
import { Alert, Button, Select, Space, Table, Tag } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

const AI_IMAGE_BATCH_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'status',
  'warningCode',
  'platform',
  'shopId',
  'batchId',
  'source',
] as const;

export default function AIImageBatchListPage() {
  const language = useLanguage();
  const emptyLocale = useListEmptyLocale('aiImageBatches');
  const { state: urlState, setState: setUrlState } =
    useUrlQueryState<Record<(typeof AI_IMAGE_BATCH_QUERY_KEYS)[number], string | undefined>>(
      AI_IMAGE_BATCH_QUERY_KEYS,
    );
  const navSource = normalizeSource(urlState.source);
  const [rows, setRows] = useState<AIProductImageBatchRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const page = parsePositiveInt(urlState.page, 1);
  const pageSize = parsePositiveInt(urlState.pageSize, 20);
  const statusFilter = urlState.status;
  const warningFilter = urlState.warningCode;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchAiProductImageBatches({ page, pageSize });
      let list = res.list;
      if (statusFilter) {
        list = list.filter((row) => row.status === statusFilter);
      }
      if (urlState.batchId) {
        list = list.filter((row) => row.id === urlState.batchId || row.batchNo === urlState.batchId);
      }
      setRows(list);
      setTotal(statusFilter || urlState.batchId ? list.length : res.pagination.total);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, urlState.batchId, warningFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const warningOptions = useMemo(
    () => [
      { label: tr("全部 warning"), value: '' },
      ...AI_IMAGE_WARNING_CODES.map((code) => ({
        label: AI_IMAGE_WARNING_LABEL[code] || code,
        value: code,
      })),
    ],
    [language],
  );

  return (
    <TmPageContainer
      title={tr("批量图片任务")}
      subTitle={tr("批量 AI 图片处理与复核")}
      extra={
        <Button type="primary" onClick={() => history.push('/product/drafts')}>
          {tr("从商品列表发起")}
        </Button>
      }
    >
      {warningFilter ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={tr("warning 筛选已写入 URL，进入批次详情后将用于定位相关子项。")}
        />
      ) : null}
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder={tr("批次状态")}
          style={{ minWidth: 160 }}
          value={statusFilter || undefined}
          options={[
            { label: tr("全部状态"), value: '' },
            { label: tr("运行中"), value: 'running' },
            { label: tr("待复核"), value: 'pending_review' },
            { label: tr("部分成功"), value: 'partial_success' },
            { label: tr("失败"), value: 'failed' },
            { label: tr("已完成"), value: 'completed' },
          ]}
          onChange={(v) =>
            setUrlState({
              status: v || undefined,
              page: undefined,
            })
          }
        />
        <Select
          allowClear
          placeholder={tr("质量 warning")}
          style={{ minWidth: 200 }}
          options={warningOptions}
          value={warningFilter || undefined}
          onChange={(v) =>
            setUrlState({
              warningCode: v || undefined,
              page: undefined,
            })
          }
        />
      </Space>
      <Table<AIProductImageBatchRow>
        rowKey="id"
        loading={loading}
        dataSource={rows}
        scroll={{ x: 960 }}
        pagination={{
          current: page,
          total,
          pageSize,
          showSizeChanger: true,
          onChange: (nextPage, nextSize) => {
            setUrlState({
              page: nextPage > 1 ? nextPage : undefined,
              pageSize: nextSize !== 20 ? nextSize : undefined,
            });
          },
        }}
        columns={[
          { title: tr("批次号"), dataIndex: 'batchNo', width: 140 },
          {
            title: tr("状态"),
            dataIndex: 'statusLabel',
            width: 100,
            render: (_, row) => {
              const meta = aiImageBatchStatusTag(row.status, row.statusLabel);
              return <Tag color={meta.color}>{meta.text}</Tag>;
            },
          },
          { title: tr("商品数"), dataIndex: 'productCount', width: 80 },
          { title: tr("图片数"), dataIndex: 'imageCount', width: 80 },
          { title: tr("子项数"), dataIndex: 'itemCount', width: 80 },
          { title: tr("待复核"), dataIndex: 'successCount', width: 70 },
          { title: tr("失败"), dataIndex: 'failedCount', width: 70 },
          { title: tr("已应用"), dataIndex: 'appliedCount', width: 80 },
          { title: tr("创建时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(v) },
          {
            title: tr("操作"),
            width: 120,
            render: (_, row) => {
              const qs = new URLSearchParams();
              if (navSource) qs.set('source', navSource);
              if (warningFilter) qs.set('warningCode', warningFilter);
              const suffix = qs.toString() ? `?${qs.toString()}` : '';
              return (
                <Space>
                  <Link to={`/product/ai-image-batches/${row.id}${suffix}`}>{tr("复核")}</Link>
                </Space>
              );
            },
          },
        ]}
        locale={emptyLocale}
      />
    </TmPageContainer>
  );
}
