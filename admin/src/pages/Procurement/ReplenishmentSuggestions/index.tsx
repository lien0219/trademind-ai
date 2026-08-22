import { DownloadOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Input, Select, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  downloadReplenishmentSuggestions,
  listWarehouses,
  queryReplenishmentSuggestions,
  type ReplenishmentSuggestion,
  type Warehouse,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import '../index.less';

const STATUS_META: Record<string, { label: string; color: string }> = {
  actionable: { label: '可人工采购', color: 'green' },
  not_needed: { label: '暂不需要', color: 'default' },
  blocked_inventory_mismatch: { label: '库存账不一致', color: 'red' },
  blocked_inventory_unmigrated: { label: '库存账待迁移', color: 'orange' },
  blocked_supplier_missing: { label: '无供应商', color: 'red' },
  blocked_supplier_selection: { label: '需选择供应商', color: 'gold' },
};

function statusTag(status: string) {
  const meta = STATUS_META[status] ?? { label: status || '未知状态', color: 'default' };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

export default function ReplenishmentSuggestionsPage() {
  const { can } = usePermission();
  const actionRef = useRef<ActionType>();
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [warehouseLoading, setWarehouseLoading] = useState(true);
  const [warehouseError, setWarehouseError] = useState('');
  const [warehouseId, setWarehouseId] = useState('');
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [listError, setListError] = useState('');

  const loadWarehouses = useCallback(async () => {
    setWarehouseLoading(true);
    setWarehouseError('');
    try {
      const result = await listWarehouses();
      setWarehouses((result.list ?? []).filter((row) => row.status === 'active'));
    } catch (error) {
      setWarehouseError((error as Error)?.message || '仓库列表加载失败，请稍后重试。');
    } finally {
      setWarehouseLoading(false);
    }
  }, []);

  useEffect(() => { void loadWarehouses(); }, [loadWarehouses]);
  useEffect(() => { void actionRef.current?.reload(); }, [warehouseId, status, keyword]);

  const warehouseLabel = useMemo(() => {
    const row = warehouses.find((item) => item.id === warehouseId);
    return row ? `${row.code} · ${row.name}` : '';
  }, [warehouseId, warehouses]);

  const columns: ProColumns<ReplenishmentSuggestion>[] = [
    {
      title: '商品 / 规格',
      dataIndex: 'productTitle',
      search: false,
      render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.productTitle || '未命名商品'}</Typography.Text><Typography.Text type="secondary">{row.skuCode || row.skuName || row.productSkuId}</Typography.Text></Space>,
    },
    { title: '可用', dataIndex: 'availableStock', width: 78, align: 'right', search: false },
    { title: '在途调拨', dataIndex: 'inTransitTransfer', width: 94, align: 'right', search: false },
    { title: '待收采购', dataIndex: 'pendingPurchase', width: 94, align: 'right', search: false },
    { title: '预警 / 安全', dataIndex: 'warningStock', width: 112, align: 'right', search: false, render: (_, row) => `${row.warningStock} / ${row.safetyStock}` },
    { title: '缺口', dataIndex: 'deficit', width: 78, align: 'right', search: false },
    { title: '建议采购量', dataIndex: 'suggestedQuantity', width: 106, align: 'right', search: false, render: (_, row) => row.suggestedQuantity || '—' },
    { title: '供应商 / MOQ', dataIndex: 'supplierName', minWidth: 160, search: false, render: (_, row) => row.supplierName ? `${row.supplierName} / ${row.minOrderQty}` : '—' },
    { title: '采购价', dataIndex: 'unitCostMinor', width: 108, align: 'right', search: false, render: (_, row) => row.supplierName ? `${row.unitCostMinor} ${row.currency}` : '—' },
    { title: '交期', dataIndex: 'leadTimeDays', width: 70, align: 'right', search: false, render: (_, row) => row.supplierName ? `${row.leadTimeDays} 天` : '—' },
    { title: '状态', dataIndex: 'status', width: 130, search: false, render: (_, row) => statusTag(row.status) },
    { title: '阻断原因', dataIndex: 'blockReason', minWidth: 230, search: false, ellipsis: true, render: (_, row) => row.blockReason ? <Typography.Text type="danger" ellipsis={{ tooltip: row.blockReason }}>{row.blockReason}</Typography.Text> : '—' },
  ];

  const statusOptions = Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.label }));
  const canRead = can(PERMISSIONS.PROCUREMENT_VIEW);

  return <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
    <TmPageContainer
      className="tm-procurement-page"
      title="补货建议"
      subTitle="选择目标仓库后，查看只读安全库存缺口与人工采购参考；本页不会创建采购单或启动任务。"
      extra={<TmPageHeaderExtra><Button icon={<DownloadOutlined />} disabled={!warehouseId} onClick={async () => { try { await downloadReplenishmentSuggestions({ warehouseId, keyword, status }); message.success('已开始下载筛选结果'); } catch (error) { message.error((error as Error)?.message || '导出失败，请稍后重试。'); } }}>导出筛选结果</Button></TmPageHeaderExtra>}
    >
      {warehouseError ? <ErrorAlert title={warehouseError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => void loadWarehouses()}>重试</Button>} /> : null}
      <Alert type="info" showIcon message="库存账和供应商资料采用 fail-closed 规则" description="库存账不一致或尚未迁移、无有效供应商、存在多个有效供应商时只展示阻断原因，不猜测采购对象。建议数量只供人工采购决策参考。" />
      <Space wrap size={[16, 12]} className="tm-replenishment-filters" style={{ width: '100%' }}>
        <Select aria-label="目标仓库" showSearch optionFilterProp="label" placeholder="请选择目标仓库（必选）" loading={warehouseLoading} disabled={!warehouseLoading && !warehouses.length} value={warehouseId || undefined} style={{ minWidth: 260 }} options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} onChange={setWarehouseId} />
        <Input.Search aria-label="搜索商品或规格" allowClear placeholder="搜索商品标题、规格编码或名称" style={{ width: 300, maxWidth: '100%' }} onSearch={setKeyword} onChange={(event) => { if (!event.target.value) setKeyword(''); }} />
        <Select aria-label="建议状态" allowClear placeholder="全部状态" value={status || undefined} style={{ width: 170 }} options={statusOptions} onChange={(value) => setStatus(value || '')} />
        {warehouseId ? <Typography.Text type="secondary">当前仓库：{warehouseLabel}</Typography.Text> : null}
      </Space>
      {!warehouseLoading && !warehouseError && !warehouses.length ? <Alert type="warning" showIcon message="暂无启用仓库，维护仓库后才能查看补货建议。" /> : null}
      {!warehouseId ? <Alert type="warning" showIcon message="必须选择目标仓库后才会加载建议。" /> : null}
      {listError ? <ErrorAlert title={listError} actionHint={<Button onClick={() => actionRef.current?.reload()}>重新加载</Button>} /> : null}
      <TmProTable<ReplenishmentSuggestion>
        actionRef={actionRef}
        rowKey="productSkuId"
        columns={columns}
        search={false}
        cardBordered
        scroll={{ x: 1500 }}
        locale={{ emptyText: warehouseId ? (listError ? '补货建议暂不可用' : '暂无符合条件的规格') : '请选择目标仓库' }}
        request={async (params) => {
          if (!warehouseId || !canRead) return { data: [], total: 0, success: true };
          try {
            const result = await queryReplenishmentSuggestions({ warehouseId, keyword, status, page: params.current, pageSize: params.pageSize });
            setListError('');
            return { data: result.list ?? [], total: result.total ?? 0, success: true };
          } catch (error) {
            setListError((error as Error)?.message || '补货建议加载失败，请稍后重试。');
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
      />
    </TmPageContainer>
  </PermissionGuard>;
}
