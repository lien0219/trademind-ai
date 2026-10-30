import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { DownloadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Input, Select, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createProcurementIdempotencyKey,
  createPurchaseOrderFromReplenishment,
  downloadReplenishmentSuggestions,
  extractProcurementAPIError,
  listWarehouses,
  procurementErrorMessage,
  queryReplenishmentSuggestions,
  type ReplenishmentSuggestion,
  type Warehouse,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import { idempotentCommandKey, type IdempotentCommandKey } from '@/utils/idempotentCommandKey';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import { captureTenantContext, isTenantContextCurrent, isTenantSwitching, type TenantContextSnapshot } from '@/utils/tenantContext';
import CreateDraftModal from './CreateDraftModal';
import { formatMinorAmount } from '../helpers';
import '../index.less';

const STATUS_META: Record<string, { source: string; color: string }> = {
  actionable: { source: "可人工采购", color: 'green' },
  not_needed: { source: "暂不需要", color: 'default' },
  blocked_inventory_mismatch: { source: "库存账不一致", color: 'red' },
  blocked_inventory_unmigrated: { source: "库存账待迁移", color: 'orange' },
  blocked_supplier_missing: { source: "无供应商", color: 'red' },
  blocked_supplier_selection: { source: "需选择供应商", color: 'gold' },
};

function statusTag(status: string) {
  const meta = STATUS_META[status] ?? { source: status || '未知状态', color: 'default' };
  return <Tag color={meta.color}>{tr(meta.source)}</Tag>;
}

function canCreateDraft(row: ReplenishmentSuggestion) {
  return (row.status === 'actionable' || row.status === 'blocked_supplier_selection')
    && row.deficit > 0
    && row.suggestionHash?.length === 64
    && (row.supplierOptions?.length ?? 0) > 0;
}

export default function ReplenishmentSuggestionsPage() {
  useLanguage();
  const { can, readonly } = usePermission();
  const actionRef = useRef<ActionType>();
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [warehouseLoading, setWarehouseLoading] = useState(true);
  const [warehouseError, setWarehouseError] = useState('');
  const [warehouseId, setWarehouseId] = useState('');
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [listError, setListError] = useState('');
  const [selectedRows, setSelectedRows] = useState<ReplenishmentSuggestion[]>([]);
  const [draftModalOpen, setDraftModalOpen] = useState(false);
  const [draftSubmitting, setDraftSubmitting] = useState(false);
  const [draftIdempotencyKey, setDraftIdempotencyKey] = useState('');
  const draftSubmittingRef = useRef(false);
  const draftCommandRef = useRef<IdempotentCommandKey>();
  const warehouseRead = useMemo(createTenantReadGuard, []);
  const suggestionsRead = useMemo(createTenantReadGuard, []);
  const draftScopeRef = useRef<{
    context: TenantContextSnapshot;
    warehouseId: string;
    items: Array<{ productSkuId: string; suggestionHash: string }>;
  }>();

  const loadWarehouses = useCallback(async () => {
    const isCurrent = warehouseRead.begin();
    setWarehouseLoading(true);
    setWarehouseError('');
    setWarehouses([]);
    try {
      const result = await listWarehouses();
      if (!isCurrent()) return;
      const active = (result.list ?? []).filter((row) => row.status === 'active');
      setWarehouses(active);
      setWarehouseId((current) => active.some((row) => row.id === current) ? current : '');
    } catch (error) {
      if (isCurrent()) setWarehouseError((error as Error)?.message || '仓库列表加载失败，请稍后重试。');
    } finally {
      if (isCurrent()) setWarehouseLoading(false);
    }
  }, [warehouseRead]);

  useEffect(() => {
    void loadWarehouses();
    return () => warehouseRead.invalidate();
  }, [loadWarehouses, warehouseRead]);

  useEffect(() => {
    suggestionsRead.invalidate();
    setSelectedRows([]);
    void actionRef.current?.reload();
  }, [suggestionsRead, warehouseId, status, keyword]);

  useEffect(() => () => {
    suggestionsRead.invalidate();
    draftScopeRef.current = undefined;
  }, [suggestionsRead]);

  const invalidateSelection = () => {
    suggestionsRead.invalidate();
    setSelectedRows([]);
    draftScopeRef.current = undefined;
    if (!draftSubmittingRef.current) setDraftModalOpen(false);
  };

  const warehouseLabel = useMemo(() => {
    const row = warehouses.find((item) => item.id === warehouseId);
    return row ? `${row.code} · ${row.name}` : '';
  }, [warehouseId, warehouses]);

  const columns: ProColumns<ReplenishmentSuggestion>[] = [
    {
      title: tr("商品 / 规格"),
      dataIndex: 'productTitle',
      search: false,
      render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.productTitle || tr('未命名商品')}</Typography.Text><Typography.Text type="secondary">{row.skuCode || row.skuName || row.productSkuId}</Typography.Text></Space>,
    },
    { title: tr("可用"), dataIndex: 'availableStock', width: 78, align: 'right', search: false },
    { title: tr("在途调拨"), dataIndex: 'inTransitTransfer', width: 94, align: 'right', search: false },
    { title: tr("待收采购"), dataIndex: 'pendingPurchase', width: 94, align: 'right', search: false },
    { title: tr("预警 / 安全"), dataIndex: 'warningStock', width: 112, align: 'right', search: false, render: (_, row) => `${row.warningStock} / ${row.safetyStock}` },
    { title: tr("缺口"), dataIndex: 'deficit', width: 78, align: 'right', search: false },
    { title: tr("建议采购量"), dataIndex: 'suggestedQuantity', width: 106, align: 'right', search: false, render: (_, row) => row.suggestedQuantity || '—' },
    { title: tr("供应商 / 起订量"), dataIndex: 'supplierName', minWidth: 170, search: false, render: (_, row) => row.supplierName ? `${row.supplierName} / ${row.minOrderQty}` : row.supplierOptions?.length ? trFormat('{value0} 个可选供应商', { value0: new Set(row.supplierOptions.map((option) => option.supplierId)).size }) : '—' },
    { title: tr("采购价"), dataIndex: 'unitCostMinor', width: 118, align: 'right', search: false, render: (_, row) => row.supplierName ? formatMinorAmount(row.unitCostMinor, row.currency) : '—' },
    { title: tr("交期"), dataIndex: 'leadTimeDays', width: 70, align: 'right', search: false, render: (_, row) => row.supplierName ? trFormat("{value0} 天", { value0: String(row.leadTimeDays) }) : '—' },
    { title: tr("状态"), dataIndex: 'status', width: 130, search: false, render: (_, row) => statusTag(row.status) },
    { title: tr("阻断原因"), dataIndex: 'blockReason', minWidth: 230, search: false, ellipsis: true, render: (_, row) => row.blockReason ? <Typography.Text type="danger" ellipsis={{ tooltip: row.blockReason }}>{row.blockReason}</Typography.Text> : '—' },
  ];

  const statusOptions = Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: tr(meta.source) }));
  const canRead = can(PERMISSIONS.PROCUREMENT_VIEW);
  const canManage = !readonly && can(PERMISSIONS.PROCUREMENT_MANAGE);

  const openDraftModal = () => {
    if (!canManage || !warehouseId || !selectedRows.length || isTenantSwitching()) return;
    if (selectedRows.some((row) => row.warehouseId !== warehouseId || !canCreateDraft(row))) return;
    draftScopeRef.current = {
      context: captureTenantContext(),
      warehouseId,
      items: selectedRows.map((row) => ({ productSkuId: row.productSkuId, suggestionHash: row.suggestionHash })),
    };
    draftCommandRef.current = undefined;
    setDraftIdempotencyKey(createProcurementIdempotencyKey('replenishment-draft'));
    setDraftModalOpen(true);
  };

  const createDraft = async (body: Parameters<typeof createPurchaseOrderFromReplenishment>[0]) => {
    if (draftSubmittingRef.current || draftSubmitting || !canManage || !draftModalOpen) return;
    const scope = draftScopeRef.current;
    if (!scope || !isTenantContextCurrent(scope.context) || isTenantSwitching()
      || scope.warehouseId !== warehouseId || body.warehouseId !== scope.warehouseId
      || body.items.length !== scope.items.length
      || body.items.some((item, index) =>
        item.productSkuId !== scope.items[index].productSkuId ||
        item.suggestionHash !== scope.items[index].suggestionHash)) return;
    const payload = { warehouseId: body.warehouseId, supplierId: body.supplierId, remark: body.remark, items: body.items };
    const command = idempotentCommandKey(
      draftCommandRef.current, 'replenishment-draft', payload,
      createProcurementIdempotencyKey, draftIdempotencyKey,
    );
    draftCommandRef.current = command;
    draftSubmittingRef.current = true;
    setDraftSubmitting(true);
    try {
      const order = await createPurchaseOrderFromReplenishment({ ...payload, idempotencyKey: command.key });
      if (draftScopeRef.current !== scope || !isTenantContextCurrent(scope.context)) return;
      message.success(tr("采购草稿已创建，请继续提交审批"));
      draftScopeRef.current = undefined;
      draftCommandRef.current = undefined;
      setDraftModalOpen(false);
      setSelectedRows([]);
      actionRef.current?.reload();
      history.push(`/procurement/purchase-orders/${order.id}`);
    } catch (error) {
      if (draftScopeRef.current !== scope || !isTenantContextCurrent(scope.context)) return;
      const apiError = extractProcurementAPIError(error);
      if (apiError.message.toLowerCase().includes('replenishment ')) {
        draftScopeRef.current = undefined;
        draftCommandRef.current = undefined;
        setDraftModalOpen(false);
        setSelectedRows([]);
        actionRef.current?.reload();
      }
      message.error(tr(procurementErrorMessage(
        apiError,
        '采购草稿创建失败，建议可能已变化，请重新加载后确认。',
      )));
    } finally {
      draftSubmittingRef.current = false;
      setDraftSubmitting(false);
    }
  };

  return <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
    <TmPageContainer
      className="tm-procurement-page"
      title={tr("补货建议")}
      subTitle={tr("选择目标仓库并复核缺口、供应商和数量后，可创建本地采购草稿；后续提交、审批和收货仍需人工完成。")}
          extra={<TmPageHeaderExtra><Button icon={<DownloadOutlined />} disabled={!warehouseId} onClick={async () => { try { await downloadReplenishmentSuggestions({ warehouseId, keyword, status }); message.success(tr("已开始下载筛选结果")); } catch (error) { message.error(tr((error as Error)?.message || tr('导出失败，请稍后重试。'))); } }}>{tr("导出筛选结果")}</Button><Button type="primary" icon={<PlusOutlined />} disabled={!canManage || !warehouseId || selectedRows.length === 0} onClick={openDraftModal}>{tr("创建采购草稿")}{selectedRows.length ? `（${selectedRows.length}）` : ''}</Button></TmPageHeaderExtra>}
    >
      {warehouseError ? <ErrorAlert title={tr(warehouseError)} actionHint={<Button icon={<ReloadOutlined />} onClick={() => void loadWarehouses()}>{tr("重试")}</Button>} /> : null}
      <Alert type="info" showIcon message={tr("库存账和供应商资料采用安全阻断规则")} description={tr("库存账不一致、尚未迁移或无有效供应商时不能创建草稿；存在多个供应商时必须人工选择。创建前服务端会重新计算并校验建议，避免使用过期数据。")} />
      {!canManage ? <Alert type="warning" showIcon message={tr("当前账号仅可查看和导出补货建议，不能创建采购草稿。")} /> : null}
      <Space wrap size={[16, 12]} className="tm-replenishment-filters" style={{ width: '100%' }}>
        <Select aria-label={tr("目标仓库")} showSearch optionFilterProp="label" placeholder={tr("请选择目标仓库（必选）")} loading={warehouseLoading} disabled={!warehouseLoading && !warehouses.length} value={warehouseId || undefined} style={{ minWidth: 260 }} options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} onChange={(value) => { invalidateSelection(); setWarehouseId(value); }} />
        <Input.Search aria-label={tr("搜索商品或规格")} allowClear placeholder={tr("搜索商品标题、规格编码或名称")} style={{ width: 300, maxWidth: '100%' }} onSearch={(value) => { invalidateSelection(); setKeyword(value); }} onChange={(event) => { if (!event.target.value) { invalidateSelection(); setKeyword(''); } }} />
        <Select aria-label={tr("建议状态")} allowClear placeholder={tr("全部状态")} value={status || undefined} style={{ width: 170 }} options={statusOptions} onChange={(value) => { invalidateSelection(); setStatus(value || ''); }} />
        {warehouseId ? <Typography.Text type="secondary">{tr("当前仓库：")}{warehouseLabel}</Typography.Text> : null}
      </Space>
      {!warehouseLoading && !warehouseError && !warehouses.length ? <Alert type="warning" showIcon message={tr("暂无启用仓库，维护仓库后才能查看补货建议。")} /> : null}
      {!warehouseId ? <Alert type="warning" showIcon message={tr("必须选择目标仓库后才会加载建议。")} /> : null}
      {listError ? <ErrorAlert title={tr(listError)} actionHint={<Button onClick={() => actionRef.current?.reload()}>{tr("重新加载")}</Button>} /> : null}
      <TmProTable<ReplenishmentSuggestion>
        actionRef={actionRef}
        rowKey="productSkuId"
        columns={columns}
        search={false}
        cardBordered
        scroll={{ x: 1500 }}
        rowSelection={canManage ? {
          selectedRowKeys: selectedRows.map((row) => row.productSkuId),
          onChange: (_, rows) => setSelectedRows(rows.filter((row) => row.warehouseId === warehouseId && canCreateDraft(row))),
          getCheckboxProps: (row) => ({
            disabled: !canCreateDraft(row),
            'aria-label': trFormat("选择 {value0}", { value0: String(row.skuCode || row.skuName || row.productSkuId) }),
          }),
        } : undefined}
        locale={{ emptyText: warehouseId ? (listError ? tr('补货建议暂不可用') : tr('暂无符合条件的规格')) : tr('请选择目标仓库') }}
        request={async (params) => {
          const isCurrent = suggestionsRead.begin();
          if (!warehouseId || !canRead) return { data: [], total: 0, success: true };
          try {
            const result = await queryReplenishmentSuggestions({ warehouseId, keyword, status, page: params.current, pageSize: params.pageSize });
            if (!isCurrent()) return { data: [], total: 0, success: false };
            setListError('');
            return { data: result.list ?? [], total: result.total ?? 0, success: true };
          } catch (error) {
            if (isCurrent()) setListError((error as Error)?.message || '补货建议加载失败，请稍后重试。');
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
      />
      <CreateDraftModal
        open={draftModalOpen}
        warehouseId={warehouseId}
        warehouseLabel={warehouseLabel}
        rows={selectedRows}
        idempotencyKey={draftIdempotencyKey}
        submitting={draftSubmitting}
        onCancel={() => {
          if (draftSubmittingRef.current) return;
          draftScopeRef.current = undefined;
          draftCommandRef.current = undefined;
          setDraftModalOpen(false);
        }}
        onSubmit={(body) => void createDraft(body)}
      />
    </TmPageContainer>
  </PermissionGuard>;
}
