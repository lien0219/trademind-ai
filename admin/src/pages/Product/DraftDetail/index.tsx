import type { UploadRequestOption } from 'rc-upload/lib/interface';
import type { ColumnsType } from 'antd/es/table';
import {
  EditableProTable,
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { Button, Card, Descriptions, Form, Image, Input, InputNumber, Modal, Popconfirm, Select, Space, Spin, Tabs, Tooltip, Typography, Upload, message } from 'antd';
import {
  ArrowUpOutlined,
  DeleteOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PRODUCT_STATUS } from '@/constants/status';
import { uploadFile } from '@/services/files';
import {
  applyAiDescription,
  applyProductAITitle,
  createProductImage,
  createProductSku,
  deleteProduct,
  deleteProductImage,
  deleteProductSku,
  fetchProductAITasks,
  fetchProductDetail,
  generateDescription,
  optimizeProductTitle,
  reorderProductImages,
  updateProduct,
  updateProductImage,
  updateProductSku,
  type AITaskRow,
  type GenerateDescriptionResult,
  type OptimizeTitleResult,
  type ProductDetail,
  type ProductImageRow,
  type ProductSKURow,
} from '@/services/products';
import { createImageTask } from '@/services/imageTasks';

type SKUEditable = ProductSKURow & { attrsText?: string };

const PRODUCT_STATUS_OPTIONS = Object.entries(PRODUCT_STATUS).map(([value, v]) => ({
  label: v.text,
  value,
}));

const IMAGE_TYPE_OPTIONS = [
  { label: '主图 (main)', value: 'main' },
  { label: '详情图 (detail)', value: 'detail' },
  { label: 'SKU 图 (sku)', value: 'sku' },
];

function attrsToText(attrs?: Record<string, unknown>): string {
  if (!attrs || typeof attrs !== 'object') return '';
  try {
    return JSON.stringify(attrs);
  } catch {
    return '';
  }
}

function imageTypeLabel(t: string): string {
  if (t === 'main') return '主图';
  if (t === 'detail' || t === 'description') return '详情图';
  if (t === 'sku') return 'SKU';
  return t;
}

export default function ProductDraftDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id ?? '';
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<ProductDetail | null>(null);
  const [err, setErr] = useState<string>();
  const [aiOpen, setAiOpen] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiResult, setAiResult] = useState<OptimizeTitleResult | null>(null);
  const [aiTasks, setAiTasks] = useState<AITaskRow[]>([]);
  const [aiForm] = Form.useForm();
  const [descOpen, setDescOpen] = useState(false);
  const [descBusy, setDescBusy] = useState(false);
  const [descResult, setDescResult] = useState<GenerateDescriptionResult | null>(null);
  const [descForm] = Form.useForm();
  const [skuRows, setSkuRows] = useState<SKUEditable[]>([]);
  const [imgModalOpen, setImgModalOpen] = useState(false);
  const [imgEdit, setImgEdit] = useState<ProductImageRow | null>(null);
  const [imgBusy, setImgBusy] = useState(false);
  const [lastUpload, setLastUpload] = useState<{ id: string; url: string; objectKey: string } | null>(null);
  const [aiImgTaskType, setAiImgTaskType] = useState<string>('resize');
  const [aiImgRowId, setAiImgRowId] = useState<string>();
  const [aiImgProvider, setAiImgProvider] = useState<string>('');
  const [aiImgBusy, setAiImgBusy] = useState(false);
  const [aiImgPrompt, setAiImgPrompt] = useState<string>('');
  const [aiImgNegPrompt, setAiImgNegPrompt] = useState<string>('');
  const [aiImgBackground, setAiImgBackground] = useState<string>('white studio background');
  const [aiImgStyle, setAiImgStyle] = useState<string>('clean ecommerce');

  const aiImgAllowNoSourceImage = useMemo(() => {
    const prov = aiImgProvider.trim().toLowerCase();
    return (
      aiImgTaskType === 'generate_scene' &&
      (prov === '' || prov === 'openai_image' || prov === 'comfyui')
    );
  }, [aiImgTaskType, aiImgProvider]);

  const reloadDetail = useCallback(async () => {
    if (!id) return;
    const d = await fetchProductDetail(id);
    setData(d);
    setSkuRows(
      (d.skus ?? []).map((s) => ({
        ...s,
        attrsText: attrsToText(s.attrs),
      })),
    );
  }, [id]);

  const reloadTasks = useCallback(async () => {
    if (!id) return;
    try {
      const { list } = await fetchProductAITasks(id);
      setAiTasks(list ?? []);
    } catch {
      setAiTasks([]);
    }
  }, [id]);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setErr(undefined);
      try {
        const d = await fetchProductDetail(id);
        if (!cancelled) {
          setData(d);
          setSkuRows(
            (d.skus ?? []).map((s) => ({
              ...s,
              attrsText: attrsToText(s.attrs),
            })),
          );
        }
        if (!cancelled) {
          try {
            const { list } = await fetchProductAITasks(id);
            if (!cancelled) setAiTasks(list ?? []);
          } catch {
            if (!cancelled) setAiTasks([]);
          }
        }
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  const sortedImages = useMemo(() => {
    const list = [...(data?.images ?? [])];
    list.sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0));
    return list;
  }, [data?.images]);

  const imageColumns: ColumnsType<ProductImageRow> = useMemo(
    () => [
      {
        title: '预览',
        width: 88,
        render: (_, r) => (
          <Image src={r.publicUrl || r.originUrl} width={56} height={56} style={{ objectFit: 'cover', borderRadius: 4 }} />
        ),
      },
      { title: '类型', dataIndex: 'imageType', width: 100, render: (v: string) => imageTypeLabel(v) },
      {
        title: 'sortOrder',
        dataIndex: 'sortOrder',
        width: 92,
      },
      {
        title: 'URL',
        ellipsis: true,
        render: (_, r) => (
          <Typography.Link href={r.publicUrl || r.originUrl} target="_blank" rel="noreferrer">
            {(r.publicUrl || r.originUrl || '').slice(0, 64)}
            {(r.publicUrl || r.originUrl || '').length > 64 ? '…' : ''}
          </Typography.Link>
        ),
      },
      {
        title: '操作',
        width: 200,
        render: (_, r) => (
          <Space wrap>
            <Button type="link" size="small" onClick={() => setImgEdit(r)}>
              编辑
            </Button>
            <Popconfirm
              title="删除该关联？"
              description="仅从商品移除关联"
              onConfirm={async () => {
                try {
                  await deleteProductImage(id, r.id);
                  message.success('已删除');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '删除失败');
                }
              }}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [id, reloadDetail],
  );

  const skuColumns = useMemo(
    () => [
      { title: '编码', dataIndex: 'skuCode', width: 140, ellipsis: true, formItemProps: { rules: [] } },
      { title: '名称', dataIndex: 'skuName', width: 180, ellipsis: true, formItemProps: { rules: [{ required: true }] } },
      {
        title: '价格',
        dataIndex: 'price',
        width: 100,
        valueType: 'digit' as const,
        fieldProps: { min: 0, precision: 2 },
      },
      {
        title: '库存',
        dataIndex: 'stock',
        width: 92,
        valueType: 'digit' as const,
        fieldProps: { min: 0 },
      },
      {
        title: '图片 URL',
        dataIndex: 'imageUrl',
        width: 160,
        ellipsis: true,
      },
      {
        title: 'attrs (JSON)',
        dataIndex: 'attrsText',
        valueType: 'textarea' as const,
        ellipsis: true,
        fieldProps: { rows: 2 },
      },
      {
        title: '操作',
        valueType: 'option' as const,
        width: 140,
        render: (_: unknown, record: SKUEditable) => (
          <Popconfirm
            title="删除该 SKU？"
            onConfirm={async () => {
              if (!record?.id?.startsWith('new_')) {
                try {
                  await deleteProductSku(id, record.id);
                  message.success('已删除');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '删除失败');
                }
              } else {
                setSkuRows((rows) => rows.filter((r) => r.id !== record.id));
              }
            }}
          >
            <Button type="link" danger size="small">
              删除
            </Button>
          </Popconfirm>
        ),
      },
    ],
    [id, reloadDetail],
  );

  if (!id) {
    return (
      <PageContainer title="商品详情">
        <Typography.Text type="danger">无效的商品 ID</Typography.Text>
      </PageContainer>
    );
  }

  return (
    <PageContainer
      title={data?.title || '商品详情'}
      loading={loading}
      extra={
        data ? (
          <Space wrap>
            <Button
              onClick={async () => {
                try {
                  await updateProduct(id, { status: 'ready' });
                  message.success('已设为「可用」');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              标记为可用
            </Button>
            <Button
              onClick={async () => {
                try {
                  await updateProduct(id, { status: 'archived' });
                  message.success('已归档');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              归档
            </Button>
            <Popconfirm
              title="确定删除草稿？"
              description="软删除，列表不可见"
              onConfirm={async () => {
                try {
                  await deleteProduct(id);
                  message.success('已删除');
                  history.push('/product/drafts');
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '删除失败');
                }
              }}
            >
              <Button danger icon={<DeleteOutlined />}>
                删除草稿
              </Button>
            </Popconfirm>
          </Space>
        ) : null
      }
    >
      {loading ? (
        <Spin />
      ) : err ? (
        <Typography.Text type="danger">{err}</Typography.Text>
      ) : data ? (
        <Tabs
          defaultActiveKey="basic"
          items={[
            {
              key: 'basic',
              label: '基础信息',
              children: (
                <Card bordered={false}>
                  <Descriptions column={2} bordered size="small" style={{ marginBottom: 16 }}>
                    <Descriptions.Item label="来源">{data.source}</Descriptions.Item>
                    <Descriptions.Item label="币种（展示）">{data.currency}</Descriptions.Item>
                    <Descriptions.Item label="来源链接" span={2}>
                      <Typography.Link href={data.sourceUrl || undefined} target="_blank" rel="noreferrer">
                        {data.sourceUrl || '—'}
                      </Typography.Link>
                    </Descriptions.Item>
                  </Descriptions>

                  <ProTable
                    key={`basic-${data.id}-${data.updatedAt}`}
                    type="form"
                    submitter={{
                      searchConfig: { submitText: '保存基础信息' },
                      submitButtonProps: { type: 'primary' },
                      resetButtonProps: false,
                    }}
                    onFinish={async (vals) => {
                      try {
                        await updateProduct(id, {
                          title: String(vals.title ?? ''),
                          originalTitle: String(vals.originalTitle ?? ''),
                          aiTitle: String(vals.aiTitle ?? ''),
                          description: String(vals.description ?? ''),
                          aiDescription: String(vals.aiDescription ?? ''),
                          currency: String(vals.currency ?? ''),
                          status: String(vals.status ?? ''),
                        });
                        message.success('已保存');
                        await reloadDetail();
                        return true;
                      } catch (e: unknown) {
                        message.error((e as Error)?.message || '保存失败');
                        return false;
                      }
                    }}
                    columns={[
                      {
                        title: '主标题',
                        dataIndex: 'title',
                        formItemProps: {
                          rules: [{ required: true, message: '必填' }],
                        },
                      },
                      { title: '原始标题', dataIndex: 'originalTitle', valueType: 'textarea', fieldProps: { rows: 2 } },
                      { title: 'AI 标题', dataIndex: 'aiTitle', valueType: 'textarea', fieldProps: { rows: 2 } },
                      {
                        title: '主描述',
                        dataIndex: 'description',
                        valueType: 'textarea',
                        fieldProps: { rows: 5 },
                      },
                      {
                        title: 'AI 描述',
                        dataIndex: 'aiDescription',
                        valueType: 'textarea',
                        fieldProps: { rows: 5 },
                      },
                      { title: '币种', dataIndex: 'currency', width: 'md', initialValue: 'CNY' },
                      {
                        title: '状态',
                        dataIndex: 'status',
                        valueType: 'select',
                        fieldProps: { options: PRODUCT_STATUS_OPTIONS },
                      },
                    ]}
                    form={{
                      layout: 'vertical',
                      grid: true,
                      colProps: { span: 12 },
                      initialValues: {
                        title: data.title,
                        originalTitle: data.originalTitle,
                        aiTitle: data.aiTitle ?? '',
                        description: data.description ?? '',
                        aiDescription: data.aiDescription ?? '',
                        currency: data.currency || 'CNY',
                        status: data.status,
                      },
                      submitterColSpanProps: { span: 24 },
                    }}
                  />
                </Card>
              ),
            },
            {
              key: 'ai',
              label: 'AI',
              children: (
                <Space direction="vertical" style={{ width: '100%' }} size="middle">
                  <Card bordered={false} bodyStyle={{ paddingBottom: 12 }}>
                    <Space wrap size="middle">
                      <Button
                        type="primary"
                        onClick={() => {
                          setAiResult(null);
                          aiForm.resetFields();
                          aiForm.setFieldsValue({ language: 'en', platform: 'TikTok Shop', maxLength: 120 });
                          setAiOpen(true);
                        }}
                      >
                        标题优化
                      </Button>
                      <Button
                        type="primary"
                        onClick={() => {
                          setDescResult(null);
                          descForm.resetFields();
                          descForm.setFieldsValue({
                            language: 'en',
                            platform: 'TikTok Shop',
                            tone: 'professional',
                          });
                          setDescOpen(true);
                        }}
                      >
                        描述生成
                      </Button>
                    </Space>
                  </Card>

                  <Card title="最近任务">
                    <ProTable<AITaskRow>
                      rowKey="id"
                      search={false}
                      options={false}
                      pagination={false}
                      dataSource={aiTasks}
                      columns={[
                        { title: '类型', dataIndex: 'taskType', width: 200 },
                        { title: '状态', dataIndex: 'status', width: 100 },
                        { title: '模型', dataIndex: 'model', ellipsis: true },
                        {
                          title: 'Tokens',
                          width: 100,
                          render: (_: unknown, row: AITaskRow) => `${row.tokenInput ?? 0}/${row.tokenOutput ?? 0}`,
                        },
                        { title: 'Prompt', dataIndex: 'promptCode', width: 160, ellipsis: true },
                        {
                          title: '时间',
                          dataIndex: 'createdAt',
                          width: 176,
                          render: (v: string) => v?.replace('T', ' ').slice(0, 19) ?? '—',
                        },
                      ]}
                      size="small"
                    />
                  </Card>

                  {data.rawData != null ? (
                    <Card title="Raw JSON" bordered={false}>
                      <pre style={{ maxHeight: 360, overflow: 'auto', fontSize: 12 }}>{JSON.stringify(data.rawData, null, 2)}</pre>
                    </Card>
                  ) : null}
                </Space>
              ),
            },
            {
              key: 'images',
              label: '图片管理',
              children: (
                <Card bordered={false}>
                  <Card title="AI 图片任务" size="small" style={{ marginBottom: 16 }} bordered={false}>
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Typography.Text type="secondary">
                        可选择商品图作为源图（场景图在 OpenAI / ComfyUI 下可无图），任务由后端入队；结果在{' '}
                        <Typography.Link onClick={() => history.push('/ai/image-tasks')}>AI 图片任务</Typography.Link>{' '}
                        查看。<Typography.Text code>remove_background</Typography.Text> 使用 <Typography.Text code>removebg</Typography.Text>；本地/存储中的商品图由后端读取并以 multipart 上传 remove.bg（公网 URL 仍可走 image_url）。<Typography.Text code>generate_scene</Typography.Text> 支持 <Typography.Text code>openai_image</Typography.Text> / <Typography.Text code>comfyui</Typography.Text>；<Typography.Text code>replace_background</Typography.Text> 请配合 <Typography.Text code>comfyui</Typography.Text> 工作流。
                      </Typography.Text>
                      <Input.TextArea
                        value={aiImgPrompt}
                        onChange={(e) => setAiImgPrompt(e.target.value)}
                        rows={3}
                        placeholder="补充说明 / Prompt（可选）"
                        style={{ maxWidth: 560 }}
                      />
                      <Input.TextArea
                        value={aiImgNegPrompt}
                        onChange={(e) => setAiImgNegPrompt(e.target.value)}
                        rows={2}
                        placeholder="Negative prompt（可选）"
                        style={{ maxWidth: 560 }}
                      />
                      <Space wrap align="start">
                        <Input
                          placeholder="背景 / 目标背景"
                          style={{ width: 260 }}
                          value={aiImgBackground}
                          onChange={(e) => setAiImgBackground(e.target.value)}
                        />
                        <Input
                          placeholder="风格 style"
                          style={{ width: 200 }}
                          value={aiImgStyle}
                          onChange={(e) => setAiImgStyle(e.target.value)}
                        />
                      </Space>
                      <Space wrap align="start">
                        <Select
                          placeholder="选择商品图片（可选，换背景/去背景/缩放建议选）"
                          allowClear
                          style={{ minWidth: 280 }}
                          value={aiImgRowId}
                          onChange={(v) => setAiImgRowId(v)}
                          options={sortedImages.map((im) => ({
                            label: `${imageTypeLabel(im.imageType)} · ${(im.publicUrl || im.originUrl || '').slice(0, 48)}${(im.publicUrl || im.originUrl || '').length > 48 ? '…' : ''}`,
                            value: im.id,
                          }))}
                        />
                        <Select
                          style={{ minWidth: 200 }}
                          value={aiImgTaskType}
                          onChange={(v) => {
                            setAiImgTaskType(v);
                            if (v === 'remove_background') {
                              setAiImgProvider('removebg');
                            }
                            if (v === 'generate_scene') {
                              setAiImgProvider('openai_image');
                            }
                            if (v === 'replace_background') {
                              setAiImgProvider('comfyui');
                            }
                          }}
                          options={[
                            { label: '去背景 remove_background', value: 'remove_background' },
                            { label: '换背景 replace_background', value: 'replace_background' },
                            { label: '场景图 generate_scene', value: 'generate_scene' },
                            { label: '缩放 resize', value: 'resize' },
                          ]}
                        />
                        <Select
                          placeholder="Provider"
                          style={{ minWidth: 220 }}
                          value={aiImgProvider}
                          onChange={(v) => setAiImgProvider(v)}
                          options={[
                            { label: '默认（跟随「图片 AI」设置）', value: '' },
                            { label: 'noop', value: 'noop' },
                            { label: 'remove.bg', value: 'removebg' },
                            { label: 'OpenAI Image', value: 'openai_image' },
                            { label: 'ComfyUI', value: 'comfyui' },
                          ]}
                        />
                        <Button
                          type="primary"
                          loading={aiImgBusy}
                          disabled={!aiImgAllowNoSourceImage && !aiImgRowId}
                          onClick={async () => {
                            if (!aiImgAllowNoSourceImage && !aiImgRowId) return;
                            setAiImgBusy(true);
                            try {
                              let input: Record<string, unknown> = {};
                              if (aiImgTaskType === 'resize') {
                                input = { width: 800, height: 800 };
                              } else if (aiImgTaskType === 'generate_scene') {
                                input = {
                                  prompt: aiImgPrompt.trim(),
                                  negativePrompt: aiImgNegPrompt.trim(),
                                  scene: 'minimal studio',
                                  style: aiImgStyle.trim() || 'clean ecommerce',
                                  size: '1024x1024',
                                  background: aiImgBackground.trim() || 'white studio background',
                                  platform: 'TikTok Shop',
                                };
                              } else if (aiImgTaskType === 'replace_background') {
                                input = {
                                  prompt: aiImgPrompt.trim(),
                                  negativePrompt: aiImgNegPrompt.trim(),
                                  background: aiImgBackground.trim() || 'white studio background',
                                  style: aiImgStyle.trim() || 'clean ecommerce',
                                };
                              }
                              const task = await createImageTask({
                                taskType: aiImgTaskType,
                                ...(aiImgProvider.trim() ? { provider: aiImgProvider.trim() } : {}),
                                productId: id,
                                ...(aiImgRowId ? { sourceImageId: aiImgRowId } : {}),
                                input,
                              });
                              if (task.status === 'pending' || task.status === 'running') {
                                message.success('图片任务已提交，可在 AI 图片任务页查看结果');
                              } else if (task.status === 'success' && task.resultUrl) {
                                message.success(`已完成：${task.resultUrl}`);
                              } else {
                                message.success('图片任务已创建');
                              }
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '创建失败');
                            } finally {
                              setAiImgBusy(false);
                            }
                          }}
                        >
                          创建图片任务
                        </Button>
                        <Button onClick={() => history.push('/ai/image-tasks')}>查看 AI 图片任务</Button>
                      </Space>
                    </Space>
                  </Card>
                  <Space style={{ marginBottom: 12 }} wrap>
                    <Button
                      type="primary"
                      icon={<PlusOutlined />}
                      onClick={() => {
                        setLastUpload(null);
                        setImgEdit(null);
                        setImgModalOpen(true);
                      }}
                    >
                      添加图片
                    </Button>
                    <Tooltip title="按当前顺序提交全部图片 ID">
                      <Button
                        icon={<ArrowUpOutlined />}
                        onClick={async () => {
                          try {
                            const ordered = [...sortedImages].sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0));
                            await reorderProductImages(id, { imageIds: ordered.map((i) => i.id) });
                            message.success('已同步');
                            await reloadDetail();
                          } catch (e: unknown) {
                            message.error((e as Error)?.message || '排序失败');
                          }
                        }}
                      >
                        同步顺序
                      </Button>
                    </Tooltip>
                  </Space>
                  <ProTable<ProductImageRow>
                    rowKey="id"
                    search={false}
                    options={false}
                    pagination={false}
                    dataSource={sortedImages}
                    columns={imageColumns}
                    size="small"
                  />
                </Card>
              ),
            },
            {
              key: 'skus',
              label: 'SKU 管理',
              children: (
                <Card bordered={false}>
                  <EditableProTable<SKUEditable>
                    rowKey="id"
                    headerTitle={false}
                    search={false}
                    options={false}
                    pagination={false}
                    value={skuRows}
                    onChange={setSkuRows}
                    recordCreatorProps={{
                      record: (): SKUEditable => ({
                        id: `new_${Date.now()}`,
                        productId: id,
                        skuCode: '',
                        skuName: '新 SKU',
                        attrsText: '{}',
                      }),
                      style: {
                        marginBottom: 16,
                      },
                      creatorButtonText: '新增 SKU',
                    }}
                    editable={{
                      type: 'multiple',
                      onSave: async (_key, row) => {
                        const attrsStr = row.attrsText?.trim() ?? '';
                        let attrs: string | Record<string, unknown> | undefined = attrsStr;
                        if (!attrsStr) attrs = '{}';
                        if (String(row.id).startsWith('new_')) {
                          await createProductSku(id, {
                            skuCode: row.skuCode ?? '',
                            skuName: row.skuName,
                            attrs,
                            price: row.price,
                            stock: row.stock,
                            imageUrl: row.imageUrl,
                          });
                          message.success('SKU 已创建');
                        } else {
                          await updateProductSku(id, row.id, {
                            skuCode: row.skuCode,
                            skuName: row.skuName,
                            attrs,
                            price: row.price,
                            stock: row.stock,
                            imageUrl: row.imageUrl,
                          });
                          message.success('SKU 已更新');
                        }
                        await reloadDetail();
                      },
                    }}
                    columns={skuColumns}
                    scroll={{ x: 1100 }}
                  />
                </Card>
              ),
            },
          ]}
        />
      ) : null}

      <ModalForm
        title={imgEdit ? '编辑商品图片' : '添加商品图片'}
        open={!!id && (imgModalOpen || !!imgEdit)}
        onOpenChange={(open) => {
          if (!open) {
            setImgModalOpen(false);
            setImgEdit(null);
            setLastUpload(null);
          }
        }}
        key={imgEdit ? `img-${imgEdit.id}` : imgModalOpen ? 'img-add' : 'img-closed'}
        modalProps={{ destroyOnClose: true, width: 560 }}
        initialValues={{
          imageType: imgEdit ? (imgEdit.imageType === 'description' ? 'detail' : imgEdit.imageType) : 'main',
          sortOrder: imgEdit?.sortOrder ?? sortedImages.length,
          publicUrl: imgEdit?.publicUrl ?? '',
          originUrl: imgEdit?.originUrl ?? '',
          objectKey: imgEdit?.objectKey ?? '',
        }}
        onFinish={async (vals) => {
          setImgBusy(true);
          try {
            const imageType = String(vals.imageType ?? 'main');
            const sortOrder = vals.sortOrder != null ? Number(vals.sortOrder) : undefined;
            if (imgEdit) {
              await updateProductImage(id, imgEdit.id, {
                imageType,
                sortOrder,
                publicUrl: String(vals.publicUrl ?? ''),
                originUrl: String(vals.originUrl ?? ''),
                objectKey: String(vals.objectKey ?? ''),
              });
              message.success('已更新');
            } else {
              const body: Parameters<typeof createProductImage>[1] = {
                imageType,
                sortOrder,
                publicUrl: String(vals.publicUrl ?? '').trim(),
                originUrl: String(vals.originUrl ?? '').trim(),
                objectKey: String(vals.objectKey ?? '').trim(),
              };
              if (lastUpload?.id) {
                body.fileId = lastUpload.id;
                if (!body.publicUrl) body.publicUrl = lastUpload.url;
                if (!body.originUrl) body.originUrl = lastUpload.url;
                if (!body.objectKey) body.objectKey = lastUpload.objectKey;
              }
              await createProductImage(id, body);
              message.success('已添加');
            }
            setImgModalOpen(false);
            setImgEdit(null);
            setLastUpload(null);
            await reloadDetail();
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
            return false;
          } finally {
            setImgBusy(false);
          }
        }}
        submitter={{
          searchConfig: { submitText: imgEdit ? '保存' : '添加' },
          submitButtonProps: { loading: imgBusy },
        }}
      >
        <ProFormSelect name="imageType" label="图片类型" options={IMAGE_TYPE_OPTIONS} rules={[{ required: true }]} />
        <ProFormDigit name="sortOrder" label="sortOrder" min={0} fieldProps={{ style: { width: '100%' } }} />
        {!imgEdit ? (
          <Form.Item label="上传文件（可选）">
            <Upload
              maxCount={1}
              showUploadList
              customRequest={async (opt: UploadRequestOption) => {
                try {
                  const f = opt.file as File;
                  const up = await uploadFile(f);
                  setLastUpload({ id: up.id, url: up.url, objectKey: up.objectKey });
                  opt.onSuccess?.(up, new XMLHttpRequest());
                  message.success('已上传，保存时将关联到商品');
                } catch (e: unknown) {
                  opt.onError?.(e as Error);
                  message.error((e as Error)?.message || '上传失败');
                }
              }}
            >
              <Button icon={<PlusOutlined />}>选择图片并上传</Button>
            </Upload>
          </Form.Item>
        ) : null}
        <ProFormText name="publicUrl" label="publicUrl" placeholder="https:// 或 /static/…" />
        <ProFormText name="originUrl" label="originUrl" placeholder="外部原图地址（可选）" />
        <ProFormText name="objectKey" label="objectKey" placeholder="存储键（可选）" />
      </ModalForm>

      <Modal
        title="AI 标题优化"
        open={aiOpen}
        onCancel={() => setAiOpen(false)}
        footer={null}
        destroyOnClose
        width={640}
      >
        <Form
          form={aiForm}
          layout="vertical"
          initialValues={{ language: 'en', platform: 'TikTok Shop', maxLength: 120 }}
          onFinish={async (v) => {
            setAiBusy(true);
            setAiResult(null);
            try {
              const res = await optimizeProductTitle(id, {
                language: String(v.language ?? ''),
                platform: String(v.platform ?? ''),
                maxLength: Number(v.maxLength ?? 120),
              });
              setAiResult(res);
              message.success('优化完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '优化失败');
            } finally {
              setAiBusy(false);
            }
          }}
        >
          <Form.Item name="language" label="语言" rules={[{ required: true }]}>
            <Input placeholder="en" />
          </Form.Item>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Input placeholder="TikTok Shop" />
          </Form.Item>
          <Form.Item name="maxLength" label="最长字符数" rules={[{ required: true }]}>
            <InputNumber min={20} max={500} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={aiBusy}>
              运行优化
            </Button>
          </Form.Item>
        </Form>

        {aiResult ? (
          <div style={{ marginTop: 16 }}>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              输出
            </Typography.Title>
            <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="优化标题">{aiResult.optimizedTitle || '—'}</Descriptions.Item>
              <Descriptions.Item label="关键词">
                {(aiResult.keywords ?? []).length ? aiResult.keywords.join('、') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="说明">{aiResult.reason || '—'}</Descriptions.Item>
              <Descriptions.Item label="任务 ID">{aiResult.taskId}</Descriptions.Item>
            </Descriptions>
            <Button
              type="primary"
              disabled={!aiResult.optimizedTitle}
              loading={aiBusy}
              onClick={async () => {
                if (!aiResult?.taskId) return;
                setAiBusy(true);
                try {
                  await applyProductAITitle(id, {
                    aiTitle: aiResult.optimizedTitle,
                    taskId: aiResult.taskId,
                  });
                  message.success('已应用为 AI 标题');
                  setAiOpen(false);
                  setAiResult(null);
                  await reloadDetail();
                  await reloadTasks();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '应用失败');
                } finally {
                  setAiBusy(false);
                }
              }}
            >
              应用为 AI 标题
            </Button>
          </div>
        ) : null}
      </Modal>

      <Modal
        title="AI 描述生成"
        open={descOpen}
        onCancel={() => setDescOpen(false)}
        footer={null}
        destroyOnClose
        width={720}
      >
        <Form
          form={descForm}
          layout="vertical"
          initialValues={{ language: 'en', platform: 'TikTok Shop', tone: 'professional' }}
          onFinish={async (v) => {
            setDescBusy(true);
            setDescResult(null);
            try {
              const res = await generateDescription(id, {
                language: String(v.language ?? ''),
                platform: String(v.platform ?? ''),
                tone: String(v.tone ?? ''),
              });
              setDescResult(res);
              message.success('生成完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '生成失败');
            } finally {
              setDescBusy(false);
            }
          }}
        >
          <Form.Item name="language" label="语言" rules={[{ required: true }]}>
            <Input placeholder="en" />
          </Form.Item>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Input placeholder="TikTok Shop" />
          </Form.Item>
          <Form.Item name="tone" label="语气" rules={[{ required: true }]}>
            <Input placeholder="professional" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={descBusy}>
              生成描述
            </Button>
          </Form.Item>
        </Form>

        {descResult ? (
          <div style={{ marginTop: 16 }}>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              输出
            </Typography.Title>
            <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="描述">{descResult.description || '—'}</Descriptions.Item>
              <Descriptions.Item label="Highlights">
                {(descResult.highlights ?? []).length ? descResult.highlights.join('；') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Specifications">
                {(descResult.specifications ?? []).length ? descResult.specifications.join('；') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Package includes">
                {(descResult.packageIncludes ?? []).length ? descResult.packageIncludes.join('；') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Notes">{descResult.notes || '—'}</Descriptions.Item>
              <Descriptions.Item label="Reason">{descResult.reason || '—'}</Descriptions.Item>
              <Descriptions.Item label="任务 ID">{descResult.taskId}</Descriptions.Item>
            </Descriptions>
            <Button
              type="primary"
              disabled={!descResult.taskId || !buildAiDescriptionText(descResult)}
              loading={descBusy}
              onClick={async () => {
                if (!descResult?.taskId) return;
                const text = buildAiDescriptionText(descResult);
                if (!text) return;
                setDescBusy(true);
                try {
                  await applyAiDescription(id, {
                    aiDescription: text,
                    taskId: descResult.taskId,
                  });
                  message.success('已应用为 AI 描述');
                  setDescOpen(false);
                  setDescResult(null);
                  await reloadDetail();
                  await reloadTasks();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '应用失败');
                } finally {
                  setDescBusy(false);
                }
              }}
            >
              应用为 AI 描述
            </Button>
          </div>
        ) : null}
      </Modal>
    </PageContainer>
  );
}

function buildAiDescriptionText(r: GenerateDescriptionResult): string {
  const lines: string[] = [];
  const d = (r.description ?? '').trim();
  if (d) lines.push(d);
  const bullets = (title: string, items: string[]) => {
    const trimmed = (items ?? []).map((x) => x.trim()).filter(Boolean);
    if (!trimmed.length) return;
    lines.push('', title);
    for (const x of trimmed) lines.push(`- ${x}`);
  };
  bullets('Product Highlights', r.highlights ?? []);
  bullets('Specifications', r.specifications ?? []);
  bullets('Package Includes', r.packageIncludes ?? []);
  const notes = (r.notes ?? '').trim();
  if (notes) {
    lines.push('', 'Notes', notes);
  }
  return lines.join('\n').trim();
}
