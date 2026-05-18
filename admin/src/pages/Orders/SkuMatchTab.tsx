import { Button, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography, Alert, message } from 'antd';
import { history } from '@umijs/max';
import { useCallback, useEffect, useState } from 'react';
import {
  bindOrderItemSku,
  getOrderSKUMatches,
  matchOrderSKUs,
  type OrderSkuMatchRow,
} from '@/services/orders';
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';

type Props = {
  orderId: string;
  onRefreshOrder: () => Promise<void>;
};

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
  const [kw, setKw] = useState('');
  const [deduct, setDeduct] = useState(false);
  const [syncPlat, setSyncPlat] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await getOrderSKUMatches(orderId);
      setRows(r.items ?? []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载 SKU 匹配失败');
    } finally {
      setLoading(false);
    }
  }, [orderId]);

  useEffect(() => {
    void load();
  }, [load]);

  const skuWorkbenchRows = rows.filter((r) =>
    ['unmatched', 'skipped', 'ambiguous'].includes(String(r.matchStatus || '')),
  );

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
          message="部分明细需要人工处理 SKU 匹配"
          description={
            <span>
              未匹配或多候选行可到异常工作台统一筛选与绑定。{' '}
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
            title: '本地 SKU',
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
                      title="使用当前行已绑定的本地 SKU 仅扣减本地库存（幂等）？"
                      onConfirm={async () => {
                        if (!r.productSkuId) {
                          message.warning('请先自动匹配或人工绑定本地 SKU');
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
                      title="使用当前行已绑定的本地 SKU：扣减库存并入队平台库存同步（幂等）？"
                      onConfirm={async () => {
                        if (!r.productSkuId) {
                          message.warning('请先自动匹配或人工绑定本地 SKU');
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
        onCancel={() => setBindOpen(false)}
        destroyOnClose
        onOk={async () => {
          if (!bindItemId || !pickedSku) {
            message.error('请选择 SKU');
            return;
          }
          try {
            await bindOrderItemSku(bindItemId, {
              productSkuId: pickedSku,
              deductInventory: deduct,
              syncInventory: syncPlat,
            });
            message.success('已绑定');
            setBindOpen(false);
            await load();
            await onRefreshOrder();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
          }
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Input.Search
            placeholder="关键词：skuCode / 名称 / 商品标题"
            onSearch={() => void runSearch()}
            onChange={(e) => setKw(e.target.value)}
            value={kw}
            enterButton="搜索 SKU"
          />
          <Select
            showSearch
            placeholder="选择本地 SKU"
            style={{ width: '100%' }}
            options={skuOpts}
            value={pickedSku}
            onChange={(v) => setPickedSku(v)}
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
