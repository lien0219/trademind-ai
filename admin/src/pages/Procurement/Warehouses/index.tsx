import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { CheckCircleOutlined, HomeOutlined, PlusOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, Modal, Radio, Switch, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { usePermission } from '@/hooks/usePermission';
import {
  createWarehouse,
  extractProcurementAPIError,
  listWarehouses,
  procurementErrorMessage,
  updateWarehouse,
  type Warehouse,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import '../index.less';

type WarehouseFormValues = {
  code: string;
  name: string;
  status: string;
  isDefault: boolean;
};

export default function WarehousesPage() {
  useLanguage();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.WAREHOUSE_MANAGE);
  const [form] = Form.useForm<WarehouseFormValues>();
  const [rows, setRows] = useState<Warehouse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [keyword, setKeyword] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Warehouse>();
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const warehouseRead = useMemo(createTenantReadGuard, []);
  const saveContext = useMemo(createTenantReadGuard, []);

  const load = useCallback(async () => {
    const isCurrent = warehouseRead.begin();
    setLoading(true);
    setError(undefined);
    setRows([]);
    try {
      const result = await listWarehouses();
      if (isCurrent()) setRows(result.list || []);
    } catch (nextError) {
      if (isCurrent()) {
        setError(procurementErrorMessage(extractProcurementAPIError(nextError), '仓库列表加载失败，请稍后重试。'));
      }
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [warehouseRead]);

  useEffect(() => {
    void load();
    return () => { warehouseRead.invalidate(); saveContext.invalidate(); };
  }, [load, warehouseRead, saveContext]);

  const filteredRows = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return rows;
    return rows.filter((row) => `${row.code} ${row.name}`.toLowerCase().includes(normalized));
  }, [keyword, rows]);

  const activeCount = useMemo(() => rows.filter((row) => row.status === 'active').length, [rows]);
  const defaultWarehouse = useMemo(() => rows.find((row) => row.isDefault), [rows]);

  const openCreate = () => {
    setEditing(undefined);
    form.setFieldsValue({ code: '', name: '', status: 'active', isDefault: rows.length === 0 });
    setModalOpen(true);
  };

  const openEdit = (row: Warehouse) => {
    setEditing(row);
    form.setFieldsValue({ code: row.code, name: row.name, status: row.status, isDefault: row.isDefault });
    setModalOpen(true);
  };

  const save = async (values: WarehouseFormValues) => {
    if (!canManage || submittingRef.current) return;
    const isCurrent = saveContext.begin();
    submittingRef.current = true;
    setSubmitting(true);
    try {
      if (editing) {
        await updateWarehouse(editing.id, {
          name: values.name.trim(),
          status: values.status,
          isDefault: values.isDefault,
        });
        if (!isCurrent()) return;
        message.success(tr("仓库信息已更新"));
      } else {
        await createWarehouse({ code: values.code.trim().toUpperCase(), name: values.name.trim(), isDefault: values.isDefault });
        if (!isCurrent()) return;
        message.success(tr("仓库已创建"));
      }
      setModalOpen(false);
      await load();
    } catch (nextError) {
      if (isCurrent()) message.error(tr(procurementErrorMessage(extractProcurementAPIError(nextError), tr('仓库保存失败，请检查填写内容。'))));
    } finally {
      submittingRef.current = false;
      if (isCurrent()) setSubmitting(false);
    }
  };

  const columns: ProColumns<Warehouse>[] = [
    { title: tr("仓库编码"), dataIndex: 'code', width: 150, copyable: true, ellipsis: true },
    { title: tr("仓库名称"), dataIndex: 'name', minWidth: 180, ellipsis: true },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 100,
      render: (_, row) => row.status === 'active' ? <Tag color="success">{tr("启用")}</Tag> : <Tag>{tr("停用")}</Tag>,
    },
    {
      title: tr("默认仓"),
      dataIndex: 'isDefault',
      width: 100,
      render: (_, row) => row.isDefault ? <Tag color="blue">{tr("默认")}</Tag> : '—',
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 100,
      render: (_, row) => canManage ? <Button type="link" size="small" onClick={() => openEdit(row)}>{tr("编辑")}</Button> : '—',
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.WAREHOUSE_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title={tr("仓库管理")}
        subTitle={tr("维护采购入库使用的仓库。当前分仓余额仍处于兼容迁移阶段，不在此页展示可用库存。")}
        extra={
          <TmPageHeaderExtra>
            <Button type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={openCreate}>{tr("新建仓库")}</Button>
          </TmPageHeaderExtra>
        }
      >
        {!canManage ? <Alert type="info" showIcon message={tr("当前账号为只读模式，可查看仓库但不能修改。")} /> : null}
        {error ? <ErrorAlert title={tr(error)} actionHint={<Button onClick={() => void load()}>{tr("重新加载")}</Button>} /> : null}
        <div className="tm-procurement-overview" aria-label={tr("仓库概览")}>
          <div className="tm-procurement-overview__item">
            <span className="tm-procurement-overview__label">{tr("仓库总数")}</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : rows.length}</strong>
          </div>
          <div className="tm-procurement-overview__item tm-procurement-overview__item--success">
            <CheckCircleOutlined aria-hidden="true" />
            <span className="tm-procurement-overview__label">{tr("启用中")}</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : activeCount}</strong>
          </div>
          <div className="tm-procurement-overview__note">
            <HomeOutlined aria-hidden="true" />
            <Typography.Text type="secondary">
              {loading ? tr('正在加载仓库信息') : `${tr('默认仓：')}${defaultWarehouse ? `${defaultWarehouse.code} · ${defaultWarehouse.name}` : tr('尚未设置')}`}
            </Typography.Text>
          </div>
        </div>
        <TmProTable<Warehouse>
          className="tm-procurement-table"
          rowKey="id"
          columns={columns}
          dataSource={filteredRows}
          loading={loading}
          filterBar={
            <Input.Search
              allowClear
              aria-label={tr("搜索仓库")}
              placeholder={tr("搜索仓库编码或名称")}
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              className="tm-procurement-search"
            />
          }
          search={false}
          options={false}
          toolBarRender={() => [
            <Typography.Text key="count" type="secondary">{tr("共")} {filteredRows.length} {tr("个仓库")}</Typography.Text>,
            <Button key="refresh" loading={loading} onClick={() => void load()}>{tr("刷新列表")}</Button>,
          ]}
          pagination={false}
          cardBordered
          scroll={{ x: 760 }}
          locale={{ emptyText: error ? tr('仓库列表暂不可用') : tr('暂无仓库，请先新建仓库。') }}
        />
        <Modal
          title={editing ? trFormat("编辑仓库 · {value0}", { value0: String(editing.code) }) : tr('新建仓库')}
          open={modalOpen}
          confirmLoading={submitting}
          okText={editing ? tr('保存仓库') : tr('创建仓库')}
          cancelText={tr("取消")}
          onCancel={() => { if (!submittingRef.current) setModalOpen(false); }}
          onOk={() => form.submit()}
          forceRender
        >
          <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void save(values)}>
            <Form.Item
              label={tr("仓库编码")}
              name="code"
              rules={[
                { required: true, message: tr("请输入仓库编码") },
                { pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, message: tr("仅支持字母、数字、下划线和短横线") },
              ]}
            >
              <Input disabled={Boolean(editing)} maxLength={64} placeholder={tr("例如 MAIN")} />
            </Form.Item>
            <Form.Item label={tr("仓库名称")} name="name" rules={[{ required: true, whitespace: true, message: tr("请输入仓库名称") }]}>
              <Input maxLength={160} placeholder={tr("例如 华东主仓")} />
            </Form.Item>
            {editing ? (
              <Form.Item label={tr("状态")} name="status" rules={[{ required: true }]}>
                <Radio.Group options={[{ label: tr("启用"), value: 'active' }, { label: tr("停用"), value: 'inactive' }]} />
              </Form.Item>
            ) : null}
            <Form.Item label={tr("设为默认仓")} name="isDefault" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Alert type="info" showIcon message={tr("默认仓必须保持启用；设为默认后，同租户原默认仓会自动取消。")} />
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
