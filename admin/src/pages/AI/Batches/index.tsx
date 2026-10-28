import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';

import {
  Button,
  Descriptions,
  Drawer,
  Space,
  Table,
  Tag,
  Typography,
  message,
  Alert,
} from 'antd';
import { history, useSearchParams } from '@umijs/max';
import { useEffect, useRef, useState } from 'react';
import { Link } from '@umijs/renderer-react';
import { AI_FIELD_COPY, commonStatusLabel } from '@/constants/copywriting';
import { taskTypeLabel } from '@/services/imageTasks';
import { isProductionBuild } from '@/utils/runtimeEnvironment';
import {
  applyAiBatchResults,
  fetchAiBatchDetail,
  fetchAiBatchTasks,
  fetchAiBatches,
  retryAiBatchFailed,
  type AIOperationBatchRow,
} from '@/services/aiBatches';

const OP_LABEL: Record<string, string> = {
  get title_optimize() { return tr("批量标题优化"); },
  get description_generate() { return tr("批量描述生成"); },
  get image_remove_background() { return tr("批量去背景"); },
  get image_generate_scene() { return tr("批量场景图"); },
  get image_replace_background() { return tr("批量换背景"); },
  get image_batch_generate_main() { return tr("批量主图生成"); },
  get image_translate_image_text() { return tr("批量图片文字翻译"); },
  get image_score() { return tr("批量图片评分"); },
  get image_select_best_main() { return tr("批量自动选主图"); },
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
  useLanguage();
  const actionRef = useRef<ActionType>();
  const [searchParams] = useSearchParams();
  const openedQueryIdRef = useRef<string>();
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
    { title: tr("创建时间"), dataIndex: 'createdAt', width: 176, valueType: 'dateTime',
      render: (_, row) => formatDateTime(row.createdAt), search: false },
    { title: tr("批次号"), dataIndex: 'batchNo', width: 140, ellipsis: true, copyable: true, search: false },
    {
      title: tr("类型"),
      dataIndex: 'operationType',
      width: 160,
      valueType: 'select',
      valueEnum: Object.fromEntries(Object.keys(OP_LABEL).map((k) => [k, { text: OP_LABEL[k] }])),
      render: (_, row) => OP_LABEL[row.operationType] ?? row.operationType,
    },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 120,
      render: (_, row) => (
        <Tag color={STATUS_COLOR[row.status] ?? 'default'}>{commonStatusLabel(row.status)}</Tag>
      ),
    },
    { title: tr("商品数"), dataIndex: 'productCount', width: 80, search: false },
    { title: tr("任务数"), dataIndex: 'taskCount', width: 80, search: false },
    { title: tr("成功"), dataIndex: 'successCount', width: 72, search: false },
    { title: tr("失败"), dataIndex: 'failedCount', width: 72, search: false },
    { title: tr("跳过"), dataIndex: 'skippedCount', width: 72, search: false },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 220,
      render: (_, row) => [
        <Typography.Link
          key="d"
          onClick={() => void openDetail(row.id)}
        >
          {tr("详情")}
        </Typography.Link>,
        <Typography.Link key="t" onClick={() => void openTasks(row)}>
          {tr("子任务")}
        </Typography.Link>,
        ...(!isProductionBuild
          ? [
              <Typography.Link key="r" onClick={() => void runRetry(row.id)}>
                {tr("重试失败")}
              </Typography.Link>,
              row.operationType === 'title_optimize' || row.operationType === 'description_generate' ? (
                <Typography.Link key="a" onClick={() => void runApply(row.id)}>
                  {tr("应用结果")}
                </Typography.Link>
              ) : null,
            ]
          : []),
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
      message.error((e as Error)?.message || tr("加载失败"));
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
      message.success(tr("已触发重试"));
      actionRef.current?.reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("重试失败"));
    }
  };

  const runApply = async (id: string) => {
    try {
      const res = await applyAiBatchResults(id, { target: 'ai_field' });
      message.success(trFormat('已应用 {count} 条到 AI 文案字段', { count: res?.applied ?? 0 }));
      actionRef.current?.reload();
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("应用失败"));
    }
  };

  useEffect(() => {
    const queryId = (searchParams.get('id') || '').trim();
    if (!queryId || openedQueryIdRef.current === queryId) return;
    openedQueryIdRef.current = queryId;
    void openDetail(queryId);
  }, [searchParams]);

  return (
    <TmPageContainer
      title={tr("历史 AI 批次")}
      subTitle={tr("查看历史批量 AI 任务的执行记录与结果。")}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        {tr("批量结果默认写入商品的")} {AI_FIELD_COPY.aiTitle} / {AI_FIELD_COPY.aiDescription} {tr("字段，或进入图片任务列表，不会自动覆盖正式标题、详情或替换主图。详情见")}{' '}
        <Link to="/settings/ai">{tr("AI 设置")}</Link>。
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
        title={tr("批次详情")}
        width={620}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        destroyOnHidden
      >
        {loadingDetail ? (
          <Typography.Text type="secondary">{tr("加载中…")}</Typography.Text>
        ) : detail?.batch ? (
          <>
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label={tr("批次号")}>{detail.batch.batchNo}</Descriptions.Item>
              <Descriptions.Item label={tr("类型")}>
                {OP_LABEL[detail.batch.operationType] ?? detail.batch.operationType}
              </Descriptions.Item>
              <Descriptions.Item label={tr("状态")}>
                <Tag color={STATUS_COLOR[detail.batch.status]}>{commonStatusLabel(detail.batch.status)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={tr("统计")}>
                {tr("商品")} {detail.batch.productCount} {tr("· 任务")} {detail.batch.taskCount} {tr("· 成功")} {detail.batch.successCount}{' '}
                {tr("· 失败")} {detail.batch.failedCount} {tr("· 跳过")} {detail.batch.skippedCount}
              </Descriptions.Item>
            </Descriptions>
            <TechnicalDetails>
              <TaskJsonBlock title={tr("批次输入")} value={detail.batch.input} />
              <TaskJsonBlock title={tr("批次输出")} value={detail.batch.output} last />
            </TechnicalDetails>
            {(detail.recentAiTasks?.length ?? 0) > 0 && (
              <>
                <Typography.Title level={5}>{tr("最近 AI 任务")}</Typography.Title>
                <Table
                  size="small"
                  pagination={false}
                  rowKey={(r) => (r as { id: string }).id}
                  dataSource={detail.recentAiTasks as object[]}
                  columns={[
                    { title: tr("状态"), dataIndex: 'status', width: 90, render: (v) => commonStatusLabel(v as string) },
                    { title: tr("商品"), dataIndex: 'productId', ellipsis: true },
                    {
                      title: tr("错误摘要"),
                      dataIndex: 'errorMessage',
                      ellipsis: true,
                    },
                  ]}
                />
              </>
            )}
            {(detail.recentImageTasks?.length ?? 0) > 0 && (
              <>
                <Typography.Title level={5}>{tr("最近图片任务")}</Typography.Title>
                <Table
                  size="small"
                  pagination={false}
                  rowKey={(r) => (r as { id: string }).id}
                  dataSource={detail.recentImageTasks as object[]}
                  columns={[
                    { title: tr("类型"), dataIndex: 'taskType', width: 140, render: (v) => taskTypeLabel(v as string) },
                    { title: tr("状态"), dataIndex: 'status', width: 96, render: (v) => commonStatusLabel(v as string) },
                    { title: tr("商品"), dataIndex: 'productId', ellipsis: true },
                  ]}
                />
              </>
            )}
            {!isProductionBuild ? (
              <Space style={{ marginTop: 16 }}>
                <Button
                  type="primary"
                  onClick={() =>
                    detail.batch &&
                    (detail.batch.operationType === 'title_optimize' ||
                      detail.batch.operationType === 'description_generate')
                      ? runApply(detail.batch.id)
                      : message.info(tr("仅文本批次可一键应用"))
                  }
                >
                  {tr("应用 AI 文案到草稿")}
                </Button>
                <Button onClick={() => currentId && runRetry(currentId)}>{tr("重试失败")}</Button>
              </Space>
            ) : null}
          </>
        ) : null}
      </Drawer>

      <Drawer
        title={tr("子任务")}
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
          request={async (p, _sort, _filter) => {
            if (!currentId)
              return { data: [] as Record<string, unknown>[], success: true, total: 0 };
            const res = await fetchAiBatchTasks(currentId, {
              page: p.current,
              pageSize: p.pageSize,
            });
            const d = res;
            setTasksKind(d?.kind ?? '');
            return {
              data: (d?.list ?? []) as Record<string, unknown>[],
              success: true,
              total: d?.pagination?.total ?? 0,
            };
          }}
          columns={
            tasksKind === 'image_tasks'
              ? [
                  { title: tr("任务编号"), dataIndex: 'id', width: 120, ellipsis: true, copyable: true },
                  { title: tr("类型"), dataIndex: 'taskType', width: 140, render: (v) => taskTypeLabel(v as string) },
                  { title: tr("状态"), dataIndex: 'status', width: 96, render: (v) => commonStatusLabel(v as string) },
                  { title: tr("商品"), dataIndex: 'productId', ellipsis: true },
                  { title: tr("错误"), dataIndex: 'errorMessage', ellipsis: true },
                ]
              : [
                  { title: tr("任务编号"), dataIndex: 'id', width: 120, ellipsis: true, copyable: true },
                  { title: tr("类型"), dataIndex: 'taskType', width: 140 },
                  { title: tr("状态"), dataIndex: 'status', width: 96, render: (v) => commonStatusLabel(v as string) },
                  { title: tr("商品"), dataIndex: 'productId', ellipsis: true },
                  { title: tr("错误摘要"), dataIndex: 'errorMessage', ellipsis: true },
                ]
          }
        />
      </Drawer>
    </TmPageContainer>
  );
}
