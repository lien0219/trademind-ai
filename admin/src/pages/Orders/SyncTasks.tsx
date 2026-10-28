import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { Alert, Button, Drawer, Popconfirm, Space, Tag, Typography, message, Table } from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { history } from '@umijs/max';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PAGE_COPY } from '@/constants/copywriting';
import { ORDER_SYNC_TASK_STATUS } from '@/constants/status';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { parsePositiveInt, queryTimeRange } from '@/utils/urlState';
import { isProductionBuild } from '@/utils/runtimeEnvironment';
import {
  getOrderSyncTask,
  queryOrderSyncTasks,
  retryOrderSyncTask,
  type OrderSyncTaskDTO,
  type OrderSyncTaskOutput,
} from '@/services/orderSync';

const ORDER_SYNC_QUERY_KEYS = [
  'page',
  'pageSize',
  'status',
  'platform',
  'shopId',
  'retryable',
  'resultStatus',
  'failedPagesOnly',
  'drawer',
  'id',
  'source',
  'start',
  'end',
  'createdFrom',
  'createdTo',
] as const;

function parseOutput(raw: unknown): OrderSyncTaskOutput | null {
  if (!raw || typeof raw !== 'object') return null;
  return raw as OrderSyncTaskOutput;
}

function tagFromStatus(raw: string) {
  const c = ORDER_SYNC_TASK_STATUS[raw as keyof typeof ORDER_SYNC_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

export default function OrderSyncTasksPage() {
  const language = useLanguage();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof ORDER_SYNC_QUERY_KEYS)[number], string | undefined>>(
      ORDER_SYNC_QUERY_KEYS,
    );
  const actionRef = useRef<ActionType>();
  const formRef = useRef<import('@ant-design/pro-components').ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const taskIdFromUrl = urlState.id;
  const statusFromUrl = urlState.status || urlState.resultStatus;
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<OrderSyncTaskDTO | null>(null);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    const createdRange = queryTimeRange(
      urlState.start,
      urlState.end,
      urlState.createdFrom,
      urlState.createdTo,
    );
    formRef.current?.setFieldsValue?.({
      status: statusFromUrl,
      platform: urlState.platform,
      shopId: urlState.shopId,
      createdRange,
    });
  }, [
    statusFromUrl,
    urlState.createdFrom,
    urlState.createdTo,
    urlState.end,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.shopId,
    urlState.start,
  ]);

  useEffect(() => {
    if (!statusFromUrl && !urlState.platform && !urlState.shopId) return;
    actionRef.current?.reload?.();
  }, [statusFromUrl, urlState.platform, urlState.shopId]);

  const openDetail = useCallback(
    async (id: string) => {
      const d = await getOrderSyncTask(id);
      setDetail(d);
      setDetailOpen(true);
      setUrlState({ drawer: 'task', id });
    },
    [setUrlState],
  );

  const closeDetail = useCallback(() => {
    setDetailOpen(false);
    setDetail(null);
    setUrlState({ drawer: undefined, id: undefined }, { replace: true });
  }, [setUrlState]);

  useEffect(() => {
    if (!taskIdFromUrl) return;
    void openDetail(taskIdFromUrl).catch(() => {
      /* ignore invalid id */
    });
  }, [taskIdFromUrl, openDetail]);

  const columns: ProColumns<OrderSyncTaskDTO>[] = useMemo(
    () => [
      {
        title: tr("创建时间范围"),
        dataIndex: 'createdRange',
        hideInTable: true,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: tr("创建时间"),
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: tr("店铺 ID"),
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'text',
      },
      {
        title: tr("店铺"),
        dataIndex: 'shopName',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.shopName || '—',
      },
      {
        title: tr("平台"),
        dataIndex: 'platform',
        width: 100,
        valueEnum: {
          douyin_shop: { text: tr("抖店") },
          tiktok: { text: 'TikTok' },
          shopee: { text: 'Shopee' },
          lazada: { text: 'Lazada' },
          amazon: { text: 'Amazon' },
          ...(!isProductionBuild ? { mock: { text: tr("模拟") } } : {}),
        },
      },
      {
        title: tr("模式"),
        dataIndex: 'mode',
        width: 96,
        search: false,
      },
      {
        title: tr("状态"),
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: ORDER_SYNC_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: tr("合计"),
        dataIndex: 'totalCount',
        width: 72,
        search: false,
      },
      {
        title: tr("成功"),
        dataIndex: 'successCount',
        width: 72,
        search: false,
      },
      {
        title: tr("失败"),
        dataIndex: 'failedCount',
        width: 72,
        search: false,
      },
      {
        title: tr("开始"),
        dataIndex: 'startedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.startedAt ? formatDateTime(r.startedAt) : '—'),
      },
      {
        title: tr("结束"),
        dataIndex: 'finishedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.finishedAt ? formatDateTime(r.finishedAt) : '—'),
      },
      {
        title: tr("错误"),
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        render: (_, r) => r.errorMessage || '—',
      },
      {
        title: tr("操作"),
        valueType: 'option',
        width: 140,
        render: (_, r) => (
          <Space>
            <a onClick={() => void openDetail(r.id)}>{tr("查看")}</a>
            {r.status === 'failed' || r.status === 'partial_success' ? (
              <Popconfirm
                title={
                  r.status === 'partial_success'
                    ? tr('重试将仅拉取失败页，不会重复拉取已成功页。确认？')
                    : tr('确认重试该同步任务？')
                }
                onConfirm={async () => {
                  await retryOrderSyncTask(r.id);
                  message.success(tr("已提交重试"));
                  actionRef.current?.reload();
                }}
              >
                <Button type="link" size="small" style={{ padding: 0 }}>
                  {tr("重试")}
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
        ),
      },
    ],
    [language, openDetail],
  );

  const output = parseOutput(detail?.output);

  return (
    <TmPageContainer title={PAGE_COPY.orderSyncTasks.title} subTitle={PAGE_COPY.orderSyncTasks.description}>
      <ProTable<OrderSyncTaskDTO>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          closeDetail();
          clearUrlState(ORDER_SYNC_QUERY_KEYS, { replace: true });
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
        headerTitle={tr("同步记录")}
        request={async (params) => {
          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            shopId: (params.shopId as string | undefined)?.trim() || urlState.shopId,
            platform: (params.platform as string | undefined)?.trim(),
            status: (params.status as string | undefined)?.trim() || statusFromUrl,
            start: typeof params.start === 'string' ? params.start : urlState.start,
            end: typeof params.end === 'string' ? params.end : urlState.end,
          };
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              shopId: qp.shopId,
              platform: qp.platform,
              status: qp.status,
              resultStatus: qp.status === 'partial_success' ? qp.status : undefined,
              start: qp.start,
              end: qp.end,
              source: urlState.source,
              drawer: urlState.drawer,
              id: urlState.id,
            },
            { replace: true },
          );
          const res = await queryOrderSyncTasks(qp);
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />

      <Drawer
        width={560}
        title={detail ? trFormat("同步任务 {value0}", { value0: detail.id }) : tr('详情')}
        open={detailOpen}
        destroyOnHidden
        onClose={closeDetail}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            {detail.status === 'partial_success' ? (
              <Alert
                showIcon
                type="warning"
                message={tr("部分成功：部分页已同步，部分页失败")}
                description={tr("已成功页的数据已写入本地订单；失败页可在下方列表查看原因，点击「重试失败页」仅重拉失败页。")}
              />
            ) : null}
            <div>
              <Typography.Text strong>{tr("状态：")}</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Space wrap>
              <Typography.Link
                onClick={() =>
                  history.push(`/ops/task-center/failures?taskType=order_sync&keyword=${encodeURIComponent(detail.id)}`)
                }
              >
                {tr("在失败任务中心查看")}
              </Typography.Link>
              <Typography.Link
                onClick={() =>
                  history.push(`/orders/exceptions?exceptionType=order_sync_partial_failed`)
                }
              >
                {tr("订单异常工作台")}
              </Typography.Link>
            </Space>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{tr("店铺：")}</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text strong>{tr("任务编号：")}</Typography.Text> {detail.id}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph type="danger">
                <Typography.Text strong>{tr("失败原因：")}</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            {output ? (
              <>
                <Typography.Paragraph style={{ marginBottom: 8 }}>
                  <Typography.Text strong>{tr("分页：")}</Typography.Text> {tr("总页")} {output.totalPages ?? 0} {tr("· 成功")}{' '}
                  {output.successPages ?? 0} {tr("· 失败")} {output.failedPages ?? 0}
                  {output.retryPagesOnly ? tr(' · 上次为重试失败页模式') : ''}
                </Typography.Paragraph>
                <Typography.Paragraph style={{ marginBottom: 8 }}>
                  <Typography.Text strong>{tr("拉取订单：")}</Typography.Text> {output.totalFetched ?? 0} {tr("条 · 新建")}{' '}
                  {output.createdOrders ?? 0} {tr("· 更新")} {output.updatedOrders ?? 0}
                </Typography.Paragraph>
                <Typography.Paragraph style={{ marginBottom: 8 }}>
                  <Typography.Text strong>{tr("规格匹配：")}</Typography.Text> {tr("已匹配")} {output.matchedItems ?? 0} {tr("· 未匹配")}{' '}
                  {output.unmatchedItems ?? 0}
                  {output.deductedStockItems ? trFormat(" · 扣库 {value0} 行", { value0: String(output.deductedStockItems) }) : ''}
                </Typography.Paragraph>
                {(output.pageErrors?.length ?? 0) > 0 ? (
                  <>
                    <Typography.Text strong>{tr("失败页列表")}</Typography.Text>
                    <Table
                      size="small"
                      style={{ marginTop: 8 }}
                      pagination={false}
                      rowKey={(r) => String(r.page)}
                      dataSource={output.pageErrors}
                      columns={[
                        { title: tr("页码"), dataIndex: 'page', width: 64 },
                        {
                          title: 'cursor / nextPage',
                          key: 'cursor',
                          width: 120,
                          render: (_, r) => r.cursor || r.nextPage || '—',
                        },
                        { title: tr("错误码"), dataIndex: 'errorCode', width: 100, render: (v) => v || '—' },
                        { title: tr("错误信息"), dataIndex: 'error', ellipsis: true },
                        {
                          title: tr("可重试"),
                          width: 72,
                          render: () => <Tag color="success">{tr("是")}</Tag>,
                        },
                      ]}
                    />
                    {detail.status === 'partial_success' || detail.status === 'failed' ? (
                      <Popconfirm
                        title={tr("仅重试失败页（不重复拉取已成功页）")}
                        onConfirm={async () => {
                          await retryOrderSyncTask(detail.id);
                          message.success(tr("已提交重试失败页"));
                          await openDetail(detail.id);
                          actionRef.current?.reload();
                        }}
                      >
                        <Button type="primary" size="small" style={{ marginTop: 8 }}>
                          {tr("重试失败页")}
                        </Button>
                      </Popconfirm>
                    ) : null}
                  </>
                ) : null}
              </>
            ) : null}
            <TechnicalDetails>
              <TaskJsonBlock title={tr("任务输入")} value={detail.input} />
              <TaskJsonBlock title={tr("任务输出")} value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>
    </TmPageContainer>
  );
}
