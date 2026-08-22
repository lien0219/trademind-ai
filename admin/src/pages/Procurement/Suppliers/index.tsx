import { CheckCircleOutlined, LinkOutlined, PlusOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
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
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.SUPPLIER_MANAGE);
  const canReadFullPII = can(PERMISSIONS.PII_READ_FULL);
  const [supplierForm] = Form.useForm<SupplierFormValues>();
  const [bindingForm] = Form.useForm<BindingFormValues>();
  const [rows, setRows] = useState<Supplier[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [keyword, setKeyword] = useState('');
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
  const searchSequence = useRef(0);

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const result = await listSuppliers();
      setRows(result.list || []);
    } catch (nextError) {
      setError(procurementErrorMessage(extractProcurementAPIError(nextError), '供应商列表加载失败，请稍后重试。'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const loadBindings = useCallback(async (supplier: Supplier) => {
    setBindingsLoading(true);
    setBindingsError(undefined);
    try {
      const result = await listSupplierSKUs(supplier.id);
      setBindings(result.list || []);
    } catch (nextError) {
      setBindingsError(procurementErrorMessage(extractProcurementAPIError(nextError), '供应商采购信息加载失败，请稍后重试。'));
    } finally {
      setBindingsLoading(false);
    }
  }, []);

  const searchSKUs = useCallback(async (query = '') => {
    const sequence = ++searchSequence.current;
    setSkuSearching(true);
    try {
      const result = await searchProductSkus({ keyword: query.trim() || undefined, limit: 30 });
      if (sequence === searchSequence.current) setSkuOptions(result.list || []);
    } catch {
      if (sequence === searchSequence.current) setSkuOptions([]);
    } finally {
      if (sequence === searchSequence.current) setSkuSearching(false);
    }
  }, []);

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
        message.success('供应商信息已更新');
      } else {
        await createSupplier({
          ...common,
          code: values.code.trim().toUpperCase(),
          phone: values.phone?.trim() || '',
          email: values.email?.trim() || '',
        });
        message.success('供应商已创建');
      }
      setSupplierModalOpen(false);
      await load();
    } catch (nextError) {
      message.error(procurementErrorMessage(extractProcurementAPIError(nextError), '供应商保存失败，请检查填写内容。'));
    } finally {
      setSupplierSubmitting(false);
    }
  };

  const openBindings = (supplier: Supplier) => {
    setSelectedSupplier(supplier);
    setBindings([]);
    void loadBindings(supplier);
  };

  const openBindingModal = (binding?: SupplierSKU) => {
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
    if (!selectedSupplier) return;
    let unitCostMinor: number;
    try {
      unitCostMinor = parseMajorAmountToMinor(values.unitCostMajor);
    } catch (nextError) {
      message.error(nextError instanceof Error ? nextError.message : '采购单价格式不正确');
      return;
    }
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
      message.success(editingBinding ? '采购信息已更新' : '商品规格已关联');
      setBindingModalOpen(false);
      await loadBindings(selectedSupplier);
    } catch (nextError) {
      message.error(procurementErrorMessage(extractProcurementAPIError(nextError), '商品规格关联失败，请检查填写内容。'));
    } finally {
      setBindingSubmitting(false);
    }
  };

  const columns: ProColumns<Supplier>[] = [
    { title: '供应商编码', dataIndex: 'code', width: 150, copyable: true, ellipsis: true },
    { title: '供应商名称', dataIndex: 'name', minWidth: 180, ellipsis: true },
    { title: '联系人', dataIndex: 'contactName', width: 130, ellipsis: true, renderText: (value) => value || '—' },
    { title: '电话', dataIndex: 'phone', width: 150, ellipsis: true, renderText: (value) => value || '—' },
    { title: '邮箱', dataIndex: 'email', minWidth: 180, ellipsis: true, renderText: (value) => value || '—' },
    { title: '状态', dataIndex: 'status', width: 90, render: (_, row) => row.status === 'active' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag> },
    {
      title: '操作', valueType: 'option', width: 190,
      render: (_, row) => (
        <Space size={0} wrap>
          <Button type="link" size="small" icon={<LinkOutlined />} onClick={() => openBindings(row)}>采购信息</Button>
          {canManage ? <Button type="link" size="small" onClick={() => openEdit(row)}>编辑</Button> : null}
        </Space>
      ),
    },
  ];

  const bindingColumns: ProColumns<SupplierSKU>[] = [
    { title: '商品', dataIndex: 'productTitle', minWidth: 170, ellipsis: true },
    { title: '本地规格', dataIndex: 'skuName', minWidth: 150, ellipsis: true, render: (_, row) => row.skuName || row.skuCode || row.productSkuId },
    { title: '供应商货号', dataIndex: 'supplierSkuCode', width: 150, ellipsis: true, renderText: (value) => value || '—' },
    { title: '采购价', dataIndex: 'unitCostMinor', width: 140, render: (_, row) => formatMinorAmount(row.unitCostMinor, row.currency) },
    { title: '起订量', dataIndex: 'minOrderQty', width: 90 },
    { title: '交期', dataIndex: 'leadTimeDays', width: 90, render: (_, row) => `${row.leadTimeDays} 天` },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => canManage ? <Button type="link" size="small" onClick={() => openBindingModal(row)}>编辑</Button> : '—' },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.SUPPLIER_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title="供应商管理"
        subTitle="维护供应商联系方式，并为本地商品规格设置供应商货号、采购价、起订量和交期。"
        extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={openCreate}>新建供应商</Button></TmPageHeaderExtra>}
      >
        {!canManage ? <Alert type="info" showIcon message="当前账号为只读模式；联系方式可能按权限脱敏展示。" /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => void load()}>重新加载</Button>} /> : null}
        <div className="tm-procurement-overview" aria-label="供应商概览">
          <div className="tm-procurement-overview__item">
            <span className="tm-procurement-overview__label">供应商总数</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : rows.length}</strong>
          </div>
          <div className="tm-procurement-overview__item tm-procurement-overview__item--success">
            <CheckCircleOutlined aria-hidden="true" />
            <span className="tm-procurement-overview__label">启用中</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : activeCount}</strong>
          </div>
          <div className="tm-procurement-overview__note">
            <SafetyCertificateOutlined aria-hidden="true" />
            <Typography.Text type="secondary">
              {loading ? '正在加载供应商信息' : canReadFullPII ? '联系方式按当前权限完整展示' : '联系方式按当前权限脱敏展示'}
            </Typography.Text>
          </div>
        </div>
        <TmProTable<Supplier>
          className="tm-procurement-table"
          rowKey="id" columns={columns} dataSource={filteredRows} loading={loading} search={false} options={false}
          headerTitle={
            <Input.Search
              allowClear
              aria-label="搜索供应商"
              placeholder="搜索编码、名称或联系人"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              className="tm-procurement-search"
            />
          }
          toolBarRender={() => [
            <Typography.Text key="count" type="secondary">共 {filteredRows.length} 家供应商</Typography.Text>,
            <Button key="refresh" loading={loading} onClick={() => void load()}>刷新列表</Button>,
          ]}
          pagination={false} cardBordered scroll={{ x: 1120 }} locale={{ emptyText: error ? '供应商列表暂不可用' : '暂无供应商，请先新建供应商。' }}
        />

        <Modal
          title={editing ? `编辑供应商 · ${editing.code}` : '新建供应商'}
          open={supplierModalOpen}
          confirmLoading={supplierSubmitting}
          okText={editing ? '保存供应商' : '创建供应商'}
          cancelText="取消"
          onCancel={() => !supplierSubmitting && setSupplierModalOpen(false)}
          onOk={() => supplierForm.submit()}
          forceRender
        >
          <Form form={supplierForm} layout="vertical" preserve={false} onFinish={(values) => void saveSupplier(values)}>
            <Form.Item label="供应商编码" name="code" rules={[{ required: true, message: '请输入供应商编码' }, { pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, message: '仅支持字母、数字、下划线和短横线' }]}>
              <Input disabled={Boolean(editing)} maxLength={64} placeholder="例如 SUP-001" />
            </Form.Item>
            <Form.Item label="供应商名称" name="name" rules={[{ required: true, whitespace: true, message: '请输入供应商名称' }]}>
              <Input maxLength={200} />
            </Form.Item>
            {editing ? <Form.Item label="状态" name="status" rules={[{ required: true }]}><Radio.Group options={[{ label: '启用', value: 'active' }, { label: '停用', value: 'inactive' }]} /></Form.Item> : null}
            <Form.Item label="联系人" name="contactName"><Input maxLength={120} /></Form.Item>
            <Form.Item
              label="联系电话"
              name="phone"
              extra={editing && !canReadFullPII ? '原号码已脱敏；留空将保留原值，填写新号码才会替换。' : undefined}
            >
              <Input maxLength={64} />
            </Form.Item>
            <Form.Item
              label="联系邮箱"
              name="email"
              extra={editing && !canReadFullPII ? '原邮箱已脱敏；留空将保留原值，填写新邮箱才会替换。' : undefined}
              rules={[{ type: 'email', message: '请输入有效邮箱地址' }]}
            >
              <Input maxLength={254} />
            </Form.Item>
          </Form>
        </Modal>

        <AppDrawer
          title={selectedSupplier ? `采购信息 · ${selectedSupplier.name}` : '供应商采购信息'}
          open={Boolean(selectedSupplier)}
          onClose={() => setSelectedSupplier(undefined)}
          extra={<Button type="primary" icon={<PlusOutlined />} disabled={!canManage || selectedSupplier?.status !== 'active'} onClick={() => openBindingModal()}>关联商品规格</Button>}
        >
          {bindingsError ? <ErrorAlert title={bindingsError} actionHint={selectedSupplier ? <Button onClick={() => void loadBindings(selectedSupplier)}>重新加载</Button> : null} /> : null}
          <TmProTable<SupplierSKU>
            rowKey="id" columns={bindingColumns} dataSource={bindings} loading={bindingsLoading} search={false} options={false}
            pagination={false} cardBordered={false} scroll={{ x: 900 }} locale={{ emptyText: bindingsError ? '采购信息暂不可用' : '尚未关联商品规格。' }}
          />
        </AppDrawer>

        <Modal
          title={editingBinding ? '编辑规格采购信息' : '关联商品规格'}
          open={bindingModalOpen}
          confirmLoading={bindingSubmitting}
          okText={editingBinding ? '保存采购信息' : '确认关联'}
          cancelText="取消"
          onCancel={() => !bindingSubmitting && setBindingModalOpen(false)}
          onOk={() => bindingForm.submit()}
          forceRender
        >
          <Form form={bindingForm} layout="vertical" preserve={false} onFinish={(values) => void saveBinding(values)}>
            <Form.Item label="本地商品规格" name="productSkuId" rules={[{ required: true, message: '请选择商品规格' }]}>
              <Select
                showSearch
                disabled={Boolean(editingBinding)}
                filterOption={false}
                loading={skuSearching}
                placeholder="输入商品标题、规格编码或规格名称搜索"
                onSearch={(value) => void searchSKUs(value)}
                onOpenChange={(open) => open && skuOptions.length === 0 && void searchSKUs()}
                options={skuOptions.map((item) => ({
                  value: item.productSkuId,
                  label: `${item.productTitle} · ${item.skuName || item.skuCode || item.productSkuId}`,
                }))}
              />
            </Form.Item>
            <Form.Item label="供应商货号" name="supplierSkuCode"><Input maxLength={128} /></Form.Item>
            <Space wrap size="middle" className="tm-procurement-form-row">
              <Form.Item label="采购单价" name="unitCostMajor" rules={[{ required: true, message: '请输入采购单价' }]}>
                <Input inputMode="decimal" suffix="主币单位" />
              </Form.Item>
              <Form.Item label="币种" name="currency" rules={[{ required: true }]}>
                <Select options={['CNY', 'USD', 'EUR'].map((value) => ({ value, label: value }))} />
              </Form.Item>
              <Form.Item label="起订量" name="minOrderQty" rules={[{ required: true }]}>
                <InputNumber min={1} max={1000000} precision={0} />
              </Form.Item>
              <Form.Item label="交期（天）" name="leadTimeDays" rules={[{ required: true }]}>
                <InputNumber min={0} max={3650} precision={0} />
              </Form.Item>
            </Space>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
