import { FileImageOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Space, Tag, Typography } from 'antd';
import { PRODUCT_STATUS } from '@/constants/status';

type Row = {
  id: string;
  title: string;
  status: string;
  updatedAt: string;
};

const columns: ProColumns<Row>[] = [
  { title: '标题', dataIndex: 'title', ellipsis: true },
  {
    title: '状态',
    dataIndex: 'status',
    width: 120,
    render: (_, row) => {
      const m = PRODUCT_STATUS[row.status as keyof typeof PRODUCT_STATUS];
      return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
    },
  },
  { title: '更新时间', dataIndex: 'updatedAt', width: 180, search: false },
  {
    title: '操作',
    valueType: 'option',
    width: 120,
    render: () => [<a key="edit">编辑</a>],
  },
];

export default function ProductDraftsPage() {
  return (
    <PageContainer
      title="商品草稿"
      subTitle="采集与手工创建的商品将在此汇总，后续对齐商品 API。"
      extra={[
        <Button key="new" type="primary">
          新建草稿
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary">
        表格为占位；接通 <Typography.Text code>/api/v1/products</Typography.Text> 后使用 ProTable request 拉数。
      </Typography.Paragraph>
      <ProTable<Row>
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10 }}
        toolBarRender={() => [
          <Button key="import" icon={<FileImageOutlined />}>
            导入图片（预留）
          </Button>,
        ]}
        columns={columns}
        dataSource={[]}
        options={{ reload: true, density: true, setting: true }}
        headerTitle={<Space>草稿列表</Space>}
      />
    </PageContainer>
  );
}
