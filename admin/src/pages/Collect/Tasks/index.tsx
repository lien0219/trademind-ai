import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProCard, ProTable } from '@ant-design/pro-components';
import { Link } from '@umijs/max';
import { Button, Form, Input, message, Tag } from 'antd';
import dayjs from 'dayjs';
import { useEffect, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import {
  createCollectTask,
  fetchCollectTasks,
  retryCollectTask,
  type CollectTaskRow,
} from '@/services/collectTasks';

export default function CollectTasksPage() {
  const actionRef = useRef<ActionType>();
  const [form] = Form.useForm<{ source: string; url: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [polling, setPolling] = useState(4000);

  useEffect(() => {
    const sync = () => setPolling(document.visibilityState === 'hidden' ? undefined : 4000);
    sync();
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, []);

  const columns: ProColumns<CollectTaskRow>[] = [
    {
      title: '来源',
      dataIndex: 'source',
      width: 88,
      valueType: 'select',
      valueEnum: { '1688': { text: '1688' } },
    },
    {
      title: '链接关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '匹配 source_url' },
      search: {
        transform: (v) => ({ keyword: v }),
      },
    },
    {
      title: '来源链接',
      dataIndex: 'sourceUrl',
      ellipsis: true,
      copyable: true,
      search: false,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 112,
      valueType: 'select',
      valueEnum: {
        ...Object.fromEntries(
          Object.entries(COLLECT_TASK_STATUS).map(([k, v]) => [k, { text: v.text }]),
        ),
        pending: { text: '处理中（排队）' },
        running: { text: '处理中' },
        retrying: { text: '处理中（重试）' },
      },
      render: (_, row) => {
        const m = COLLECT_TASK_STATUS[row.status as keyof typeof COLLECT_TASK_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: '商品草稿',
      dataIndex: 'resultProductId',
      width: 280,
      search: false,
      ellipsis: true,
      render: (_, row) =>
        row.resultProductId ? (
          <Link to={`/product/drafts/${row.resultProductId}`}>{row.resultProductId}</Link>
        ) : (
          '—'
        ),
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      ellipsis: true,
      search: false,
    },
    {
      title: '开始时间',
      dataIndex: 'startedAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.startedAt),
    },
    {
      title: '结束时间',
      dataIndex: 'finishedAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.finishedAt),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.createdAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 88,
      search: false,
      render: (_, row) =>
        row.status === 'failed'
          ? [
              <a
                key="retry"
                onClick={async () => {
                  try {
                    await retryCollectTask(row.id);
                    message.success('已重新入队');
                    actionRef.current?.reload();
                  } catch (e) {
                    message.error(e instanceof Error ? e.message : '重试失败');
                  }
                }}
              >
                重试
              </a>,
            ]
          : [],
    },
  ];

  return (
    <PageContainer title="采集任务">
      <ProCard bordered style={{ marginBottom: 16 }} bodyStyle={{ paddingBottom: 8 }}>
        <Form
          form={form}
          layout="inline"
          initialValues={{ source: '1688', url: '' }}
          onFinish={async (vals) => {
            const url = vals.url?.trim();
            if (!url) {
              message.warning('请填写商品链接');
              return;
            }
            setSubmitting(true);
            try {
              await createCollectTask({ source: vals.source?.trim() || '1688', url });
              message.success('采集任务已提交，正在后台处理');
              actionRef.current?.reload();
            } catch (e) {
              message.error(e instanceof Error ? e.message : '采集失败');
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <Form.Item label="来源" name="source" rules={[{ required: true }]}>
            <Input style={{ width: 120 }} placeholder="1688" />
          </Form.Item>
          <Form.Item label="链接" name="url" rules={[{ required: true, message: '必填' }]}>
            <Input style={{ width: 480, maxWidth: '100%' }} placeholder="https://detail.1688.com/offer/..." />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={submitting}>
              提交
            </Button>
          </Form.Item>
        </Form>
      </ProCard>

      <ProTable<CollectTaskRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        polling={polling}
        headerTitle={false}
        toolBarRender={() => []}
        request={async (params) => {
          const res = await fetchCollectTasks({
            page: params.current,
            pageSize: params.pageSize,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
            keyword: params.keyword as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />
    </PageContainer>
  );
}

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}
