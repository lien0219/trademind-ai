import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Tag, Typography, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { CUSTOMER_CONVERSATION_STATUS } from '@/constants/status';
import {
  createConversation,
  queryConversations,
  syncCustomerMessages,
  type ConversationRow,
} from '@/services/customer';
import { queryShops } from '@/services/shops';

export default function CustomerConversationsPage() {
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [pullOpen, setPullOpen] = useState(false);
  const [shopOptions, setShopOptions] = useState<{ label: string; value: string }[]>([]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 500 });
        setShopOptions(
          res.list.map((s) => ({
            label: `${s.shopName} (${s.platform})`,
            value: s.id,
          })),
        );
      } catch {
        /* ignore */
      }
    })();
  }, []);

  const columns: ProColumns<ConversationRow>[] = [
    {
      title: '店铺',
      dataIndex: 'shopId',
      width: 200,
      hideInTable: true,
      valueType: 'select',
      fieldProps: {
        options: shopOptions,
        showSearch: true,
        placeholder: '按店铺筛选',
        allowClear: true,
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
      title: 'platform',
      dataIndex: 'platform',
      width: 120,
      valueType: 'text',
    },
    {
      title: '店铺',
      dataIndex: 'shopName',
      width: 140,
      search: false,
      ellipsis: true,
      render: (_, row) =>
        row.shopName ? (
          <span>
            {row.shopName}
            {row.shopPlatform ? ` / ${row.shopPlatform}` : ''}
          </span>
        ) : (
          '—'
        ),
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
          <Button key="pull" onClick={() => setPullOpen(true)}>
            拉取平台消息
          </Button>,
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
            shopId: params.shopId as string | undefined,
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
        title="拉取平台客服消息"
        open={pullOpen}
        modalProps={{ destroyOnHidden: true, onCancel: () => setPullOpen(false) }}
        initialValues={{ mode: 'incremental', limit: 50, cursor: '', start: '', end: '' }}
        onFinish={async (vals) => {
          const sid = vals.shopId as string | undefined;
          if (!sid) {
            message.warning('请选择店铺');
            return false;
          }
          try {
            await syncCustomerMessages(sid, {
              mode: vals.mode as string,
              start: (vals.start as string | undefined) || undefined,
              end: (vals.end as string | undefined) || undefined,
              cursor: (vals.cursor as string | undefined) || undefined,
              limit: vals.limit as number | undefined,
            });
          } catch (e: unknown) {
            message.error(e instanceof Error ? e.message : '提交失败');
            return false;
          }
          message.success('客服消息同步任务已提交');
          setPullOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="shopId"
          label="店铺"
          options={shopOptions}
          rules={[{ required: true, message: '请选择店铺' }]}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
        />
        <ProFormRadio.Group
          name="mode"
          label="同步模式"
          options={[
            { label: '增量 incremental', value: 'incremental' },
            { label: '全量 full', value: 'full' },
            { label: '手动 manual', value: 'manual' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormText name="start" label="开始时间（可选 RFC3339）" placeholder="2026-05-01T00:00:00Z" />
        <ProFormText name="end" label="结束时间（可选 RFC3339）" placeholder="2026-05-16T23:59:59Z" />
        <ProFormText name="cursor" label="游标（可选）" />
        <ProFormDigit name="limit" label="每页条数" min={1} max={200} fieldProps={{ precision: 0 }} />
      </ModalForm>

      <ModalForm
        title="新建客服会话"
        open={createOpen}
        modalProps={{ destroyOnHidden: true, onCancel: () => setCreateOpen(false) }}
        onFinish={async (vals) => {
          await createConversation({
            platform: (vals.platform as string) || 'manual',
            shopId: (vals.shopId as string) || undefined,
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
        <ProFormSelect
          name="shopId"
          label="关联店铺（可选）"
          options={shopOptions}
          fieldProps={{ allowClear: true, showSearch: true }}
        />
        <ProFormText name="customerName" label="客户名称" rules={[{ required: true }]} />
        <ProFormText name="customerLanguage" label="语言" initialValue="en" placeholder="如 en" />
      </ModalForm>
    </PageContainer>
  );
}
