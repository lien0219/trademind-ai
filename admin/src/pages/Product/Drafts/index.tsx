import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Image, Tag, Typography } from 'antd';
import { useRef, useState } from 'react';
import { PRODUCT_STATUS } from '@/constants/status';
import { createProduct, fetchProducts, type ProductListRow } from '@/services/products';

export default function ProductDraftsPage() {
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);

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

  return (
    <PageContainer title="商品草稿">
      <ProTable<ProductListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        headerTitle={false}
        toolBarRender={() => [
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
    </PageContainer>
  );
}
