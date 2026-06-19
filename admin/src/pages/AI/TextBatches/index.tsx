import { TmPageContainer } from '@/components/ui';
import { aiTextBatchStatusTag } from '@/constants/aiProductText';
import { fetchAiProductTextBatches, type AIProductTextBatchRow } from '@/services/aiProductText';
import { formatDateTime } from '@/utils/formatTime';
import { Link, history } from '@umijs/max';
import { Button, Space, Table, Tag } from 'antd';
import { useCallback, useEffect, useState } from 'react';

export default function AITextBatchListPage() {
  const [rows, setRows] = useState<AIProductTextBatchRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchAiProductTextBatches({ page, pageSize: 20 });
      setRows(res.list);
      setTotal(res.pagination.total);
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer
      title="批量文案任务"
      subTitle="批量 AI 标题 / 描述生成与复核"
      extra={
        <Button type="primary" onClick={() => history.push('/product/drafts')}>
          从商品列表发起
        </Button>
      }
    >
      <Table<AIProductTextBatchRow>
        rowKey="id"
        loading={loading}
        dataSource={rows}
        pagination={{
          current: page,
          total,
          pageSize: 20,
          onChange: setPage,
        }}
        columns={[
          { title: '批次号', dataIndex: 'batchNo', width: 140 },
          {
            title: '状态',
            dataIndex: 'statusLabel',
            width: 100,
            render: (_, row) => {
              const meta = aiTextBatchStatusTag(row.status, row.statusLabel);
              return <Tag color={meta.color}>{meta.text}</Tag>;
            },
          },
          { title: '商品数', dataIndex: 'productCount', width: 80 },
          { title: '子项数', dataIndex: 'itemCount', width: 80 },
          { title: '成功', dataIndex: 'successCount', width: 70 },
          { title: '失败', dataIndex: 'failedCount', width: 70 },
          { title: '已应用', dataIndex: 'appliedCount', width: 80 },
          {
            title: '创建时间',
            dataIndex: 'createdAt',
            width: 170,
            render: (v) => formatDateTime(v),
          },
          {
            title: '操作',
            width: 120,
            render: (_, row) => (
              <Space>
                <Link to={`/product/ai-text-batches/${row.id}`}>复核</Link>
              </Space>
            ),
          },
        ]}
      />
    </TmPageContainer>
  );
}
