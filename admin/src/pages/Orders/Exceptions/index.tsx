import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import { confirmSkuManualBind } from '@/constants/sensitiveActions';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Col,
  Drawer,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tag,
  Typography,
  Descriptions,
  message,
} from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { OrderExceptionRow, OrderExceptionSummary } from '@/services/orderExceptions';
import {
  deleteOrderExceptionMark,
  postOrderExceptionBindSku,
  postOrderExceptionHandle,
  postOrderExceptionIgnore,
  postOrderExceptionRetryDeduct,
  postOrderExceptionRetryInventorySync,
  queryOrderExceptions,
} from '@/services/orderExceptions';
import { getOrderItemSkuCandidates, type SkuCandidateRow } from '@/services/skuCandidates';
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';
import { queryShops } from '@/services/shops';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { parsePositiveInt, queryTimeRange } from '@/utils/urlState';

const EXCEPTION_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'exceptionType',
  'platform',
  'shopId',
  'status',
  'severity',
  'source',
  'orderId',
  'start',
  'end',
] as const;

const EX_TYPES: Record<string, { source: string }> = {
  sku_unmatched: { source: "规格未匹配" },
  sku_ambiguous: { source: "规格多候选" },
  insufficient_stock: { source: "库存不足" },
  inventory_deduct_failed: { source: "扣库存失败" },
  inventory_restore_failed: { source: "恢复库存失败" },
  inventory_sync_failed: { source: "库存同步失败" },
  order_sync_partial_failed: { source: "页级同步失败" },
  missing_order_item: { source: "缺明细" },
  unknown: { source: "未知" },
};

const SEV_LABEL: Record<string, string> = {
  critical: '紧急',
  high: '高',
  medium: '中',
  low: '低',
};

function formatSkuAttrs(attrs: unknown): string {
  if (!attrs || typeof attrs !== 'object') return '—';
  const entries = Object.entries(attrs as Record<string, unknown>);
  if (!entries.length) return '—';
  return entries.map(([k, v]) => `${k}: ${String(v ?? '')}`).join(' · ');
}

function exceptionDetailContent(row: OrderExceptionRow) {
  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <Descriptions column={1} size="small" bordered>
        <Descriptions.Item label={tr("异常类型")}>
          {EX_TYPES[row.exceptionType] ? tr(EX_TYPES[row.exceptionType].source) : row.exceptionType || '—'}
        </Descriptions.Item>
        <Descriptions.Item label={tr("严重程度")}>
          {SEV_LABEL[row.severity] ? tr(SEV_LABEL[row.severity]) : row.severity || '—'}
        </Descriptions.Item>
        <Descriptions.Item label={tr("处理状态")}>{row.status || '—'}</Descriptions.Item>
        <Descriptions.Item label={tr("订单编号")}>{row.orderNo || row.orderId || '—'}</Descriptions.Item>
        <Descriptions.Item label={tr("错误说明")}>{row.errorMessage || '—'}</Descriptions.Item>
        <Descriptions.Item label={tr("建议操作")}>{row.suggestedAction || '—'}</Descriptions.Item>
      </Descriptions>
      <TechnicalDetails>
        <TaskJsonBlock
          title={tr("完整记录")}
          value={{
            exceptionType: row.exceptionType,
            severity: row.severity,
            status: row.status,
            sourceType: row.sourceType,
            sourceId: row.sourceId,
            orderId: row.orderId,
            errorMessage: row.errorMessage,
            suggestedAction: row.suggestedAction,
          }}
          last
        />
      </TechnicalDetails>
    </Space>
  );
}

function sevColor(s: string) {
  switch (s) {
    case 'critical':
      return 'red';
    case 'high':
      return 'orange';
    case 'medium':
      return 'gold';
    default:
      return 'blue';
  }
}

function candTrustBadge(conf: number) {
  if (conf >= 90) return <Tag color="green">{tr("高可信")}</Tag>;
  if (conf >= 70) return <Tag color="gold">{tr("中可信")}</Tag>;
  if (conf >= 40) return <Tag>{tr("低可信")}</Tag>;
  return <Tag color="default">{tr("参考")}</Tag>;
}

