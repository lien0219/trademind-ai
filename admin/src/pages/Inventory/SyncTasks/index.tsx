import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { Button, Drawer, Space, Tag, Typography, Alert, message } from 'antd';
import { confirmFailureTaskRetry } from '@/constants/sensitiveActions';
import { formatDateTime } from '@/utils/formatTime';
import dayjs from 'dayjs';
import { useLocation } from '@umijs/max';
import { useEffect, useMemo, useRef, useState } from 'react';
import InventorySyncDisabledBanner from '@/components/inventory/InventorySyncDisabledBanner';
import {
  INVENTORY_SKU_AMBIGUOUS_MESSAGE,
  INVENTORY_SKU_NOT_BOUND_MESSAGE,
} from '@/constants/inventoryLabels';
import { PAGE_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { platformLabel } from '@/constants/userFriendly';
import {
  getInventorySyncTask,
  queryInventorySyncTasks,
  retryInventorySyncTask,
  retryInventorySyncTasksBatch,
  type InventorySyncTaskDTO,
} from '@/services/inventory';

function tagFromStatus(raw: string) {
  const c = COLLECT_TASK_STATUS[raw as keyof typeof COLLECT_TASK_STATUS];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color}>{c.text}</Tag>;
}

const BATCH_RETRY_LIMIT = 100;

