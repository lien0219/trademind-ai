import { ModalForm, ProFormDigit, ProFormRadio, ProFormSelect, ProFormText } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';

import { history, useLocation } from '@umijs/max';
import { Button, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import {
  CUSTOMER_CONVERSATION_STATUS,
  CUSTOMER_SEND_STATUS,
  CUSTOMER_SUGGESTION_STATUS,
} from '@/constants/status';
import { PLATFORM_OPTIONS, platformLabel } from '@/constants/userFriendly';
import {
  createConversation,
  queryConversations,
  syncCustomerMessages,
  type ConversationRow,
} from '@/services/customer';
import { queryShops } from '@/services/shops';

function tagFrom(raw: string | undefined, map: Record<string, { text: string; color: string }>) {
  const k = (raw || '').trim();
  if (!k) return '—';
  const m = map[k as keyof typeof map];
  return m ? <Tag color={m.color}>{m.text}</Tag> : <Tag>{k}</Tag>;
}

export default function CustomerConversationsPage() {
  const actionRef = useRef<ActionType>();
  const location = useLocation();
  const [createOpen, setCreateOpen] = useState(false);
  const emptyLocale = useListEmptyLocale('customerConversations', {
    permissionScoped: true,
    onAction: () => setCreateOpen(true),
    actionLabel: '新建会话',
  });
  const [pullOpen, setPullOpen] = useState(false);
  const [shopOptions, setShopOptions] = useState<{ label: string; value: string }[]>([]);

  const urlFilters = useMemo(() => {
    const q = new URLSearchParams(location.search || '');
    return {
      pendingReply: q.get('pendingReply') === '1',
      hasAiSuggestion: q.get('hasAiSuggestion') === '1',
      sendFailed: q.get('sendFailed') === '1',
      hasOrder: q.get('hasOrder') === '1',
    };
  }, [location.search]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 500 });
        setShopOptions(
          res.list.map((s) => ({
            label: `${s.shopName} (${platformLabel(s.platform)})`,
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
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '买家 / 会话 ID / 订单' },
    },
    {
      title: '待回复',
      dataIndex: 'pendingReply',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: '是' }, false: { text: '否' } },
    },
    {
      title: '有 AI 建议',
      dataIndex: 'hasAiSuggestion',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: '是' }, false: { text: '否' } },
    },
    {
      title: '发送失败',
      dataIndex: 'sendFailed',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: '是' }, false: { text: '否' } },
    },
    {
      title: '有关联订单',
      dataIndex: 'hasOrder',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: '是' }, false: { text: '否' } },
    },
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
      title: '平台',
      dataIndex: 'platform',
      width: 100,
      valueType: 'select',
      fieldProps: {
        showSearch: true,
        optionFilterProp: 'label',
        options: PLATFORM_OPTIONS,
        allowClear: true,
      },
      render: (_, row) => platformLabel(row.platform),
    },
    {
      title: '店铺',
      dataIndex: 'shopName',
      width: 120,
      search: false,
      ellipsis: true,
      render: (_, row) => row.shopName || '—',
    },
    {
      title: '买家',
      dataIndex: 'customerName',
      width: 120,
      fieldProps: { placeholder: '筛选' },
      render: (_, row) => row.customerNameMasked || row.customerName,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 96,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(CUSTOMER_CONVERSATION_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => tagFrom(row.status, CUSTOMER_CONVERSATION_STATUS),
    },
    {
      title: '最近消息',
      dataIndex: 'latestMessage',
      ellipsis: true,
      search: false,
    },
    {
      title: '关联订单',
      dataIndex: 'orderNo',
      width: 120,
      search: false,
      render: (_, row) =>
        row.orderNo ? (
          <Typography.Link onClick={() => history.push(`/orders/${row.orderId}`)}>{row.orderNo}</Typography.Link>
        ) : (
          '—'
        ),
    },
    {
      title: '关联商品',
      dataIndex: 'productTitle',
      width: 140,
      search: false,
      ellipsis: true,
    },
    {
      title: 'AI 建议',
      dataIndex: 'aiSuggestionStatus',
      width: 96,
      search: false,
      render: (_, row) => tagFrom(row.aiSuggestionStatus, CUSTOMER_SUGGESTION_STATUS),
    },
    {
      title: '发送状态',
      dataIndex: 'sendStatus',
      width: 96,
      search: false,
      render: (_, row) => tagFrom(row.sendStatus, CUSTOMER_SEND_STATUS),
    },
    {
      title: '更新时间',
      dataIndex: 'lastMessageAt',
      width: 160,
      search: false,
      render: (_, row) => formatDateTime(row.lastMessageAt || row.updatedAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 160,
      render: (_, row) => [
        <Typography.Link key="open" onClick={() => history.push(`/customer/conversations/${row.id}`)}>
          查看会话
        </Typography.Link>,
        row.openFailureCount ? (
          <Typography.Link
            key="fail"
            onClick={() => history.push(`/ops/task-center/failures?taskType=customer_failure`)}
          >
            失败任务
          </Typography.Link>
        ) : null,
      ],
    },
  ];

  return (
    <TmPageContainer title="会话列表" subTitle="所有回复需人工确认；系统不会自动发送消息。">
      <ProTable<ConversationRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        form={{
          initialValues: {
            pendingReply: urlFilters.pendingReply ? 'true' : undefined,
            hasAiSuggestion: urlFilters.hasAiSuggestion ? 'true' : undefined,
            sendFailed: urlFilters.sendFailed ? 'true' : undefined,
            hasOrder: urlFilters.hasOrder ? 'true' : undefined,
          },
        }}
        locale={emptyLocale}
        toolBarRender={() => [
          <Button key="hub" onClick={() => history.push('/customer/hub')}>
            客服中心
          </Button>,
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
            keyword: params.keyword as string | undefined,
            pendingReply: params.pendingReply as boolean | string | undefined,
            hasAiSuggestion: params.hasAiSuggestion as boolean | string | undefined,
            sendFailed: params.sendFailed as boolean | string | undefined,
            hasOrder: params.hasOrder as boolean | string | undefined,
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
            { label: '增量', value: 'incremental' },
            { label: '全量', value: 'full' },
            { label: '手动', value: 'manual' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormText name="start" label="开始时间（可选）" placeholder="2026-05-01T00:00:00Z" />
        <ProFormText name="end" label="结束时间（可选）" placeholder="2026-05-16T23:59:59Z" />
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
        <ProFormSelect
          name="platform"
          label="平台"
          initialValue="manual"
          options={PLATFORM_OPTIONS}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
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
    </TmPageContainer>
  );
}
