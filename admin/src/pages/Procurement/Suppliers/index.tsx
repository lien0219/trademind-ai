import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { CheckCircleOutlined, LinkOutlined, PlusOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { useSearchParams } from '@umijs/max';
import { Alert, Button, Form, Input, InputNumber, Modal, Radio, Select, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import AppDrawer from '@/components/AppDrawer';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { searchProductSkus, type ProductSkuSearchHit } from '@/services/products';
import {
  bindSupplierSKU,
  createSupplier,
  extractProcurementAPIError,
  listSupplierSKUs,
  listSuppliers,
  procurementErrorMessage,
  updateSupplier,
  type Supplier,
  type SupplierSKU,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import { formatMinorAmount, parseMajorAmountToMinor } from '../helpers';
import '../index.less';

type SupplierFormValues = {
  code: string;
  name: string;
  status: string;
  contactName: string;
  phone?: string;
  email?: string;
};

type BindingFormValues = {
  productSkuId: string;
  supplierSkuCode: string;
  unitCostMajor: string;
  currency: string;
  minOrderQty: number;
  leadTimeDays: number;
};

export default function SuppliersPage() {
	useLanguage();
	const [searchParams] = useSearchParams();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.SUPPLIER_MANAGE);
  const canReadFullPII = can(PERMISSIONS.PII_READ_FULL);
  const [supplierForm] = Form.useForm<SupplierFormValues>();
  const [bindingForm] = Form.useForm<BindingFormValues>();
  const [rows, setRows] = useState<Supplier[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [keyword, setKeyword] = useState(() => searchParams.get('keyword')?.trim() || '');
  const [supplierModalOpen, setSupplierModalOpen] = useState(false);
  const [editing, setEditing] = useState<Supplier>();
  const [supplierSubmitting, setSupplierSubmitting] = useState(false);
  const [selectedSupplier, setSelectedSupplier] = useState<Supplier>();
  const [bindings, setBindings] = useState<SupplierSKU[]>([]);
  const [bindingsLoading, setBindingsLoading] = useState(false);
  const [bindingsError, setBindingsError] = useState<string>();
  const [bindingModalOpen, setBindingModalOpen] = useState(false);
  const [editingBinding, setEditingBinding] = useState<SupplierSKU>();
  const [bindingSubmitting, setBindingSubmitting] = useState(false);
  const [skuOptions, setSkuOptions] = useState<ProductSkuSearchHit[]>([]);
  const [skuSearching, setSkuSearching] = useState(false);
  const suppliersRead = useMemo(createTenantReadGuard, []);
  const bindingsRead = useMemo(createTenantReadGuard, []);
  const skuRead = useMemo(createTenantReadGuard, []);
  const supplierSubmitRef = useRef(false);
  const bindingSubmitRef = useRef(false);

  const load = useCallback(async () => {
    const isCurrent = suppliersRead.begin();
    setLoading(true);
    setError(undefined);
    setRows([]);
    try {
      const result = await listSuppliers();
      if (isCurrent()) setRows(result.list || []);
    } catch (nextError) {
      if (isCurrent()) setError(procurementErrorMessage(extractProcurementAPIError(nextError), '供应商列表加载失败，请稍后重试。'));
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [suppliersRead]);

  useEffect(() => {
    void load();
    return () => suppliersRead.invalidate();
  }, [load, suppliersRead]);

  const loadBindings = useCallback(async (supplier: Supplier) => {
    const isCurrent = bindingsRead.begin();
    setBindingsLoading(true);
    setBindingsError(undefined);
    setBindings([]);
    try {
      const result = await listSupplierSKUs(supplier.id);
      if (isCurrent()) setBindings(result.list || []);
    } catch (nextError) {
      if (isCurrent()) setBindingsError(procurementErrorMessage(extractProcurementAPIError(nextError), '供应商采购信息加载失败，请稍后重试。'));
    } finally {
      if (isCurrent()) setBindingsLoading(false);
    }
  }, [bindingsRead]);

  const searchSKUs = useCallback(async (query = '') => {
    const isCurrent = skuRead.begin();
    setSkuSearching(true);
    setSkuOptions([]);
    try {
      const result = await searchProductSkus({ keyword: query.trim() || undefined, limit: 30 });
      if (isCurrent()) setSkuOptions(result.list || []);
    } catch {
      if (isCurrent()) setSkuOptions([]);
    } finally {
      if (isCurrent()) setSkuSearching(false);
    }
  }, [skuRead]);

  useEffect(() => () => {
    bindingsRead.invalidate();
    skuRead.invalidate();
  }, [bindingsRead, skuRead]);

  const filteredRows = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return rows;
    return rows.filter((row) => `${row.code} ${row.name} ${row.contactName || ''}`.toLowerCase().includes(normalized));
  }, [keyword, rows]);

  const activeCount = useMemo(() => rows.filter((row) => row.status === 'active').length, [rows]);

  const openCreate = () => {
    setEditing(undefined);
    supplierForm.setFieldsValue({ code: '', name: '', status: 'active', contactName: '', phone: '', email: '' });
    setSupplierModalOpen(true);
  };

  const openEdit = (row: Supplier) => {
    setEditing(row);
    supplierForm.setFieldsValue({
      code: row.code,
      name: row.name,
      status: row.status,
      contactName: row.contactName || '',
      phone: canReadFullPII ? row.phone || '' : undefined,
      email: canReadFullPII ? row.email || '' : undefined,
    });
    setSupplierModalOpen(true);
  };

  const saveSupplier = async (values: SupplierFormValues) => {
    if (!canManage || supplierSubmitRef.current) return;
    supplierSubmitRef.current = true;
    setSupplierSubmitting(true);
    try {
      const common = {
        name: values.name.trim(),
        contactName: values.contactName?.trim() || '',
      };
      if (editing) {
        const phone = values.phone?.trim();
        const email = values.email?.trim();
        await updateSupplier(editing.id, {
          ...common,
          status: values.status,
          ...(canReadFullPII || phone ? { phone: phone || '' } : {}),
          ...(canReadFullPII || email ? { email: email || '' } : {}),
        });
        message.success(tr("供应商信息已更新"));
      } else {
        await createSupplier({
          ...common,
          code: values.code.trim().toUpperCase(),
          phone: values.phone?.trim() || '',
          email: values.email?.trim() || '',
        });
        message.success(tr("供应商已创建"));
      }
      setSupplierModalOpen(false);
      await load();
    } catch (nextError) {
      message.error(tr(procurementErrorMessage(extractProcurementAPIError(nextError), tr('供应商保存失败，请检查填写内容。'))));
    } finally {
      supplierSubmitRef.current = false;
      setSupplierSubmitting(false);
    }
  };

  const openBindings = (supplier: Supplier) => {
    skuRead.invalidate();
    setSelectedSupplier(supplier);
    setBindings([]);
    void loadBindings(supplier);
  };

  const openBindingModal = (binding?: SupplierSKU) => {
    skuRead.invalidate();
    setSkuSearching(false);
    setEditingBinding(binding);
    if (binding) {
      setSkuOptions([{
        productId: '',
        productTitle: binding.productTitle,
        productSkuId: binding.productSkuId,
        skuCode: binding.skuCode,
        skuName: binding.skuName,
      }]);
      bindingForm.setFieldsValue({
        productSkuId: binding.productSkuId,
        supplierSkuCode: binding.supplierSkuCode || '',
        unitCostMajor: (binding.unitCostMinor / 100).toFixed(2),
        currency: binding.currency,
        minOrderQty: binding.minOrderQty,
        leadTimeDays: binding.leadTimeDays,
      });
    } else {
      setSkuOptions([]);
      bindingForm.setFieldsValue({
        productSkuId: '', supplierSkuCode: '', unitCostMajor: '0.00', currency: 'CNY', minOrderQty: 1, leadTimeDays: 0,
      });
      void searchSKUs();
    }
    setBindingModalOpen(true);
  };

  const saveBinding = async (values: BindingFormValues) => {
    if (!canManage || !selectedSupplier || bindingSubmitRef.current) return;
    let unitCostMinor: number;
    try {
      unitCostMinor = parseMajorAmountToMinor(values.unitCostMajor);
    } catch (nextError) {
      message.error(tr(nextError instanceof Error ? nextError.message : tr('采购单价格式不正确')));
      return;
    }
    bindingSubmitRef.current = true;
    setBindingSubmitting(true);
    try {
      await bindSupplierSKU(selectedSupplier.id, {
        productSkuId: values.productSkuId,
        supplierSkuCode: values.supplierSkuCode?.trim() || '',
        unitCostMinor,
        currency: values.currency,
        minOrderQty: values.minOrderQty,
        leadTimeDays: values.leadTimeDays,
      });
      message.success(editingBinding ? tr('采购信息已更新') : tr('商品规格已关联'));
      setBindingModalOpen(false);
      await loadBindings(selectedSupplier);
    } catch (nextError) {
      message.error(tr(procurementErrorMessage(extractProcurementAPIError(nextError), tr('商品规格关联失败，请检查填写内容。'))));
    } finally {
      bindingSubmitRef.current = false;
      setBindingSubmitting(false);
    }
  };

  const columns: ProColumns<Supplier>[] = [
    { title: tr("供应商编码"), dataIndex: 'code', width: 150, copyable: true, ellipsis: true },
    { title: tr("供应商名称"), dataIndex: 'name', minWidth: 180, ellipsis: true },
    { title: tr("联系人"), dataIndex: 'contactName', width: 130, ellipsis: true, renderText: (value) => value || '—' },
    { title: tr("电话"), dataIndex: 'phone', width: 150, ellipsis: true, renderText: (value) => value || '—' },
    { title: tr("邮箱"), dataIndex: 'email', minWidth: 180, ellipsis: true, renderText: (value) => value || '—' },
    { title: tr("状态"), dataIndex: 'status', width: 90, render: (_, row) => row.status === 'active' ? <Tag color="success">{tr("启用")}</Tag> : <Tag>{tr("停用")}</Tag> },
    {
      title: tr("操作"), valueType: 'option', width: 190,
      render: (_, row) => (
        <Space size={0} wrap>
          <Button type="link" size="small" icon={<LinkOutlined />} onClick={() => openBindings(row)}>{tr("采购信息")}</Button>
          {canManage ? <Button type="link" size="small" onClick={() => openEdit(row)}>{tr("编辑")}</Button> : null}
        </Space>
      ),
    },
  ];

  const bindingColumns: ProColumns<SupplierSKU>[] = [
    { title: tr("商品"), dataIndex: 'productTitle', minWidth: 170, ellipsis: true },
    { title: tr("本地规格"), dataIndex: 'skuName', minWidth: 150, ellipsis: true, render: (_, row) => row.skuName || row.skuCode || row.productSkuId },
    { title: tr("供应商货号"), dataIndex: 'supplierSkuCode', width: 150, ellipsis: true, renderText: (value) => value || '—' },
    { title: tr("采购价"), dataIndex: 'unitCostMinor', width: 140, render: (_, row) => formatMinorAmount(row.unitCostMinor, row.currency) },
    { title: tr("起订量"), dataIndex: 'minOrderQty', width: 90 },
    { title: tr("交期"), dataIndex: 'leadTimeDays', width: 90, render: (_, row) => trFormat("{value0} 天", { value0: String(row.leadTimeDays) }) },
    { title: tr("操作"), valueType: 'option', width: 90, render: (_, row) => canManage ? <Button type="link" size="small" onClick={() => openBindingModal(row)}>{tr("编辑")}</Button> : '—' },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.SUPPLIER_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title={tr("供应商管理")}
        subTitle={tr("维护供应商联系方式，并为本地商品规格设置供应商货号、采购价、起订量和交期。")}
        extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={openCreate}>{tr("新建供应商")}</Button></TmPageHeaderExtra>}
      >
        {!canManage ? <Alert type="info" showIcon message={tr("当前账号为只读模式；联系方式可能按权限脱敏展示。")} /> : null}
        {error ? <ErrorAlert title={tr(error)} actionHint={<Button onClick={() => void load()}>{tr("重新加载")}</Button>} /> : null}
        <div className="tm-procurement-overview" aria-label={tr("供应商概览")}>
          <div className="tm-procurement-overview__item">
            <span className="tm-procurement-overview__label">{tr("供应商总数")}</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : rows.length}</strong>
          </div>
          <div className="tm-procurement-overview__item tm-procurement-overview__item--success">
            <CheckCircleOutlined aria-hidden="true" />
            <span className="tm-procurement-overview__label">{tr("启用中")}</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : activeCount}</strong>
          </div>
          <div className="tm-procurement-overview__note">
            <SafetyCertificateOutlined aria-hidden="true" />
            <Typography.Text type="secondary">
              {loading ? tr('正在加载供应商信息') : canReadFullPII ? tr('联系方式按当前权限完整展示') : tr('联系方式按当前权限脱敏展示')}
            </Typography.Text>
          </div>
        </div>
        <TmProTable<Supplier>
          className="tm-procurement-table"
          rowKey="id" columns={columns} dataSource={filteredRows} loading={loading} search={false} options={false}
          filterBar={
            <Input.Search
              allowClear
              aria-label={tr("搜索供应商")}
              placeholder={tr("搜索编码、名称或联系人")}
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              className="tm-procurement-search"
            />
          }
          toolBarRender={() => [
            <Typography.Text key="count" type="secondary">{tr("共")} {filteredRows.length} {tr("家供应商")}</Typography.Text>,
            <Button key="refresh" loading={loading} onClick={() => void load()}>{tr("刷新列表")}</Button>,
          ]}
          pagination={false} cardBordered scroll={{ x: 1120 }} locale={{ emptyText: error ? tr('供应商列表暂不可用') : tr('暂无供应商，请先新建供应商。') }}
        />

        <Modal
          title={editing ? trFormat("编辑供应商 · {value0}", { value0: String(editing.code) }) : tr('新建供应商')}
          open={supplierModalOpen}
          confirmLoading={supplierSubmitting}
          okText={editing ? tr('保存供应商') : tr('创建供应商')}
          cancelText={tr("取消")}
          onCancel={() => !supplierSubmitting && setSupplierModalOpen(false)}
          onOk={() => supplierForm.submit()}
          forceRender
        >
          <Form form={supplierForm} layout="vertical" preserve={false} onFinish={(values) => void saveSupplier(values)}>
            <Form.Item label={tr("供应商编码")} name="code" rules={[{ required: true, message: tr("请输入供应商编码") }, { pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, message: tr("仅支持字母、数字、下划线和短横线") }]}>
              <Input disabled={Boolean(editing)} maxLength={64} placeholder={tr("例如 SUP-001")} />
            </Form.Item>
            <Form.Item label={tr("供应商名称")} name="name" rules={[{ required: true, whitespace: true, message: tr("请输入供应商名称") }]}>
              <Input maxLength={200} />
            </Form.Item>
            {editing ? <Form.Item label={tr("状态")} name="status" rules={[{ required: true }]}><Radio.Group options={[{ label: tr("启用"), value: 'active' }, { label: tr("停用"), value: 'inactive' }]} /></Form.Item> : null}
            <Form.Item label={tr("联系人")} name="contactName"><Input maxLength={120} /></Form.Item>
            <Form.Item
              label={tr("联系电话")}
              name="phone"
              extra={editing && !canReadFullPII ? tr('原号码已脱敏；留空将保留原值，填写新号码才会替换。') : undefined}
            >
              <Input maxLength={64} />
            </Form.Item>
            <Form.Item
              label={tr("联系邮箱")}
              name="email"
              extra={editing && !canReadFullPII ? tr('原邮箱已脱敏；留空将保留原值，填写新邮箱才会替换。') : undefined}
              rules={[{ type: 'email', message: tr("请输入有效邮箱地址") }]}
            >
              <Input maxLength={254} />
            </Form.Item>
          </Form>
        </Modal>

        <AppDrawer
          title={selectedSupplier ? trFormat("采购信息 · {value0}", { value0: String(selectedSupplier.name) }) : tr('供应商采购信息')}
          open={Boolean(selectedSupplier)}
          onClose={() => {
             bindingsRead.invalidate();
             skuRead.invalidate();
             setSelectedSupplier(undefined);
             setBindings([]);
             setBindingsError(undefined);
             setBindingModalOpen(false);
           }}
          extra={<Button type="primary" icon={<PlusOutlined />} disabled={!canManage || selectedSupplier?.status !== 'active'} onClick={() => openBindingModal()}>{tr("关联商品规格")}</Button>}
        >
          {bindingsError ? <ErrorAlert title={tr(bindingsError)} actionHint={selectedSupplier ? <Button onClick={() => void loadBindings(selectedSupplier)}>{tr("重新加载")}</Button> : null} /> : null}
          <TmProTable<SupplierSKU>
            rowKey="id" columns={bindingColumns} dataSource={bindings} loading={bindingsLoading} search={false} options={false}
            pagination={false} cardBordered={false} scroll={{ x: 900 }} locale={{ emptyText: bindingsError ? tr('采购信息暂不可用') : tr('尚未关联商品规格。') }}
          />
        </AppDrawer>

        <Modal
          title={editingBinding ? tr('编辑规格采购信息') : tr('关联商品规格')}
          open={bindingModalOpen}
          confirmLoading={bindingSubmitting}
          okText={editingBinding ? tr('保存采购信息') : tr('确认关联')}
          cancelText={tr("取消")}
          onCancel={() => {
            if (bindingSubmitRef.current) return;
            skuRead.invalidate();
            setBindingModalOpen(false);
            setSkuOptions([]);
          }}
          onOk={() => bindingForm.submit()}
          forceRender
        >
          <Form form={bindingForm} layout="vertical" preserve={false} onFinish={(values) => void saveBinding(values)}>
            <Form.Item label={tr("本地商品规格")} name="productSkuId" rules={[{ required: true, message: tr("请选择商品规格") }]}>
              <Select
                showSearch
                disabled={Boolean(editingBinding)}
                filterOption={false}
                loading={skuSearching}
                placeholder={tr("输入商品标题、规格编码或规格名称搜索")}
                onSearch={(value) => void searchSKUs(value)}
                onOpenChange={(open) => open && skuOptions.length === 0 && void searchSKUs()}
                options={skuOptions.map((item) => ({
                  value: item.productSkuId,
                  label: `${item.productTitle} · ${item.skuName || item.skuCode || item.productSkuId}`,
                }))}
              />
            </Form.Item>
            <Form.Item label={tr("供应商货号")} name="supplierSkuCode"><Input maxLength={128} /></Form.Item>
            <Space wrap size="middle" className="tm-procurement-form-row">
              <Form.Item label={tr("采购单价")} name="unitCostMajor" rules={[{ required: true, message: tr("请输入采购单价") }]}>
                <Input inputMode="decimal" suffix={tr("主币单位")} />
              </Form.Item>
              <Form.Item label={tr("币种")} name="currency" rules={[{ required: true }]}>
                <Select options={['CNY', 'USD', 'EUR'].map((value) => ({ value, label: value }))} />
              </Form.Item>
              <Form.Item label={tr("起订量")} name="minOrderQty" rules={[{ required: true }]}>
                <InputNumber min={1} max={1000000} precision={0} />
              </Form.Item>
              <Form.Item label={tr("交期（天）")} name="leadTimeDays" rules={[{ required: true }]}>
                <InputNumber min={0} max={3650} precision={0} />
              </Form.Item>
            </Space>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