export default function InventorySyncTasksPage() {
  const emptyLocale = useListEmptyLocale('inventorySyncTasks', { permissionScoped: true });
  const location = useLocation();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const batchIdFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('batchId')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);

  const taskIdFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('id')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);

  const skuIdFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('productSkuId')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);

  const statusFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('status')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);

  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<InventorySyncTaskDTO | null>(null);
  const [failedSelectedIds, setFailedSelectedIds] = useState<string[]>([]);

  useEffect(() => {
    if (!batchIdFromUrl) return;
    formRef.current?.setFieldsValue?.({ batchId: batchIdFromUrl });
    actionRef.current?.reload?.();
  }, [batchIdFromUrl]);

  useEffect(() => {
    if (!statusFromUrl) return;
    formRef.current?.setFieldsValue?.({ status: statusFromUrl });
    actionRef.current?.reload?.();
  }, [statusFromUrl]);

  useEffect(() => {
    if (!skuIdFromUrl) return;
    formRef.current?.setFieldsValue?.({ productSkuId: skuIdFromUrl });
    actionRef.current?.reload?.();
  }, [skuIdFromUrl]);

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

  const columns: ProColumns<InventorySyncTaskDTO>[] = useMemo(
    () => [
      {
        title: '批次 ID',
        dataIndex: 'batchId',
        hideInTable: true,
      },
      {
        title: '创建时间范围',
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
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '商品 ID',
        dataIndex: 'productId',
        hideInTable: true,
      },
      {
        title: '规格编号',
        dataIndex: 'productSkuId',
        hideInTable: true,
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        hideInTable: true,
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.shopName || '—',
      },
      {
        title: '商品标题',
        dataIndex: 'productTitle',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => r.productTitle || '—',
      },
      {
        title: '规格编码',
        dataIndex: 'skuCode',
        width: 120,
        search: false,
        ellipsis: true,
        render: (_, r) => r.skuCode || '—',
      },
      {
        title: '批次号',
        dataIndex: 'batchNo',
        width: 132,
        search: false,
        ellipsis: true,
        render: (_, r) => r.batchNo || '—',
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 100,
        render: (_, r) => platformLabel(r.platform),
      },
      {
        title: '目标库存',
        dataIndex: 'targetStock',
        width: 92,
        search: false,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: COLLECT_TASK_STATUS,
        render: (_, r) => tagFromStatus(r.status),
      },
      {
        title: '开始',
        dataIndex: 'startedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.startedAt ? formatDateTime(r.startedAt) : '—'),
      },
      {
        title: '结束',
        dataIndex: 'finishedAt',
        width: 156,
        search: false,
        render: (_, r) => (r.finishedAt ? formatDateTime(r.finishedAt) : '—'),
      },
      {
        title: '错误',
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        render: (_, r) => r.errorMessage || '—',
      },
      {
        title: '操作',
        valueType: 'option',
        width: 140,
        render: (_, r) => (
          <Space>
            <a
              onClick={async () => {
                const d = await getInventorySyncTask(r.id);
                setDetail(d);
                setDetailOpen(true);
              }}
            >
              查看
            </a>
            {r.status === 'failed' || r.status === 'partial_success' ? (
              <Button
                type="link"
                size="small"
                style={{ padding: 0 }}
                onClick={() =>
                  confirmFailureTaskRetry(1, async () => {
                    await retryInventorySyncTask(r.id);
                    message.success('已提交重试');
                    actionRef.current?.reload();
                  })
                }
              >
                重试
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
        message="库存同步前置条件"
        description={
          <>
            {INVENTORY_SKU_NOT_BOUND_MESSAGE} {INVENTORY_SKU_AMBIGUOUS_MESSAGE}
            须开启「启用库存同步」后，可在商品详情 → 库存 Tab 或库存预警页人工发起同步；失败任务支持重试。
          </>
        }
      />
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        TikTok Shop、Shopee、Lazada、抖店已支持库存同步（测试中）；Amazon 仍在规划中。模拟店铺仍走模拟库存同步。
      </Typography.Paragraph>
      <ProTable<InventorySyncTaskDTO>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        headerTitle="任务列表"
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
                message.warning(`单次最多选择 ${BATCH_RETRY_LIMIT} 条失败任务`);
                return;
              }
              confirmFailureTaskRetry(failedSelectedIds.length, async () => {
                try {
                  const batch = await retryInventorySyncTasksBatch(failedSelectedIds);
                  message.success(`已创建批次 ${batch.batchNo}`);
                  setFailedSelectedIds([]);
                  actionRef.current?.reload();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '批量重试失败');
                  throw e;
                }
              });
            }}
          >
            批量重试失败（≤{BATCH_RETRY_LIMIT}）
          </Button>,
        ]}
        request={async (params) => {
          const bid =
            typeof params.batchId === 'string' && params.batchId.trim()
              ? params.batchId.trim()
              : batchIdFromUrl;
          const res = await queryInventorySyncTasks({
            page: params.current,
            pageSize: params.pageSize,
            batchId: bid,
            shopId: params.shopId as string | undefined,
            productId: params.productId as string | undefined,
            productSkuId: params.productSkuId as string | undefined,
            platform: params.platform as string | undefined,
            status: params.status as string | undefined,
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          });
          return { data: res.list, total: res.pagination.total, success: true };
        }}
      />

      <Drawer
        width={560}
        title={detail ? `库存同步 ${detail.id}` : '详情'}
        open={detailOpen}
        destroyOnHidden
        onClose={() => {
          setDetailOpen(false);
          setDetail(null);
        }}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div>
              <Typography.Text strong>状态：</Typography.Text> {tagFromStatus(detail.status)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>店铺：</Typography.Text> {detail.shopName || detail.shopId}{' '}
              <Typography.Text type="secondary">({detail.platform})</Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>商品：</Typography.Text> {detail.productTitle || detail.productId}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>SKU：</Typography.Text> {detail.skuCode || detail.productSkuId || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>目标库存：</Typography.Text> {detail.targetStock}
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: detail.id }}>
              <Typography.Text strong>任务编号：</Typography.Text> {detail.id}
            </Typography.Paragraph>
            {detail.errorMessage ? (
              <Typography.Paragraph>
                <Typography.Text strong>失败原因：</Typography.Text> {detail.errorMessage}
              </Typography.Paragraph>
            ) : null}
            <TechnicalDetails>
              <TaskJsonBlock title="任务输入" value={detail.input} />
              <TaskJsonBlock title="任务输出" value={detail.output} last />
            </TechnicalDetails>
          </Space>
        )}
      </Drawer>
    </TmPageContainer>
  );
}
