import { ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Space, Tag, Typography } from 'antd';
import { COLLECT_TASK_STATUS } from '@/constants/status';

type Row = {
  id: string;
  sourceUrl: string;
  status: string;
  message?: string;
  updatedAt: string;
};

const columns: ProColumns<Row>[] = [
  {
    title: '来源链接',
    dataIndex: 'sourceUrl',
    ellipsis: true,
    copyable: true,
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 120,
    valueType: 'select',
    valueEnum: Object.fromEntries(
      Object.entries(COLLECT_TASK_STATUS).map(([k, v]) => [k, { text: v.text }]),
    ),
    render: (_, row) => {
      const m = COLLECT_TASK_STATUS[row.status as keyof typeof COLLECT_TASK_STATUS];
      return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
    },
  },
  { title: '说明', dataIndex: 'message', ellipsis: true, search: false },
  { title: '更新时间', dataIndex: 'updatedAt', width: 180, search: false },
  {
    title: '操作',
    valueType: 'option',
    width: 100,
    render: () => [<a key="retry">重试</a>],
  },
];

export default function CollectTasksPage() {
  return (
    <PageContainer
      title="采集任务"
      subTitle="1688 等链接采集走独立 Node 服务，主业务仅编排任务与结果入库。"
      extra={[
        <Button key="new" type="primary">
          新建采集任务
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary">
        占位列表；后续对接 <Typography.Text code>/api/v1/collect/tasks</Typography.Text>，失败原因与重试入口对齐规则。
      </Typography.Paragraph>
      <ProTable<Row>
        rowKey="id"
        search={{ labelWidth: 'auto' }}
        pagination={{ pageSize: 10 }}
        toolBarRender={() => [
          <Button key="reload" icon={<ReloadOutlined />}>
            刷新
          </Button>,
        ]}
        columns={columns}
        dataSource={[]}
        options={{ reload: true, density: true, setting: true }}
        headerTitle={<Space>任务列表</Space>}
      />
    </PageContainer>
  );
}
