import { tr, trFormat } from '@/i18n';
import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { Button, Drawer, Space, Tag, Typography, Alert, message } from 'antd';
import { confirmFailureTaskRetry } from '@/constants/sensitiveActions';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useEffect, useMemo, useRef, useState } from 'react';
import InventorySyncDisabledBanner from '@/components/inventory/InventorySyncDisabledBanner';
import {
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
} from '@/constants/inventoryLabels';
import { PAGE_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { platformLabel } from '@/constants/userFriendly';
import { parsePositiveInt } from '@/utils/urlState';
import {
  getInventorySyncTask,
  queryInventorySyncTasks,
  retryInventorySyncTask,
  retryInventorySyncTasksBatch,
  type InventorySyncTaskDTO,
} from '@/services/inventory';

const SYNC_TASK_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'status',
  'syncStatus',
  'platform',
  'shopId',
  'productSkuId',
  'batchId',
  'drawer',
  'id',
  'source',
] as const;

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

const BATCH_RETRY_LIMIT = 100;

export default function InventorySyncTasksPage() {
  const emptyLocale = useListEmptyLocale('inventorySyncTasks', { permissionScoped: true });
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof SYNC_TASK_QUERY_KEYS)[number], string | undefined>>(
      SYNC_TASK_QUERY_KEYS,
    );
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);

  const batchIdFromUrl = urlState.batchId;
  const taskIdFromUrl = urlState.id;
  const skuIdFromUrl = urlState.productSkuId;
  const statusFromUrl = urlState.status || urlState.syncStatus;

  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<InventorySyncTaskDTO | null>(null);
  const [failedSelectedIds, setFailedSelectedIds] = useState<string[]>([]);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      batchId: batchIdFromUrl,
      status: statusFromUrl,
      productSkuId: skuIdFromUrl,
      platform: urlState.platform,
      shopId: urlState.shopId,
      keyword: urlState.keyword,
    });
  }, [
    batchIdFromUrl,
    skuIdFromUrl,
    statusFromUrl,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.shopId,
  ]);

  useEffect(() => {
    if (!batchIdFromUrl && !statusFromUrl && !skuIdFromUrl) return;
    actionRef.current?.reload?.();
  }, [batchIdFromUrl, skuIdFromUrl, statusFromUrl]);

  useEffect(() => {
    if (!taskIdFromUrl) return;
    void (async () => {
      try {
        const d = await getInventorySyncTask(taskIdFromUrl);
        setDetail(d);
        setDetailOpen(true);
      } catch {
        /* ignore invalid id */
      }
    })();
  }, [taskIdFromUrl]);

  const openTaskDetail = async (taskId: string) => {
    const d = await getInventorySyncTask(taskId);
    setDetail(d);
    setDetailOpen(true);
    setUrlState({ drawer: 'task', id: taskId });
  };

  const closeTaskDetail = () => {
    setDetailOpen(false);
    setDetail(null);
    setUrlState({ drawer: undefined, id: undefined }, { replace: true });
  };

  const columns: ProColumns<InventorySyncTaskDTO>[] = useMemo(
    () => [
      {
        title: tr("批次 ID"),
        dataIndex: 'batchId',
        hideInTable: true,
      },
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
        title: tr("商品 ID"),
        dataIndex: 'productId',
        hideInTable: true,
      },
      {
        title: tr("规格编号"),
        dataIndex: 'productSkuId',
        hideInTable: true,
      },
      {
        title: tr("店铺 ID"),
        dataIndex: 'shopId',
        hideInTable: true,
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
        title: tr("商品标题"),
        dataIndex: 'productTitle',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.productTitle || '—',
      },
      {
        title: tr("规格编码"),
        dataIndex: 'skuCode',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuCode || '—',
      },
      {
        title: tr("批次号"),
        dataIndex: 'batchNo',
        width: 132,
        search: false,
        ellipsis: true,
        render: (_, r) => r.batchNo || '—',
      },
      {
        title: tr("平台"),
        dataIndex: 'platform',
        width: 100,
        render: (_, r) => platformLabel(r.platform),
      },
      {
        title: tr("目标库存"),
        dataIndex: 'targetStock',
        width: 92,
        search: false,
      },
      {
        title: tr("状态"),
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: COLLECT_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
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
            <a
              onClick={() => {
                void openTaskDetail(r.id);
              }}
            >
              {tr("查看")}
            </a>
            {r.status === 'failed' || r.status === 'partial_success' ? (
              <Button
                type="link"
                size="small"
                style={{ padding: 0 }}
                onClick={() =>
                  confirmFailureTaskRetry(1, async () => {
                    await retryInventorySyncTask(r.id);
                    message.success(tr("已提交重试"));
                    actionRef.current?.reload();
                  })
                }
              >
                {tr("重试")}
              </Button>
            ) : null}
          </Space>
        ),
      },
    ],
    [],
  );

  return (
    <TmPageContainer title={PAGE_COPY.inventorySyncTasks.title} subTitle={PAGE_COPY.inventorySyncTasks.description}>
      <InventorySyncDisabledBanner />
      <Alert
        showIcon
        type="info"
        style={{ marginBottom: 16 }}
        message={tr("库存同步前置条件")}
        description={
          <>
            {INVENTORY_SKU_NOT_BOUND_MESSAGE} {INVENTORY_SKU_AMBIGUOUS_MESSAGE}
            {tr("须开启「启用库存同步」后，可在商品详情 → 库存 Tab 或库存预警页人工发起同步；失败任务支持重试。")}
          </>
        }
      />
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        {tr("可用平台由店铺授权与平台能力状态决定；仅已开放库存同步能力的平台可以创建任务。")}
      </Typography.Paragraph>
      <ProTable<InventorySyncTaskDTO>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          closeTaskDetail();
          clearUrlState(SYNC_TASK_QUERY_KEYS, { replace: true });
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
        headerTitle={tr("任务列表")}
        rowSelection={{
          selectedRowKeys: failedSelectedIds,
          onChange: (keys) => setFailedSelectedIds(keys.map(String)),
          getCheckboxProps: (record) => ({
            disabled: record.status !== 'failed',
          }),
        }}
        tableAlertRender={false}
        locale={emptyLocale}
        toolBarRender={() => [
          <Button
            key="batch-retry"
            disabled={failedSelectedIds.length === 0}
            onClick={() => {
              if (failedSelectedIds.length > BATCH_RETRY_LIMIT) {
                message.warning(trFormat("单次最多选择 {value0} 条失败任务", { value0: BATCH_RETRY_LIMIT }));
                return;
              }
              confirmFailureTaskRetry(failedSelectedIds.length, async () => {
                try {
                  const batch = await retryInventorySyncTasksBatch(failedSelectedIds);
                  message.success(trFormat("已创建批次 {value0}", { value0: batch.batchNo }));
                  setFailedSelectedIds([]);
                  actionRef.current?.reload();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || tr('批量重试失败'));
                  throw e;
                }
              });
            }}
          >
            {trFormat("批量重试失败（最多 {value0} 条）", { value0: BATCH_RETRY_LIMIT })}
          </Button>,
        ]}
        request={async (params) => {
          const bid =
            typeof params.batchId === 'string' && params.batchId.trim()
              ? params.batchId.trim()
              : batchIdFromUrl;
          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            batchId: bid,
            shopId: (params.shopId as string | undefined)?.trim(),
            productId: (params.productId as string | undefined)?.trim(),
            productSkuId:
              (params.productSkuId as string | undefined)?.trim() || skuIdFromUrl,
            platform: (params.platform as string | undefined)?.trim(),
            status:
              (params.status as string | undefined)?.trim() || statusFromUrl,
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          };
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              batchId: qp.batchId,
              shopId: qp.shopId,
              productSkuId: qp.productSkuId,
              platform: qp.platform,
              status: qp.status,
              syncStatus: qp.status,
              source: urlState.source,
              drawer: urlState.drawer,
              id: urlState.id,
            },
            { replace: true },
          );
          const res = await queryInventorySyncTasks({
            page: qp.page,
            pageSize: qp.pageSize,
            batchId: qp.batchId,
            shopId: qp.shopId,
            productId: qp.productId,
            productSkuId: qp.productSkuId,
            platform: qp.platform,
            status: qp.status,
            start: qp.start,
            end: qp.end,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />

      <Drawer
        width={560}
        title={detail ? `${tr("库存同步")} ${detail.id}` : tr("详情")}
        open={detailOpen}
        destroyOnHidden
        onClose={closeTaskDetail}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Typography.Text strong>{tr("状态：")}</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{tr("店铺：")}</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{tr("商品：")}</Typography.Text> {detail.productTitle || detail.productId}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>SKU：</Typography.Text> {detail.skuCode || detail.productSkuId || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{tr("目标库存：")}</Typography.Text> {detail.targetStock}
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text strong>{tr("任务编号：")}</Typography.Text> {detail.id}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph>
                <Typography.Text strong>{tr("失败原因：")}</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
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
