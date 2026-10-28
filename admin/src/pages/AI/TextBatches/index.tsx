import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { TmPageContainer } from '@/components/ui';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { aiTextBatchStatusTag, AI_TEXT_BATCH_STATUS } from '@/constants/aiProductText';
import { fetchAiProductTextBatches, type AIProductTextBatchRow } from '@/services/aiProductText';
import { formatDateTime } from '@/utils/formatTime';
import { normalizeSource, parsePositiveInt } from '@/utils/urlState';
import { Link, history } from '@umijs/max';
import { Alert, Button, Select, Space, Table, Tag } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

const AI_TEXT_BATCH_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'status',
  'platform',
  'shopId',
  'batchId',
  'source',
] as const;

export default function AITextBatchListPage() {
  const language = useLanguage();
  const emptyLocale = useListEmptyLocale('aiTextBatches');
  const { state: urlState, setState: setUrlState } =
    useUrlQueryState<Record<(typeof AI_TEXT_BATCH_QUERY_KEYS)[number], string | undefined>>(
      AI_TEXT_BATCH_QUERY_KEYS,
    );
  const navSource = normalizeSource(urlState.source);
  const [rows, setRows] = useState<AIProductTextBatchRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const page = parsePositiveInt(urlState.page, 1);
  const pageSize = parsePositiveInt(urlState.pageSize, 20);
  const statusFilter = urlState.status;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchAiProductTextBatches({ page, pageSize });
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
  }, [page, pageSize, statusFilter, urlState.batchId]);

  useEffect(() => {
    void load();
  }, [load]);

  const statusOptions = useMemo(
    () => [
      { label: tr("全部状态"), value: '' },
      ...Object.entries(AI_TEXT_BATCH_STATUS).map(([value, meta]) => ({
        label: meta.label,
        value,
      })),
    ],
    [language],
  );

  return (
    <TmPageContainer
      title={tr("批量文案任务")}
      subTitle={tr("批量 AI 标题 / 描述生成与复核")}
      extra={
        <Button type="primary" onClick={() => history.push('/product/drafts')}>
          {tr("从商品列表发起")}
        </Button>
      }
    >
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder={tr("批次状态")}
          style={{ minWidth: 160 }}
          options={statusOptions}
          value={statusFilter || undefined}
          onChange={(v) =>
            setUrlState({
              status: v || undefined,
              page: undefined,
            })
          }
        />
      </Space>
      <Table<AIProductTextBatchRow>
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
              const meta = aiTextBatchStatusTag(row.status, row.statusLabel);
              return <Tag color={meta.color}>{meta.text}</Tag>;
            },
          },
          { title: tr("商品数"), dataIndex: 'productCount', width: 80 },
          { title: tr("子项数"), dataIndex: 'itemCount', width: 80 },
          { title: tr("成功"), dataIndex: 'successCount', width: 70 },
          { title: tr("失败"), dataIndex: 'failedCount', width: 70 },
          { title: tr("已应用"), dataIndex: 'appliedCount', width: 80 },
          {
            title: tr("创建时间"),
            dataIndex: 'createdAt',
            width: 170,
            render: (v) => formatDateTime(v),
          },
          {
            title: tr("操作"),
            width: 120,
            render: (_, row) => {
              const qs = new URLSearchParams();
              if (navSource) qs.set('source', navSource);
              const suffix = qs.toString() ? `?${qs.toString()}` : '';
              return (
                <Space>
                  <Link to={`/product/ai-text-batches/${row.id}${suffix}`}>{tr("复核")}</Link>
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
