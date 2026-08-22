import { EyeOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Col, Drawer, Form, Input, InputNumber, Modal, Row, Select, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { usePermission } from '@/hooks/usePermission';
import {
  actOnInventoryStocktake,
  createInventoryIdempotencyKey,
  createInventoryStocktake,
  getInventoryStocktake,
  listInventoryWarehouses,
  queryInventoryCenter,
  queryInventoryStocktakes,
  updateInventoryStocktakeItem,
  type InventoryCenterRow,
  type InventoryStocktake,
  type InventoryWarehouse,
} from '@/services/inventory';
import { PERMISSIONS } from '@/utils/permission';

const STATUS_META: Record<string, { label: string; color: string }> = {
  counting: { label: '盘点中', color: 'processing' },
  pending_review: { label: '待审核', color: 'gold' },
  approved: { label: '待过账', color: 'blue' },
  posted: { label: '已过账', color: 'success' },
  cancelled: { label: '已取消', color: 'error' },
};

type CreateValues = { warehouseId: string; productSkuId: string; reason?: string; remark?: string };

export default function StocktakesPage() {
  const { can, readonly } = usePermission();
  const canOperate = !readonly && can(PERMISSIONS.INVENTORY_OPERATE);
  const canApprove = !readonly && can(PERMISSIONS.INVENTORY_APPROVE);
  const actionRef = useRef<ActionType>();
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const [skuRows, setSkuRows] = useState<InventoryCenterRow[]>([]);
  const [detail, setDetail] = useState<InventoryStocktake>();
  const [createOpen, setCreateOpen] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [countDrafts, setCountDrafts] = useState<Record<string, number | null>>({});
  const [error, setError] = useState('');
  const [status, setStatus] = useState<string>();
  const [form] = Form.useForm<CreateValues>();

  const loadOptions = useCallback(async () => {
    try {
      const [warehouseResult, skuResult] = await Promise.all([
        listInventoryWarehouses(),
        queryInventoryCenter({ page: 1, pageSize: 100 }),
      ]);
      setWarehouses((warehouseResult.list ?? []).filter((row) => row.status === 'active'));
      setSkuRows(skuResult.list ?? []);
    } catch (nextError) {
      message.error((nextError as Error)?.message || '盘点表单数据加载失败');
    }
  }, []);

  useEffect(() => { void loadOptions(); }, [loadOptions]);
  useEffect(() => { void actionRef.current?.reload(); }, [status]);

  const openCreate = () => {
    form.resetFields();
    const defaultWarehouse = warehouses.find((row) => row.isDefault)?.id;
    form.setFieldsValue(defaultWarehouse ? { warehouseId: defaultWarehouse } : {});
    setCreateOpen(true);
    void loadOptions();
  };

  const submitCreate = async (values: CreateValues) => {
    setSubmitting(true);
    try {
      await createInventoryStocktake({
        idempotencyKey: createInventoryIdempotencyKey('stocktake-create'),
        warehouseId: values.warehouseId,
        reason: values.reason?.trim(),
        remark: values.remark?.trim(),
        items: [{ productSkuId: values.productSkuId }],
      });
      message.success('盘点单已创建');
      setCreateOpen(false);
      await actionRef.current?.reload();
    } catch (nextError) {
      message.error((nextError as Error)?.message || '盘点单创建失败');
    } finally { setSubmitting(false); }
  };

  const openDetail = async (row: InventoryStocktake) => {
    setLoadingDetail(true);
    try {
      const next = await getInventoryStocktake(row.id);
      setDetail(next);
      setCountDrafts(Object.fromEntries((next.items ?? []).map((item) => [item.id, item.countedOnHand ?? null])));
    }
    catch (nextError) { message.error((nextError as Error)?.message || '盘点详情加载失败'); }
    finally { setLoadingDetail(false); }
  };

  const updateCount = async (item: NonNullable<InventoryStocktake['items']>[number]) => {
    const value = countDrafts[item.id];
    if (!detail || detail.status !== 'counting' || typeof value !== 'number' || !Number.isInteger(value) || value < 0) return;
    setSubmitting(true);
    try {
      const next = await updateInventoryStocktakeItem(detail.id, item.id, {
        expectedRevision: detail.revision,
        idempotencyKey: createInventoryIdempotencyKey('stocktake-count'),
        countedOnHand: value,
      });
      setDetail(next);
      setCountDrafts(Object.fromEntries((next.items ?? []).map((nextItem) => [nextItem.id, nextItem.countedOnHand ?? null])));
      message.success('盘点数量已保存');
    } catch (nextError) { message.error((nextError as Error)?.message || '盘点数量保存失败'); }
    finally { setSubmitting(false); }
  };

  const action = (row: InventoryStocktake, name: 'submit' | 'approve' | 'post' | 'cancel', label: string, danger = false) => {
    Modal.confirm({
      title: `${label}${name === 'post' ? '盘点结果' : '盘点单'}？`,
      content: `盘点单 ${row.stocktakeNo} 当前状态为${STATUS_META[row.status]?.label ?? row.status}。`,
      okText: label,
      cancelText: '取消',
      okButtonProps: { danger },
      onOk: async () => {
        setSubmitting(true);
        try {
          const next = await actOnInventoryStocktake(row.id, name, { expectedRevision: row.revision, idempotencyKey: createInventoryIdempotencyKey(`stocktake-${name}`) });
          message.success(`盘点单已${label}`);
          await actionRef.current?.reload();
          if (detail?.id === row.id) setDetail(next);
        } catch (nextError) { message.error((nextError as Error)?.message || `盘点单${label}失败`); throw nextError; }
        finally { setSubmitting(false); }
      },
    });
  };

  const columns: ProColumns<InventoryStocktake>[] = [
    { title: '盘点单号', dataIndex: 'stocktakeNo', copyable: true, ellipsis: true, width: 170 },
    { title: '仓库', dataIndex: 'warehouseName', render: (_, row) => `${row.warehouseCode ?? ''} · ${row.warehouseName ?? row.warehouseId}` },
    { title: '明细', dataIndex: 'itemCount', width: 70, search: false },
    { title: '状态', dataIndex: 'status', width: 105, search: false, render: (_, row) => { const meta = STATUS_META[row.status] ?? { label: row.status || '未知', color: 'default' }; return <Tag color={meta.color}>{meta.label}</Tag>; } },
    { title: '创建时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 170, search: false },
    { title: '操作', valueType: 'option', width: 320, render: (_, row) => <Space wrap size={4}>
      <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => void openDetail(row)}>查看</Button>
      {canOperate && row.status === 'counting' ? <Button type="link" size="small" onClick={() => action(row, 'submit', '提交审核')}>提交审核</Button> : null}
      {canApprove && row.status === 'pending_review' ? <Button type="link" size="small" onClick={() => action(row, 'approve', '审核通过')}>审核通过</Button> : null}
      {canOperate && row.status === 'approved' ? <Button type="link" size="small" onClick={() => action(row, 'post', '过账')}>过账</Button> : null}
      {canOperate && ['counting', 'pending_review', 'approved'].includes(row.status) ? <Button danger type="link" size="small" onClick={() => action(row, 'cancel', '取消', true)}>取消</Button> : null}
    </Space> },
  ];

  const skuOptions = skuRows.map((row) => ({ value: row.productSkuId, label: `${row.skuCode || row.productSkuId} · ${row.productTitle || '未命名商品'}` }));
  const items = detail?.items ?? [];
  const hasUnsavedCounts = items.some((item) => (countDrafts[item.id] ?? null) !== (item.countedOnHand ?? null));

  return <PermissionGuard require={PERMISSIONS.INVENTORY_VIEW} showForbiddenPage>
    <TmPageContainer title="库存盘点" subTitle="记录仓库实盘数量，审核后过账差异并保留不可变库存流水。" extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canOperate} onClick={openCreate}>新建盘点</Button></TmPageHeaderExtra>}>
      {!canOperate ? <Alert type="info" showIcon message="当前账号为只读模式，可查看盘点记录但不能创建或修改盘点。" style={{ marginBottom: 16 }} /> : null}
      {error ? <ErrorAlert title={error} /> : null}
      <TmProTable<InventoryStocktake> rowKey="id" actionRef={actionRef} columns={columns} search={false} scroll={{ x: 1120 }} cardBordered locale={{ emptyText: '暂无库存盘点记录' }} toolBarRender={() => [<Select key="status" allowClear placeholder="全部状态" value={status} style={{ width: 140 }} options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.label }))} onChange={setStatus} />]} request={async (params) => {
        try { const result = await queryInventoryStocktakes({ page: params.current, pageSize: params.pageSize, status }); setError(''); return { data: result.list ?? [], success: true, total: result.total ?? 0 }; }
        catch (nextError) { const msg = (nextError as Error)?.message || '盘点列表加载失败'; setError(msg); message.error(msg); return { data: [], success: false, total: 0 }; }
      }} />
      <Modal title="新建库存盘点" open={createOpen} width={560} confirmLoading={submitting} okText="创建盘点单" cancelText="取消" onCancel={() => !submitting && setCreateOpen(false)} onOk={() => form.submit()} destroyOnHidden>
        <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void submitCreate(values)}>
          <Form.Item label="盘点仓库" name="warehouseId" rules={[{ required: true, message: '请选择盘点仓库' }]}><Select showSearch optionFilterProp="label" placeholder="请选择盘点仓库" options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} /></Form.Item>
          <Form.Item label="商品规格" name="productSkuId" rules={[{ required: true, message: '请选择商品规格' }]}><Select showSearch optionFilterProp="label" options={skuOptions} placeholder="选择需要盘点的商品规格" /></Form.Item>
          <Row gutter={16}><Col xs={24} sm={12}><Form.Item label="盘点原因" name="reason"><Input maxLength={128} placeholder="例如：月末盘点" /></Form.Item></Col><Col xs={24} sm={12}><Form.Item label="备注" name="remark"><Input maxLength={520} /></Form.Item></Col></Row>
        </Form>
      </Modal>
      <Drawer title={detail ? `盘点详情 · ${detail.stocktakeNo}` : '盘点详情'} open={Boolean(detail)} onClose={() => { setDetail(undefined); setCountDrafts({}); }} loading={loadingDetail} width={620}>
        {detail ? <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Typography.Text>状态：<Tag color={STATUS_META[detail.status]?.color}>{STATUS_META[detail.status]?.label ?? detail.status}</Tag></Typography.Text>
          <Typography.Text>仓库：{detail.warehouseCode ?? ''} · {detail.warehouseName ?? detail.warehouseId}</Typography.Text>
          <Typography.Text>版本：{detail.revision}</Typography.Text>
          {detail.reason ? <Typography.Text>原因：{detail.reason}</Typography.Text> : null}
          <Typography.Text type="secondary">盘点过账只调整在手库存；预占、残损和在途数量由库存账控制。</Typography.Text>
          {items.map((item) => <Row key={item.id} gutter={[12, 8]} align="middle" style={{ borderBottom: '1px solid var(--ant-color-border-secondary)', paddingBottom: 10 }}>
            <Col xs={24} sm={14}><Typography.Text strong>{item.skuCode || item.productSkuId}</Typography.Text><br /><Typography.Text type="secondary">账面 {item.snapshotOnHand}，预占 {item.snapshotReserved}，残损 {item.snapshotDamaged}</Typography.Text></Col>
            <Col xs={24} sm={10}><Space.Compact block><InputNumber aria-label={`${item.skuCode || item.productSkuId}实盘数量`} min={0} precision={0} disabled={!canOperate || detail.status !== 'counting' || submitting} value={countDrafts[item.id]} placeholder="实盘数量" style={{ width: '100%' }} onChange={(value) => setCountDrafts((current) => ({ ...current, [item.id]: value }))} /><Button aria-label={`保存${item.skuCode || item.productSkuId}实盘数量`} icon={<SaveOutlined />} disabled={!canOperate || detail.status !== 'counting' || submitting || !Number.isInteger(countDrafts[item.id]) || countDrafts[item.id] === item.countedOnHand} onClick={() => void updateCount(item)}>保存</Button></Space.Compact></Col>
          </Row>)}
          <Space wrap>
            {canOperate && detail.status === 'counting' ? <Button disabled={hasUnsavedCounts} onClick={() => action(detail, 'submit', '提交审核')}>提交审核</Button> : null}
            {canApprove && detail.status === 'pending_review' ? <Button onClick={() => action(detail, 'approve', '审核通过')}>审核通过</Button> : null}
            {canOperate && detail.status === 'approved' ? <Button type="primary" onClick={() => action(detail, 'post', '过账')}>过账</Button> : null}
          </Space>
        </Space> : null}
      </Drawer>
    </TmPageContainer>
  </PermissionGuard>;
}
