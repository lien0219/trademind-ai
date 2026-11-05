import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { PlusOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, InputNumber, Modal, Select, Space, Tag, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';
import {
  createProcurementIdempotencyKey,
  createPurchaseOrder,
  extractProcurementAPIError,
  listPurchaseOrders,
  listSupplierSKUs,
  listSuppliers,
  listWarehouses,
  procurementErrorMessage,
  type Supplier,
  type SupplierSKU,
  type Warehouse,
  type PurchaseOrder,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import { idempotentCommandKey, type IdempotentCommandKey } from '@/utils/idempotentCommandKey';
import { formatDateTime } from '@/utils/formatTime';
import { formatMinorAmount, parseMajorAmountToMinor, PURCHASE_ORDER_STATUS } from '../helpers';
import '../index.less';

type PurchaseLineForm = {
  productSkuId: string;
  quantity: number;
  unitCostMajor: string;
};

type PurchaseOrderFormValues = {
  supplierId: string;
  warehouseId: string;
  currency: string;
  remark: string;
  items: PurchaseLineForm[];
};

function statusTag(status: string) {
  const meta = PURCHASE_ORDER_STATUS[status] || { source: "未知状态", color: 'default' };
  return <Tag color={meta.color}>{tr(meta.source)}</Tag>;
}

export default function PurchaseOrdersPage() {
  useLanguage();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.PROCUREMENT_MANAGE);
  const actionRef = useRef<ActionType>();
  const [form] = Form.useForm<PurchaseOrderFormValues>();
  const [masterLoading, setMasterLoading] = useState(true);
  const [masterError, setMasterError] = useState<string>();
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [supplierBindings, setSupplierBindings] = useState<SupplierSKU[]>([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [skuOptions, setSkuOptions] = useState<ProductSkuSearchHit[]>([]);
  const [skuSearching, setSkuSearching] = useState(false);
  const [listError, setListError] = useState<string>();
  const [supplierBindingsLoading, setSupplierBindingsLoading] = useState(false);
  const masterRead = useMemo(createTenantReadGuard, []);
  const supplierBindingsRead = useMemo(createTenantReadGuard, []);
  const skuRead = useMemo(createTenantReadGuard, []);
  const listRead = useMemo(createTenantReadGuard, []);
  const submittingRef = useRef(false);
  const purchaseCommandRef = useRef<IdempotentCommandKey>();

  const loadMasterData = useCallback(async () => {
    const isCurrent = masterRead.begin();
    setMasterLoading(true);
    setMasterError(undefined);
    setWarehouses([]);
    setSuppliers([]);
    try {
      const [warehouseResult, supplierResult] = await Promise.all([listWarehouses(), listSuppliers()]);
      if (!isCurrent()) return;
      setWarehouses(warehouseResult.list || []);
      setSuppliers(supplierResult.list || []);
    } catch (nextError) {
      if (isCurrent()) setMasterError(procurementErrorMessage(extractProcurementAPIError(nextError), '采购基础资料加载失败，请稍后重试。'));
    } finally {
      if (isCurrent()) setMasterLoading(false);
    }
  }, [masterRead]);

  useEffect(() => {
    void loadMasterData();
    return () => masterRead.invalidate();
  }, [loadMasterData, masterRead]);

  const searchSKUs = useCallback(async (query = '') => {
    const isCurrent = skuRead.begin();
    setSkuSearching(true);
    setSkuOptions([]);
    try {
      const result = await searchProductSkus({ keyword: query.trim() || undefined, limit: 50 });
      if (isCurrent()) setSkuOptions(result.list || []);
    } catch {
      if (isCurrent()) setSkuOptions([]);
    } finally {
      if (isCurrent()) setSkuSearching(false);
    }
  }, [skuRead]);

  const activeWarehouses = useMemo(() => warehouses.filter((row) => row.status === 'active'), [warehouses]);
  const activeSuppliers = useMemo(() => suppliers.filter((row) => row.status === 'active'), [suppliers]);
  const warehouseNames = useMemo(() => new Map(warehouses.map((row) => [row.id, row.name])), [warehouses]);
  const supplierNames = useMemo(() => new Map(suppliers.map((row) => [row.id, row.name])), [suppliers]);
  const bindingBySKU = useMemo(() => new Map(supplierBindings.map((row) => [row.productSkuId, row])), [supplierBindings]);

  const changeSupplier = async (supplierId: string) => {
    const isCurrent = supplierBindingsRead.begin();
    form.setFieldValue('items', [{ productSkuId: '', quantity: 1, unitCostMajor: '0.00' }]);
    setSupplierBindings([]);
    setSupplierBindingsLoading(Boolean(supplierId));
    if (!supplierId) return;
    try {
      const result = await listSupplierSKUs(supplierId);
      // A previously selected supplier must never overwrite the active supplier's pricing.
      if (isCurrent() && form.getFieldValue('supplierId') === supplierId) {
        setSupplierBindings(result.list || []);
      }
    } catch (nextError) {
      if (isCurrent() && form.getFieldValue('supplierId') === supplierId) {
        message.error(tr(procurementErrorMessage(extractProcurementAPIError(nextError), tr('供应商采购信息加载失败。'))));
      }
    } finally {
      if (isCurrent()) setSupplierBindingsLoading(false);
    }
  };

  useEffect(() => () => {
    supplierBindingsRead.invalidate();
    skuRead.invalidate();
    listRead.invalidate();
  }, [supplierBindingsRead, skuRead, listRead]);

  const closeCreate = () => {
    purchaseCommandRef.current = undefined;
    supplierBindingsRead.invalidate();
    skuRead.invalidate();
    setSupplierBindingsLoading(false);
    setModalOpen(false);
  };

  const openCreate = () => {
    purchaseCommandRef.current = undefined;
    const defaultWarehouse = activeWarehouses.find((row) => row.isDefault) || activeWarehouses[0];
    setIdempotencyKey(createProcurementIdempotencyKey('purchase-order'));
    setSupplierBindings([]);
    setSkuOptions([]);
    form.setFieldsValue({
      supplierId: activeSuppliers[0]?.id || '',
      warehouseId: defaultWarehouse?.id || '',
      currency: 'CNY',
      remark: '',
      items: [{ productSkuId: '', quantity: 1, unitCostMajor: '0.00' }],
    });
    if (activeSuppliers[0]) void changeSupplier(activeSuppliers[0].id);
    void searchSKUs();
    setModalOpen(true);
  };

  const applyBindingDefaults = (fieldName: number, productSkuId: string) => {
    const binding = bindingBySKU.get(productSkuId);
    if (!binding) return;
    const items = (form.getFieldValue('items') || []).map((item: PurchaseLineForm, index: number) => (
      index === fieldName
        ? {
            ...item,
            productSkuId,
            quantity: Math.max(1, binding.minOrderQty),
            unitCostMajor: (binding.unitCostMinor / 100).toFixed(2),
          }
        : { ...item }
    ));
    form.setFieldsValue({ items, currency: binding.currency });
  };

  const save = async (values: PurchaseOrderFormValues) => {
    if (submittingRef.current || submitting || supplierBindingsLoading || !canManage) return;
    const seen = new Set<string>();
    try {
      const items = values.items.map((item) => {
        if (seen.has(item.productSkuId)) throw new Error('同一商品规格不能重复添加');
        seen.add(item.productSkuId);
        const binding = bindingBySKU.get(item.productSkuId);
        return {
          productSkuId: item.productSkuId,
          supplierSkuId: binding?.id,
          quantity: item.quantity,
          unitCostMinor: parseMajorAmountToMinor(item.unitCostMajor),
        };
      });
      const payload = {
        supplierId: values.supplierId,
        warehouseId: values.warehouseId,
        currency: values.currency,
        remark: values.remark?.trim() || '',
        items,
      };
      const command = idempotentCommandKey(
        purchaseCommandRef.current, 'purchase-order', payload,
        createProcurementIdempotencyKey, idempotencyKey,
      );
      purchaseCommandRef.current = command;
      submittingRef.current = true;
      setSubmitting(true);
      const order = await createPurchaseOrder({ ...payload, idempotencyKey: command.key });
      message.success(tr("采购单草稿已创建"));
      closeCreate();
      actionRef.current?.reload();
      history.push(`/procurement/purchase-orders/${order.id}`);
    } catch (nextError) {
      if (nextError instanceof Error && !(nextError instanceof TypeError) && nextError.message.includes('商品规格')) {
        message.error(tr(nextError.message));
      } else if (nextError instanceof Error && nextError.message.includes('金额')) {
        message.error(tr(nextError.message));
      } else {
        message.error(tr(procurementErrorMessage(extractProcurementAPIError(nextError), tr('采购单创建失败，请检查填写内容。'))));
      }
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const columns: ProColumns<PurchaseOrder>[] = [
    {
      title: tr("采购单号"), dataIndex: 'purchaseOrderNo', width: 190, copyable: true,
      render: (_, row) => <Button type="link" size="small" onClick={() => history.push(`/procurement/purchase-orders/${row.id}`)}>{row.purchaseOrderNo}</Button>,
    },
    { title: tr("供应商"), dataIndex: 'supplierId', minWidth: 160, ellipsis: true, render: (_, row) => supplierNames.get(row.supplierId) || row.supplierId },
    { title: tr("收货仓库"), dataIndex: 'warehouseId', minWidth: 150, ellipsis: true, render: (_, row) => warehouseNames.get(row.warehouseId) || row.warehouseId },
    { title: tr("状态"), dataIndex: 'status', width: 110, render: (_, row) => statusTag(row.status) },
    { title: tr("采购金额"), dataIndex: 'totalAmountMinor', width: 150, align: 'right', render: (_, row) => formatMinorAmount(row.totalAmountMinor, row.currency) },
    { title: tr("版本"), dataIndex: 'revision', width: 80, align: 'center' },
    { title: tr("创建时间"), dataIndex: 'createdAt', width: 180, render: (_, row) => formatDateTime(row.createdAt) },
    { title: tr("操作"), valueType: 'option', width: 90, render: (_, row) => <Button type="link" size="small" onClick={() => history.push(`/procurement/purchase-orders/${row.id}`)}>{tr("查看")}</Button> },
  ];

  const missingMasterData = activeWarehouses.length === 0 || activeSuppliers.length === 0;

  return (
    <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title={tr("采购单")}
        subTitle={tr("创建采购草稿并按权限完成提交、审批和分批收货；所有状态写入都使用版本号防止并发覆盖。")}
        extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canManage || missingMasterData || masterLoading} onClick={openCreate}>{tr("新建采购单")}</Button></TmPageHeaderExtra>}
      >
        {!canManage ? <Alert type="info" showIcon message={tr("当前账号可以查看采购单；提交、审批和收货按钮将按职责权限分别显示。")} /> : null}
        {missingMasterData && !masterLoading ? (
          <Alert
            type="warning"
            showIcon
            message={tr("创建采购单前需要至少一个启用的仓库和供应商。")}
            action={<Space wrap><Button onClick={() => history.push('/procurement/warehouses')}>{tr("仓库管理")}</Button><Button onClick={() => history.push('/procurement/suppliers')}>{tr("供应商管理")}</Button></Space>}
          />
        ) : null}
        {masterError ? <ErrorAlert title={tr(masterError)} actionHint={<Button onClick={() => void loadMasterData()}>{tr("重新加载基础资料")}</Button>} /> : null}
        {listError ? <ErrorAlert title={tr(listError)} actionHint={<Button onClick={() => actionRef.current?.reload()}>{tr("重新加载采购单")}</Button>} /> : null}
        <TmProTable<PurchaseOrder>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1180 }}
          pagination={{ defaultPageSize: 20, showSizeChanger: true }}
          request={async (params) => {
            const isCurrent = listRead.begin();
            setListError(undefined);
            try {
              const result = await listPurchaseOrders({ page: params.current, pageSize: params.pageSize });
              if (!isCurrent()) return { data: [], total: 0, success: false };
              return { data: result.list || [], total: result.total, success: true };
            } catch (nextError) {
              if (isCurrent()) setListError(procurementErrorMessage(extractProcurementAPIError(nextError), '采购单列表加载失败，请稍后重试。'));
              return { data: [], total: 0, success: false };
            }
          }}
          locale={{ emptyText: listError ? tr('采购单列表暂不可用') : tr('暂无采购单。') }}
        />

        <Modal
          title={tr("新建采购单")}
          open={modalOpen}
          width={920}
          confirmLoading={submitting}
          okButtonProps={{ disabled: supplierBindingsLoading || !canManage }}
          okText={tr("创建采购单草稿")}
          cancelText={tr("取消")}
          onCancel={() => !submitting && closeCreate()}
          onOk={() => form.submit()}
          forceRender
        >
          <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void save(values)}>
            <div className="tm-procurement-form-grid">
              <Form.Item label={tr("供应商")} name="supplierId" rules={[{ required: true, message: tr("请选择供应商") }]}>
                <Select showSearch optionFilterProp="label" onChange={(value) => void changeSupplier(value)} options={activeSuppliers.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))} />
              </Form.Item>
              <Form.Item label={tr("收货仓库")} name="warehouseId" rules={[{ required: true, message: tr("请选择收货仓库") }]}>
                <Select showSearch optionFilterProp="label" options={activeWarehouses.map((row) => ({ value: row.id, label: trFormat("{value0} · {value1}{value2}", { value0: row.code, value1: row.name, value2: row.isDefault ? tr('（默认）') : '' }) }))} />
              </Form.Item>
              <Form.Item label={tr("币种")} name="currency" rules={[{ required: true }]}>
                <Select options={['CNY', 'USD', 'EUR'].map((value) => ({ value, label: value }))} />
              </Form.Item>
            </div>
          <Form.List name="items" rules={[{ validator: async (_, items) => { if (!items || items.length < 1) throw new Error(tr('至少添加一项商品规格')); } }]}>
              {(fields, { add, remove }, { errors }) => (
                <>
                  <div className="tm-procurement-lines-head">
                    <strong>{tr("采购明细")}</strong>
                    <Button onClick={() => add({ productSkuId: '', quantity: 1, unitCostMajor: '0.00' })}>{tr("添加明细")}</Button>
                  </div>
                  {fields.map((field, index) => (
                    <div className="tm-procurement-line" key={field.key}>
                      <Form.Item label={trFormat("商品规格 {value0}", { value0: index + 1 })} name={[field.name, 'productSkuId']} rules={[{ required: true, message: tr("请选择商品规格") }]}>
                        <Select
                          showSearch
                          filterOption={false}
                          loading={skuSearching}
                          placeholder={tr("输入商品标题、规格编码或名称搜索")}
                          onSearch={(value) => void searchSKUs(value)}
                          onOpenChange={(open) => open && skuOptions.length === 0 && void searchSKUs()}
                          onChange={(value) => applyBindingDefaults(field.name, value)}
                          options={skuOptions.map((item) => ({ value: item.productSkuId, label: `${item.productTitle} · ${item.skuName || item.skuCode || item.productSkuId}` }))}
                        />
                      </Form.Item>
                      <Form.Item label={tr("数量")} name={[field.name, 'quantity']} rules={[{ required: true, type: 'number', min: 1, message: tr("数量至少为 1") }]}>
                        <InputNumber min={1} max={1000000} precision={0} />
                      </Form.Item>
                      <Form.Item label={tr("采购单价")} name={[field.name, 'unitCostMajor']} rules={[{ required: true, message: tr("请输入采购单价") }]}>
                        <Input inputMode="decimal" suffix={tr("主币单位")} />
                      </Form.Item>
                      <Button danger disabled={fields.length === 1} onClick={() => remove(field.name)}>{tr("移除")}</Button>
                    </div>
                  ))}
                  <Form.ErrorList errors={errors} />
                </>
              )}
            </Form.List>
            <Form.Item label={tr("备注")} name="remark"><Input.TextArea maxLength={1000} rows={3} showCount /></Form.Item>
            <Alert type="info" showIcon message={tr("创建后为草稿，不会自动提交审批或增加库存。采购入库必须在审批后由有收货权限的账号确认。")} />
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
