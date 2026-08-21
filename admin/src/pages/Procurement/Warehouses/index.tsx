import { CheckCircleOutlined, HomeOutlined, PlusOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, Modal, Radio, Switch, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { ErrorAlert, OperationToolbar, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
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
import '../index.less';

type WarehouseFormValues = {
  code: string;
  name: string;
  status: string;
  isDefault: boolean;
};

export default function WarehousesPage() {
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

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const result = await listWarehouses();
      setRows(result.list || []);
    } catch (nextError) {
      setError(procurementErrorMessage(extractProcurementAPIError(nextError), '仓库列表加载失败，请稍后重试。'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

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
    setSubmitting(true);
    try {
      if (editing) {
        await updateWarehouse(editing.id, {
          name: values.name.trim(),
          status: values.status,
          isDefault: values.isDefault,
        });
        message.success('仓库信息已更新');
      } else {
        await createWarehouse({ code: values.code.trim().toUpperCase(), name: values.name.trim(), isDefault: values.isDefault });
        message.success('仓库已创建');
      }
      setModalOpen(false);
      await load();
    } catch (nextError) {
      message.error(procurementErrorMessage(extractProcurementAPIError(nextError), '仓库保存失败，请检查填写内容。'));
    } finally {
      setSubmitting(false);
    }
  };

  const columns: ProColumns<Warehouse>[] = [
    { title: '仓库编码', dataIndex: 'code', width: 150, copyable: true, ellipsis: true },
    { title: '仓库名称', dataIndex: 'name', minWidth: 180, ellipsis: true },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => row.status === 'active' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>,
    },
    {
      title: '默认仓',
      dataIndex: 'isDefault',
      width: 100,
      render: (_, row) => row.isDefault ? <Tag color="blue">默认</Tag> : '—',
    },
    {
      title: '操作',
      valueType: 'option',
      width: 100,
      render: (_, row) => canManage ? <Button type="link" size="small" onClick={() => openEdit(row)}>编辑</Button> : '—',
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.WAREHOUSE_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title="仓库管理"
        subTitle="维护采购入库使用的仓库。当前分仓余额仍处于兼容迁移阶段，不在此页展示可用库存。"
        extra={
          <TmPageHeaderExtra>
            <Button type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={openCreate}>新建仓库</Button>
          </TmPageHeaderExtra>
        }
      >
        {!canManage ? <Alert type="info" showIcon message="当前账号为只读模式，可查看仓库但不能修改。" /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => void load()}>重新加载</Button>} /> : null}
        <div className="tm-procurement-overview" aria-label="仓库概览">
          <div className="tm-procurement-overview__item">
            <span className="tm-procurement-overview__label">仓库总数</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : rows.length}</strong>
          </div>
          <div className="tm-procurement-overview__item tm-procurement-overview__item--success">
            <CheckCircleOutlined aria-hidden="true" />
            <span className="tm-procurement-overview__label">启用中</span>
            <strong className="tm-procurement-overview__value">{loading ? '—' : activeCount}</strong>
          </div>
          <div className="tm-procurement-overview__note">
            <HomeOutlined aria-hidden="true" />
            <Typography.Text type="secondary">
              {loading ? '正在加载仓库信息' : `默认仓：${defaultWarehouse ? `${defaultWarehouse.code} · ${defaultWarehouse.name}` : '尚未设置'}`}
            </Typography.Text>
          </div>
        </div>
        <OperationToolbar className="tm-procurement-toolbar" extra={<Typography.Text type="secondary">共 {filteredRows.length} 个仓库</Typography.Text>}>
          <Input.Search
            allowClear
            aria-label="搜索仓库"
            placeholder="搜索仓库编码或名称"
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            className="tm-procurement-search"
          />
          <Button loading={loading} onClick={() => void load()}>刷新列表</Button>
        </OperationToolbar>
        <TmProTable<Warehouse>
          className="tm-procurement-table"
          rowKey="id"
          columns={columns}
          dataSource={filteredRows}
          loading={loading}
          search={false}
          options={false}
          pagination={false}
          cardBordered
          scroll={{ x: 760 }}
          locale={{ emptyText: error ? '仓库列表暂不可用' : '暂无仓库，请先新建仓库。' }}
        />
        <Modal
          title={editing ? `编辑仓库 · ${editing.code}` : '新建仓库'}
          open={modalOpen}
          confirmLoading={submitting}
          okText={editing ? '保存仓库' : '创建仓库'}
          cancelText="取消"
          onCancel={() => !submitting && setModalOpen(false)}
          onOk={() => form.submit()}
          forceRender
        >
          <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void save(values)}>
            <Form.Item
              label="仓库编码"
              name="code"
              rules={[
                { required: true, message: '请输入仓库编码' },
                { pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/, message: '仅支持字母、数字、下划线和短横线' },
              ]}
            >
              <Input disabled={Boolean(editing)} maxLength={64} placeholder="例如 MAIN" />
            </Form.Item>
            <Form.Item label="仓库名称" name="name" rules={[{ required: true, whitespace: true, message: '请输入仓库名称' }]}>
              <Input maxLength={160} placeholder="例如 华东主仓" />
            </Form.Item>
            {editing ? (
              <Form.Item label="状态" name="status" rules={[{ required: true }]}>
                <Radio.Group options={[{ label: '启用', value: 'active' }, { label: '停用', value: 'inactive' }]} />
              </Form.Item>
            ) : null}
            <Form.Item label="设为默认仓" name="isDefault" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Alert type="info" showIcon message="默认仓必须保持启用；设为默认后，同租户原默认仓会自动取消。" />
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
