import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Drawer, Form, Image, Select, Space, Table, Tag, Typography, message, Checkbox, Alert, Radio, Input, InputNumber } from 'antd';
import { useRef, useState, useMemo, useEffect } from 'react';
import { history, useLocation } from '@umijs/max';
import { PRODUCT_STATUS } from '@/constants/status';
import { createProductImagesBatch, createProductTextBatch } from '@/services/aiBatches';
import { createProduct, fetchProducts, type ProductListRow } from '@/services/products';
import { batchCheckProductReadiness, type ProductReadinessResult } from '@/services/productReadiness';
import { queryShops, type ShopListRow } from '@/services/shops';

export default function ProductDraftsPage() {
  const location = useLocation();
  const urlFilters = useMemo(() => {
    const sp = new URLSearchParams(location.search);
    return {
      missingAiTitle: sp.get('missingAiTitle') === '1',
      missingAiDescription: sp.get('missingAiDescription') === '1',
      readinessBlocked: sp.get('readiness') === 'blocked',
      publishable: sp.get('publishable') === '1',
    };
  }, [location.search]);

  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [batchOpen, setBatchOpen] = useState(false);
  const [batchLoading, setBatchLoading] = useState(false);
  const [batchPlat, setBatchPlat] = useState<string>('tiktok');
  const [batchShopId, setBatchShopId] = useState<string>('');
  const [batchResult, setBatchResult] = useState<ProductReadinessResult[]>([]);
  const [shopsList, setShopsList] = useState<ShopListRow[]>([]);
  const [listFilters, setListFilters] = useState<{ keyword?: string; status?: string; source?: string }>({});
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkLoading, setBulkLoading] = useState(false);
  const [bulkForm] = Form.useForm();
  const [bulkOp, setBulkOp] = useState<string>('title_optimize');
  const [bulkConfirmFiltered, setBulkConfirmFiltered] = useState(false);

  useEffect(() => {
    actionRef.current?.reload();
  }, [location.search]);

  const columns: ProColumns<ProductListRow>[] = [
    {
      title: '商品图',
      dataIndex: 'coverUrl',
      width: 88,
      search: false,
      render: (_, row) =>
        row.coverUrl ? (
          <Image src={row.coverUrl} width={56} height={56} style={{ objectFit: 'cover', borderRadius: 4 }} />
        ) : (
          <Typography.Text type="secondary">—</Typography.Text>
        ),
    },
    {
      title: '标题',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '搜索标题' },
      search: {
        transform: (v) => ({ keyword: v }),
      },
    },
    {
      title: '标题',
      dataIndex: 'title',
      ellipsis: true,
      search: false,
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 96,
      valueType: 'select',
      valueEnum: {
        manual: { text: 'manual' },
        '1688': { text: '1688' },
      },
      render: (_, row) => <Typography.Text code>{row.source}</Typography.Text>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(PRODUCT_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => {
        const m = PRODUCT_STATUS[row.status as keyof typeof PRODUCT_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 172,
      search: false,
      valueType: 'dateTime',
    },
    {
      title: '操作',
      valueType: 'option',
      width: 88,
      render: (_, row) => [
        <Typography.Link key="detail" href={`/product/drafts/${row.id}`}>
          详情
        </Typography.Link>,
      ],
    },
  ];

  const eligibleBatchPlatforms = ['tiktok', 'shopee', 'lazada', 'amazon', 'mock'];

  const shopsForBatchPlat = shopsList.filter(
    (s) =>
      (s.platform || '').toLowerCase() === batchPlat.toLowerCase() && s.authStatus === 'authorized',
  );

  const openBatchDrawer = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先勾选商品');
      return;
    }
    if (selectedRowKeys.length > 100) {
      message.error('单次最多检查 100 个商品');
      return;
    }
    setBatchOpen(true);
    setBatchResult([]);
    try {
      const shops = await queryShops({ page: 1, pageSize: 500, authStatus: 'authorized' });
      setShopsList(Array.isArray(shops.list) ? shops.list : []);
    } catch {
      setShopsList([]);
    }
  };

  const runBatchReadiness = async () => {
    if (!batchShopId) {
      message.error('请选择店铺');
      return;
    }
    setBatchLoading(true);
    try {
      const { list } = await batchCheckProductReadiness({
        productIds: selectedRowKeys,
        platform: batchPlat,
        shopId: batchShopId,
      });
      setBatchResult(Array.isArray(list) ? list : []);
      message.success('检查完成');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setBatchLoading(false);
    }
  };

  const submitBulkAI = async () => {
    try {
      const vals = await bulkForm.validateFields();
      const productIds = [...selectedRowKeys];
      if (productIds.length === 0 && !bulkConfirmFiltered) {
        message.error('未勾选商品时，请勾选「按当前筛选」确认项');
        return;
      }
      const narrow = !!(listFilters.keyword || listFilters.status || listFilters.source);
      if (productIds.length === 0 && !narrow) {
        message.error('当前无任何列表筛选；若需全表批量，请先在列表中设置状态/来源/关键字筛选，或勾选商品。');
        return;
      }
      setBulkLoading(true);
      const filtBase =
        productIds.length === 0
          ? {
              keyword: listFilters.keyword ?? '',
              status: listFilters.status ?? '',
              source: listFilters.source ?? '',
            }
          : {};
      if (
        bulkOp === 'title_optimize' ||
        bulkOp === 'description_generate'
      ) {
        await createProductTextBatch({
          operationType: bulkOp,
          productIds,
          filters: filtBase,
          options: {
            language: vals.language,
            platform: vals.platform,
            maxLength: vals.maxLength,
            tone: vals.tone,
          },
          applyMode: vals.applyMode,
          confirmAll: productIds.length === 0,
        });
      } else {
        await createProductImagesBatch({
          operationType: bulkOp,
          productIds,
          filters: {
            ...filtBase,
            onlyHasMainImage: true,
          },
          options: {
            provider: vals.provider,
            prompt: vals.prompt,
            backgroundPrompt: vals.backgroundPrompt,
            style: vals.style,
          },
          confirmAll: productIds.length === 0,
        });
      }
      message.success('批次已创建');
      setBulkOpen(false);
      history.push('/ai/batches');
      actionRef.current?.reload();
    } catch (e: unknown) {
      if ((e as { errorFields?: unknown })?.errorFields) return;
      message.error((e as Error)?.message || '创建失败');
    } finally {
      setBulkLoading(false);
    }
  };

  return (
    <PageContainer title="商品草稿">
      {(urlFilters.missingAiTitle ||
        urlFilters.missingAiDescription ||
        urlFilters.readinessBlocked ||
        urlFilters.publishable) && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="已从运营看板带入列表筛选（只影响本页查询，不写库）。"
        />
      )}
      <ProTable<ProductListRow>
        rowKey="id"
        actionRef={actionRef}
        rowSelection={{
          type: 'checkbox',
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys as string[]),
        }}
        tableAlertRender={({ selectedRowKeys: keys }) => (
          <Space>
            <span>已选 {keys.length} 项</span>
          </Space>
        )}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        headerTitle={false}
        toolBarRender={() => [
          <Button
            key="bulkAi"
            onClick={() => {
              bulkForm.resetFields();
              bulkForm.setFieldsValue({
                language: 'en',
                platform: 'TikTok Shop',
                maxLength: 120,
                tone: 'professional',
                applyMode: 'save_ai_field',
                provider: 'removebg',
              });
              setBulkOp('title_optimize');
              setBulkConfirmFiltered(false);
              setBulkOpen(true);
            }}
          >
            批量 AI
          </Button>,
          <Button
            key="readiness"
            disabled={selectedRowKeys.length === 0}
            onClick={() => void openBatchDrawer()}
          >
            批量发布检查
          </Button>,
          <Button key="new" type="primary" onClick={() => setCreateOpen(true)}>
            新建草稿
          </Button>,
        ]}
        request={async (params) => {
          setListFilters({
            keyword: params.keyword as string | undefined,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
          });
          const res = await fetchProducts({
            page: params.current,
            pageSize: params.pageSize,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
            keyword: params.keyword as string | undefined,
            missingAiTitle: urlFilters.missingAiTitle || undefined,
            missingAiDescription: urlFilters.missingAiDescription || undefined,
            readinessBlocked: urlFilters.readinessBlocked || undefined,
            publishable: urlFilters.publishable || undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />

      <ModalForm
        title="新建商品草稿"
        open={createOpen}
        modalProps={{ destroyOnHidden: true, onCancel: () => setCreateOpen(false) }}
        onFinish={async (vals) => {
          await createProduct({
            title: vals.title,
            source: vals.source || 'manual',
            sourceUrl: vals.sourceUrl,
            description: vals.description,
          });
          setCreateOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="title" label="标题" rules={[{ required: true, message: '必填' }]} />
        <ProFormText name="source" label="来源" initialValue="manual" />
        <ProFormText name="sourceUrl" label="来源链接" />
        <ProFormTextArea name="description" label="描述" fieldProps={{ rows: 3 }} />
      </ModalForm>

      <Drawer
        title="批量发布检查"
        width={720}
        open={batchOpen}
        onClose={() => setBatchOpen(false)}
        destroyOnHidden
        extra={
          <Button type="primary" loading={batchLoading} onClick={() => void runBatchReadiness()}>
            开始检查
          </Button>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <Form layout="vertical">
            <Form.Item label="平台">
              <Select
                value={batchPlat}
                onChange={(v) => {
                  setBatchPlat(String(v));
                  setBatchShopId('');
                }}
                options={eligibleBatchPlatforms.map((p) => ({ label: p, value: p }))}
              />
            </Form.Item>
            <Form.Item label="店铺">
              <Select
                placeholder="选择已授权店铺"
                value={batchShopId || undefined}
                onChange={(v) => setBatchShopId(v ? String(v) : '')}
                options={shopsForBatchPlat.map((s) => ({
                  label: `${s.shopName} (${s.platform})`,
                  value: s.id,
                }))}
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>
          </Form>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            已选 {selectedRowKeys.length} 个商品；单次最多 100 个。检查不修改商品数据，不调用平台 API。
          </Typography.Paragraph>
          <Table<ProductReadinessResult>
            size="small"
            rowKey="productId"
            dataSource={batchResult}
            pagination={false}
            columns={[
              {
                title: '商品 ID',
                dataIndex: 'productId',
                ellipsis: true,
                render: (v: string) => (
                  <Typography.Link href={`/product/drafts/${v}?tab=readiness`}>{v}</Typography.Link>
                ),
              },
              {
                title: '状态',
                width: 100,
                render: (_, r) => {
                  if (!r.canPublish) return <Tag color="red">阻止</Tag>;
                  if (r.warningCount > 0) return <Tag color="orange">警告</Tag>;
                  return <Tag color="green">就绪</Tag>;
                },
              },
              { title: '分', dataIndex: 'score', width: 64 },
              { title: '错', dataIndex: 'errorCount', width: 56 },
              { title: '警', dataIndex: 'warningCount', width: 56 },
            ]}
          />
        </Space>
      </Drawer>

      <Drawer
        title="批量 AI（商品草稿）"
        width={560}
        open={bulkOpen}
        onClose={() => setBulkOpen(false)}
        destroyOnHidden
        extra={
          <Button
            type="primary"
            loading={bulkLoading}
            onClick={() => void submitBulkAI()}
          >
            创建批次
          </Button>
        }
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="不会自动覆盖正式标题/详情，不会替换主图，不会刊登。文本结果见 AI 任务与草稿「AI 标题/描述」；图片结果见「图片任务」。"
        />
        <Typography.Paragraph type="secondary">
          已勾选 <strong>{selectedRowKeys.length}</strong> 个商品。
          {selectedRowKeys.length === 0 ? (
            <>未勾选时将使用下方「与列表相同的筛选条件」；必须勾选确认项。</>
          ) : null}
        </Typography.Paragraph>
        <Form form={bulkForm} layout="vertical">
          <Form.Item label="操作类型" required>
            <Radio.Group
              value={bulkOp}
              onChange={(e) => setBulkOp(e.target.value)}
              options={[
                { label: 'AI 标题优化', value: 'title_optimize' },
                { label: 'AI 描述生成', value: 'description_generate' },
                { label: '去背景（主图）', value: 'image_remove_background' },
                { label: '生成场景图（主图）', value: 'image_generate_scene' },
              ]}
            />
          </Form.Item>
          {(bulkOp === 'title_optimize' || bulkOp === 'description_generate') && (
            <>
              <Form.Item name="language" label="语言" rules={[{ required: true }]}>
                <Input placeholder="en" />
              </Form.Item>
              <Form.Item name="platform" label="平台口径" rules={[{ required: true }]}>
                <Input placeholder="TikTok Shop" />
              </Form.Item>
              {bulkOp === 'title_optimize' && (
                <Form.Item name="maxLength" label="最大长度">
                  <InputNumber min={20} max={300} style={{ width: '100%' }} />
                </Form.Item>
              )}
              <Form.Item name="tone" label="语气 / 风格">
                <Input placeholder="professional" />
              </Form.Item>
              <Form.Item name="applyMode" label="应用策略" tooltip="save_ai_field：成功后写入草稿的 ai_title / ai_description">
                <Select
                  options={[
                    { label: '仅任务记录（不写 AI 草稿字段）', value: 'task_only' },
                    { label: '写入 AI 草稿字段（ai_title / ai_description）', value: 'save_ai_field' },
                  ]}
                />
              </Form.Item>
            </>
          )}
          {(bulkOp === 'image_remove_background' || bulkOp === 'image_generate_scene') && (
            <>
              <Form.Item
                name="provider"
                label="图片 Provider"
                tooltip={bulkOp === 'image_remove_background' ? '后端会强制 removebg' : '如 openai_image / comfyui'}
              >
                <Input placeholder={bulkOp === 'image_remove_background' ? 'removebg' : 'openai_image'} />
              </Form.Item>
              <Form.Item name="prompt" label="Prompt（摘要入任务，可选）">
                <Input.TextArea rows={3} placeholder="场景/风格提示；勿在公开场合粘贴完整商业秘密" />
              </Form.Item>
            </>
          )}
          {selectedRowKeys.length === 0 && (
            <Form.Item>
              <Checkbox checked={bulkConfirmFiltered} onChange={(e) => setBulkConfirmFiltered(e.target.checked)}>
                我确认：对当前列表<strong>完全相同</strong>的筛选条件下的商品执行批量 AI（关键字 / 状态 / 来源）。
              </Checkbox>
            </Form.Item>
          )}
        </Form>
      </Drawer>
    </PageContainer>
  );
}
