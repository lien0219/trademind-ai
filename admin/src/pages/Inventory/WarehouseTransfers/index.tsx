import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { EyeOutlined, PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Col, Drawer, Form, Input, InputNumber, Modal, Row, Select, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { usePermission } from '@/hooks/usePermission';
import {
  actOnWarehouseTransfer,
  createInventoryIdempotencyKey,
  createWarehouseTransfer,
  getWarehouseTransfer,
  listInventoryWarehouses,
  queryInventoryCenter,
  queryWarehouseTransfers,
  type InventoryCenterRow,
  type InventoryWarehouse,
  type WarehouseTransfer,
} from '@/services/inventory';
import { PERMISSIONS } from '@/utils/permission';

const STATUS_META: Record<string, { label: () => string; color: string }> = {
  draft: { label: () => tr("草稿"), color: 'default' },
  pending_approval: { label: () => tr("待审批"), color: 'gold' },
  approved: { label: () => tr("已审批"), color: 'blue' },
  in_transit: { label: () => tr("运输中"), color: 'processing' },
  received: { label: () => tr("已收货"), color: 'success' },
  cancelled: { label: () => tr("已取消"), color: 'error' },
};

type FormValues = { sourceWarehouseId: string; targetWarehouseId: string; productSkuId: string; quantity: number; reason?: string; remark?: string };

export default function WarehouseTransfersPage() {
  const language = useLanguage();
  const { can, readonly } = usePermission();
  const canOperate = !readonly && can(PERMISSIONS.INVENTORY_OPERATE);
  const canApprove = !readonly && can(PERMISSIONS.INVENTORY_APPROVE);
  const actionRef = useRef<ActionType>();
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const [skuRows, setSkuRows] = useState<InventoryCenterRow[]>([]);
  const [form] = Form.useForm<FormValues>();
  const [createOpen, setCreateOpen] = useState(false);
  const [detail, setDetail] = useState<WarehouseTransfer>();
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [status, setStatus] = useState<string>();

  const loadFormOptions = useCallback(async () => {
    try {
      const [warehouseResult, skuResult] = await Promise.all([
        listInventoryWarehouses(),
        queryInventoryCenter({ page: 1, pageSize: 100 }),
      ]);
      setWarehouses((warehouseResult.list ?? []).filter((item) => item.status === 'active'));
      setSkuRows(skuResult.list ?? []);
    } catch (nextError) {
      message.error((nextError as Error)?.message || tr('调拨表单数据加载失败'));
    }
  }, []);

  useEffect(() => { void loadFormOptions(); }, [loadFormOptions]);
  useEffect(() => { void actionRef.current?.reload(); }, [status]);

  const initializeCreateForm = () => {
    const active = warehouses.filter((row) => row.status === 'active');
    form.resetFields();
    form.setFieldsValue({ sourceWarehouseId: active.find((row) => row.isDefault)?.id, quantity: 1 });
  };

  const openCreate = () => {
    setCreateOpen(true);
    void loadFormOptions();
  };

  const submitCreate = async (values: FormValues) => {
    setSubmitting(true);
    try {
      await createWarehouseTransfer({
        idempotencyKey: createInventoryIdempotencyKey('warehouse-transfer-create'),
        sourceWarehouseId: values.sourceWarehouseId,
        targetWarehouseId: values.targetWarehouseId,
        reason: values.reason?.trim(),
        remark: values.remark?.trim(),
        items: [{ productSkuId: values.productSkuId, quantity: values.quantity }],
      });
      message.success(tr("调拨单已创建"));
      setCreateOpen(false);
      await actionRef.current?.reload();
    } catch (nextError) {
      message.error((nextError as Error)?.message || tr('调拨单创建失败'));
    } finally { setSubmitting(false); }
  };

  const openDetail = async (row: WarehouseTransfer) => {
    setLoadingDetail(true);
    try { setDetail(await getWarehouseTransfer(row.id)); } catch (nextError) { message.error((nextError as Error)?.message || tr('调拨详情加载失败')); } finally { setLoadingDetail(false); }
  };

  const action = (row: WarehouseTransfer, name: 'submit' | 'approve' | 'dispatch' | 'receive' | 'cancel', label: string, danger = false) => {
    Modal.confirm({ title: trFormat("{value0}调拨单？", { value0: label }), content: trFormat("调拨单 {value0} 当前状态为{value1}。", { value0: row.transferNo, value1: STATUS_META[row.status]?.label() ?? row.status }), okText: label, cancelText: tr("取消"), okButtonProps: { danger }, onOk: async () => {
      setSubmitting(true);
      try { await actOnWarehouseTransfer(row.id, name, { expectedRevision: row.revision, idempotencyKey: createInventoryIdempotencyKey(`warehouse-transfer-${name}`) }); message.success(trFormat("调拨单已{value0}", { value0: label })); await actionRef.current?.reload(); if (detail?.id === row.id) await openDetail(row); } catch (nextError) { message.error((nextError as Error)?.message || trFormat("调拨单{value0}失败", { value0: label })); throw nextError; } finally { setSubmitting(false); }
    } });
  };

  const columns: ProColumns<WarehouseTransfer>[] = [
    { title: tr("调拨单号"), dataIndex: 'transferNo', copyable: true, ellipsis: true, width: 170 },
    { title: tr("调出仓"), dataIndex: 'sourceWarehouseName', render: (_, row) => `${row.sourceWarehouseCode ?? ''} · ${row.sourceWarehouseName ?? row.sourceWarehouseId}` },
    { title: tr("调入仓"), dataIndex: 'targetWarehouseName', render: (_, row) => `${row.targetWarehouseCode ?? ''} · ${row.targetWarehouseName ?? row.targetWarehouseId}` },
    { title: tr("明细"), dataIndex: 'itemCount', width: 70, search: false },
    { title: tr("状态"), dataIndex: 'status', width: 105, search: false, render: (_, row) => { const meta = STATUS_META[row.status]; return <Tag color={meta?.color ?? 'default'}>{meta?.label() ?? (row.status || tr('未知'))}</Tag>; } },
    { title: tr("创建时间"), dataIndex: 'createdAt', valueType: 'dateTime', width: 170, search: false },
    { title: tr("操作"), valueType: 'option', width: 280, render: (_, row) => <Space wrap size={4}>
      <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => void openDetail(row)}>{tr("查看")}</Button>
      {canOperate && row.status === 'draft' ? <Button type="link" size="small" onClick={() => action(row, 'submit', tr('提交'))}>{tr("提交")}</Button> : null}
      {canApprove && row.status === 'pending_approval' ? <Button type="link" size="small" onClick={() => action(row, 'approve', tr('审批'))}>{tr("审批")}</Button> : null}
      {canOperate && row.status === 'approved' ? <Button type="link" size="small" onClick={() => action(row, 'dispatch', tr('发出'))}>{tr("发出")}</Button> : null}
      {canOperate && row.status === 'in_transit' ? <Button type="link" size="small" onClick={() => action(row, 'receive', tr('收货'))}>{tr("收货")}</Button> : null}
      {canOperate && ['draft', 'pending_approval', 'approved'].includes(row.status) ? <Button danger type="link" size="small" onClick={() => action(row, 'cancel', tr('取消'), true)}>{tr("取消")}</Button> : null}
    </Space> },
  ];

  const skuOptions = useMemo(() => skuRows.map((row) => ({ value: row.productSkuId, label: trFormat("{value0} · {value1}", { value0: row.skuCode || row.productSkuId, value1: row.productTitle || tr('未命名商品') }), search: `${row.skuCode} ${row.skuName} ${row.productTitle} ${row.productSkuId}` })), [skuRows, language]);

  return <PermissionGuard require={PERMISSIONS.INVENTORY_VIEW} showForbiddenPage>
    <TmPageContainer title={tr("仓库调拨")} subTitle={tr("在租户仓库之间移动库存，发出和收货均记录不可变库存流水。")} extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canOperate} onClick={openCreate}>{tr("新建调拨")}</Button></TmPageHeaderExtra>}>
      {!canOperate ? <Alert type="info" showIcon message={tr("当前账号为只读模式，可查看调拨记录但不能执行调拨动作。")} style={{ marginBottom: 16 }} /> : null}
      {error ? <ErrorAlert title={error} /> : null}
      <TmProTable<WarehouseTransfer> rowKey="id" actionRef={actionRef} columns={columns} search={false} scroll={{ x: 1120 }} cardBordered locale={{ emptyText: tr('暂无仓库调拨记录') }} filterBar={<Select aria-label={tr("调拨状态")} allowClear placeholder={tr("全部状态")} value={status} style={{ width: 140 }} options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.label() }))} onChange={setStatus} />} request={async (params) => {
        try { const result = await queryWarehouseTransfers({ page: params.current, pageSize: params.pageSize, status }); setError(''); return { data: result.list ?? [], success: true, total: result.total ?? 0 }; } catch (nextError) { const msg = (nextError as Error)?.message || tr('调拨列表加载失败'); setError(msg); message.error(msg); return { data: [], success: false, total: 0 }; }
      }} />
      <Modal title={tr("新建仓库调拨")} open={createOpen} width={560} confirmLoading={submitting} okText={tr("创建调拨单")} cancelText={tr("取消")} onCancel={() => !submitting && setCreateOpen(false)} onOk={() => form.submit()} afterOpenChange={(open) => { if (open) initializeCreateForm(); }} destroyOnHidden>
        <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void submitCreate(values)}>
          <Row gutter={16}>
            <Col xs={24} sm={12} data-testid="source-warehouse-field">
              <Form.Item label={tr("调出仓")} name="sourceWarehouseId" rules={[{ required: true, message: tr("请选择调出仓") }]}>
                <Select showSearch optionFilterProp="label" placeholder={tr("请选择调出仓")} style={{ width: '100%' }} options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} data-testid="target-warehouse-field">
              <Form.Item label={tr("调入仓")} name="targetWarehouseId" rules={[{ required: true, message: tr("请选择调入仓") }]}>
                <Select showSearch optionFilterProp="label" placeholder={tr("请选择调入仓")} style={{ width: '100%' }} options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label={tr("商品规格")} name="productSkuId" rules={[{ required: true, message: tr("请选择商品规格") }]}><Select showSearch optionFilterProp="search" options={skuOptions} placeholder={tr("选择库存中心中的商品规格")} /></Form.Item>
          <Form.Item label={tr("调拨数量")} name="quantity" rules={[{ required: true, type: 'number', min: 1, message: tr("请输入大于 0 的数量") }]}><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label={tr("原因")} name="reason"><Input maxLength={128} placeholder={tr("例如：仓间补货")} /></Form.Item>
          <Form.Item label={tr("备注")} name="remark"><Input.TextArea maxLength={520} rows={3} /></Form.Item>
        </Form>
      </Modal>
      <Drawer title={detail ? `${tr("调拨详情")} · ${detail.transferNo}` : tr("调拨详情")} open={Boolean(detail)} onClose={() => setDetail(undefined)} loading={loadingDetail} width={520}>
        {detail ? <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Typography.Text>{tr("状态：")}<Tag color={STATUS_META[detail.status]?.color}>{STATUS_META[detail.status]?.label() ?? detail.status}</Tag></Typography.Text>
          <Typography.Text>{tr("调出仓：")}{detail.sourceWarehouseName ?? detail.sourceWarehouseId}</Typography.Text>
          <Typography.Text>{tr("调入仓：")}{detail.targetWarehouseName ?? detail.targetWarehouseId}</Typography.Text>
          <Typography.Text>{tr("版本：")}{detail.revision}</Typography.Text>
          {detail.reason ? <Typography.Text>{tr("原因：")}{detail.reason}</Typography.Text> : null}
          <Typography.Text strong>{tr("调拨明细")}</Typography.Text>
          {(detail.items ?? []).map((item) => <Typography.Paragraph key={item.id} style={{ marginBottom: 0 }}>{trFormat('{value0}：{value1} 件，已收 {value2}', { value0: item.skuCode || item.productSkuId, value1: item.quantity, value2: item.receivedQuantity })}</Typography.Paragraph>)}
        </Space> : null}
      </Drawer>
    </TmPageContainer>
  </PermissionGuard>;
}
