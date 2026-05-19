import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import {
  Button,
  Descriptions,
  Drawer,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useRef, useState } from 'react';
import { Link } from '@umijs/renderer-react';
import {
  applyAiBatchResults,
  fetchAiBatchDetail,
  fetchAiBatchTasks,
  fetchAiBatches,
  retryAiBatchFailed,
  type AIOperationBatchRow,
} from '@/services/aiBatches';

const OP_LABEL: Record<string, string> = {
  title_optimize: '批量标题优化',
  description_generate: '批量描述生成',
  image_remove_background: '批量去背景',
  image_generate_scene: '批量场景图',
  image_replace_background: '批量换背景',
};

const STATUS_COLOR: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  success: 'success',
  partial_success: 'warning',
  failed: 'error',
  cancelled: 'default',
};

export default function AiBatchesPage() {
  const actionRef = useRef<ActionType>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [tasksOpen, setTasksOpen] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [currentId, setCurrentId] = useState<string>();
  const [detail, setDetail] = useState<{
    batch: AIOperationBatchRow;
    recentAiTasks: unknown[];
    recentImageTasks: unknown[];
  } | null>(null);
  const [tasksKind, setTasksKind] = useState<string>('');

  const columns: ProColumns<AIOperationBatchRow>[] = [
    { title: '创建时间', dataIndex: 'createdAt', width: 176, valueType: 'dateTime', search: false },
    { title: '批次号', dataIndex: 'batchNo', width: 140, ellipsis: true, copyable: true, search: false },
    {
      title: '类型',
      dataIndex: 'operationType',
      width: 160,
      valueType: 'select',
      valueEnum: Object.fromEntries(Object.keys(OP_LABEL).map((k) => [k, { text: OP_LABEL[k] }])),
      render: (_, row) => OP_LABEL[row.operationType] ?? row.operationType,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      render: (_, row) => (
        <Tag color={STATUS_COLOR[row.status] ?? 'default'}>{row.status}</Tag>
      ),
    },
    { title: '商品数', dataIndex: 'productCount', width: 80, search: false },
    { title: '任务数', dataIndex: 'taskCount', width: 80, search: false },
    { title: '成功', dataIndex: 'successCount', width: 72, search: false },
    { title: '失败', dataIndex: 'failedCount', width: 72, search: false },
    { title: '跳过', dataIndex: 'skippedCount', width: 72, search: false },
    {
      title: '操作',
      valueType: 'option',
      width: 220,
      render: (_, row) => [
        <Typography.Link
          key="d"
          onClick={() => void openDetail(row.id)}
        >
          详情
        </Typography.Link>,
        <Typography.Link key="t" onClick={() => void openTasks(row)}>
          子任务
        </Typography.Link>,
        <Typography.Link key="r" onClick={() => void runRetry(row.id)}>
          重试失败
        </Typography.Link>,
        row.operationType === 'title_optimize' || row.operationType === 'description_generate' ? (
          <Typography.Link key="a" onClick={() => void runApply(row.id)}>
            应用结果
          </Typography.Link>
        ) : null,
      ],
    },
  ];

  const openDetail = async (id: string) => {
    setCurrentId(id);
    setDetailOpen(true);
    setLoadingDetail(true);
    try {
      const res = await fetchAiBatchDetail(id);
      setDetail(res ?? null);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoadingDetail(false);
    }
  };

  const openTasks = async (row: AIOperationBatchRow) => {
    setCurrentId(row.id);
    setTasksOpen(true);
    setTasksKind('');
    actionRef.current?.reload();
    try {
      const res = await fetchAiBatchTasks(row.id, { page: 1, pageSize: 20 });
      setTasksKind(res?.kind ?? '');
    } catch {
      setTasksKind('');
    }
  };

  const runRetry = async (id: string) => {
    try {
      await retryAiBatchFailed(id);
      message.success('已触发重试');
      actionRef.current?.reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '重试失败');
    }
  };

  const runApply = async (id: string) => {
    try {
      const res = await applyAiBatchResults(id, { target: 'ai_field' });
      message.success(`已应用 ${res?.applied ?? 0} 条到 AI 文案字段`);
      actionRef.current?.reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '应用失败');
    }
  };

  return (
    <PageContainer title="AI 批次">
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        批量结果默认进入 AI 文案字段（<Typography.Text code>ai_title</Typography.Text> /{' '}
        <Typography.Text code>ai_description</Typography.Text>）或图片任务表，不会自动覆盖正式标题/详情或替换主图。详情见{' '}
        <Link to="/settings/ai">AI 设置</Link>。
      </Typography.Paragraph>

      <ProTable<AIOperationBatchRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true }}
        request={async (params) => {
          const d = await fetchAiBatches({
            page: params.current,
            pageSize: params.pageSize,
            operationType: params.operationType as string | undefined,
            status: params.status as string | undefined,
          });
          return {
            data: d.list ?? [],
            success: true,
            total: d.pagination?.total ?? 0,
          };
        }}
      />

      <Drawer
        title="批次详情"
        width={620}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        destroyOnHidden
      >
        {loadingDetail ? (
          <Typography.Text type="secondary">加载中…</Typography.Text>
        ) : detail?.batch ? (
          <>
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="批次号">{detail.batch.batchNo}</Descriptions.Item>
              <Descriptions.Item label="类型">
                {OP_LABEL[detail.batch.operationType] ?? detail.batch.operationType}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={STATUS_COLOR[detail.batch.status]}>{detail.batch.status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="统计">
                商品 {detail.batch.productCount} · 任务 {detail.batch.taskCount} · 成功 {detail.batch.successCount}{' '}
                · 失败 {detail.batch.failedCount} · 跳过 {detail.batch.skippedCount}
              </Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5} style={{ marginTop: 16 }}>
              Input 摘要
            </Typography.Title>
            <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200 }}>
              {JSON.stringify(detail.batch.input ?? {}, null, 2)}
            </pre>
            <Typography.Title level={5}>Output 摘要</Typography.Title>
            <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200 }}>
              {JSON.stringify(detail.batch.output ?? {}, null, 2)}
            </pre>
            {(detail.recentAiTasks?.length ?? 0) > 0 && (
              <>
                <Typography.Title level={5}>最近 AI 任务</Typography.Title>
                <Table
                  size="small"
                  pagination={false}
                  rowKey={(r) => (r as { id: string }).id}
                  dataSource={detail.recentAiTasks as object[]}
                  columns={[
                    { title: '状态', dataIndex: 'status', width: 90 },
                    { title: '商品', dataIndex: 'productId', ellipsis: true },
                    {
                      title: '错误摘要',
                      dataIndex: 'errorMessage',
                      ellipsis: true,
                    },
                  ]}
                />
              </>
            )}
            {(detail.recentImageTasks?.length ?? 0) > 0 && (
              <>
                <Typography.Title level={5}>最近图片任务</Typography.Title>
                <Table
                  size="small"
                  pagination={false}
                  rowKey={(r) => (r as { id: string }).id}
                  dataSource={detail.recentImageTasks as object[]}
                  columns={[
                    { title: '类型', dataIndex: 'taskType', width: 140 },
                    { title: '状态', dataIndex: 'status', width: 96 },
                    { title: '商品', dataIndex: 'productId', ellipsis: true },
                  ]}
                />
              </>
            )}
            <Space style={{ marginTop: 16 }}>
              <Button
                type="primary"
                onClick={() =>
                  detail.batch &&
                  (detail.batch.operationType === 'title_optimize' ||
                    detail.batch.operationType === 'description_generate')
                    ? runApply(detail.batch.id)
                    : message.info('仅文本批次可一键应用')
                }
              >
                应用 AI 文案到草稿（仅 ai_* 字段）
              </Button>
              <Button onClick={() => currentId && runRetry(currentId)}>重试失败</Button>
            </Space>
          </>
        ) : null}
      </Drawer>

      <Drawer
        title="子任务"
        width={720}
        open={tasksOpen}
        onClose={() => setTasksOpen(false)}
        destroyOnHidden
      >
        <ProTable
          rowKey="id"
          search={false}
          options={false}
          pagination={{ pageSize: 20 }}
          request={async (p) => {
            if (!currentId)
              return { data: [], success: true, total: 0 };
            const res = await fetchAiBatchTasks(currentId, {
              page: p.current,
              pageSize: p.pageSize,
            });
            const d = res;
            setTasksKind(d?.kind ?? '');
            return {
              data: (d?.list ?? []) as object[],
              success: true,
              total: d?.pagination?.total ?? 0,
            };
          }}
          columns={
            tasksKind === 'image_tasks'
              ? [
                  { title: 'ID', dataIndex: 'id', width: 120, ellipsis: true, copyable: true },
                  { title: '类型', dataIndex: 'taskType', width: 140 },
                  { title: '状态', dataIndex: 'status', width: 96 },
                  { title: '商品', dataIndex: 'productId', ellipsis: true },
                  { title: '错误', dataIndex: 'errorMessage', ellipsis: true },
                ]
              : [
                  { title: 'ID', dataIndex: 'id', width: 120, ellipsis: true, copyable: true },
                  { title: '类型', dataIndex: 'taskType', width: 140 },
                  { title: '状态', dataIndex: 'status', width: 96 },
                  { title: '商品', dataIndex: 'productId', ellipsis: true },
                  { title: '错误摘要', dataIndex: 'errorMessage', ellipsis: true },
                ]
          }
        />
      </Drawer>
    </PageContainer>
  );
}