export default function OrderExceptionsPage() {
  const language = useLanguage();
  const emptyLocale = useListEmptyLocale('orderExceptions', { permissionScoped: true });
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof EXCEPTION_QUERY_KEYS)[number], string | undefined>>(
      EXCEPTION_QUERY_KEYS,
    );
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
  const [summary, setSummary] = useState<OrderExceptionSummary | null>(null);
  const [shopOpts, setShopOpts] = useState<{ label: string; value: string }[]>([]);

  const [bindOpen, setBindOpen] = useState(false);
  const [bindRow, setBindRow] = useState<OrderExceptionRow | null>(null);
  const [skuKw, setSkuKw] = useState('');
  const [skuHits, setSkuHits] = useState<ProductSkuSearchHit[]>([]);
  const [pickedSku, setPickedSku] = useState<string>();
  const [pickedCandMeta, setPickedCandMeta] = useState<{ confidence: number; source: string } | null>(
    null,
  );
  const [deduct, setDeduct] = useState(true);
  const [syncPlat, setSyncPlat] = useState(false);
  const [candLoading, setCandLoading] = useState(false);
  const [candRows, setCandRows] = useState<SkuCandidateRow[]>([]);
  const [candModalOpen, setCandModalOpen] = useState(false);
  const [candModalRows, setCandModalRows] = useState<SkuCandidateRow[]>([]);
  const [candModalTitle, setCandModalTitle] = useState('');

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 500 });
        setShopOpts(res.list.map((s) => ({ label: `${s.shopName} (${s.platform})`, value: s.id })));
      } catch {
        /* ignore */
      }
    })();
  }, []);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword,
      exceptionType: urlState.exceptionType,
      platform: urlState.platform,
      shopId: urlState.shopId,
      status: urlState.status,
      severity: urlState.severity,
      orderId: urlState.orderId,
      createdAt: queryTimeRange(urlState.start, urlState.end),
    });
    actionRef.current?.reload();
  }, [
    urlState.end,
    urlState.exceptionType,
    urlState.keyword,
    urlState.orderId,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.severity,
    urlState.shopId,
    urlState.start,
    urlState.status,
  ]);

  const reload = useCallback(() => {
    actionRef.current?.reload();
  }, []);

  const openBind = useCallback((row: OrderExceptionRow) => {
    setBindRow(row);
    setSkuKw('');
    setSkuHits([]);
    setPickedSku(undefined);
    setPickedCandMeta(null);
    setCandRows([]);
    setDeduct(true);
    setSyncPlat(false);
    setBindOpen(true);
  }, []);

  const refreshDrawerCandidates = useCallback(async (orderItemId: string) => {
    setCandLoading(true);
    try {
      const r = await getOrderItemSkuCandidates(orderItemId, { limit: 10 });
      setCandRows(r.list ?? []);
    } catch {
      message.error(tr("候选加载失败"));
      setCandRows([]);
    } finally {
      setCandLoading(false);
    }
  }, [language]);

  const openCandModalOnly = useCallback(async (row: OrderExceptionRow) => {
    if (!row.orderItemId) {
      message.warning(tr("缺少明细行 ID，可到订单「规格匹配」页查看候选"));
      return;
    }
    setCandModalTitle(row.orderNo || row.orderItemId || '候选');
    setCandModalOpen(true);
    try {
      const r = await getOrderItemSkuCandidates(row.orderItemId, { limit: 15 });
      setCandModalRows(r.list ?? []);
    } catch {
      message.error(tr("加载候选失败"));
      setCandModalRows([]);
    }
  }, [language]);

  useEffect(() => {
    const oid = bindRow?.orderItemId;
    if (!bindOpen || !oid || !bindRow) return;
    const et = bindRow.exceptionType;
    if (et !== 'sku_unmatched' && et !== 'sku_ambiguous') return;
    void refreshDrawerCandidates(oid);
  }, [bindOpen, bindRow?.orderItemId, bindRow?.exceptionType, refreshDrawerCandidates]);

  const runSkuSearch = async () => {
    try {
      const r = await searchProductSkus({ keyword: skuKw.trim(), limit: 30 });
      setSkuHits(r.list ?? []);
    } catch (e: unknown) {
      message.error(tr((e as Error)?.message || tr('搜索失败')));
    }
  };

  const pickedHit = useMemo(
    () => skuHits.find((h) => h.productSkuId === pickedSku),
    [skuHits, pickedSku],
  );

  const maxCandConf = useMemo(
    () => (candRows.length ? candRows.reduce((m, x) => Math.max(m, x.confidence), 0) : 0),
    [candRows],
  );

  const columns: ProColumns<OrderExceptionRow>[] = useMemo(
    () => [
      {
        title: tr("创建时间"),
        dataIndex: 'createdAt',
        width: 168,
        valueType: 'dateTimeRange',
        hideInTable: true,
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: tr("异常类型"),
        dataIndex: 'exceptionType',
        width: 138,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(EX_TYPES).map(([key, meta]) => [key, { text: tr(meta.source) }]),
        ),
      },
      {
        title: tr("严重程度"),
        dataIndex: 'severity',
        width: 100,
        valueType: 'select',
        valueEnum: {
          low: { text: tr("低") },
          medium: { text: tr("中") },
          high: { text: tr("高") },
          critical: { text: tr("紧急") },
        },
        render: (_, r) => <Tag color={sevColor(r.severity)}>{r.severity}</Tag>,
      },
      {
        title: tr("视图状态"),
        dataIndex: 'status',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          open: { text: tr("未处理（默认）") },
          handled: { text: tr("已处理（标记）") },
          ignored: { text: tr("已忽略（标记）") },
        },
      },
      {
        title: tr("平台"),
        dataIndex: 'platform',
        width: 96,
      },
      {
        title: tr("店铺"),
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'select',
        fieldProps: { options: shopOpts, allowClear: true, showSearch: true },
      },
      {
        title: tr("订单"),
        dataIndex: 'orderId',
        hideInTable: true,
      },
      {
        title: tr("关键词"),
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: keywordFieldProps,
      },
      {
        title: tr("店铺"),
        dataIndex: 'shopName',
        search: false,
        width: 140,
        ellipsis: true,
      },
      {
        title: tr("订单号"),
        dataIndex: 'orderNo',
        search: false,
        width: 132,
        ellipsis: true,
      },
      {
        title: tr("外部单号"),
        dataIndex: 'externalOrderId',
        search: false,
        width: 132,
        ellipsis: true,
      },
      {
        title: tr("平台规格编码"),
        key: 'skuCol',
        search: false,
        width: 120,
        ellipsis: true,
        render: (_, r) => r.skuCode || r.externalSkuId || '—',
      },
      {
        title: tr("本地商品/规格"),
        key: 'localSku',
        search: false,
        width: 160,
        ellipsis: true,
        render: (_, r) =>
          [r.productTitle, r.localSkuCode || r.productSkuId].filter(Boolean).join(' · ') || '—',
      },
      {
        title: tr("数量"),
        dataIndex: 'quantity',
        search: false,
        width: 64,
      },
      {
        title: tr("错误信息"),
        dataIndex: 'errorMessage',
        search: false,
        ellipsis: true,
      },
      {
        title: tr("建议动作"),
        dataIndex: 'suggestedAction',
        search: false,
        ellipsis: true,
      },
      {
        title: tr("标记"),
        dataIndex: 'status',
        search: false,
        width: 96,
        render: (_, r) =>
          r.handled ? <Tag color="success">{tr("已处理")}</Tag> : r.ignored ? <Tag>{tr("已忽略")}</Tag> : <Tag color="processing">{tr("待处理")}</Tag>,
      },
      {
        title: tr("创建时间"),
        dataIndex: 'createdAt',
        search: false,
        width: 156,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: tr("操作"),
        valueType: 'option',
        width: 360,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size={4}>
            {r.orderId ? (
              <a
                onClick={() => {
                  history.push(`/orders/${encodeURIComponent(r.orderId!)}`);
                }}
              >
                {tr("订单")}
              </a>
            ) : null}
            {r.taskCenterUrl ? (
              <a onClick={() => history.push(r.taskCenterUrl!)}>{tr("失败任务")}</a>
            ) : r.orderId ? (
              <a
                onClick={() =>
                  history.push(`/ops/task-center/failures?taskType=order_sync&keyword=${encodeURIComponent(r.orderId!)}`)
                }
              >
                {tr("失败任务")}
              </a>
            ) : null}
            {r.syncTaskId ? (
              <a
                onClick={() =>
                  history.push(`/orders/sync-tasks?id=${encodeURIComponent(r.syncTaskId!)}`)
                }
              >
                {tr("同步任务")}
              </a>
            ) : null}
            {(r.exceptionType === 'sku_unmatched' || r.exceptionType === 'sku_ambiguous') && (
              <>
                <a onClick={() => void openCandModalOnly(r)}>{tr("查看候选")}</a>
                <a onClick={() => openBind(r)}>{tr("绑定 SKU（候选）")}</a>
              </>
            )}
            {r.orderId &&
              (r.exceptionType === 'insufficient_stock' ||
                r.exceptionType === 'inventory_deduct_failed' ||
                r.exceptionType === 'sku_unmatched') && (
                <Popconfirm
                  title={tr("对该订单再次尝试扣减库存？")}
                  onConfirm={async () => {
                    try {
                      await postOrderExceptionRetryDeduct(r.sourceType, r.sourceId, false);
                      message.success(tr("已触发扣减"));
                      reload();
                    } catch (e: unknown) {
                      message.error(tr((e as Error)?.message || tr('失败')));
                    }
                  }}
                >
                  <a>{tr("重试扣库存")}</a>
                </Popconfirm>
              )}
            {r.exceptionType === 'inventory_sync_failed' && (
              <Popconfirm
                title={tr("重试该库存同步任务？")}
                onConfirm={async () => {
                  try {
                    await postOrderExceptionRetryInventorySync(r.sourceType, r.sourceId);
                    message.success(tr("已重试入队"));
                    reload();
                  } catch (e: unknown) {
                    message.error(tr((e as Error)?.message || tr('失败')));
                  }
                }}
              >
                <a>{tr("重试同步")}</a>
              </Popconfirm>
            )}
            <a
              onClick={() => {
                Modal.info({
                  title: tr("异常详情"),
                  width: 640,
                  content: exceptionDetailContent(r),
                });
              }}
            >
              {tr("详情")}
            </a>
            <a
              onClick={() => {
                let remark = '';
                Modal.confirm({
                  title: tr("标记已处理"),
                  content: (
                    <Input.TextArea
                      rows={3}
                      placeholder={tr("备注（可选）")}
                      onChange={(e) => {
                        remark = e.target.value;
                      }}
                    />
                  ),
                  onOk: async () => {
                    await postOrderExceptionHandle(r.sourceType, r.sourceId, {
                      exceptionType: r.exceptionType,
                      remark: remark.trim(),
                    });
                    message.success(tr("已标记"));
                    reload();
                  },
                });
              }}
            >
              {tr("已处理")}
            </a>
            <a
              onClick={() => {
                Modal.confirm({
                  title: tr("忽略该异常（工作台视图）"),
                  onOk: async () => {
                    await postOrderExceptionIgnore(r.sourceType, r.sourceId, { exceptionType: r.exceptionType });
                    message.success(tr("已忽略"));
                    reload();
                  },
                });
              }}
            >
              {tr("忽略")}
            </a>
            <Popconfirm
              title={tr("取消标记并回到待处理列表？")}
              onConfirm={async () => {
                await deleteOrderExceptionMark(r.sourceType, r.sourceId);
                message.success(tr("已取消标记"));
                reload();
              }}
            >
              <a>{tr("取消标记")}</a>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [language, reload, shopOpts, openCandModalOnly, openBind, keywordFieldProps],
  );

  return (
    <TmPageContainer
      title={tr("订单异常工作台")}
      subTitle={tr("处理订单同步与规格匹配中的异常情况。")}
    >
      <Typography.Paragraph type="secondary">
        {tr("聚合规格未匹配、扣库存失败与库存同步失败等需人工处理的问题；标记仅影响本列表视图，不改订单与任务原始状态。\r\n        抖店订单同步后若 SKU 未绑定，请在此绑定本地规格后再扣库存或同步抖店库存。")}
      </Typography.Paragraph>
      <KeywordSafetyHint visible={showSensitiveHint} />
      {summary ? (
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title={tr("未处理总数")} value={summary.totalOpen} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title={tr("规格未匹配")} value={summary.skuUnmatched} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title={tr("规格多候选")} value={summary.skuAmbiguous} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title={tr("库存不足")} value={summary.insufficientStock} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title={tr("扣库存失败")} value={summary.inventoryDeductFailed} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title={tr("库存同步失败")} value={summary.inventorySyncFailed} />
            </Card>
          </Col>
        </Row>
      ) : null}

      <ProTable<OrderExceptionRow>
        rowKey={(r) => `${r.exceptionType}-${r.sourceType}-${r.sourceId}`}
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        params={{
          current: tablePage,
          pageSize: tablePageSize,
          keyword: urlState.keyword,
          exceptionType: urlState.exceptionType,
          severity: urlState.severity,
          platform: urlState.platform,
          shopId: urlState.shopId,
          orderId: urlState.orderId,
          status: urlState.status,
          start: urlState.start,
          end: urlState.end,
        }}
        search={{ layout: 'vertical', defaultCollapsed: false }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(EXCEPTION_QUERY_KEYS, { replace: true });
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
        locale={emptyLocale}
        request={async (params) => {
          let handled: boolean | undefined;
          let ignored: boolean | undefined;
          const st = params.status as string | undefined;
          if (st === 'handled') handled = true;
          else if (st === 'ignored') ignored = true;

          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            exceptionType: (params.exceptionType as string | undefined)?.trim(),
            severity: (params.severity as string | undefined)?.trim(),
            platform: (params.platform as string | undefined)?.trim(),
            shopId: (params.shopId as string | undefined)?.trim(),
            orderId: (params.orderId as string | undefined)?.trim(),
            keyword: prepareKeyword(params.keyword),
            start: typeof params.start === 'string' ? params.start : undefined,
            end: typeof params.end === 'string' ? params.end : undefined,
          };
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              keyword: qp.keyword,
              exceptionType: qp.exceptionType,
              severity: qp.severity,
              platform: qp.platform,
              shopId: qp.shopId,
              status: st,
              orderId: qp.orderId,
              start: qp.start,
              end: qp.end,
              source: urlState.source,
            },
            { replace: true },
          );

          const res = await queryOrderExceptions({
            page: qp.page,
            pageSize: qp.pageSize,
            exceptionType: qp.exceptionType,
            severity: qp.severity,
            platform: qp.platform,
            shopId: qp.shopId,
            orderId: qp.orderId,
            keyword: qp.keyword,
            handled,
            ignored,
            start: qp.start,
            end: qp.end,
          });
          setSummary(res.summary);
          return { data: res.list, total: res.total, success: true };
        }}
      />

      <Drawer
        title={tr("绑定本地商品规格")}
        width={640}
        open={bindOpen}
        onClose={() => setBindOpen(false)}
        destroyOnHidden
        footer={
          <Space>
            <Button onClick={() => setBindOpen(false)}>{tr("取消")}</Button>
            <Button
              type="primary"
              onClick={() => {
                if (!bindRow || !pickedSku) {
                  message.warning(tr("请选择本地商品规格"));
                  return;
                }
                confirmSkuManualBind(async () => {
                  try {
                    const out = await postOrderExceptionBindSku(bindRow.sourceType, bindRow.sourceId, {
                      exceptionType: bindRow.exceptionType,
                      productSkuId: pickedSku,
                      deductInventory: deduct,
                      syncInventory: syncPlat,
                      autoMarkHandled: true,
                      candidateConfidence: pickedCandMeta?.confidence,
                      candidateSource: pickedCandMeta?.source,
                    });
                    if (out.inventoryDeductionError) {
                      message.error(tr(out.inventoryDeductionError));
                    } else {
                      message.success(tr("处理完成"));
                    }
                    setBindOpen(false);
                    reload();
                  } catch (e: unknown) {
                    message.error(tr((e as Error)?.message || tr('失败')));
                  }
                });
              }}
            >
              {tr("确认")}
            </Button>
          </Space>
        }
      >
        {bindRow ? (
          <>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message={
                <span>
                  {tr("订单")} {bindRow.orderNo || bindRow.orderId || '—'} {tr("· 平台")} {bindRow.platform || '—'} {tr("· 平台规格")}{' '}
                  {bindRow.skuCode || bindRow.externalSkuId || '—'}
                </span>
              }
            />
            <Typography.Paragraph type="secondary">
              {tr("匹配状态：")}{EX_TYPES[bindRow.exceptionType]?.source ? tr(EX_TYPES[bindRow.exceptionType].source) : bindRow.exceptionType} ·{' '}
              {bindRow.suggestedAction || '—'}
            </Typography.Paragraph>
            <Typography.Title level={5}>{tr("候选推荐（只读 · 不自动绑定）")}</Typography.Title>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
              {tr("默认不选中；最高分行已浅绿高亮。点「选择该候选」后再用底部「确认」完成绑定与库存动作（二次确认）。")}
            </Typography.Paragraph>
            <Table
              loading={candLoading}
              size="small"
              pagination={false}
              style={{ marginBottom: 16 }}
              dataSource={candRows}
              rowKey={(r) => r.productSkuId}
              onRow={(r) => ({
                style:
                  candRows.length > 1 && r.confidence === maxCandConf
                    ? { background: 'var(--ant-color-success-bg)' }
                    : undefined,
              })}
              columns={[
                {
                  title: tr("推荐分"),
                  dataIndex: 'confidence',
                  width: 120,
                  render: (v: number) => (
                    <Space size={4} wrap>
                      <Typography.Text strong>{v}</Typography.Text>
                      {candTrustBadge(v)}
                    </Space>
                  ),
                },
                { title: tr("原因 / 信号"), key: 'rs', ellipsis: true, render: (_, r) => `${r.reason || '—'} ${(r.matchSignals || []).join(',')}` },
                { title: tr("商品标题"), dataIndex: 'productTitle', width: 140, ellipsis: true },
                { title: tr("规格编码"), dataIndex: 'skuCode', width: 112, ellipsis: true },
                { title: tr("规格名称"), dataIndex: 'skuName', width: 112, ellipsis: true },
                { title: tr("库存"), dataIndex: 'stock', width: 72, render: (v: number | undefined) => v ?? '—' },
                {
                  title: tr("操作"),
                  key: 'pick',
                  width: 112,
                  render: (_, r) => (
                    <Button
                      size="small"
                      type={pickedSku === r.productSkuId ? 'primary' : 'default'}
                      onClick={() => {
                        setPickedSku(r.productSkuId);
                        setPickedCandMeta({ confidence: r.confidence, source: r.source });
                        message.success(tr("已选为绑定目标"));
                      }}
                    >
                      {tr("选择该候选")}
                    </Button>
                  ),
                },
              ]}
              locale={{ emptyText: candLoading ? tr('加载中…') : tr('暂无候选（可尝试勾选「低置信」查询参数或改用下方手动搜索）') }}
            />

            <Typography.Title level={5}>{tr("手动搜索")}</Typography.Title>
            <Space wrap style={{ marginBottom: 8 }}>
              <Input.Search
                placeholder={tr("搜索本地商品规格 / 商品")}
                style={{ width: 280 }}
                value={skuKw}
                onChange={(e) => setSkuKw(e.target.value)}
                onSearch={() => void runSkuSearch()}
              />
              <Button type="primary" onClick={() => void runSkuSearch()}>
                {tr("搜索")}
              </Button>
            </Space>
            <Select
              style={{ width: '100%', marginBottom: 16 }}
              placeholder={tr("选择商品规格")}
              value={pickedSku}
              onChange={(v) => {
                setPickedSku(v);
                setPickedCandMeta(null);
              }}
              options={skuHits.map((h) => ({
                value: h.productSkuId,
                label: `${h.skuCode || '—'} · ${h.productTitle || ''} · stock=${h.stock ?? '?'}`,
              }))}
              showSearch={false}
            />
            {pickedHit ? (
              <>
                <Typography.Paragraph style={{ marginBottom: 16 }} type="secondary">
                  <Typography.Text strong>{tr("已选：")}</Typography.Text> {pickedHit.productTitle || '—'} ·{' '}
                  {pickedHit.skuCode || pickedHit.productSkuId}
                  {pickedHit.skuName ? `（${pickedHit.skuName}）` : ''}
                  <br />
                  {tr("库存：")}{pickedHit.stock ?? '—'}
                  {pickedHit.attrs != null ? (
                    <>
                      <br />
                      {tr("规格属性：")}{formatSkuAttrs(pickedHit.attrs)}
                    </>
                  ) : null}
                </Typography.Paragraph>
                {pickedHit.attrs != null ? (
                  <TechnicalDetails label={tr("规格属性详情")}>
                    <TaskJsonBlock title={tr("规格属性")} value={pickedHit.attrs} last />
                  </TechnicalDetails>
                ) : null}
              </>
            ) : null}
            <Space direction="vertical">
              <Space>
                <span>{tr("绑定后扣减库存")}</span>
                <Switch checked={deduct} onChange={setDeduct} />
              </Space>
              <Space>
                <span>{tr("扣减后同步平台库存任务")}</span>
                <Switch checked={syncPlat} onChange={setSyncPlat} />
              </Space>
            </Space>
          </>
        ) : null}
      </Drawer>

      <Modal
        title={trFormat("查看候选 · {value0}", { value0: tr(candModalTitle) })}
        width={900}
        open={candModalOpen}
        footer={null}
        onCancel={() => setCandModalOpen(false)}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {tr("仅浏览：不修改数据。需绑定时请点「绑定 SKU（候选）」进入抽屉操作。")}
        </Typography.Paragraph>
        <Table
          size="small"
          pagination={false}
          dataSource={candModalRows}
          rowKey={(r) => r.productSkuId}
          columns={[
            {
              title: tr("推荐分"),
              dataIndex: 'confidence',
              width: 120,
              render: (v: number) => (
                <Space wrap>
                  <Typography.Text strong>{v}</Typography.Text>
                  {candTrustBadge(v)}
                </Space>
              ),
            },
            { title: tr("原因"), dataIndex: 'reason', ellipsis: true },
            { title: tr("来源"), dataIndex: 'source', width: 160, ellipsis: true },
            { title: tr("商品标题"), dataIndex: 'productTitle', width: 160, ellipsis: true },
            { title: tr("规格编码"), dataIndex: 'skuCode', width: 120 },
            { title: tr("规格名称"), dataIndex: 'skuName', width: 120, ellipsis: true },
            { title: tr("库存"), dataIndex: 'stock', width: 72, render: (v: number | undefined) => v ?? '—' },
            {
              title: tr("信号"),
              dataIndex: 'matchSignals',
              ellipsis: true,
              render: (s: string[]) => (s?.length ? s.join(' · ') : '—'),
            },
          ]}
        />
      </Modal>
    </TmPageContainer>
  );
}
