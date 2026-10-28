import { tr, trFormat } from '@/i18n';
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

const STATUS_META: Record<string, { label: () => string; color: string }> = {
  counting: { label: () => tr("盘点中"), color: 'processing' },
  pending_review: { label: () => tr("待审核"), color: 'gold' },
  approved: { label: () => tr("待过账"), color: 'blue' },
  posted: { label: () => tr("已过账"), color: 'success' },
  cancelled: { label: () => tr("已取消"), color: 'error' },
};

const ACTION_LABEL: Record<'submit' | 'approve' | 'post' | 'cancel', () => string> = {
  submit: () => tr("提交审核"),
  approve: () => tr("审核通过"),
  post: () => tr("过账"),
  cancel: () => tr("取消"),
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
      message.error((nextError as Error)?.message || tr('盘点表单数据加载失败'));
    }
  }, []);

  useEffect(() => { void loadOptions(); }, [loadOptions]);
  useEffect(() => { void actionRef.current?.reload(); }, [status]);

  const initializeCreateForm = () => {
    form.resetFields();
    const defaultWarehouse = warehouses.find((row) => row.isDefault)?.id;
    form.setFieldsValue(defaultWarehouse ? { warehouseId: defaultWarehouse } : {});
  };

  const openCreate = () => {
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
      message.success(tr("盘点单已创建"));
      setCreateOpen(false);
      await actionRef.current?.reload();
    } catch (nextError) {
      message.error((nextError as Error)?.message || tr('盘点单创建失败'));
    } finally { setSubmitting(false); }
  };

  const openDetail = async (row: InventoryStocktake) => {
    setLoadingDetail(true);
    try {
      const next = await getInventoryStocktake(row.id);
      setDetail(next);
      setCountDrafts(Object.fromEntries((next.items ?? []).map((item) => [item.id, item.countedOnHand ?? null])));
    }
    catch (nextError) { message.error((nextError as Error)?.message || tr('盘点详情加载失败')); }
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
      message.success(tr("盘点数量已保存"));
    } catch (nextError) { message.error((nextError as Error)?.message || tr('盘点数量保存失败')); }
    finally { setSubmitting(false); }
  };

  const action = (row: InventoryStocktake, name: 'submit' | 'approve' | 'post' | 'cancel', danger = false) => {
    const label = ACTION_LABEL[name]();
    Modal.confirm({
      title: trFormat("{value0}{value1}？", { value0: label, value1: tr(name === 'post' ? '盘点结果' : '盘点单') }),
      content: trFormat("盘点单 {value0} 当前状态为{value1}。", { value0: row.stocktakeNo, value1: STATUS_META[row.status]?.label() ?? row.status }),
      okText: tr(label),
      cancelText: tr("取消"),
      okButtonProps: { danger },
      onOk: async () => {
        setSubmitting(true);
        try {
          const next = await actOnInventoryStocktake(row.id, name, { expectedRevision: row.revision, idempotencyKey: createInventoryIdempotencyKey(`stocktake-${name}`) });
          message.success(trFormat("盘点单已{value0}", { value0: label }));
          await actionRef.current?.reload();
          if (detail?.id === row.id) setDetail(next);
        } catch (nextError) { message.error((nextError as Error)?.message || trFormat('盘点单{value0}失败', { value0: label })); throw nextError; }
        finally { setSubmitting(false); }
      },
    });
  };

  const columns: ProColumns<InventoryStocktake>[] = [
    { title: tr("盘点单号"), dataIndex: 'stocktakeNo', copyable: true, ellipsis: true, width: 170 },
    { title: tr("仓库"), dataIndex: 'warehouseName', render: (_, row) => `${row.warehouseCode ?? ''} · ${row.warehouseName ?? row.warehouseId}` },
    { title: tr("明细"), dataIndex: 'itemCount', width: 70, search: false },
    { title: tr("状态"), dataIndex: 'status', width: 105, search: false, render: (_, row) => { const meta = STATUS_META[row.status]; return <Tag color={meta?.color ?? 'default'}>{meta?.label() ?? (row.status || tr('未知'))}</Tag>; } },
    { title: tr("创建时间"), dataIndex: 'createdAt', valueType: 'dateTime', width: 170, search: false },
    { title: tr("操作"), valueType: 'option', width: 320, render: (_, row) => <Space wrap size={4}>
      <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => void openDetail(row)}>{tr("查看")}</Button>
      {canOperate && row.status === 'counting' ? <Button type="link" size="small" onClick={() => action(row, 'submit')}>{tr("提交审核")}</Button> : null}
      {canApprove && row.status === 'pending_review' ? <Button type="link" size="small" onClick={() => action(row, 'approve')}>{tr("审核通过")}</Button> : null}
      {canOperate && row.status === 'approved' ? <Button type="link" size="small" onClick={() => action(row, 'post')}>{tr("过账")}</Button> : null}
      {canOperate && ['counting', 'pending_review', 'approved'].includes(row.status) ? <Button danger type="link" size="small" onClick={() => action(row, 'cancel', true)}>{tr("取消")}</Button> : null}
    </Space> },
  ];

  const skuOptions = skuRows.map((row) => ({ value: row.productSkuId, label: trFormat("{value0} · {value1}", { value0: row.skuCode || row.productSkuId, value1: row.productTitle || tr('未命名商品') }) }));
  const items = detail?.items ?? [];
  const hasUnsavedCounts = items.some((item) => (countDrafts[item.id] ?? null) !== (item.countedOnHand ?? null));

  return <PermissionGuard require={PERMISSIONS.INVENTORY_VIEW} showForbiddenPage>
    <TmPageContainer title={tr("库存盘点")} subTitle={tr("记录仓库实盘数量，审核后过账差异并保留不可变库存流水。")} extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canOperate} onClick={openCreate}>{tr("新建盘点")}</Button></TmPageHeaderExtra>}>
      {!canOperate ? <Alert type="info" showIcon message={tr("当前账号为只读模式，可查看盘点记录但不能创建或修改盘点。")} style={{ marginBottom: 16 }} /> : null}
      {error ? <ErrorAlert title={error} /> : null}
      <TmProTable<InventoryStocktake> rowKey="id" actionRef={actionRef} columns={columns} search={false} scroll={{ x: 1120 }} cardBordered locale={{ emptyText: tr("暂无库存盘点记录") }} filterBar={<Select aria-label={tr("盘点状态")} allowClear placeholder={tr("全部状态")} value={status} style={{ width: 140 }} options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.label() }))} onChange={setStatus} />} request={async (params) => {
        try { const result = await queryInventoryStocktakes({ page: params.current, pageSize: params.pageSize, status }); setError(''); return { data: result.list ?? [], success: true, total: result.total ?? 0 }; }
        catch (nextError) { const msg = (nextError as Error)?.message || tr('盘点列表加载失败'); setError(msg); message.error(msg); return { data: [], success: false, total: 0 }; }
      }} />
      <Modal title={tr("新建库存盘点")} open={createOpen} width={560} confirmLoading={submitting} okText={tr("创建盘点单")} cancelText={tr("取消")} onCancel={() => !submitting && setCreateOpen(false)} onOk={() => form.submit()} afterOpenChange={(open) => { if (open) initializeCreateForm(); }} destroyOnHidden>
        <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void submitCreate(values)}>
          <Form.Item label={tr("盘点仓库")} name="warehouseId" rules={[{ required: true, message: tr("请选择盘点仓库") }]}><Select showSearch optionFilterProp="label" placeholder={tr("请选择盘点仓库")} options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} /></Form.Item>
          <Form.Item label={tr("商品规格")} name="productSkuId" rules={[{ required: true, message: tr("请选择商品规格") }]}><Select showSearch optionFilterProp="label" options={skuOptions} placeholder={tr("选择需要盘点的商品规格")} /></Form.Item>
          <Row gutter={16}><Col xs={24} sm={12}><Form.Item label={tr("盘点原因")} name="reason"><Input maxLength={128} placeholder={tr("例如：月末盘点")} /></Form.Item></Col><Col xs={24} sm={12}><Form.Item label={tr("备注")} name="remark"><Input maxLength={520} /></Form.Item></Col></Row>
        </Form>
      </Modal>
      <Drawer title={detail ? `${tr("盘点详情")} · ${detail.stocktakeNo}` : tr("盘点详情")} open={Boolean(detail)} onClose={() => { setDetail(undefined); setCountDrafts({}); }} loading={loadingDetail} width={620}>
        {detail ? <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Typography.Text>{tr("状态：")}<Tag color={STATUS_META[detail.status]?.color}>{STATUS_META[detail.status]?.label() ?? detail.status}</Tag></Typography.Text>
          <Typography.Text>{tr("仓库：")}{detail.warehouseCode ?? ''} · {detail.warehouseName ?? detail.warehouseId}</Typography.Text>
          <Typography.Text>{tr("版本：")}{detail.revision}</Typography.Text>
          {detail.reason ? <Typography.Text>{tr("原因：")}{detail.reason}</Typography.Text> : null}
          <Typography.Text type="secondary">{tr("盘点过账只调整在手库存；预占、残损和在途数量由库存账控制。")}</Typography.Text>
          {items.map((item) => <Row key={item.id} gutter={[12, 8]} align="middle" style={{ borderBottom: '1px solid var(--ant-color-border-secondary)', paddingBottom: 10 }}>
            <Col xs={24} sm={14}><Typography.Text strong>{item.skuCode || item.productSkuId}</Typography.Text><br /><Typography.Text type="secondary">{tr("账面")} {item.snapshotOnHand}{tr("，预占")} {item.snapshotReserved}{tr("，残损")} {item.snapshotDamaged}</Typography.Text></Col>
            <Col xs={24} sm={10}><Space.Compact block><InputNumber aria-label={trFormat("{value0}实盘数量", { value0: item.skuCode || item.productSkuId })} min={0} precision={0} disabled={!canOperate || detail.status !== 'counting' || submitting} value={countDrafts[item.id]} placeholder={tr("实盘数量")} style={{ width: '100%' }} onChange={(value) => setCountDrafts((current) => ({ ...current, [item.id]: value }))} /><Button aria-label={trFormat("保存{value0}实盘数量", { value0: item.skuCode || item.productSkuId })} icon={<SaveOutlined />} disabled={!canOperate || detail.status !== 'counting' || submitting || !Number.isInteger(countDrafts[item.id]) || countDrafts[item.id] === item.countedOnHand} onClick={() => void updateCount(item)}>{tr("保存")}</Button></Space.Compact></Col>
          </Row>)}
          <Space wrap>
            {canOperate && detail.status === 'counting' ? <Button disabled={hasUnsavedCounts} onClick={() => action(detail, 'submit')}>{tr("提交审核")}</Button> : null}
            {canApprove && detail.status === 'pending_review' ? <Button onClick={() => action(detail, 'approve')}>{tr("审核通过")}</Button> : null}
            {canOperate && detail.status === 'approved' ? <Button type="primary" onClick={() => action(detail, 'post')}>{tr("过账")}</Button> : null}
          </Space>
        </Space> : null}
      </Drawer>
    </TmPageContainer>
  </PermissionGuard>;
}
