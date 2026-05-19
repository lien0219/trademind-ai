import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, PageContainer, ProFormDependency, ProFormSelect, ProFormText, ProFormTextArea, ProTable } from '@ant-design/pro-components';
import { CopyOutlined } from '@ant-design/icons';
import { Button, Descriptions, Drawer, Form, Image, Space, Spin, Tag, message, Alert } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { ImageTaskDetail, ImageTaskListRow } from '@/services/imageTasks';
import { displayNameForProvider, isProviderSelectable } from '@/constants/imageProviders';
import { useImageProviders } from '@/hooks/useImageProviders';
import { createImageTask, getImageTask, queryImageTasks, retryImageTask } from '@/services/imageTasks';
import { createProductImage } from '@/services/products';

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
  if (s === 'success') return <Tag color="success">success</Tag>;
  if (s === 'failed') return <Tag color="error">failed</Tag>;
  if (s === 'retrying') return <Tag color="warning">等待重试</Tag>;
  if (s === 'running' || s === 'pending') return <Tag color="processing">处理中</Tag>;
  if (s === 'cancelled') return <Tag>cancelled</Tag>;
  return <Tag>{s || '-'}</Tag>;
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

const TASK_TYPES = [
  { label: 'remove_background', value: 'remove_background' },
  { label: 'replace_background', value: 'replace_background' },
  { label: 'generate_scene', value: 'generate_scene' },
  { label: 'resize', value: 'resize' },
  { label: 'enhance', value: 'enhance' },
  { label: 'translate_image', value: 'translate_image' },
  { label: 'poster_generate', value: 'poster_generate' },
];

