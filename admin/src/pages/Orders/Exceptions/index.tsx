import { PageContainer, ProTable, type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { history, useLocation } from '@umijs/max';
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
  Tag,
  Typography,
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
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';
import { queryShops } from '@/services/shops';

const EX_TYPES: Record<string, { text: string }> = {
  sku_unmatched: { text: '未匹配 SKU' },
  sku_ambiguous: { text: 'SKU 多候选' },
  insufficient_stock: { text: '库存不足' },
  inventory_deduct_failed: { text: '扣库存失败' },
  inventory_restore_failed: { text: '恢复库存失败' },
  inventory_sync_failed: { text: '库存同步失败' },
  order_sync_partial_failed: { text: '订单同步部分失败' },
  missing_order_item: { text: '缺明细' },
  unknown: { text: '未知' },
};

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

export default function OrderExceptionsPage() {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const { search: locSearch } = useLocation();
  const [summary, setSummary] = useState<OrderExceptionSummary | null>(null);
  const [shopOpts, setShopOpts] = useState<{ label: string; value: string }[]>([]);

  const [bindOpen, setBindOpen] = useState(false);
  const [bindRow, setBindRow] = useState<OrderExceptionRow | null>(null);
  const [skuKw, setSkuKw] = useState('');
  const [skuHits, setSkuHits] = useState<ProductSkuSearchHit[]>([]);
  const [pickedSku, setPickedSku] = useState<string>();
  const [deduct, setDeduct] = useState(true);
  const [syncPlat, setSyncPlat] = useState(false);

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
    const sp = new URLSearchParams(locSearch);
    const oid = sp.get('orderId')?.trim();
    const et = sp.get('exceptionType')?.trim();
    if (!oid && !et) return;
    formRef.current?.setFieldsValue({
      ...(oid ? { orderId: oid } : {}),
      ...(et ? { exceptionType: et } : {}),
    });
    actionRef.current?.reload();
  }, [locSearch]);

  const reload = useCallback(() => {
    actionRef.current?.reload();
  }, []);

  const openBind = (row: OrderExceptionRow) => {
    setBindRow(row);
    setSkuKw('');
    setSkuHits([]);
    setPickedSku(undefined);
    setDeduct(true);
    setSyncPlat(false);
    setBindOpen(true);
  };

  const runSkuSearch = async () => {
    try {
      const r = await searchProductSkus({ keyword: skuKw.trim(), limit: 30 });
      setSkuHits(r.list ?? []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '搜索失败');
    }
  };

  const pickedHit = useMemo(
    () => skuHits.find((h) => h.productSkuId === pickedSku),
    [skuHits, pickedSku],
  );

  const columns: ProColumns<OrderExceptionRow>[] = useMemo(
    () => [
      {
        title: '创建时间',
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
        title: '异常类型',
        dataIndex: 'exceptionType',
        width: 138,
        valueType: 'select',
        valueEnum: EX_TYPES,
      },
      {
        title: '严重程度',
        dataIndex: 'severity',
        width: 100,
        valueType: 'select',
        valueEnum: {
          low: { text: '低' },
          medium: { text: '中' },
          high: { text: '高' },
          critical: { text: '紧急' },
        },
        render: (_, r) => <Tag color={sevColor(r.severity)}>{r.severity}</Tag>,
      },
      {
        title: '视图状态',
        dataIndex: 'status',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          open: { text: '未处理（默认）' },
          handled: { text: '已处理（标记）' },
          ignored: { text: '已忽略（标记）' },
        },
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 96,
      },
      {
        title: '店铺',
        dataIndex: 'shopId',
        hideInTable: true,
        valueType: 'select',
        fieldProps: { options: shopOpts, allowClear: true, showSearch: true },
      },
      {
        title: '订单',
        dataIndex: 'orderId',
        hideInTable: true,
      },
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        search: false,
        width: 140,
        ellipsis: true,
      },
      {
        title: '订单号',
        dataIndex: 'orderNo',
        search: false,
        width: 132,
        ellipsis: true,
      },
      {
        title: '外部单号',
        dataIndex: 'externalOrderId',
        search: false,
        width: 132,
        ellipsis: true,
      },
      {
        title: '外部 SKU',
        key: 'skuCol',
        search: false,
        width: 120,
        ellipsis: true,
        render: (_, r) => r.skuCode || r.externalSkuId || '—',
      },
      {
        title: '本地商品/SKU',
        key: 'localSku',
        search: false,
        width: 160,
        ellipsis: true,
        render: (_, r) =>
          [r.productTitle, r.localSkuCode || r.productSkuId].filter(Boolean).join(' · ') || '—',
      },
      {
        title: '数量',
        dataIndex: 'quantity',
        search: false,
        width: 64,
      },
      {
        title: '错误信息',
        dataIndex: 'errorMessage',
        search: false,
        ellipsis: true,
      },
      {
        title: '建议动作',
        dataIndex: 'suggestedAction',
        search: false,
        ellipsis: true,
      },
      {
        title: '标记',
        dataIndex: 'status',
        search: false,
        width: 96,
        render: (_, r) =>
          r.handled ? <Tag color="success">已处理</Tag> : r.ignored ? <Tag>已忽略</Tag> : <Tag color="processing">待处理</Tag>,
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        search: false,
        width: 156,
        render: (_, r) => dayjs(r.createdAt).format('YYYY-MM-DD HH:mm'),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 280,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size={4}>
            {r.orderId ? (
              <a
                onClick={() => {
                  history.push(`/orders?jumpOrder=${encodeURIComponent(r.orderId!)}`);
                }}
              >
                订单
              </a>
            ) : null}
            {(r.exceptionType === 'sku_unmatched' || r.exceptionType === 'sku_ambiguous') && (
              <a onClick={() => openBind(r)}>绑定 SKU</a>
            )}
            {r.orderId &&
              (r.exceptionType === 'insufficient_stock' ||
                r.exceptionType === 'inventory_deduct_failed' ||
                r.exceptionType === 'sku_unmatched') && (
                <Popconfirm
                  title="对该订单再次尝试扣减库存？"
                  onConfirm={async () => {
                    try {
                      await postOrderExceptionRetryDeduct(r.sourceType, r.sourceId, false);
                      message.success('已触发扣减');
                      reload();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '失败');
                    }
                  }}
                >
                  <a>重试扣库存</a>
                </Popconfirm>
              )}
            {r.exceptionType === 'inventory_sync_failed' && (
              <Popconfirm
                title="重试该库存同步任务？"
                onConfirm={async () => {
                  try {
                    await postOrderExceptionRetryInventorySync(r.sourceType, r.sourceId);
                    message.success('已重试入队');
                    reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '失败');
                  }
                }}
              >
                <a>重试同步</a>
              </Popconfirm>
            )}
            <a
              onClick={() => {
                Modal.info({
                  title: '异常详情',
                  width: 720,
                  content: (
                    <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>
                      {JSON.stringify(
                        {
                          exceptionType: r.exceptionType,
                          severity: r.severity,
                          status: r.status,
                          sourceType: r.sourceType,
                          sourceId: r.sourceId,
                          orderId: r.orderId,
                          errorMessage: r.errorMessage,
                          suggestedAction: r.suggestedAction,
                        },
                        null,
                        2,
                      )}
                    </pre>
                  ),
                });
              }}
            >
              详情
            </a>
            <a
              onClick={() => {
                let remark = '';
                Modal.confirm({
                  title: '标记已处理',
                  content: (
                    <Input.TextArea
                      rows={3}
                      placeholder="备注（可选）"
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
                    message.success('已标记');
                    reload();
                  },
                });
              }}
            >
              已处理
            </a>
            <a
              onClick={() => {
                Modal.confirm({
                  title: '忽略该异常（工作台视图）',
                  onOk: async () => {
                    await postOrderExceptionIgnore(r.sourceType, r.sourceId, { exceptionType: r.exceptionType });
                    message.success('已忽略');
                    reload();
                  },
                });
              }}
            >
              忽略
            </a>
            <Popconfirm
              title="取消标记并回到待处理列表？"
              onConfirm={async () => {
                await deleteOrderExceptionMark(r.sourceType, r.sourceId);
                message.success('已取消标记');
                reload();
              }}
            >
              <a>取消标记</a>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [reload, shopOpts],
  );

  return (
    <PageContainer title="订单异常工作台">
      <Typography.Paragraph type="secondary">
        聚合未匹配 SKU、扣库存失败与库存同步失败等需人工处理的问题；标记仅影响本列表视图，不改订单与任务原始状态。
      </Typography.Paragraph>
      {summary ? (
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title="未处理总数" value={summary.totalOpen} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title="未匹配 SKU" value={summary.skuUnmatched} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title="SKU 多候选" value={summary.skuAmbiguous} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title="库存不足" value={summary.insufficientStock} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title="扣库存失败" value={summary.inventoryDeductFailed} />
            </Card>
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Card size="small">
              <Statistic title="库存同步失败" value={summary.inventorySyncFailed} />
            </Card>
          </Col>
        </Row>
      ) : null}

      <ProTable<OrderExceptionRow>
        rowKey={(r) => `${r.exceptionType}-${r.sourceType}-${r.sourceId}`}
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ layout: 'vertical', defaultCollapsed: false }}
        pagination={{ pageSize: 20 }}
        request={async (params) => {
          let handled: boolean | undefined;
          let ignored: boolean | undefined;
          const st = params.status as string | undefined;
          if (st === 'handled') handled = true;
          else if (st === 'ignored') ignored = true;

          const res = await queryOrderExceptions({
            page: params.current,
            pageSize: params.pageSize,
            exceptionType: params.exceptionType as string | undefined,
            severity: params.severity as string | undefined,
            platform: params.platform as string | undefined,
            shopId: params.shopId as string | undefined,
            orderId: params.orderId as string | undefined,
            keyword: params.keyword as string | undefined,
            handled,
            ignored,
            start: params.start as string | undefined,
            end: params.end as string | undefined,
          });
          setSummary(res.summary);
          return { data: res.list, total: res.total, success: true };
        }}
      />

      <Drawer
        title="绑定本地 SKU"
        width={520}
        open={bindOpen}
        onClose={() => setBindOpen(false)}
        destroyOnClose
        footer={
          <Space>
            <Button onClick={() => setBindOpen(false)}>取消</Button>
            <Popconfirm
              title="确认绑定并执行所选库存动作？"
              onConfirm={async () => {
                if (!bindRow || !pickedSku) {
                  message.warning('请选择本地 SKU');
                  return;
                }
                try {
                  const out = await postOrderExceptionBindSku(bindRow.sourceType, bindRow.sourceId, {
                    exceptionType: bindRow.exceptionType,
                    productSkuId: pickedSku,
                    deductInventory: deduct,
                    syncInventory: syncPlat,
                    autoMarkHandled: true,
                  });
                  if (out.inventoryDeductionError) {
                    message.error(out.inventoryDeductionError);
                  } else {
                    message.success('处理完成');
                  }
                  setBindOpen(false);
                  reload();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              <Button type="primary">确认</Button>
            </Popconfirm>
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
                  订单 {bindRow.orderNo || bindRow.orderId || '—'} · 平台 {bindRow.platform || '—'} · 外部 SKU{' '}
                  {bindRow.skuCode || bindRow.externalSkuId || '—'}
                </span>
              }
            />
            <Typography.Paragraph type="secondary">
              匹配状态：{bindRow.exceptionType} · {bindRow.suggestedAction}
            </Typography.Paragraph>
            <Space wrap style={{ marginBottom: 8 }}>
              <Input.Search
                placeholder="搜索本地 SKU / 商品"
                style={{ width: 280 }}
                value={skuKw}
                onChange={(e) => setSkuKw(e.target.value)}
                onSearch={() => void runSkuSearch()}
              />
              <Button type="primary" onClick={() => void runSkuSearch()}>
                搜索
              </Button>
            </Space>
            <Select
              style={{ width: '100%', marginBottom: 16 }}
              placeholder="选择 SKU"
              value={pickedSku}
              onChange={setPickedSku}
              options={skuHits.map((h) => ({
                value: h.productSkuId,
                label: `${h.skuCode || '—'} · ${h.productTitle || ''} · stock=${h.stock ?? '?'}`,
              }))}
              showSearch={false}
            />
            {pickedHit ? (
              <Typography.Paragraph style={{ marginBottom: 16 }} type="secondary">
                <Typography.Text strong>已选：</Typography.Text> {pickedHit.productTitle || '—'} ·{' '}
                {pickedHit.skuCode || pickedHit.productSkuId}
                {pickedHit.skuName ? `（${pickedHit.skuName}）` : ''}
                <br />
                库存：{pickedHit.stock ?? '—'}
                {pickedHit.attrs != null ? (
                  <>
                    <br />
                    属性：<Typography.Text code>{JSON.stringify(pickedHit.attrs)}</Typography.Text>
                  </>
                ) : null}
              </Typography.Paragraph>
            ) : null}
            <Space direction="vertical">
              <Space>
                <span>绑定后扣减库存</span>
                <Switch checked={deduct} onChange={setDeduct} />
              </Space>
              <Space>
                <span>扣减后同步平台库存任务</span>
                <Switch checked={syncPlat} onChange={setSyncPlat} />
              </Space>
            </Space>
          </>
        ) : null}
      </Drawer>
    </PageContainer>
  );
}
