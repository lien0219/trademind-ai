import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Drawer, Form, Image, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useRef, useState } from 'react';
import { PRODUCT_STATUS } from '@/constants/status';
import { createProduct, fetchProducts, type ProductListRow } from '@/services/products';
import { batchCheckProductReadiness, type ProductReadinessResult } from '@/services/productReadiness';
import { queryShops, type ShopListRow } from '@/services/shops';

export default function ProductDraftsPage() {
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [batchOpen, setBatchOpen] = useState(false);
  const [batchLoading, setBatchLoading] = useState(false);
  const [batchPlat, setBatchPlat] = useState<string>('tiktok');
  const [batchShopId, setBatchShopId] = useState<string>('');
  const [batchResult, setBatchResult] = useState<ProductReadinessResult[]>([]);
  const [shopsList, setShopsList] = useState<ShopListRow[]>([]);

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

  return (
    <PageContainer title="商品草稿">
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
          const res = await fetchProducts({
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

      <ModalForm
        title="新建商品草稿"
        open={createOpen}
        modalProps={{ destroyOnClose: true, onCancel: () => setCreateOpen(false) }}
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
        destroyOnClose
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
    </PageContainer>
  );
}
