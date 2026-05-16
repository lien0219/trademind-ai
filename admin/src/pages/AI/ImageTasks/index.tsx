import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, PageContainer, ProFormSelect, ProFormText, ProFormTextArea, ProTable } from '@ant-design/pro-components';
import { CopyOutlined } from '@ant-design/icons';
import { Button, Descriptions, Drawer, Form, Image, Space, Spin, Tag, message } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { ImageTaskDetail, ImageTaskListRow } from '@/services/imageTasks';
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
    if (detail.status !== 'pending' && detail.status !== 'running') return;
    const id = detail.id;
    const iv = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      void (async () => {
        try {
          const row = await getImageTask(id);
          setDetail(row);
          if (row.status !== 'pending' && row.status !== 'running') {
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
        success: { text: 'success' },
        failed: { text: 'failed' },
        cancelled: { text: 'cancelled' },
      },
      render: (_, row) => statusTag(row.status),
    },
    {
      title: 'Provider',
      dataIndex: 'provider',
      width: 100,
      ellipsis: true,
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
        productId: string;
        sourceImageUrl: string;
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
          sourceImageUrl: '',
          inputJson: '{}',
        }}
        modalProps={{ destroyOnClose: true }}
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
          try {
            const task = await createImageTask({
              taskType: values.taskType,
              provider: values.provider?.trim() || undefined,
              productId: values.productId?.trim() || undefined,
              sourceImageUrl: values.sourceImageUrl?.trim() || undefined,
              input: extra,
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
          options={TASK_TYPES}
          rules={[{ required: true }]}
          fieldProps={{
            onChange: (v: string) => {
              if (v === 'remove_background') {
                createForm.setFieldsValue({ provider: 'removebg' });
              }
            },
          }}
        />
        <ProFormSelect
          name="provider"
          label="Provider"
          options={[
            { label: '默认（跟随系统「图片 AI」设置里的 provider）', value: '' },
            { label: 'noop', value: 'noop' },
            { label: 'remove.bg', value: 'removebg' },
          ]}
          extra="去背景建议选择 remove.bg，且源图 URL 须公网可访问"
        />
        <ProFormText name="productId" label="商品 ID（可选）" />
        <ProFormText
          name="sourceImageUrl"
          label="源图 URL"
          rules={[{ required: true, message: '请填写可访问的源图 URL' }]}
        />
        <ProFormTextArea
          name="inputJson"
          label="Input（JSON）"
          fieldProps={{ rows: 6, style: { fontFamily: 'monospace' } }}
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
        destroyOnClose
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