export default function ImageTasksPage() {
  const { caps, optionsForTask } = useImageProviders();
  const [createForm] = Form.useForm();
  const actionRef = useRef<ActionType>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<ImageTaskDetail | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  useEffect(() => {
    const iv = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      actionRef.current?.reload?.();
    }, 4000);
    return () => window.clearInterval(iv);
  }, []);

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
      render: (_, row) => dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '任务类型',
      dataIndex: 'taskType',
      width: 180,
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      valueType: 'select',
      valueEnum: {
        pending: { text: 'pending' },
        running: { text: 'running' },
        retrying: { text: 'retrying' },
        success: { text: 'success' },
        failed: { text: 'failed' },
        cancelled: { text: 'cancelled' },
      },
      render: (_, row) => statusTag(row.status),
    },
    {
      title: 'Provider',
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
      render: (_, row) => (row.nextRetryAt ? dayjs(row.nextRetryAt).format('YYYY-MM-DD HH:mm:ss') : '—'),
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
      ellipsis: true,
      search: false,
    },
    {
      title: '结果 URL',
      dataIndex: 'resultUrl',
      ellipsis: true,
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
    <PageContainer title="图片任务">
      <ProTable<ImageTaskListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1400 }}
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
          <Button key="create" type="primary" onClick={() => setCreateOpen(true)}>
            新建任务
          </Button>,
        ]}
      />

      <ModalForm<{
        taskType: string;
        provider?: string;
        productId?: string;
        sourceImageId?: string;
        sourceImageUrl?: string;
        prompt?: string;
        negativePrompt?: string;
        scene?: string;
        style?: string;
        size?: string;
        background?: string;
        platform?: string;
        rbPrompt?: string;
        rbNegativePrompt?: string;
        rbBackground?: string;
        rbStyle?: string;
        rbPlatform?: string;
        rbSize?: string;
        inputJson: string;
      }>
        form={createForm}
        title="新建图片任务"
        open={createOpen}
        onOpenChange={setCreateOpen}
        initialValues={{
          taskType: 'resize',
          provider: '',
          productId: '',
          sourceImageId: '',
          sourceImageUrl: '',
          prompt: '',
          negativePrompt: '',
          scene: 'minimal studio',
          style: 'clean ecommerce',
          size: '1024x1024',
          background: 'white studio background',
          platform: 'TikTok Shop',
          rbPrompt: '',
          rbNegativePrompt: '',
          rbBackground: 'white studio background',
          rbStyle: 'clean ecommerce',
          rbPlatform: 'TikTok Shop',
          rbSize: '1024x1024',
          inputJson: '{}',
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          let extra: Record<string, unknown> = {};
          const raw = (values.inputJson ?? '').trim();
          if (raw) {
            try {
              extra = JSON.parse(raw) as Record<string, unknown>;
            } catch {
              message.error('input 需为合法 JSON');
              return false;
            }
          }
          const input: Record<string, unknown> = { ...extra };
          const tt = (values.taskType ?? '').trim();
          if (tt === 'generate_scene') {
            const pick = {
              prompt: (values.prompt ?? '').trim(),
              negativePrompt: (values.negativePrompt ?? '').trim(),
              scene: (values.scene ?? '').trim(),
              style: (values.style ?? '').trim(),
              size: (values.size ?? '').trim(),
              background: (values.background ?? '').trim(),
              platform: (values.platform ?? '').trim(),
            };
            Object.assign(input, pick);
          }
          if (tt === 'replace_background') {
            const pick = {
              prompt: (values.rbPrompt ?? '').trim(),
              negativePrompt: (values.rbNegativePrompt ?? '').trim(),
              background: (values.rbBackground ?? '').trim(),
              style: (values.rbStyle ?? '').trim(),
              platform: (values.rbPlatform ?? '').trim(),
              size: (values.rbSize ?? '').trim(),
            };
            Object.assign(input, pick);
          }
          try {
            const task = await createImageTask({
              taskType: values.taskType,
              provider: values.provider?.trim() || undefined,
              productId: values.productId?.trim() || undefined,
              sourceImageId: values.sourceImageId?.trim() || undefined,
              sourceImageUrl: values.sourceImageUrl?.trim() || undefined,
              input,
            });
            if (task.status === 'pending' || task.status === 'running') {
              message.success('图片任务已提交，正在后台处理');
            } else if (task.status === 'success') {
              message.success('任务已完成');
            } else if (task.status === 'failed') {
              message.warning(task.errorMessage || '任务失败');
            } else {
              message.success('已创建');
            }
            actionRef.current?.reload();
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || '创建失败');
            return false;
          }
        }}
      >
        <ProFormSelect
          name="taskType"
          label="任务类型"
          options={TASK_TYPES.map((t) => ({
            ...t,
            disabled: !caps.some(
              (c) => isProviderSelectable(c) && c.supportedTasks.includes(t.value),
            ),
          }))}
          rules={[{ required: true }]}
          fieldProps={{
            onChange: (v: string) => {
              const opts = optionsForTask(v);
              const first = opts.find((o) => o.value && !o.disabled);
              if (v === 'remove_background') {
                createForm.setFieldsValue({ provider: 'removebg' });
              } else if (v === 'generate_scene') {
                createForm.setFieldsValue({
                  provider: first?.value === 'removebg' ? 'dashscope_image' : first?.value ?? 'dashscope_image',
                });
              } else if (v === 'replace_background') {
                createForm.setFieldsValue({ provider: first?.value ?? '' });
              } else {
                createForm.setFieldsValue({ provider: first?.value ?? '' });
              }
            },
          }}
        />
        <ProFormDependency name={['taskType']}>
          {({ taskType }: { taskType?: string }) => (
            <ProFormSelect
              name="provider"
              label="Provider"
              options={optionsForTask(taskType ?? '')}
              extra="仅显示支持当前任务类型的 Provider；去背景推荐 remove.bg；场景图推荐通义万相 / OpenAI / 火山方舟；高级自定义推荐 ComfyUI"
            />
          )}
        </ProFormDependency>
        <ProFormDependency name={['taskType', 'provider']}>
          {(dep: { taskType?: string; provider?: string }) =>
            dep.taskType === 'replace_background' &&
            String(dep.provider ?? '')
              .trim()
              .toLowerCase() === 'openai_image' ? (
              <Alert
                style={{ marginBottom: 16 }}
                type="info"
                showIcon
                message="OpenAI replace_background 会由后端读取源图并提交给 OpenAI，不需要前端直连。"
              />
            ) : null
          }
        </ProFormDependency>
        <ProFormText name="productId" label="商品 ID（可选）" />
        <ProFormText
          name="sourceImageId"
          label="源图 sourceImageId（可选）"
          placeholder="files.id 或 product_images.id（UUID）"
          extra="填写后可使用本地/云端非公网源图；与源图 URL 二选一或同时提供（优先按 ID 解析）。"
        />
        <ProFormDependency name={['taskType', 'provider', 'sourceImageId']}>
          {(dep: { taskType?: string; provider?: string; sourceImageId?: string }) => {
            const tt = dep.taskType ?? '';
            const p = String(dep.provider ?? '')
              .trim()
              .toLowerCase();
            const optionalSrc =
              tt === 'generate_scene' && (p === '' || p === 'openai_image' || p === 'comfyui');
            return (
              <ProFormText
                name="sourceImageUrl"
                label={optionalSrc ? '源图 URL（可选）' : '源图 URL'}
                extra={
                  optionalSrc
                    ? 'OpenAI / ComfyUI 场景可不填；有参考图时请填公网可访问 URL，或在商品详情用「商品图」创建'
                    : tt === 'remove_background'
                      ? '可选：须公网可访问的直链；或使用上方的 sourceImageId（推荐本地/存储图）。'
                      : tt === 'replace_background' && (p === 'openai_image' || p === '')
                      ? '可选：公网直链或与 sourceImageId 二选一；OpenAI 换背景由后端读取源图并提交，无需前端直连 OpenAI。'
                      : '请填写可从公网抓取的可访问 HTTPS 图像地址'
                }
                rules={[
                  {
                    validator: async (_rule, val) => {
                      if (optionalSrc) {
                        return Promise.resolve();
                      }
                      const id = String(dep.sourceImageId ?? '').trim();
                      if ((tt === 'remove_background' || tt === 'replace_background') && id) {
                        return Promise.resolve();
                      }
                      if (String(val ?? '').trim()) {
                        return Promise.resolve();
                      }
                      if (tt === 'remove_background' || tt === 'replace_background') {
                        return Promise.reject(new Error('请填写源图 URL 或 sourceImageId'));
                      }
                      return Promise.reject(new Error('请填写可访问的源图 URL'));
                    },
                  },
                ]}
              />
            );
          }}
        </ProFormDependency>
        <ProFormDependency name={['taskType']}>
          {(dep: { taskType?: string }) =>
            dep.taskType === 'generate_scene' ? (
              <>
                <ProFormTextArea name="prompt" label="Prompt（可选）" fieldProps={{ rows: 4 }} />
                <ProFormText name="negativePrompt" label="Negative prompt（可选）" />
                <ProFormText name="scene" label="Scene（可选）" placeholder="minimal studio" />
                <ProFormText name="style" label="Style（可选）" placeholder="clean ecommerce" />
                <ProFormText name="size" label="尺寸（可选）" placeholder="1024x1024" />
                <ProFormText name="background" label="背景（可选）" placeholder="white studio background" />
                <ProFormText name="platform" label="平台（可选）" placeholder="TikTok Shop" />
              </>
            ) : null
          }
        </ProFormDependency>
        <ProFormDependency name={['taskType']}>
          {(dep: { taskType?: string }) =>
            dep.taskType === 'replace_background' ? (
              <>
                <ProFormTextArea name="rbPrompt" label="Prompt（可选）" fieldProps={{ rows: 3 }} />
                <ProFormText name="rbNegativePrompt" label="Negative prompt（可选）" />
                <ProFormText name="rbBackground" label="目标背景" placeholder="white studio background" />
                <ProFormText name="rbStyle" label="风格（可选）" placeholder="clean ecommerce" />
                <ProFormText name="rbPlatform" label="平台（可选）" placeholder="TikTok Shop" />
                <ProFormText name="rbSize" label="尺寸（可选）" placeholder="1024x1024" />
              </>
            ) : null
          }
        </ProFormDependency>
        <ProFormTextArea
          name="inputJson"
          label="追加 Input（JSON，可选）"
          fieldProps={{ rows: 4, style: { fontFamily: 'monospace' } }}
          extra="将与上方字段合并后提交；结构化字段可被此处同名键覆盖"
        />
      </ModalForm>

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
                    message.success('已提交重试，正在后台处理');
                    const row = await getImageTask(detail.id);
                    setDetail(row);
                    actionRef.current?.reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '重试失败');
                  }
                }}
              >
                重试
              </Button>
            ) : null}
            {detail?.status === 'success' && detail.resultUrl ? (
              <Button
                icon={<CopyOutlined />}
                onClick={() => {
                  void navigator.clipboard.writeText(detail.resultUrl!);
                  message.success('已复制结果 URL');
                }}
              >
                复制结果 URL
              </Button>
            ) : null}
            {detail?.status === 'success' && detail.productId && detail.resultFileId ? (
              <Button
                type="primary"
                ghost
                onClick={async () => {
                  try {
                    await createProductImage(detail.productId!, {
                      fileId: detail.resultFileId,
                      imageType: 'detail',
                    });
                    message.success('已添加到商品详情图');
                    actionRef.current?.reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '添加失败');
                  }
                }}
              >
                添加到商品图片
              </Button>
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
              <Descriptions.Item label="任务类型">{detail.taskType}</Descriptions.Item>
              <Descriptions.Item label="状态">{statusTag(detail.status)}</Descriptions.Item>
              <Descriptions.Item label="Provider">{detail.provider || '—'}</Descriptions.Item>
              <Descriptions.Item label="商品 ID">{detail.productId || '—'}</Descriptions.Item>
              <Descriptions.Item label="源图 ID">{detail.sourceImageId || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建者">{detail.createdBy || '—'}</Descriptions.Item>
              <Descriptions.Item label="开始时间">
                {detail.startedAt ? dayjs(detail.startedAt).format('YYYY-MM-DD HH:mm:ss') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间">
                {detail.finishedAt ? dayjs(detail.finishedAt).format('YYYY-MM-DD HH:mm:ss') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {dayjs(detail.createdAt).format('YYYY-MM-DD HH:mm:ss')}
              </Descriptions.Item>
              <Descriptions.Item label="自动重试">
                {detail.retryCount ?? 0} / {detail.maxRetries ?? '—'}
              </Descriptions.Item>
              <Descriptions.Item label="下次自动重试">
                {detail.nextRetryAt ? dayjs(detail.nextRetryAt).format('YYYY-MM-DD HH:mm:ss') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="错误信息">{detail.errorMessage || '—'}</Descriptions.Item>
            </Descriptions>
            {(detail.sourceImageUrl || detail.resultUrl) && (
              <Space align="start" size={24} wrap style={{ marginBottom: 24 }}>
                {detail.sourceImageUrl ? (
                  <div>
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>源图</div>
                    <Image src={detail.sourceImageUrl} width={200} style={{ objectFit: 'contain', borderRadius: 6 }} />
                  </div>
                ) : null}
                {detail.resultUrl ? (
                  <div>
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>结果图</div>
                    <Image src={detail.resultUrl} width={200} style={{ objectFit: 'contain', borderRadius: 6 }} />
                  </div>
                ) : null}
              </Space>
            )}
            <JsonBlock title="Input（JSON）" value={detail.input} />
            <JsonBlock title="Output（JSON）" value={detail.output} />
          </>
        ) : (
          <div style={{ color: 'var(--ant-color-text-secondary)' }}>暂无数据</div>
        )}
      </Drawer>
    </PageContainer>
  );
}
