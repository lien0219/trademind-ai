import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PERMISSIONS } from '@/utils/permission';
import { confirmSensitiveAction } from '@/utils/sensitiveConfirm';
import { formatDateTime } from '@/utils/formatTime';
import {
  createAdminUser,
  fetchAdminUsers,
  setAdminUserStorePermissions,
  updateAdminUser,
  type AdminUserRow,
} from '@/services/adminUsers';
import { queryShops, type ShopListRow } from '@/services/shops';
import { Button, Form, Input, Modal, Select, Space, Tag, message } from 'antd';
import { useCallback, useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';

const ROLE_OPTIONS = [
  { label: '管理员', value: 'admin' },
  { label: '运营', value: 'operator' },
  { label: '只读', value: 'readonly' },
];

const STATUS_OPTIONS = [
  { label: '正常', value: 'active' },
  { label: '禁用', value: 'disabled' },
];

const SCOPE_OPTIONS = [
  { label: '只读', value: 'view' },
  { label: '运营', value: 'operate' },
  { label: '管理', value: 'manage' },
];

function roleTag(role: string) {
  const r = (role || '').toLowerCase();
  if (r === 'admin') return <Tag color="blue">管理员</Tag>;
  if (r === 'operator') return <Tag color="cyan">运营</Tag>;
  if (r === 'readonly') return <Tag>只读</Tag>;
  return <Tag>{role}</Tag>;
}

export default function SettingsUsersPage() {
  const actionRef = useRef<ActionType>();
  const { canManageUsers, user: currentUser } = usePermission();
  const [createOpen, setCreateOpen] = useState(false);
  const [permOpen, setPermOpen] = useState(false);
  const [editUser, setEditUser] = useState<AdminUserRow | null>(null);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [createForm] = Form.useForm();
  const [permForm] = Form.useForm();

  const loadShops = useCallback(async () => {
    try {
      const res = await queryShops({ page: 1, pageSize: 200 });
      setShops(res.list || []);
    } catch {
      setShops([]);
    }
  }, []);

  const columns: ProColumns<AdminUserRow>[] = [
    { title: '显示名', dataIndex: 'displayName', width: 140, ellipsis: true },
    { title: '邮箱', dataIndex: 'email', width: 180, ellipsis: true, search: false },
    { title: '手机', dataIndex: 'phone', width: 120, search: false },
    {
      title: '角色',
      dataIndex: 'role',
      width: 100,
      valueType: 'select',
      fieldProps: { options: ROLE_OPTIONS },
      render: (_, row) => roleTag(row.role),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      valueType: 'select',
      fieldProps: { options: STATUS_OPTIONS },
      render: (_, row) =>
        row.status === 'disabled' ? <Tag color="error">禁用</Tag> : <Tag color="success">正常</Tag>,
    },
    {
      title: '授权店铺',
      dataIndex: 'storePermissions',
      search: false,
      ellipsis: true,
      render: (_, row) =>
        row.role === 'admin'
          ? '全部'
          : (row.storePermissions || []).map((p) => p.storeName || p.storeId).join('、') || '—',
    },
    {
      title: '最近操作',
      dataIndex: 'lastOperationAt',
      width: 168,
      search: false,
      render: (_, row) => formatDateTime(row.lastOperationAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 220,
      render: (_, row) => [
        <Button
          key="role"
          type="link"
          size="small"
          disabled={!canManageUsers}
          onClick={() => {
            confirmSensitiveAction({
              title: '修改用户角色',
              content: `将用户 ${row.displayName} 的角色修改为所选值。`,
              impacts: ['该用户菜单与 API 权限将立即变化'],
              onOk: async () => {
                Modal.info({ title: '请在编辑弹窗中修改角色' });
              },
            });
            Modal.confirm({
              title: '修改角色',
              content: (
                <Select
                  defaultValue={row.role}
                  style={{ width: '100%', marginTop: 8 }}
                  options={ROLE_OPTIONS}
                  onChange={async (v) => {
                    await updateAdminUser(row.id, { role: v });
                    message.success('角色已更新');
                    actionRef.current?.reload();
                  }}
                />
              ),
              okButtonProps: { style: { display: 'none' } },
              cancelText: '关闭',
            });
          }}
        >
          改角色
        </Button>,
        row.role !== 'admin' ? (
          <Button
            key="perm"
            type="link"
            size="small"
            onClick={async () => {
              setEditUser(row);
              await loadShops();
              permForm.setFieldsValue({
                items: (row.storePermissions || []).map((p) => ({
                  storeId: p.storeId,
                  permissionScope: p.permissionScope || 'operate',
                })),
              });
              setPermOpen(true);
            }}
          >
            店铺权限
          </Button>
        ) : null,
        row.id !== currentUser?.id ? (
          <Button
            key="disable"
            type="link"
            size="small"
            danger={row.status !== 'disabled'}
            onClick={() => {
              const next = row.status === 'disabled' ? 'active' : 'disabled';
              confirmSensitiveAction({
                title: next === 'disabled' ? '禁用用户' : '启用用户',
                content: `确认${next === 'disabled' ? '禁用' : '启用'}用户 ${row.displayName}？`,
                impacts: ['该用户将无法登录或恢复访问'],
                reversible: next !== 'disabled',
                onOk: async () => {
                  await updateAdminUser(row.id, { status: next });
                  message.success('已更新');
                  actionRef.current?.reload();
                },
              });
            }}
          >
            {row.status === 'disabled' ? '启用' : '禁用'}
          </Button>
        ) : null,
      ],
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.USER_MANAGE} showForbiddenPage>
      <TmPageContainer title="用户与权限" subTitle="管理员可管理后台账号、角色与店铺授权">
        <ProTable<AdminUserRow>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={{ labelWidth: 80 }}
          toolBarRender={() => [
            <Button key="create" type="primary" onClick={() => setCreateOpen(true)}>
              新建用户
            </Button>,
          ]}
          request={async (params) => {
            const res = await fetchAdminUsers({
              page: params.current,
              pageSize: params.pageSize,
              role: params.role,
              status: params.status,
              keyword: params.displayName,
            });
            return {
              data: res.list || [],
              total: res.pagination?.total || 0,
              success: true,
            };
          }}
        />

        <Modal
          title="新建用户"
          open={createOpen}
          onCancel={() => setCreateOpen(false)}
          onOk={() => createForm.submit()}
          destroyOnClose
        >
          <Form
            form={createForm}
            layout="vertical"
            onFinish={async (v) => {
              await createAdminUser(v);
              message.success('用户已创建');
              setCreateOpen(false);
              createForm.resetFields();
              actionRef.current?.reload();
            }}
          >
            <Form.Item name="email" label="邮箱" rules={[{ required: true }]}>
              <Input placeholder="demo_operator@example.com" />
            </Form.Item>
            <Form.Item name="password" label="初始密码" rules={[{ required: true, min: 6 }]}>
              <Input.Password />
            </Form.Item>
            <Form.Item name="displayName" label="显示名">
              <Input />
            </Form.Item>
            <Form.Item name="role" label="角色" initialValue="operator">
              <Select options={ROLE_OPTIONS} />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title={`分配店铺权限 — ${editUser?.displayName || ''}`}
          open={permOpen}
          width={640}
          onCancel={() => setPermOpen(false)}
          onOk={() => {
            confirmSensitiveAction({
              title: '保存店铺权限',
              content: '将覆盖该用户的店铺授权列表。',
              impacts: ['订单/库存/客服/失败任务将按新范围过滤'],
              onOk: () => permForm.submit(),
            });
          }}
          destroyOnClose
        >
          <Form
            form={permForm}
            layout="vertical"
            onFinish={async (v) => {
              if (!editUser) return;
              await setAdminUserStorePermissions(editUser.id, v.items || []);
              message.success('店铺权限已保存');
              setPermOpen(false);
              actionRef.current?.reload();
            }}
          >
            <Form.List name="items">
              {(fields, { add, remove }) => (
                <>
                  {fields.map((field) => (
                    <Space key={field.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                      <Form.Item
                        {...field}
                        name={[field.name, 'storeId']}
                        rules={[{ required: true, message: '选择店铺' }]}
                      >
                        <Select
                          style={{ width: 260 }}
                          placeholder="选择店铺"
                          options={shops.map((s) => ({
                            label: `${s.shopName || s.id} (${s.platform})`,
                            value: s.id,
                          }))}
                        />
                      </Form.Item>
                      <Form.Item {...field} name={[field.name, 'permissionScope']} initialValue="operate">
                        <Select style={{ width: 120 }} options={SCOPE_OPTIONS} />
                      </Form.Item>
                      <Button type="link" onClick={() => remove(field.name)}>
                        移除
                      </Button>
                    </Space>
                  ))}
                  <Button type="dashed" onClick={() => add({ permissionScope: 'operate' })} block>
                    添加店铺
                  </Button>
                </>
              )}
            </Form.List>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
