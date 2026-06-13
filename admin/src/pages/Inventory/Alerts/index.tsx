import { type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import {
  Button,
  Checkbox,
  Descriptions,
  Form,
  InputNumber,
  Modal,
  Popover,
  Radio,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { formatDateTime } from '@/utils/formatTime';
import { history } from '@umijs/max';
import { Link } from '@umijs/renderer-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  INVENTORY_SYNC_BATCHES_LABEL,
  INVENTORY_SYNC_TASKS_LABEL,
} from '@/constants/userFriendly';
import {
  adjustSkuStock,
  batchUpdateStockSettings,
  createInventorySyncBatch,
  previewBatchStockSettings,
  queryInventoryAlerts,
  syncPublicationSkuInventory,
  type InventoryAlertRow,
} from '@/services/inventory';
import { updateProductSkuStockSettings } from '@/services/products';

const BATCH_STOCK_DEFAULT_MAX = 500;

function alertsStockBatchNeedsConfirmAll(p: {
  productSkuIds?: string[];
  productId?: string;
  platform?: string;
  shopId?: string;
  keyword?: string;
  stockStatus?: string;
  onlyPublished?: boolean;
  alertTypes?: string[];
  includeNormal?: boolean;
}): boolean {
  if (p.productSkuIds?.length) return false;
  if ((p.productId ?? '').trim()) return false;
  if ((p.platform ?? '').trim()) return false;
  if ((p.shopId ?? '').trim()) return false;
  if ((p.keyword ?? '').trim()) return false;
  if ((p.stockStatus ?? '').trim()) return false;
  if (p.onlyPublished) return false;
  if (p.alertTypes?.length) return false;
  if (!p.includeNormal) return false;
  return true;
}

const ALERT_TYPE_LABEL: Record<string, string> = {
  out_of_stock: '售罄',
  low_stock: '低库存',
  below_safety_stock: '低于安全线',
  platform_stock_mismatch: '平台不一致',
  platform_stock_unknown: '平台库存未知',
  inventory_sync_failed: '同步失败',
};

const STOCK_STATUS_LABEL: Record<string, { text: string; color: string }> = {
  normal: { text: '正常', color: 'green' },
  low_stock: { text: '低库存', color: 'orange' },
  out_of_stock: { text: '售罄', color: 'red' },
  below_safety_stock: { text: '低于安全线', color: 'gold' },
};

const PLATFORM_ST_LABEL: Record<string, { text: string; color: string }> = {
  platform_stock_synced: { text: '一致', color: 'green' },
  platform_stock_mismatch: { text: '不一致', color: 'orange' },
  platform_stock_unknown: { text: '未知', color: 'default' },
};

function stockStatusTag(raw: string) {
  const m = STOCK_STATUS_LABEL[raw];
  if (!m) return <Tag>{raw}</Tag>;
  return <Tag color={m.color}>{m.text}</Tag>;
}

export default function InventoryAlertsPage() {
  const actionRef = useRef<ActionType>();
  const searchFormRef = useRef<ProFormInstance>();
  const [selectedSkuIds, setSelectedSkuIds] = useState<string[]>([]);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkIncludeLocalAlerts, setBulkIncludeLocalAlerts] = useState(false);
  const [bulkSubmitting, setBulkSubmitting] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [active, setActive] = useState<InventoryAlertRow | null>(null);
  const [adjustSubmitting, setAdjustSubmitting] = useState(false);
  const [settingsSubmitting, setSettingsSubmitting] = useState(false);
  const [adjustForm] = Form.useForm<{ stock: number }>();
  const [settingsForm] = Form.useForm<{ warningStock: number; safetyStock: number }>();
  const [batchStockOpen, setBatchStockOpen] = useState(false);
  const [batchStockScope, setBatchStockScope] = useState<'selected' | 'filter'>('selected');
  const [batchMatched, setBatchMatched] = useState<number | null>(null);
  const [batchPreviewLoading, setBatchPreviewLoading] = useState(false);
  const [batchStockSubmitting, setBatchStockSubmitting] = useState(false);
  const [batchStockForm] = Form.useForm<{ warningStock: number; safetyStock: number }>();

  const buildStockBatchPayload = () => {
    const fv = searchFormRef.current?.getFieldsValue?.() ?? {};
    const alertType = typeof fv.alertType === 'string' ? fv.alertType.trim() : '';
    const alertTypes = alertType ? [alertType] : [];
    const base = {
      keyword: (fv.keyword as string)?.trim() || undefined,
      platform: (fv.platform as string)?.trim().toLowerCase() || undefined,
      shopId: (fv.shopId as string)?.trim() || undefined,
      stockStatus: (fv.stockStatus as string)?.trim() || undefined,
      onlyPublished: Boolean(fv.onlyPublished),
      includeNormal: Boolean(fv.includeNormal),
      alertTypes: alertTypes.length ? alertTypes : undefined,
    };
    if (batchStockScope === 'selected' && selectedSkuIds.length > 0) {
      return { ...base, productSkuIds: selectedSkuIds };
    }
    return base;
  };

  const runBatchPreview = async () => {
    setBatchPreviewLoading(true);
    try {
      const raw = buildStockBatchPayload();
      const res = await previewBatchStockSettings({
        ...raw,
        page: 1,
        pageSize: 10,
      });
      setBatchMatched(res.matchedCount);
    } catch (e) {
      setBatchMatched(null);
      message.error((e as Error)?.message || '预览失败');
    } finally {
      setBatchPreviewLoading(false);
    }
  };

  useEffect(() => {
    if (!batchStockOpen) return;
    void runBatchPreview();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [batchStockOpen, batchStockScope]);

  const columns: ProColumns<InventoryAlertRow>[] = useMemo(
    () => [
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: { placeholder: '标题 / SKU 编码 / 名称' },
      },
      {
        title: '平台',
        dataIndex: 'platform',
        hideInTable: true,
        fieldProps: { placeholder: 'tiktok / shopee / lazada / amazon' },
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        hideInTable: true,
      },
      {
        title: '预警类型',
        dataIndex: 'alertType',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          out_of_stock: { text: '售罄' },
          low_stock: { text: '低库存' },
          below_safety_stock: { text: '低于安全线' },
          platform_stock_mismatch: { text: '平台不一致' },
          platform_stock_unknown: { text: '平台未知' },
          inventory_sync_failed: { text: '同步失败' },
        },
      },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          normal: { text: '正常' },
          low_stock: { text: '低库存' },
          out_of_stock: { text: '售罄' },
          below_safety_stock: { text: '低于安全线' },
        },
      },
      {
        title: '仅已刊登',
        dataIndex: 'onlyPublished',
        hideInTable: true,
        renderFormItem: () => <Switch checkedChildren="是" unCheckedChildren="否" />,
      },
      {
        title: '含正常',
        dataIndex: 'includeNormal',
        hideInTable: true,
        renderFormItem: () => <Switch checkedChildren="是" unCheckedChildren="否" />,
      },
      {
        title: '商品',
        dataIndex: 'productTitle',
        width: 180,
        search: false,
        ellipsis: true,
        render: (_, r) => (
          <Link to={`/product/drafts/${r.productId}?tab=inventory`}>{r.productTitle || '—'}</Link>
        ),
      },
      {
        title: '规格编号',
        dataIndex: 'skuCode',
        width: 140,
        search: false,
        ellipsis: true,
        render: (_, r) => (
          <Typography.Text>
            {r.skuCode || '—'}
            <Typography.Paragraph type="secondary" style={{ margin: 0, fontSize: 12 }} ellipsis>
              {r.skuName || r.productSkuId}
            </Typography.Paragraph>
          </Typography.Text>
        ),
      },
      {
        title: '本地库存',
        dataIndex: 'stock',
        width: 88,
        search: false,
      },
      {
        title: '预警线',
        dataIndex: 'warningStock',
        width: 72,
        search: false,
      },
      {
        title: '安全线',
        dataIndex: 'safetyStock',
        width: 72,
        search: false,
      },
      {
        title: '库存状态',
        dataIndex: 'stockStatus',
        width: 108,
        search: false,
        render: (_, r) => stockStatusTag(r.stockStatus),
      },
      {
        title: '预警',
        dataIndex: 'alertTypes',
        width: 220,
        search: false,
        render: (_, r) => (
          <Space wrap size={[0, 4]}>
            {(r.alertTypes ?? []).map((t) => (
              <Tag
                key={t}
                color={t.includes('failed') ? 'red' : t.includes('mismatch') ? 'orange' : undefined}
              >
                {ALERT_TYPE_LABEL[t] || t}
              </Tag>
            ))}
          </Space>
        ),
      },
      {
        title: '平台库存',
        dataIndex: 'platformStocks',
        width: 260,
        search: false,
        render: (_, r) => {
          if (!r.platformStocks?.length) return <Typography.Text type="secondary">—</Typography.Text>;
          return (
            <Popover
              title="刊登 SKU 明细"
              content={
                <div style={{ maxWidth: 420 }}>
                  {r.platformStocks.map((p) => {
                    const st = PLATFORM_ST_LABEL[p.platformStockStatus] || {
                      text: p.platformStockStatus,
                      color: 'default',
                    };
                    return (
                      <Descriptions key={p.publicationSkuId} size="small" column={1} style={{ marginBottom: 12 }}>
                        <Descriptions.Item label="店铺">{p.shopName || p.shopId}</Descriptions.Item>
                        <Descriptions.Item label="平台">{p.platform}</Descriptions.Item>
                        <Descriptions.Item label="平台库存">
                          {typeof p.platformStock === 'number' ? p.platformStock : '—'}
                        </Descriptions.Item>
                        <Descriptions.Item label="状态">
                          <Tag color={st.color}>{st.text}</Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label="最近任务">{p.lastSyncStatus || '—'}</Descriptions.Item>
                      </Descriptions>
                    );
                  })}
                </div>
              }
            >
              <Space wrap size={[0, 4]}>
                {r.platformStocks.slice(0, 3).map((p) => {
                  const st = PLATFORM_ST_LABEL[p.platformStockStatus] || {
                    text: p.platformStockStatus,
                    color: 'default',
                  };
                  return (
                    <Tag key={p.publicationSkuId} color={st.color}>
                      {p.platform}:{typeof p.platformStock === 'number' ? p.platformStock : '?'}
                    </Tag>
                  );
                })}
                {r.platformStocks.length > 3 ? <Tag>+{r.platformStocks.length - 3}</Tag> : null}
              </Space>
            </Popover>
          );
        },
      },
      {
        title: '最近同步',
        dataIndex: 'lastSyncStatus',
        width: 108,
        search: false,
        render: (_, r) => {
          if (!r.lastSyncStatus) return '—';
          const fail = r.lastSyncStatus === 'failed';
          return <Tag color={fail ? 'red' : 'default'}>{r.lastSyncStatus}</Tag>;
        },
      },
      {
        title: '最近库存变更',
        dataIndex: 'lastInventoryChangeAt',
        width: 156,
        search: false,
        render: (_, r) =>
          r.lastInventoryChangeAt ? formatDateTime(r.lastInventoryChangeAt) : '—',
      },
      {
        title: '操作',
        valueType: 'option' as const,
        width: 280,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size="small">
            <Link to={`/product/drafts/${r.productId}?tab=inventory`}>商品</Link>
            <Button
              type="link"
              size="small"
              style={{ padding: 0 }}
              onClick={() => {
                setActive(r);
                adjustForm.setFieldsValue({ stock: r.stock });
                setAdjustOpen(true);
              }}
            >
              调整库存
            </Button>
            <Button
              type="link"
              size="small"
              style={{ padding: 0 }}
              onClick={() => {
                setActive(r);
                settingsForm.setFieldsValue({ warningStock: r.warningStock, safetyStock: r.safetyStock });
                setSettingsOpen(true);
              }}
            >
              预警线
            </Button>
            <Link
              to={`/inventory/logs?productId=${encodeURIComponent(r.productId)}&productSkuId=${encodeURIComponent(r.productSkuId)}`}
            >
              流水
            </Link>
            <Link to="/inventory/sync-tasks">同步任务</Link>
          </Space>
        ),
      },
    ],
    [adjustForm, settingsForm],
  );

  return (
    <TmPageContainer title="库存预警" subTitle="查看低库存、缺货与平台库存不一致等预警，可按需创建同步任务。">
      <Typography.Paragraph type="secondary">
        仅查询与提醒，不自动改平台库存；推送仍走{' '}
        <Link to="/inventory/sync-tasks">{INVENTORY_SYNC_TASKS_LABEL}</Link>
        （可勾选 SKU 行后批量创建{' '}
        <Link to="/inventory/sync-batches">{INVENTORY_SYNC_BATCHES_LABEL}</Link>
        ）。抖店 SKU 须先在商品详情完成 SKU 绑定后再同步库存。
      </Typography.Paragraph>
      <ProTable<InventoryAlertRow>
        rowKey={(r) => r.productSkuId}
        actionRef={actionRef}
        formRef={searchFormRef}
        columns={columns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 100 }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        rowSelection={{
          selectedRowKeys: selectedSkuIds,
          onChange: (keys) => setSelectedSkuIds(keys.map(String)),
        }}
        tableAlertRender={false}
        toolBarRender={() => [
          <Button
            key="batch-stock"
            onClick={() => {
              const scope = selectedSkuIds.length ? 'selected' : 'filter';
              setBatchStockScope(scope);
              batchStockForm.setFieldsValue({ warningStock: 10, safetyStock: 2 });
              setBatchStockOpen(true);
            }}
          >
            批量设置预警线
          </Button>,
          <Button
            key="bulk-sync"
            type="primary"
            disabled={selectedSkuIds.length === 0}
            onClick={() => {
              setBulkIncludeLocalAlerts(false);
              setBulkOpen(true);
            }}
          >
            批量同步库存
          </Button>,
        ]}
        expandable={{
          expandedRowRender: (r) =>
            r.platformStocks?.length ? (
              <Space direction="vertical" style={{ width: '100%' }}>
                {r.platformStocks.map((p) => {
                  const st = PLATFORM_ST_LABEL[p.platformStockStatus] || {
                    text: p.platformStockStatus,
                    color: 'default',
                  };
                  const runnable =
                    (p.platform || '').toLowerCase() === 'tiktok' ||
                    (p.platform || '').toLowerCase() === 'shopee' ||
                    (p.platform || '').toLowerCase() === 'lazada' ||
                    (p.platform || '').toLowerCase() === 'amazon';
                  return (
                    <Space key={p.publicationSkuId} wrap align="start">
                      <Typography.Text>
                        [{p.platform}] {p.shopName || p.shopId} — 平台库存{' '}
                        {typeof p.platformStock === 'number' ? p.platformStock : '—'}{' '}
                        <Tag color={st.color}>{st.text}</Tag>
                      </Typography.Text>
                      <Button
                        type="link"
                        size="small"
                        disabled={!runnable}
                        onClick={async () => {
                          const stock = r.stock;
                          try {
                            await syncPublicationSkuInventory(p.publicationSkuId, {
                              stock,
                              fromInventoryAlert: true,
                            });
                            message.success('已创建同步任务');
                            actionRef.current?.reload();
                          } catch (e: unknown) {
                            message.error((e as Error)?.message || '失败');
                          }
                        }}
                      >
                        同步库存
                      </Button>
                    </Space>
                  );
                })}
              </Space>
            ) : (
              <Typography.Text type="secondary">无刊登映射</Typography.Text>
            ),
        }}
        request={async (params, sort, filter) => {
          void sort;
          void filter;
          try {
            const res = await queryInventoryAlerts({
              keyword: (params.keyword as string) || undefined,
              platform: (params.platform as string) || undefined,
              shopId: (params.shopId as string) || undefined,
              alertType: (params.alertType as string) || undefined,
              stockStatus: (params.stockStatus as string) || undefined,
              onlyPublished: Boolean(params.onlyPublished),
              includeNormal: Boolean(params.includeNormal),
              page: params.current,
              pageSize: params.pageSize,
            });
            return {
              data: res.list ?? [],
              success: true,
              total: res.pagination?.total ?? 0,
            };
          } catch (e: unknown) {
            message.error((e as Error)?.message || '加载失败');
            return { data: [], success: false, total: 0 };
          }
        }}
      />

      <Modal
        title="批量同步库存"
        open={bulkOpen}
        onCancel={() => setBulkOpen(false)}
        okText="创建批次"
        confirmLoading={bulkSubmitting}
        onOk={async () => {
          if (selectedSkuIds.length === 0) return;
          const fv = searchFormRef.current?.getFieldsValue?.() ?? {};
          const platformRaw = typeof fv.platform === 'string' ? fv.platform.trim().toLowerCase() : '';
          const shopRaw = typeof fv.shopId === 'string' ? fv.shopId.trim() : '';
          const alertTypes = bulkIncludeLocalAlerts
            ? [
                'platform_stock_mismatch',
                'inventory_sync_failed',
                'low_stock',
                'out_of_stock',
                'below_safety_stock',
              ]
            : ['platform_stock_mismatch', 'inventory_sync_failed'];
          setBulkSubmitting(true);
          try {
            const batch = await createInventorySyncBatch({
              source: 'inventory_alert',
              platform: platformRaw || undefined,
              shopId: shopRaw || undefined,
              productSkuIds: selectedSkuIds,
              onlyAlerts: true,
              alertTypes,
              onlyPublished: true,
            });
            message.success(`已创建批次 ${batch.batchNo}（跳过 ${batch.skippedCount}）`);
            setBulkOpen(false);
            setSelectedSkuIds([]);
            actionRef.current?.reload();
            history.push(`/inventory/sync-tasks?batchId=${encodeURIComponent(batch.id)}`);
          } catch (e: unknown) {
            message.error((e as Error)?.message || '创建失败');
          } finally {
            setBulkSubmitting(false);
          }
        }}
      >
        <Typography.Paragraph>
          将为选中的 <Typography.Text strong>{selectedSkuIds.length}</Typography.Text> 个 SKU 创建库存同步批次。
          默认仅包含「平台库存不一致」「同步失败」类预警对应的刊登 SKU；不包含单纯的本地低库存/售罄，除非你勾选下方选项。
        </Typography.Paragraph>
        <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
          列表筛选中的 platform / shopId 会一并传给后端以收窄刊登映射（若留空则由服务端按 SKU 聚合）。
        </Typography.Paragraph>
        <Checkbox checked={bulkIncludeLocalAlerts} onChange={(e) => setBulkIncludeLocalAlerts(e.target.checked)}>
          同时包含本地低库存 / 售罄 / 低于安全线预警（将把对应刊登 SKU 推送到平台队列）
        </Checkbox>
      </Modal>

      <Modal
        title={active ? `调整库存 · ${active.skuCode}` : '调整库存'}
        open={adjustOpen}
        onCancel={() => setAdjustOpen(false)}
        okButtonProps={{ loading: adjustSubmitting }}
        onOk={async () => {
          if (!active) return;
          const v = await adjustForm.validateFields();
          setAdjustSubmitting(true);
          try {
            await adjustSkuStock(active.productId, active.productSkuId, {
              stock: v.stock,
              reason: 'manual_adjust',
              remark: 'from_inventory_alerts',
            });
            message.success('已更新');
            setAdjustOpen(false);
            actionRef.current?.reload();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
          } finally {
            setAdjustSubmitting(false);
          }
        }}
      >
        <Form form={adjustForm} layout="vertical">
          <Form.Item name="stock" label="库存（≥0）" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={active ? `预警线 · ${active.skuCode}` : '预警线'}
        open={settingsOpen}
        onCancel={() => setSettingsOpen(false)}
        okButtonProps={{ loading: settingsSubmitting }}
        onOk={async () => {
          if (!active) return;
          const v = await settingsForm.validateFields();
          if (v.safetyStock > v.warningStock) {
            message.error('安全线不能大于预警线');
            return;
          }
          setSettingsSubmitting(true);
          try {
            await updateProductSkuStockSettings(active.productId, active.productSkuId, {
              warningStock: v.warningStock,
              safetyStock: v.safetyStock,
            });
            message.success('已保存');
            setSettingsOpen(false);
            actionRef.current?.reload();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
          } finally {
            setSettingsSubmitting(false);
          }
        }}
      >
        <Form form={settingsForm} layout="vertical">
          <Form.Item name="warningStock" label="预警库存线" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="safetyStock" label="安全库存线" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="批量设置预警线"
        open={batchStockOpen}
        onCancel={() => {
          setBatchStockOpen(false);
          setBatchMatched(null);
        }}
        okText="应用"
        confirmLoading={batchStockSubmitting}
        onOk={() => {
          return batchStockForm
            .validateFields()
            .then((v) => {
              if (v.safetyStock > v.warningStock) {
                message.error('安全线不能大于预警线');
                return Promise.reject(new Error('validation'));
              }
              if (batchStockScope === 'selected' && selectedSkuIds.length === 0) {
                message.error('请先勾选 SKU，或改用「当前筛选结果」');
                return Promise.reject(new Error('validation'));
              }
              const payload = buildStockBatchPayload();
              const needAll = alertsStockBatchNeedsConfirmAll({
                ...payload,
                productSkuIds: payload.productSkuIds,
              });
              return new Promise<void>((resolve, reject) => {
                Modal.confirm({
                  title: '确认仅修改预警线？',
                  width: 520,
                  content: (
                    <div>
                      <Typography.Paragraph>
                        将修改 <Typography.Text strong>{batchMatched ?? '—'}</Typography.Text>{' '}
                        个 SKU 的预警线 / 安全线；不修改本地实际库存，不写入库存流水，不创建平台同步任务。
                      </Typography.Paragraph>
                      {needAll ? (
                        <Typography.Paragraph type="warning">
                          当前为「含正常 SKU 的全表筛选」，将附加 confirmAll 提交。
                        </Typography.Paragraph>
                      ) : null}
                      {(batchMatched ?? 0) > BATCH_STOCK_DEFAULT_MAX ? (
                        <Typography.Paragraph type="warning">
                          匹配数超过默认单次上限 {BATCH_STOCK_DEFAULT_MAX}，将附加 confirmLarge。
                        </Typography.Paragraph>
                      ) : null}
                    </div>
                  ),
                  okText: '确认应用',
                  onOk: async () => {
                    setBatchStockSubmitting(true);
                    try {
                      const raw = buildStockBatchPayload();
                      await batchUpdateStockSettings({
                        ...raw,
                        warningStock: v.warningStock,
                        safetyStock: v.safetyStock,
                        confirm: true,
                        confirmLarge: (batchMatched ?? 0) > BATCH_STOCK_DEFAULT_MAX,
                        confirmAll: needAll,
                      });
                      message.success('已批量更新预警线');
                      setBatchStockOpen(false);
                      setBatchMatched(null);
                      setSelectedSkuIds([]);
                      actionRef.current?.reload();
                      resolve();
                    } catch (e) {
                      message.error((e as Error)?.message || '失败');
                      reject(e);
                    } finally {
                      setBatchStockSubmitting(false);
                    }
                  },
                });
              });
            })
            .catch((e: unknown) => {
              if ((e as Error)?.message === 'validation') return;
              throw e;
            });
        }}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          仅更新 product_skus.warning_stock / safety_stock，并重新计算 stock_status。
        </Typography.Paragraph>
        <Form form={batchStockForm} layout="vertical" initialValues={{ warningStock: 10, safetyStock: 2 }}>
          <Form.Item label="应用范围">
            <Radio.Group
              value={batchStockScope}
              onChange={(e) => setBatchStockScope(e.target.value as 'selected' | 'filter')}
            >
              <Radio value="selected" disabled={selectedSkuIds.length === 0}>
                仅选中行（{selectedSkuIds.length}）
              </Radio>
              <Radio value="filter">当前筛选结果（含列表搜索条件）</Radio>
            </Radio.Group>
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
            影响数量：{batchPreviewLoading ? '计算中…' : batchMatched !== null ? `${batchMatched} 个 SKU` : '—'}
          </Typography.Paragraph>
          <Form.Item name="warningStock" label="预警库存线" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="safetyStock" label="安全库存线" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Button type="link" size="small" onClick={() => void runBatchPreview()} loading={batchPreviewLoading}>
            刷新匹配数
          </Button>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
