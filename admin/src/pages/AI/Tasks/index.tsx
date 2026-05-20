import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Collapse, Descriptions, Drawer, Spin, Tag } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useRef, useState } from 'react';
import type { AiTaskDetail, AiTaskListRow } from '@/services/aiTasks';
import { getAiTask, queryAiTasks } from '@/services/aiTasks';

function formatJsonPretty(v: unknown): string {
  if (v == null) return '';
  try {
    return typeof v === 'string' ? v : JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

const AI_TASK_STATUS_LABEL: Record<string, { text: string; color?: string }> = {
  pending: { text: '排队中' },
  running: { text: '执行中', color: 'processing' },
  success: { text: '成功', color: 'success' },
  failed: { text: '失败', color: 'error' },
};

function statusTag(status: string) {
  const s = status?.trim() || '';
  const m = AI_TASK_STATUS_LABEL[s];
  if (m) return <Tag color={m.color}>{m.text}</Tag>;
  return <Tag>{s || '—'}</Tag>;
}

function JsonBlock({ title, value }: { title: string; value: unknown }) {
  const text = formatJsonPretty(value);
  if (!text) {
    return (
      <div style={{ marginBottom: 16 }}>
        <strong>{title}</strong>
        <div style={{ marginTop: 8, color: 'var(--ant-color-text-secondary)' }}>—</div>
      </div>
    );
  }
  return (
    <div style={{ marginBottom: 16 }}>
      <strong>{title}</strong>
      <pre
        style={{
          marginTop: 8,
          marginBottom: 0,
          maxHeight: 360,
          overflow: 'auto',
          padding: 12,
          background: 'var(--ant-color-fill-quaternary, #f5f5f5)',
          borderRadius: 6,
          fontSize: 12,
          lineHeight: 1.5,
        }}
      >
        {text}
      </pre>
    </div>
  );
}

export default function AiTasksPage() {
  const actionRef = useRef<ActionType>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<AiTaskDetail | null>(null);

  const openDetail = useCallback(async (id: string) => {
    setDrawerOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const row = await getAiTask(id);
      setDetail(row);
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const columns: ProColumns<AiTaskListRow>[] = [
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 172,
      search: false,
      render: (_, row) => dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '任务类型',
      dataIndex: 'taskType',
      width: 200,
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      valueType: 'select',
      valueEnum: {
        pending: { text: '排队中' },
        running: { text: '执行中' },
        success: { text: '成功' },
        failed: { text: '失败' },
      },
      render: (_, row) => statusTag(row.status),
    },
    {
      title: 'AI 服务商',
      dataIndex: 'provider',
      width: 140,
      ellipsis: true,
    },
    {
      title: '模型',
      dataIndex: 'model',
      width: 160,
      ellipsis: true,
    },
    {
      title: '技能模板',
      dataIndex: 'promptCode',
      width: 200,
      ellipsis: true,
    },
    {
      title: '商品 ID',
      dataIndex: 'productId',
      width: 280,
      ellipsis: true,
      copyable: true,
    },
    {
      title: '会话 ID',
      dataIndex: 'conversationId',
      width: 280,
      ellipsis: true,
      copyable: true,
    },
    {
      title: '输入量',
      dataIndex: 'tokenInput',
      width: 88,
      search: false,
    },
    {
      title: '输出量',
      dataIndex: 'tokenOutput',
      width: 92,
      search: false,
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      ellipsis: true,
      search: false,
    },
    {
      title: '时间范围',
      dataIndex: 'dateRange',
      valueType: 'dateTimeRange',
      hideInTable: true,
      search: {
        transform: (value) => {
          if (!value || !Array.isArray(value) || value.length < 2) return {};
          const a = dayjs(value[0] as string | dayjs.Dayjs);
          const b = dayjs(value[1] as string | dayjs.Dayjs);
          if (!a.isValid() || !b.isValid()) return {};
          return { start: a.toISOString(), end: b.toISOString() };
        },
      },
    },
    {
      title: '操作',
      valueType: 'option',
      width: 112,
      search: false,
      render: (_, row) => [
        <Button key="detail" type="link" onClick={() => void openDetail(row.id)}>
          查看详情
        </Button>,
      ],
    },
  ];

  return (
    <PageContainer title="AI 任务记录" subTitle="查看标题优化、描述生成等 AI 调用记录与失败原因">
      <ProTable<AiTaskListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1400 }}
        request={async (params) => {
          const res = await queryAiTasks({
            page: params.current,
            pageSize: params.pageSize,
            taskType: params.taskType as string | undefined,
            status: params.status as string | undefined,
            provider: params.provider as string | undefined,
            model: params.model as string | undefined,
            promptCode: params.promptCode as string | undefined,
            productId: params.productId as string | undefined,
            conversationId: params.conversationId as string | undefined,
            start: params.start as string | undefined,
            end: params.end as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
        headerTitle={false}
        toolBarRender={() => []}
      />

      <Drawer
        title="AI 任务详情"
        width={720}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setDetail(null);
        }}
        destroyOnHidden
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Spin />
          </div>
        ) : detail ? (
          <>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 24 }}>
              <Descriptions.Item label="ID">{detail.id}</Descriptions.Item>
              <Descriptions.Item label="任务类型">{detail.taskType}</Descriptions.Item>
              <Descriptions.Item label="状态">{statusTag(detail.status)}</Descriptions.Item>
              <Descriptions.Item label="AI 服务商">{detail.provider || '—'}</Descriptions.Item>
              <Descriptions.Item label="模型">{detail.model || '—'}</Descriptions.Item>
              <Descriptions.Item label="技能模板">{detail.promptCode || '—'}</Descriptions.Item>
              <Descriptions.Item label="商品 ID">{detail.productId || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建者">{detail.createdBy || '—'}</Descriptions.Item>
              <Descriptions.Item label="输入 tokens">{detail.tokenInput ?? 0}</Descriptions.Item>
              <Descriptions.Item label="输出 tokens">{detail.tokenOutput ?? 0}</Descriptions.Item>
              <Descriptions.Item label="费用">{detail.costAmount ?? 0}</Descriptions.Item>
              <Descriptions.Item label="开始时间">
                {detail.startedAt ? dayjs(detail.startedAt).format('YYYY-MM-DD HH:mm:ss') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间">
                {detail.finishedAt ? dayjs(detail.finishedAt).format('YYYY-MM-DD HH:mm:ss') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {dayjs(detail.createdAt).format('YYYY-MM-DD HH:mm:ss')}
              </Descriptions.Item>
              <Descriptions.Item label="错误信息">{detail.errorMessage || '—'}</Descriptions.Item>
            </Descriptions>
            <Collapse
              ghost
              items={[
                {
                  key: 'tech',
                  label: '展开查看技术详情（输入/输出原始数据）',
                  children: (
                    <>
                      <JsonBlock title="请求内容" value={detail.input} />
                      <JsonBlock title="返回内容" value={detail.output} />
                      <JsonBlock title="模型原始响应" value={detail.rawResponse} />
                    </>
                  ),
                },
              ]}
            />
          </>
        ) : (
          <div style={{ color: 'var(--ant-color-text-secondary)' }}>暂无数据</div>
        )}
      </Drawer>
    </PageContainer>
  );
}
