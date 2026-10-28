import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ModalForm, ProFormDigit, ProFormRadio, ProFormSelect, ProFormText } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';

import { history, useLocation } from '@umijs/max';
import { Button, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { parsePositiveInt } from '@/utils/urlState';
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

const CONVERSATION_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'replyStatus',
  'aiSuggestionStatus',
  'sendStatus',
  'platform',
  'shopId',
  'conversationId',
  'suggestionId',
  'drawer',
  'source',
  'pendingReply',
  'hasAiSuggestion',
  'sendFailed',
  'hasOrder',
  'status',
] as const;

function readConversationLegacyFilters(search: string) {
  const sp = new URLSearchParams(search);
  const replyStatus = sp.get('replyStatus')?.trim();
  const aiSuggestionStatus = sp.get('aiSuggestionStatus')?.trim();
  const sendStatus = sp.get('sendStatus')?.trim();
  return {
    pendingReply:
      sp.get('pendingReply') === '1' ||
      replyStatus === 'pending' ||
      replyStatus === 'pending_reply' ||
      sp.get('status') === 'pending_reply',
    hasAiSuggestion:
      sp.get('hasAiSuggestion') === '1' ||
      aiSuggestionStatus === 'pending',
    sendFailed: sp.get('sendFailed') === '1' || sendStatus === 'failed',
    hasOrder: sp.get('hasOrder') === '1',
    conversationId: sp.get('conversationId')?.trim(),
    suggestionId: sp.get('suggestionId')?.trim(),
  };
}

function tagFrom(raw: string | undefined, map: Record<string, { text: string; color: string }>) {
  const k = (raw || '').trim();
  if (!k) return '—';
  const m = map[k as keyof typeof map];
  return m ? <Tag color={m.color}>{m.text}</Tag> : <Tag>{k}</Tag>;
}

