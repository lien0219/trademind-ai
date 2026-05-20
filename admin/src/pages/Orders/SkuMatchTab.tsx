import { Button, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography, Alert, message } from 'antd';
import { history } from '@umijs/max';
import { useCallback, useEffect, useState } from 'react';
import {
  bindOrderItemSku,
  getOrderSKUMatches,
  matchOrderSKUs,
  type OrderSkuMatchRow,
} from '@/services/orders';
import { getOrderItemSkuCandidates, type SkuCandidateRow } from '@/services/skuCandidates';
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';

type Props = {
  orderId: string;
  onRefreshOrder: () => Promise<void>;
};

function candTrustBadge(conf: number) {
  if (conf >= 90) return <Tag color="green">高可信</Tag>;
  if (conf >= 70) return <Tag color="gold">中可信</Tag>;
  if (conf >= 40) return <Tag>低可信</Tag>;
  return <Tag color="default">参考</Tag>;
}

function statusColor(s: string | undefined) {
  switch (s) {
    case 'matched':
      return 'success';
    case 'manual_bound':
      return 'processing';
    case 'ambiguous':
      return 'warning';
    case 'unmatched':
      return 'error';
    case 'skipped':
      return 'default';
    default:
      return 'default';
  }
}

export default function OrderSkuMatchTab({ orderId, onRefreshOrder }: Props) {
  const [rows, setRows] = useState<OrderSkuMatchRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [matchLoading, setMatchLoading] = useState(false);
  const [bindOpen, setBindOpen] = useState(false);
  const [bindItemId, setBindItemId] = useState<string | null>(null);
  const [skuOpts, setSkuOpts] = useState<{ label: string; value: string }[]>([]);
  const [pickedSku, setPickedSku] = useState<string | undefined>();
  const [pickedCandMeta, setPickedCandMeta] = useState<{ confidence: number; source: string } | null>(null);
  const [kw, setKw] = useState('');
  const [deduct, setDeduct] = useState(false);
  const [syncPlat, setSyncPlat] = useState(false);

  const [candCache, setCandCache] = useState<Record<string, SkuCandidateRow[]>>({});
  const [rowCandLoading, setRowCandLoading] = useState<Record<string, boolean>>({});
  const [bindDrawerCands, setBindDrawerCands] = useState<SkuCandidateRow[]>([]);
  const [bindCandLoading, setBindCandLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await getOrderSKUMatches(orderId);
      setRows(r.items ?? []);
      setCandCache({});
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载规格匹配失败');
    } finally {
      setLoading(false);
    }
  }, [orderId]);

  useEffect(() => {
    void load();
  }, [load]);

  const refreshBindDrawerCandidates = useCallback(async (itemId: string) => {
    setBindCandLoading(true);
    try {
      const r = await getOrderItemSkuCandidates(itemId, { limit: 10 });
      setBindDrawerCands(r.list ?? []);
    } catch {
      message.error('候选加载失败');
      setBindDrawerCands([]);
    } finally {
      setBindCandLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!bindOpen || !bindItemId) return;
    void refreshBindDrawerCandidates(bindItemId);
  }, [bindOpen, bindItemId, refreshBindDrawerCandidates]);

  const loadRowCandidatesIfNeeded = useCallback(async (itemId: string) => {
    setRowCandLoading((m) => ({ ...m, [itemId]: true }));
    try {
      const r = await getOrderItemSkuCandidates(itemId, { limit: 10 });
      setCandCache((c) => ({ ...c, [itemId]: r.list ?? [] }));
    } catch {
      message.error('候选加载失败');
      setCandCache((c) => ({ ...c, [itemId]: [] }));
    } finally {
      setRowCandLoading((m) => {
        const n = { ...m };
        delete n[itemId];
        return n;
      });
    }
  }, []);

  const skuWorkbenchRows = rows.filter((r) =>
    ['unmatched', 'skipped', 'ambiguous'].includes(String(r.matchStatus || '')),
  );

  const maxBindCandConf = bindDrawerCands.length
    ? bindDrawerCands.reduce((m, x) => Math.max(m, x.confidence), 0)
    : 0;

  const expandedRowRender = (r: OrderSkuMatchRow) => {
    const id = r.orderItemId;
    if (!id || !['unmatched', 'skipped', 'ambiguous'].includes(String(r.matchStatus || ''))) {
      return <Typography.Text type="secondary">—</Typography.Text>;
    }
    const list = candCache[id] ?? [];
    const loadingRow = !!rowCandLoading[id];
    return (
      <div style={{ padding: '8px 0', maxWidth: 960 }}>
        <Typography.Text type="secondary">规则推荐 SKU（须人工确认，不自动绑定）</Typography.Text>
        <Table
          size="small"
          pagination={false}
          loading={loadingRow && list.length === 0}
          dataSource={list}
          rowKey={(row) => row.productSkuId}
          onRow={(row) => ({
            style:
              list.length > 1 && row.confidence === Math.max(...list.map((x) => x.confidence))
                ? { background: '#f6ffed' }
                : undefined,
          })}
          columns={[
            {
              title: '分',
              width: 100,
              dataIndex: 'confidence',
              render: (v: number) => (
                <Space size={4} wrap>
                  <Typography.Text strong>{v}</Typography.Text>
                  {candTrustBadge(v)}
                </Space>
              ),
            },
            { title: '原因', dataIndex: 'reason', ellipsis: true },
            { title: '来源', dataIndex: 'source', width: 120, ellipsis: true },
            { title: 'SKU', width: 200, ellipsis: true, render: (_, row) => row.skuCode || row.productSkuId },
            { title: '库存', dataIndex: 'stock', width: 64, render: (v?: number) => v ?? '—' },
            {
              title: '操作',
              key: 'pick',
              width: 100,
              render: (_, row) => (
                <Button
                  size="small"
                  onClick={() => {
                    setBindItemId(id);
                    setPickedSku(row.productSkuId);
                    setPickedCandMeta({ confidence: row.confidence, source: row.source });
                    setBindOpen(true);
                  }}
                >
                  以此为候选绑定
                </Button>
              ),
            },
          ]}
        />
      </div>
    );
  };

  const runSearch = async () => {
    try {
      const r = await searchProductSkus({ keyword: kw.trim(), limit: 20 });
      setSkuOpts(
        (r.list ?? []).map((h: ProductSkuSearchHit) => ({
          value: h.productSkuId,
          label: `${h.skuCode || '—'} · ${h.productTitle || ''}`.slice(0, 120),
        })),
      );
    } catch (e: unknown) {
      message.error((e as Error)?.message || '搜索失败');
    }
  };

  const openBind = (orderItemId: string) => {
    setBindItemId(orderItemId);
    setPickedSku(undefined);
    setPickedCandMeta(null);
    setKw('');
    setSkuOpts([]);
    setDeduct(false);
    setSyncPlat(false);
    setBindOpen(true);
  };

  return (
    <>
      {skuWorkbenchRows.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message="部分明细需要人工处理规格匹配"
          description={
            <span>
              未匹配或多候选行可点击下方行展开<strong>查看候选</strong>，或到异常工作台筛选。{' '}
              <Typography.Link
                onClick={() =>
                  history.push(`/orders/exceptions?orderId=${encodeURIComponent(orderId)}`)
                }
              >
                跳转异常工作台
              </Typography.Link>
            </span>
          }
        />
      ) : null}
      <Space wrap style={{ marginBottom: 12 }}>
        <Popconfirm
          title="对未锁定行执行自动匹配？已 manual_bound 默认不覆盖。"
          onConfirm={async () => {
            setMatchLoading(true);
            try {
              await matchOrderSKUs(orderId, { overwrite: false, force: false });
              message.success('已触发匹配');
              await load();
              await onRefreshOrder();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '失败');
            } finally {
              setMatchLoading(false);
            }
          }}
        >
          <Button type="primary" loading={matchLoading}>
            自动匹配整单
          </Button>
        </Popconfirm>
        <Typography.Link href={`/orders/sku-matches?orderId=${encodeURIComponent(orderId)}`}>
          全局匹配记录
        </Typography.Link>
      </Space>
      <Table<OrderSkuMatchRow>
        rowKey={(r) => r.orderItemId || r.id || ''}
        loading={loading}
        size="small"
        pagination={false}
        dataSource={rows}
        expandable={{
          expandRowByClick: true,
          onExpand: (expanded, record) => {
            const id = record.orderItemId;
            if (!expanded || !id) return;
            void loadRowCandidatesIfNeeded(id);
          },
          expandedRowRender,
          rowExpandable: (record) =>
            ['unmatched', 'skipped', 'ambiguous'].includes(String(record.matchStatus || '')),
        }}
        columns={[
          { title: '明细标题', dataIndex: 'productTitle', key: 'productTitle', width: 160, ellipsis: true },
          {
            title: '平台 SKU',
            key: 'ext',
            width: 120,
            render: (_, r) => r.externalSkuId || '—',
          },
          { title: 'Seller/Code', key: 'code', width: 120, render: (_, r) => r.sellerSku || r.skuCode || '—' },
          {
            title: '候选',
            key: 'cand',
            width: 140,
            ellipsis: true,
            render: (_, r) =>
              r.candidateSkus?.length ? (
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {r.candidateSkus
                    .slice(0, 4)
                    .map((c) => c.skuCode || c.productSkuId)
                    .join(' · ')}
                  {r.candidateSkus.length > 4 ? ' …' : ''}
                </Typography.Text>
              ) : (
                '—'
              ),
          },
          {
            title: '状态',
            dataIndex: 'matchStatus',
            key: 'matchStatus',
            width: 110,
            render: (v: string) => <Tag color={statusColor(v)}>{v || '—'}</Tag>,
          },
          { title: '类型', dataIndex: 'matchType', key: 'matchType', width: 130 },
          { title: '置信度', dataIndex: 'confidence', key: 'confidence', width: 72 },
          {
            title: '本地商品规格',
            key: 'local',
            width: 120,
            render: (_, r) => r.localSkuCode || r.productSkuId || '—',
          },
          {
            title: '原因',
            dataIndex: 'reason',
            key: 'reason',
            ellipsis: true,
          },
          {
            title: '操作',
            key: 'op',
            width: 300,
            render: (_, r) => (
              <Space wrap size="small">
                {r.orderItemId ? (
                  <>
                    {['unmatched', 'skipped', 'ambiguous'].includes(String(r.matchStatus || '')) ? (
                      <Typography.Link
                        onClick={() =>
                          history.push(`/orders/exceptions?orderId=${encodeURIComponent(orderId)}`)
                        }
                      >
                        去工作台处理
                      </Typography.Link>
                    ) : null}
                    <Button size="small" onClick={() => openBind(r.orderItemId!)}>
                      绑定 SKU
                    </Button>
                    <Popconfirm
                      title="使用当前行已绑定的本地商品规格扣减库存？"
                      onConfirm={async () => {
                        if (!r.productSkuId) {
                          message.warning('请先自动匹配或人工绑定本地商品规格');
                          return;
                        }
                        try {
                          await bindOrderItemSku(r.orderItemId!, {
                            productSkuId: r.productSkuId,
                            deductInventory: true,
                            syncInventory: false,
                          });
                          message.success('已尝试扣库');
                          await load();
                          await onRefreshOrder();
                        } catch (e: unknown) {
                          message.error((e as Error)?.message || '失败');
                        }
                      }}
                    >
                      <Button size="small" disabled={!r.productSkuId}>
                        扣库
                      </Button>
                    </Popconfirm>
                    <Popconfirm
                      title="使用当前行已绑定的本地商品规格：扣减库存并入队平台库存同步（幂等）？"
                      onConfirm={async () => {
                        if (!r.productSkuId) {
                          message.warning('请先自动匹配或人工绑定本地商品规格');
                          return;
                        }
                        try {
                          await bindOrderItemSku(r.orderItemId!, {
                            productSkuId: r.productSkuId,
                            deductInventory: true,
                            syncInventory: true,
                          });
                          message.success('已处理');
                          await load();
                          await onRefreshOrder();
                        } catch (e: unknown) {
                          message.error((e as Error)?.message || '失败');
                        }
                      }}
                    >
                      <Button size="small" disabled={!r.productSkuId}>
                        扣库+同步
                      </Button>
                    </Popconfirm>
                  </>
                ) : null}
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title="人工绑定 SKU"
        open={bindOpen}
        destroyOnHidden
        width={720}
        footer={
          <Space>
            <Button onClick={() => setBindOpen(false)}>关闭</Button>
            <Popconfirm
              title={
                pickedCandMeta
                  ? `确认绑定所选 SKU（候选分 ${pickedCandMeta.confidence} · ${pickedCandMeta.source}）并执行所选库存策略？`
                  : '确认绑定所选 SKU（未使用候选摘要）并执行所选库存策略？'
              }
              okText="确认"
              cancelText="再想想"
              onConfirm={async () => {
                if (!bindItemId || !pickedSku) {
                  message.warning('请选择 SKU');
                  return;
                }
                try {
                  await bindOrderItemSku(bindItemId, {
                    productSkuId: pickedSku,
                    deductInventory: deduct,
                    syncInventory: syncPlat,
                    candidateConfidence: pickedCandMeta?.confidence,
                    candidateSource: pickedCandMeta?.source,
                  });
                  message.success('已绑定');
                  setBindOpen(false);
                  setCandCache({});
                  await load();
                  await onRefreshOrder();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              <Button type="primary">二次确认绑定</Button>
            </Popconfirm>
          </Space>
        }
        onCancel={() => setBindOpen(false)}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Typography.Title level={5}>规格匹配候选</Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            只读推荐；请选择一行或用手动搜索。「二次确认绑定」前请核对扣库与平台同步开关。
          </Typography.Paragraph>
          <Table
            loading={bindCandLoading}
            size="small"
            pagination={false}
            dataSource={bindDrawerCands}
            rowKey={(row) => row.productSkuId}
            onRow={(row) => ({
              style:
                bindDrawerCands.length > 1 && row.confidence === maxBindCandConf
                  ? { background: '#f6ffed' }
                  : undefined,
            })}
            columns={[
              {
                title: '分',
                width: 110,
                dataIndex: 'confidence',
                render: (v: number) => (
                  <Space wrap>
                    <Typography.Text strong>{v}</Typography.Text>
                    {candTrustBadge(v)}
                  </Space>
                ),
              },
              { title: '原因', dataIndex: 'reason', ellipsis: true },
              { title: '标题', dataIndex: 'productTitle', width: 140, ellipsis: true },
              { title: 'Code', dataIndex: 'skuCode', width: 120, ellipsis: true },
              { title: '名称', dataIndex: 'skuName', width: 120, ellipsis: true },
              { title: '库存', dataIndex: 'stock', width: 64, render: (v?: number) => v ?? '—' },
              {
                title: ' ',
                key: 'p',
                width: 116,
                render: (_, row) => (
                  <Button
                    size="small"
                    type={pickedSku === row.productSkuId ? 'primary' : 'default'}
                    onClick={() => {
                      setPickedSku(row.productSkuId);
                      setPickedCandMeta({ confidence: row.confidence, source: row.source });
                    }}
                  >
                    选择候选
                  </Button>
                ),
              },
            ]}
          />
          <Typography.Title level={5}>手动搜索 SKU</Typography.Title>
          <Input.Search
            placeholder="关键词：skuCode / 名称 / 商品标题"
            onSearch={() => void runSearch()}
            onChange={(e) => setKw(e.target.value)}
            value={kw}
            enterButton="搜索 SKU"
          />
          <Select
            showSearch
            placeholder="选择本地商品规格"
            style={{ width: '100%' }}
            options={skuOpts}
            value={pickedSku}
            onChange={(v) => {
              setPickedSku(v);
              setPickedCandMeta(null);
            }}
            optionFilterProp="label"
          />
          <Space wrap>
            <Button type={deduct ? 'primary' : 'default'} size="small" onClick={() => setDeduct((v) => !v)}>
              {deduct ? '将扣减本地库存' : '绑定后扣减库存（切换）'}
            </Button>
            <Button type={syncPlat ? 'primary' : 'default'} size="small" onClick={() => setSyncPlat((v) => !v)}>
              {syncPlat ? '将推平台库存同步' : '扣库后同步平台（切换）'}
            </Button>
          </Space>
        </Space>
      </Modal>
    </>
  );
}
