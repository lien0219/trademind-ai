import { ModalForm, ProFormText } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Tag, Typography } from 'antd';
import { useRef, useState } from 'react';
import { CUSTOMER_CONVERSATION_STATUS } from '@/constants/status';
import {
  createConversation,
  queryConversations,
  type ConversationRow,
} from '@/services/customer';

export default function CustomerConversationsPage() {
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);

  const columns: ProColumns<ConversationRow>[] = [
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 172,
      search: false,
      valueType: 'dateTime',
    },
    {
      title: 'platform',
      dataIndex: 'platform',
      width: 120,
      valueType: 'text',
    },
    {
      title: '客户名',
      dataIndex: 'customerName',
      width: 140,
      fieldProps: { placeholder: '筛选' },
    },
    {
      title: '语言',
      dataIndex: 'customerLanguage',
      width: 88,
      search: false,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(CUSTOMER_CONVERSATION_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => {
        const m = CUSTOMER_CONVERSATION_STATUS[row.status as keyof typeof CUSTOMER_CONVERSATION_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: '最新消息',
      dataIndex: 'latestMessage',
      ellipsis: true,
      search: false,
    },
    {
      title: '最后消息时间',
      dataIndex: 'lastMessageAt',
      width: 172,
      search: false,
      valueType: 'dateTime',
    },
    {
      title: '操作',
      valueType: 'option',
      width: 100,
      render: (_, row) => [
        <Typography.Link key="open" onClick={() => history.push(`/customer/conversations/${row.id}`)}>
          打开会话
        </Typography.Link>,
      ],
    },
  ];

  return (
    <PageContainer title="会话列表">
      <ProTable<ConversationRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        headerTitle={false}
        toolBarRender={() => [
          <Button key="new" type="primary" onClick={() => setCreateOpen(true)}>
            新建会话
          </Button>,
        ]}
        request={async (params) => {
          const res = await queryConversations({
            page: params.current,
            pageSize: params.pageSize,
            platform: params.platform as string | undefined,
            status: params.status as string | undefined,
            customerName: params.customerName as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />

      <ModalForm
        title="新建客服会话"
        open={createOpen}
        modalProps={{ destroyOnClose: true, onCancel: () => setCreateOpen(false) }}
        onFinish={async (vals) => {
          await createConversation({
            platform: (vals.platform as string) || 'manual',
            customerName: vals.customerName as string,
            customerLanguage: (vals.customerLanguage as string) || 'en',
          });
          setCreateOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="platform"
          label="platform"
          initialValue="manual"
          placeholder="默认 manual"
        />
        <ProFormText name="customerName" label="客户名称" rules={[{ required: true }]} />
        <ProFormText name="customerLanguage" label="语言" initialValue="en" placeholder="如 en" />
      </ModalForm>
    </PageContainer>
  );
}