export default function CustomerConversationsPage() {
  const language = useLanguage();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const location = useLocation();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof CONVERSATION_QUERY_KEYS)[number], string | undefined>>(
      CONVERSATION_QUERY_KEYS,
    );
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });
  const legacyFilters = useMemo(
    () => readConversationLegacyFilters(location.search),
    [location.search],
  );
  const [createOpen, setCreateOpen] = useState(false);
  const emptyLocale = useListEmptyLocale('customerConversations', {
    permissionScoped: true,
    onAction: () => setCreateOpen(true),
    actionLabel: tr('新建会话'),
  });
  const [pullOpen, setPullOpen] = useState(false);
  const [shopOptions, setShopOptions] = useState<{ label: string; value: string }[]>([]);

  const urlFilters = legacyFilters;

  useEffect(() => {
    const cid = legacyFilters.conversationId;
    if (!cid) return;
    const sp = new URLSearchParams(location.search);
    sp.delete('conversationId');
    const qs = sp.toString();
    history.replace(qs ? `/customer/conversations/${cid}?${qs}` : `/customer/conversations/${cid}`);
  }, [legacyFilters.conversationId, location.search]);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      platform: urlState.platform,
      shopId: urlState.shopId,
      pendingReply: urlFilters.pendingReply ? 'true' : undefined,
      hasAiSuggestion: urlFilters.hasAiSuggestion ? 'true' : undefined,
      sendFailed: urlFilters.sendFailed ? 'true' : undefined,
      hasOrder: urlFilters.hasOrder ? 'true' : undefined,
    });
  }, [
    urlFilters.hasAiSuggestion,
    urlFilters.hasOrder,
    urlFilters.pendingReply,
    urlFilters.sendFailed,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.shopId,
  ]);

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

  const columns: ProColumns<ConversationRow>[] = useMemo(
    () => [
    {
      title: tr("关键词"),
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: tr("买家 / 会话 ID / 订单"), ...keywordFieldProps },
    },
    {
      title: tr("待回复"),
      dataIndex: 'pendingReply',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: tr("是") }, false: { text: tr("否") } },
    },
    {
      title: tr("有 AI 建议"),
      dataIndex: 'hasAiSuggestion',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: tr("是") }, false: { text: tr("否") } },
    },
    {
      title: tr("发送失败"),
      dataIndex: 'sendFailed',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: tr("是") }, false: { text: tr("否") } },
    },
    {
      title: tr("有关联订单"),
      dataIndex: 'hasOrder',
      hideInTable: true,
      valueType: 'select',
      valueEnum: { true: { text: tr("是") }, false: { text: tr("否") } },
    },
    {
      title: tr("店铺"),
      dataIndex: 'shopId',
      width: 200,
      hideInTable: true,
      valueType: 'select',
      fieldProps: {
        options: shopOptions,
        showSearch: true,
        placeholder: tr("按店铺筛选"),
        allowClear: true,
      },
    },
    {
      title: tr("平台"),
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
      title: tr("店铺"),
      dataIndex: 'shopName',
      width: 120,
      search: false,
      ellipsis: true,
      render: (_, row) => row.shopName || '—',
    },
    {
      title: tr("买家"),
      dataIndex: 'customerName',
      width: 120,
      fieldProps: { placeholder: tr("筛选") },
      render: (_, row) => row.customerNameMasked || row.customerName,
    },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 96,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(CUSTOMER_CONVERSATION_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => tagFrom(row.status, CUSTOMER_CONVERSATION_STATUS),
    },
    {
      title: tr("最近消息"),
      dataIndex: 'latestMessage',
      ellipsis: true,
      search: false,
    },
    {
      title: tr("关联订单"),
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
      title: tr("关联商品"),
      dataIndex: 'productTitle',
      width: 140,
      search: false,
      ellipsis: true,
    },
    {
      title: tr("AI 建议"),
      dataIndex: 'aiSuggestionStatus',
      width: 96,
      search: false,
      render: (_, row) => tagFrom(row.aiSuggestionStatus, CUSTOMER_SUGGESTION_STATUS),
    },
    {
      title: tr("发送状态"),
      dataIndex: 'sendStatus',
      width: 96,
      search: false,
      render: (_, row) => tagFrom(row.sendStatus, CUSTOMER_SEND_STATUS),
    },
    {
      title: tr("更新时间"),
      dataIndex: 'lastMessageAt',
      width: 160,
      search: false,
      render: (_, row) => formatDateTime(row.lastMessageAt || row.updatedAt),
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 160,
      render: (_, row) => [
        <Typography.Link
          key="open"
          onClick={() => {
            const sp = new URLSearchParams(location.search);
            sp.delete('conversationId');
            const qs = sp.toString();
            history.push(
              qs
                ? `/customer/conversations/${row.id}?${qs}`
                : `/customer/conversations/${row.id}`,
            );
          }}
        >
          {tr('查看会话')}
        </Typography.Link>,
        row.openFailureCount ? (
          <Typography.Link
            key="fail"
            onClick={() => history.push(`/ops/task-center/failures?taskType=customer_failure`)}
          >
            {tr('失败任务')}
          </Typography.Link>
        ) : null,
      ],
    },
  ],
    [keywordFieldProps, language, location.search],
  );

  return (
    <TmPageContainer title={tr("会话列表")} subTitle={tr("所有回复需人工确认；系统不会自动发送消息。")}>
      <KeywordSafetyHint visible={showSensitiveHint} />
      <ProTable<ConversationRow>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        params={{
          current: tablePage,
          pageSize: tablePageSize,
          keyword: urlState.keyword,
          platform: urlState.platform,
          shopId: urlState.shopId,
          pendingReply: urlFilters.pendingReply ? 'true' : undefined,
          hasAiSuggestion: urlFilters.hasAiSuggestion ? 'true' : undefined,
          sendFailed: urlFilters.sendFailed ? 'true' : undefined,
          hasOrder: urlFilters.hasOrder ? 'true' : undefined,
        }}
        search={{ labelWidth: 'auto' }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(CONVERSATION_QUERY_KEYS, { replace: true });
        }}
        pagination={{
          current: tablePage,
          pageSize: tablePageSize,
          showSizeChanger: true,
          onChange: (page, pageSize) => {
            setTablePage(page);
            setTablePageSize(pageSize);
            setUrlState({
              page: page > 1 ? page : undefined,
              pageSize: pageSize !== 20 ? pageSize : undefined,
            });
          },
        }}
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
            {tr("客服中心")}
          </Button>,
          <Button key="pull" onClick={() => setPullOpen(true)}>
            {tr("拉取平台消息")}
          </Button>,
          <Button key="new" type="primary" onClick={() => setCreateOpen(true)}>
            {tr("新建会话")}
          </Button>,
        ]}
        request={async (params) => {
          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            platform: (params.platform as string | undefined)?.trim(),
            shopId: (params.shopId as string | undefined)?.trim(),
            keyword: prepareKeyword(params.keyword),
            pendingReply: params.pendingReply as boolean | string | undefined,
            hasAiSuggestion: params.hasAiSuggestion as boolean | string | undefined,
            sendFailed: params.sendFailed as boolean | string | undefined,
            hasOrder: params.hasOrder as boolean | string | undefined,
          };
          const replyStatus =
            qp.pendingReply === 'true' || qp.pendingReply === true ? 'pending_reply' : undefined;
          const aiSuggestionStatus =
            qp.hasAiSuggestion === 'true' || qp.hasAiSuggestion === true ? 'pending' : undefined;
          const sendStatus =
            qp.sendFailed === 'true' || qp.sendFailed === true ? 'failed' : undefined;
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              keyword: qp.keyword,
              platform: qp.platform,
              shopId: qp.shopId,
              replyStatus,
              aiSuggestionStatus,
              sendStatus,
              pendingReply: replyStatus ? '1' : undefined,
              hasAiSuggestion: aiSuggestionStatus ? '1' : undefined,
              sendFailed: sendStatus ? '1' : undefined,
              hasOrder:
                qp.hasOrder === 'true' || qp.hasOrder === true ? '1' : undefined,
              suggestionId: urlState.suggestionId || legacyFilters.suggestionId,
              source: urlState.source,
            },
            { replace: true },
          );
          const res = await queryConversations({
            page: qp.page,
            pageSize: qp.pageSize,
            platform: qp.platform,
            status: params.status as string | undefined,
            shopId: qp.shopId,
            customerName: params.customerName as string | undefined,
            keyword: qp.keyword,
            pendingReply: qp.pendingReply,
            hasAiSuggestion: qp.hasAiSuggestion,
            sendFailed: qp.sendFailed,
            hasOrder: qp.hasOrder,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />

      <ModalForm
        title={tr("拉取平台客服消息")}
        open={pullOpen}
        modalProps={{ destroyOnHidden: true, onCancel: () => setPullOpen(false) }}
        initialValues={{ mode: 'incremental', limit: 50, cursor: '', start: '', end: '' }}
        onFinish={async (vals) => {
          const sid = vals.shopId as string | undefined;
          if (!sid) {
            message.warning(tr("请选择店铺"));
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
            message.error(e instanceof Error ? e.message : tr('提交失败'));
            return false;
          }
          message.success(tr("客服消息同步任务已提交"));
          setPullOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="shopId"
          label={tr("店铺")}
          options={shopOptions}
          rules={[{ required: true, message: tr("请选择店铺") }]}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
        />
        <ProFormRadio.Group
          name="mode"
          label={tr("同步模式")}
          options={[
            { label: tr("增量"), value: 'incremental' },
            { label: tr("全量"), value: 'full' },
            { label: tr("手动"), value: 'manual' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormText name="start" label={tr("开始时间（可选）")} placeholder="2026-05-01T00:00:00Z" />
        <ProFormText name="end" label={tr("结束时间（可选）")} placeholder="2026-05-16T23:59:59Z" />
        <ProFormText name="cursor" label={tr("游标（可选）")} />
        <ProFormDigit name="limit" label={tr("每页条数")} min={1} max={200} fieldProps={{ precision: 0 }} />
      </ModalForm>

      <ModalForm
        title={tr("新建客服会话")}
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
          label={tr("平台")}
          initialValue="manual"
          options={PLATFORM_OPTIONS}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
        />
        <ProFormSelect
          name="shopId"
          label={tr("关联店铺（可选）")}
          options={shopOptions}
          fieldProps={{ allowClear: true, showSearch: true }}
        />
        <ProFormText name="customerName" label={tr("客户名称")} rules={[{ required: true }]} />
        <ProFormText name="customerLanguage" label={tr("语言")} initialValue="en" placeholder={tr("如 en")} />
      </ModalForm>
    </TmPageContainer>
  );
}
