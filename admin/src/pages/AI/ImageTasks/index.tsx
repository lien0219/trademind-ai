import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { CopyOutlined } from '@ant-design/icons';
import { Button, Card, Descriptions, Drawer, Image, Space, Spin, Tag, message, Typography } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation } from '@umijs/max';
import { CreateImageTaskModal } from '@/components/CreateImageTaskModal';
import type { ImageTaskDetail, ImageTaskItemRow, ImageTaskListRow } from '@/services/imageTasks';
import { displayNameForProvider } from '@/constants/imageProviders';
import { useImageProviders } from '@/hooks/useImageProviders';
import {
  applyImageTaskResult,
  getImageTask,
  IMAGE_TASK_TEMPLATES,
  isImageTaskSuccessStatus,
  listImageTaskItems,
  parseTranslateTaskOutput,
  queryImageTasks,
  retryImageTask,
  saveImageTaskItemToProduct,
  setImageTaskItemAsMain,
  taskTypeLabel,
  translateLangLabel,
  translateTaskWarnings,
} from '@/services/imageTasks';
import { COLLECT_TASK_STATUS } from '@/constants/status';

function formatJsonPretty(v: unknown): string {
  if (v == null) return '';
  try {
    return typeof v === 'string' ? v : JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

function statusTag(status: string) {
  const s = status?.trim() || '';
  const m = COLLECT_TASK_STATUS[s as keyof typeof COLLECT_TASK_STATUS];
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

function TranslateResultPanel({ output }: { output: unknown }) {
  const parsed = parseTranslateTaskOutput(output);
  if (!parsed) return null;
  const blocks = parsed.ocr?.blocks ?? [];
  const layout = parsed.quality?.layout;
  const warnings = translateTaskWarnings(parsed);
  const hasOverflow = (layout?.overflowBlocks ?? 0) > 0;
  return (
    <Card size="small" title="翻译结果摘要" style={{ marginBottom: 24 }}>
      <Descriptions column={2} size="small">
        <Descriptions.Item label="源语言">
          {translateLangLabel(parsed.sourceLanguage || parsed.ocr?.detectedLanguage || '—')}
        </Descriptions.Item>
        <Descriptions.Item label="目标语言">{translateLangLabel(parsed.targetLanguage || '—')}</Descriptions.Item>
        <Descriptions.Item label="识别文字数量">
          {parsed.quality?.textBlocksCount ?? parsed.ocr?.textBlocksCount ?? blocks.length}
        </Descriptions.Item>
        <Descriptions.Item label="已翻译数量">{parsed.quality?.translatedBlocksCount ?? '—'}</Descriptions.Item>
        <Descriptions.Item label="自动换行数量">{layout?.autoWrappedBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="自动缩小字号数量">{layout?.fontResizedBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="自动精简文案数量">{layout?.simplifiedBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="是否存在文字溢出">{hasOverflow ? '是' : '否'}</Descriptions.Item>
      </Descriptions>
      {warnings.length > 0 ? (
        <div style={{ marginTop: 12 }}>
          {warnings.map((w) => (
            <Tag key={w} color="warning" style={{ marginBottom: 4 }}>
              {w}
            </Tag>
          ))}
        </div>
      ) : null}
      {blocks.length > 0 ? (
        <div style={{ marginTop: 12 }}>
          <Typography.Text strong>翻译文本预览</Typography.Text>
          <ul style={{ marginTop: 8, paddingLeft: 20, marginBottom: 0 }}>
            {blocks.slice(0, 12).map((b, i) => (
              <li key={`${b.text}-${i}`} style={{ marginBottom: 4 }}>
                <Typography.Text type="secondary">{b.text || '—'}</Typography.Text>
                {' → '}
                <Typography.Text>{b.translatedText || '—'}</Typography.Text>
                {b.shortTranslatedText && b.shortTranslatedText !== b.translatedText ? (
                  <>
                    {' '}
                    <Typography.Text type="secondary">（精简：{b.shortTranslatedText}）</Typography.Text>
                  </>
                ) : null}
              </li>
            ))}
          </ul>
          {blocks.length > 12 ? (
            <Typography.Text type="secondary">… 共 {blocks.length} 段文字</Typography.Text>
          ) : null}
        </div>
      ) : null}
    </Card>
  );
}

export default function ImageTasksPage() {
  const location = useLocation();
  const { caps } = useImageProviders();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const statusFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('status')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<ImageTaskDetail | null>(null);
  const [taskItems, setTaskItems] = useState<ImageTaskItemRow[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [createPrefill, setCreatePrefill] = useState<{ taskType?: string }>({});

  useEffect(() => {
    const iv = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      actionRef.current?.reload?.();
    }, 4000);
    return () => window.clearInterval(iv);
  }, []);

  useEffect(() => {
    if (!statusFromUrl) return;
    formRef.current?.setFieldsValue?.({ status: statusFromUrl });
    actionRef.current?.reload?.();
  }, [statusFromUrl]);

  useEffect(() => {
    if (!drawerOpen || !detail) return;
    if (detail.status !== 'pending' && detail.status !== 'running' && detail.status !== 'retrying') return;
    const id = detail.id;
    const iv = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      void (async () => {
        try {
          const row = await getImageTask(id);
          setDetail(row);
          if (row.status !== 'pending' && row.status !== 'running' && row.status !== 'retrying') {
            actionRef.current?.reload?.();
          }
        } catch {
          /* ignore transient errors during poll */
        }
      })();
    }, 4000);
    return () => window.clearInterval(iv);
  }, [drawerOpen, detail?.id, detail?.status]);

  const openDetail = useCallback(async (id: string) => {
    setDrawerOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const row = await getImageTask(id);
      setDetail(row);
      try {
        const itemsRes = await listImageTaskItems(id);
        setTaskItems(itemsRes.list ?? []);
      } catch {
        setTaskItems([]);
      }
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const columns: ProColumns<ImageTaskListRow>[] = [
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 172,
      search: false,
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '任务类型',
      dataIndex: 'taskType',
      width: 180,
      ellipsis: true,
      render: (_, row) => taskTypeLabel(row.taskType),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 148,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(COLLECT_TASK_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => statusTag(row.status),
    },
    {
      title: '图片服务',
      dataIndex: 'provider',
      width: 120,
      ellipsis: true,
      render: (_, row) => displayNameForProvider(caps, row.provider ?? ''),
    },
    {
      title: '重试',
      width: 130,
      search: false,
      render: (_, row) => {
        const rc = row.retryCount ?? 0;
        const mr = row.maxRetries ?? 0;
        return (
          <span>
            {rc}/{mr || '—'}
          </span>
        );
      },
    },
    {
      title: '下次自动重试',
      dataIndex: 'nextRetryAt',
      width: 172,
      search: false,
      render: (_, row) => (row.nextRetryAt ? formatDateTime(row.nextRetryAt) : '—'),
    },
    {
      title: '商品 ID',
      dataIndex: 'productId',
      width: 260,
      ellipsis: true,
      copyable: true,
    },
    {
      title: '源图 URL',
      dataIndex: 'sourceImageUrl',
      width: 240,
      ellipsis: true,
      search: false,
    },
    {
      title: '结果 URL',
      dataIndex: 'resultUrl',
      width: 240,
      ellipsis: true,
      search: false,
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      width: 200,
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
      width: 160,
      search: false,
      render: (_, row) => [
        <Button key="detail" type="link" onClick={() => void openDetail(row.id)}>
          查看详情
        </Button>,
        row.status === 'failed' ? (
          <Button
            key="retry"
            type="link"
            onClick={async () => {
              try {
                await retryImageTask(row.id);
                message.success('已提交重试，正在后台处理');
                actionRef.current?.reload();
              } catch (e: unknown) {
                message.error((e as Error)?.message || '重试失败');
              }
            }}
          >
            重试
          </Button>
        ) : null,
      ],
    },
  ];

  return (
    <PageContainer title="AI 图片任务">
      <Card size="small" style={{ marginBottom: 16 }} title="快捷模板">
        <Space wrap>
          {IMAGE_TASK_TEMPLATES.map((tpl) => (
            <Button
              key={tpl.taskType}
              onClick={() => {
                setCreatePrefill({ taskType: tpl.taskType });
                setCreateOpen(true);
              }}
            >
              {tpl.title}
            </Button>
          ))}
        </Space>
        <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
          所有 AI 结果图会自动上传到「设置 → 存储设置」当前启用的存储位置，不会直接使用 Provider 临时 URL。
        </Typography.Paragraph>
      </Card>
      <ProTable<ImageTaskListRow>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1960 }}
        tableStyle={{ width: '100%', minWidth: '100%' }}
        request={async (params) => {
          const res = await queryImageTasks({
            page: params.current,
            pageSize: params.pageSize,
            taskType: params.taskType as string | undefined,
            status: params.status as string | undefined,
            provider: params.provider as string | undefined,
            productId: params.productId as string | undefined,
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
        toolBarRender={() => [
          <Button
            key="create"
            type="primary"
            onClick={() => {
              setCreatePrefill({});
              setCreateOpen(true);
            }}
          >
            新建任务
          </Button>,
        ]}
      />

      <CreateImageTaskModal
        open={createOpen}
        onOpenChange={setCreateOpen}
        prefill={createPrefill}
        allowProductIdInput
        onSuccess={() => actionRef.current?.reload()}
      />

      <Drawer
        title="图片任务详情"
        width={720}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setDetail(null);
        }}
        destroyOnHidden
        extra={
          <Space wrap>
            {detail?.status === 'failed' ? (
              <Button
                type="primary"
                onClick={async () => {
                  if (!detail?.id) return;
                  try {
                    await retryImageTask(detail.id);
                    message.success(
                      detail.taskType === 'translate_image_text' ? '已重新提交翻译任务' : '已提交重试，正在后台处理',
                    );
                    const row = await getImageTask(detail.id);
                    setDetail(row);
                    actionRef.current?.reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '重试失败');
                  }
                }}
              >
                {detail.taskType === 'translate_image_text' ? '重新翻译' : '重试'}
              </Button>
            ) : null}
            {detail && isImageTaskSuccessStatus(detail.status) && detail.resultUrl ? (
              <>
              <Button
                icon={<CopyOutlined />}
                onClick={() => {
                  void navigator.clipboard.writeText(detail.resultUrl!);
                  message.success('已复制结果 URL');
                }}
              >
                复制结果 URL
              </Button>
              <Button
                href={detail.resultUrl}
                target="_blank"
                rel="noreferrer"
              >
                下载图片
              </Button>
              </>
            ) : null}
            {detail && isImageTaskSuccessStatus(detail.status) && detail.productId && (detail.resultUrl || detail.resultFileId) ? (
              <>
                <Button
                  type="primary"
                  ghost
                  onClick={async () => {
                    try {
                      await applyImageTaskResult(detail.id, {
                        productId: detail.productId!,
                        applyMode: 'ai_generated',
                      });
                      message.success('已保存到商品图片库');
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '保存失败');
                    }
                  }}
                >
                  保存到商品图片
                </Button>
                <Button
                  onClick={async () => {
                    try {
                      await applyImageTaskResult(detail.id, {
                        productId: detail.productId!,
                        applyMode: 'main',
                        setBest: true,
                      });
                      message.success('已设为主图');
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '设置失败');
                    }
                  }}
                >
                  设为主图
                </Button>
                <Button
                  onClick={async () => {
                    try {
                      await applyImageTaskResult(detail.id, {
                        productId: detail.productId!,
                        applyMode: 'detail',
                      });
                      message.success('已设为详情图');
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '设置失败');
                    }
                  }}
                >
                  设为详情图
                </Button>
              </>
            ) : null}
          </Space>
        }
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Spin />
          </div>
        ) : detail ? (
          <>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 24 }}>
              <Descriptions.Item label="ID">{detail.id}</Descriptions.Item>
              <Descriptions.Item label="任务类型">{taskTypeLabel(detail.taskType)}</Descriptions.Item>
              <Descriptions.Item label="状态">{statusTag(detail.status)}</Descriptions.Item>
              <Descriptions.Item label="图片服务">{detail.provider || '—'}</Descriptions.Item>
              <Descriptions.Item label="商品 ID">{detail.productId || '—'}</Descriptions.Item>
              <Descriptions.Item label="源图 ID">{detail.sourceImageId || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建者">{detail.createdBy || '—'}</Descriptions.Item>
              <Descriptions.Item label="开始时间">
                {detail.startedAt ? formatDateTime(detail.startedAt) : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间">
                {detail.finishedAt ? formatDateTime(detail.finishedAt) : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {formatDateTime(detail.createdAt)}
              </Descriptions.Item>
              <Descriptions.Item label="自动重试">
                {detail.retryCount ?? 0} / {detail.maxRetries ?? '—'}
              </Descriptions.Item>
              <Descriptions.Item label="下次自动重试">
                {detail.nextRetryAt ? formatDateTime(detail.nextRetryAt) : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="错误信息">{detail.errorMessage || '—'}</Descriptions.Item>
            </Descriptions>
            {(detail.sourceImageUrl || detail.resultUrl) && (
              <Space align="start" size={24} wrap style={{ marginBottom: 24 }}>
                {detail.sourceImageUrl ? (
                  <div>
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>原图</div>
                    <Image src={detail.sourceImageUrl} width={200} style={{ objectFit: 'contain', borderRadius: 6 }} />
                  </div>
                ) : null}
                {detail.resultUrl ? (
                  <div>
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>翻译后图片</div>
                    <Image src={detail.resultUrl} width={200} style={{ objectFit: 'contain', borderRadius: 6 }} />
                  </div>
                ) : null}
              </Space>
            )}
            {detail.taskType === 'translate_image_text' && detail.output ? (
              <TranslateResultPanel output={detail.output} />
            ) : null}
            {taskItems.length > 0 ? (
              <div style={{ marginBottom: 24 }}>
                <div style={{ marginBottom: 8, fontWeight: 600 }}>子任务结果</div>
                <Space direction="vertical" style={{ width: '100%' }}>
                  {taskItems.map((item) => (
                    <Card key={item.id} size="small">
                      <Space align="start" wrap>
                        {item.sourceImageUrl ? (
                          <div>
                            <div style={{ fontSize: 12, marginBottom: 4 }}>原图</div>
                            <Image src={item.sourceImageUrl} width={120} />
                          </div>
                        ) : null}
                        {item.outputImageUrl ? (
                          <div>
                            <div style={{ fontSize: 12, marginBottom: 4 }}>结果</div>
                            <Image src={item.outputImageUrl} width={120} />
                          </div>
                        ) : null}
                        <div>
                          <div>状态：{statusTag(item.status)}</div>
                          {item.isSelectedBest ? <Tag color="gold">推荐主图</Tag> : null}
                          {item.scoreJson ? (
                            <pre style={{ fontSize: 11, maxWidth: 280, overflow: 'auto' }}>
                              {formatJsonPretty(item.scoreJson)}
                            </pre>
                          ) : null}
                          {detail.productId && item.status === 'success' && item.outputImageUrl ? (
                            <Space wrap style={{ marginTop: 8 }}>
                              <Button
                                size="small"
                                onClick={() =>
                                  void setImageTaskItemAsMain(item.id, { productId: detail.productId! }).then(() =>
                                    message.success('已设为主图'),
                                  )
                                }
                              >
                                设为主图
                              </Button>
                              <Button
                                size="small"
                                onClick={() =>
                                  void saveImageTaskItemToProduct(item.id, {
                                    productId: detail.productId!,
                                    applyMode: 'detail',
                                  }).then(() => message.success('已设为详情图'))
                                }
                              >
                                设为详情图
                              </Button>
                              <Button
                                size="small"
                                onClick={() =>
                                  void saveImageTaskItemToProduct(item.id, {
                                    productId: detail.productId!,
                                    applyMode: 'ai_generated',
                                  }).then(() => message.success('已保存到商品图库'))
                                }
                              >
                                保存到商品
                              </Button>
                            </Space>
                          ) : null}
                        </div>
                      </Space>
                    </Card>
                  ))}
                </Space>
              </div>
            ) : null}
            <JsonBlock title="任务输入" value={detail.input} />
            <JsonBlock title="任务输出" value={detail.output} />
          </>
        ) : (
          <div style={{ color: 'var(--ant-color-text-secondary)' }}>暂无数据</div>
        )}
      </Drawer>
    </PageContainer>
  );
}
