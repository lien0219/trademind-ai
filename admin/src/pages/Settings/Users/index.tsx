import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  confirmAssignStorePermissions,
  confirmChangeUserRole,
  confirmDisableUser,
} from '@/constants/sensitiveActions';
import { formatDateTime } from '@/utils/formatTime';
import {
  createAdminUser,
  createAdminRole,
  fetchAdminRoles,
  fetchRolePermissionCatalog,
  updateAdminRole,
  type AdminRoleRow,
  fetchAdminUsers,
  setAdminUserStorePermissions,
  updateAdminUser,
  resetAdminUserPassword,
  type AdminUserRow,
} from '@/services/adminUsers';
import { queryShops, type ShopListRow } from '@/services/shops';
import { Button, Checkbox, Divider, Form, Input, Modal, Select, Space, Tag, Tabs, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { PERMISSIONS } from '@/utils/permission';

const ROLE_OPTIONS = [
  { label: '管理员', value: 'admin' },
  { label: '运营', value: 'operator' },
  { label: '只读', value: 'readonly' },
  { label: '审核', value: 'reviewer' },
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
  if (r === 'reviewer') return <Tag color="purple">审核</Tag>;
  return <Tag>{role}</Tag>;
}

function adminUserLabel(row: Pick<AdminUserRow, 'displayName' | 'email' | 'username'>): string {
  return (row.displayName || '').trim() || (row.email || '').trim() || row.username || '该用户';
}

export default function SettingsUsersPage() {
  const actionRef = useRef<ActionType>();
  const { canManageUsers, user: currentUser } = usePermission();
  const [createOpen, setCreateOpen] = useState(false);
  const [temporaryPassword, setTemporaryPassword] = useState<string | null>(null);
  const [roles, setRoles] = useState<AdminRoleRow[]>([]);
  const [permissionCatalog, setPermissionCatalog] = useState<string[]>([]);
  const [rolesLoading, setRolesLoading] = useState(false);
  const [roleEdit, setRoleEdit] = useState<AdminRoleRow | null>(null);
  const [roleOpen, setRoleOpen] = useState(false);
  const [roleForm] = Form.useForm();
  const [activeTab, setActiveTab] = useState('users');
  const loadRoles = useCallback(async () => {
    setRolesLoading(true);
    try {
      const [result, permissions] = await Promise.all([fetchAdminRoles(), fetchRolePermissionCatalog()]);
      setRoles(result.list || []);
      setPermissionCatalog(permissions.permissions || []);
    } catch (error) {
      message.error((error as Error)?.message || '角色权限加载失败');
    } finally {
      setRolesLoading(false);
    }
  }, []);
  useEffect(() => {
    if (canManageUsers) void loadRoles();
  }, [canManageUsers, loadRoles]);
  const [compareRole, setCompareRole] = useState<string>();
  const permissionGroups = useMemo(() => {
    const groups = new Map<string, string[]>();
    for (const key of permissionCatalog) {
      const prefix = key.split('.')[0] || 'other';
      const existing = groups.get(prefix) || [];
      existing.push(key);
      groups.set(prefix, existing);
    }
    return Array.from(groups.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [permissionCatalog]);
  const roleOptions = useMemo(() => roles
    .filter((r) => r.enabled)
    .map((r) => ({ label: r.name, value: r.code })), [roles]);
  const emptyLocale = useListEmptyLocale('usersSettings', {
    onAction: () => setCreateOpen(true),
    actionLabel: '创建用户',
  });
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
      fieldProps: { options: roleOptions.length ? roleOptions : ROLE_OPTIONS },
      render: (_, row) => <Tag>{roles.find((r) => r.code === row.role)?.name || roleTag(row.role)}</Tag>,
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
            let selectedRole = row.role;
            Modal.confirm({
              title: '修改角色',
              content: (
                <Select
                  defaultValue={row.role}
                  style={{ width: '100%', marginTop: 8 }}
                  options={roleOptions}
                  onChange={(v) => {
                    selectedRole = v;
                  }}
                />
              ),
              okText: '确认修改',
              onOk: async () => {
                if (selectedRole === row.role) return;
                const roleLabel = roleOptions.find((o) => o.value === selectedRole)?.label || selectedRole;
                return new Promise<void>((resolve, reject) => {
                  confirmChangeUserRole(adminUserLabel(row), roleLabel, async () => {
                    try {
                      await updateAdminUser(row.id, { role: selectedRole });
                      message.success('角色已更新');
                      actionRef.current?.reload();
                      resolve();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '更新失败');
                      reject(e);
                    }
                  });
                });
              },
            });
          }}
        >
          改角色
        </Button>,
        row.role !== 'admin' ? (
          <Button
            key="perm"
            disabled={!canManageUsers}
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
            key="resetPassword"
            type="link"
            disabled={!canManageUsers}
            size="small"
            onClick={() => Modal.confirm({
              title: '重置用户密码',
              content: `将重置「${adminUserLabel(row)}」的登录密码并使旧登录令牌失效。请通过安全渠道交付临时密码。`,
              onOk: async () => {
                const result = await resetAdminUserPassword(row.id);
                setTemporaryPassword(result.temporaryPassword);
                message.success('密码已重置，请妥善保存临时密码');
              },
            })}
          >重置密码</Button>
        ) : null,
        row.id !== currentUser?.id ? (
          <Button
            key="disable"
            disabled={!canManageUsers}
            type="link"
            size="small"
            danger={row.status !== 'disabled'}
            onClick={() => {
              const next = row.status === 'disabled' ? 'active' : 'disabled';
              if (next === 'disabled') {
                confirmDisableUser(adminUserLabel(row), async () => {
                  await updateAdminUser(row.id, { status: next });
                  message.success('已更新');
                  actionRef.current?.reload();
                });
              } else {
                void updateAdminUser(row.id, { status: next }).then(() => {
                  message.success('已更新');
                  actionRef.current?.reload();
                });
              }
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
      <TmPageContainer title={PAGE_COPY.usersSettings.title} subTitle={PAGE_COPY.usersSettings.description}>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          { key: 'users', label: '用户管理' },
          { key: 'roles', label: '角色与权限矩阵' },
        ]} />
        {activeTab === 'roles' ? (
          <>
            <Space style={{ marginBottom: 16 }}>
              <Button type="primary" disabled={!canManageUsers} onClick={() => {
                setRoleEdit(null);
                roleForm.resetFields();
                roleForm.setFieldsValue({ permissions: [], enabled: true });
                setRoleOpen(true);
              }}>新建角色</Button>
              <Button loading={rolesLoading} onClick={() => void loadRoles()}>刷新角色</Button>
              <Select
                allowClear
                placeholder="选择对比角色"
                style={{ minWidth: 210 }}
                value={compareRole}
                options={roles.map((r) => ({ label: r.name, value: r.code }))}
                onChange={setCompareRole}
              />
            </Space>
            <div role="table" aria-label="角色权限管理">
              {roles.map((r) => (
                <div key={r.code} style={{ padding: 12, borderBottom: '1px solid var(--ant-color-border-secondary, #eee)' }}>
                  <Space wrap>
                    <strong>{r.name}</strong>
                    <Tag>{r.system ? '系统角色' : '自定义角色'}</Tag>
                    <Tag color={r.enabled ? 'success' : 'default'}>{r.enabled ? '启用' : '已停用'}</Tag>
                    <span>成员：{r.memberCount}</span>
                    <span>权限：{r.permissions.length}</span>
                    <Button size="small" disabled={r.system || !canManageUsers} onClick={() => {
                      setRoleEdit(r);
                      roleForm.setFieldsValue({
                        name: r.name, description: r.description,
                        permissions: r.permissions, enabled: r.enabled,
                      });
                      setRoleOpen(true);
                    }}>编辑权限</Button>
                    {!r.system && (
                      <Button size="small" danger={r.enabled} disabled={!canManageUsers} onClick={() => {
                        Modal.confirm({
                          title: r.enabled ? '停用角色并立即撤权？' : '重新启用角色？',
                          content: `角色「${r.name}」现有 ${r.memberCount} 位成员，变更将影响其访问权限。`,
                          onOk: async () => {
                            await updateAdminRole(r.code, { enabled: !r.enabled });
                            message.success('角色状态已更新');
                            await loadRoles();
                          },
                        });
                      }}>{r.enabled ? '停用' : '启用'}</Button>
                    )}
                  </Space>
                  {r.description && <div style={{ marginTop: 6 }}>{r.description}</div>}
                  <div style={{ marginTop: 6, fontSize: 12, opacity: 0.75 }}>{r.permissions.join('、') || '无授权权限'}</div>
                  {compareRole && compareRole !== r.code && (() => {
                    const other = roles.find((role) => role.code === compareRole);
                    if (!other) return null;
                    const additions = r.permissions.filter((key) => !other.permissions.includes(key));
                    const removals = other.permissions.filter((key) => !r.permissions.includes(key));
                    return (
                      <div style={{ marginTop: 8 }}>
                        <div>相对于「{other.name}」的权限差异</div>
                        <Space wrap>
                          {additions.map((key) => <Tag key={key} color="green">+ {key}</Tag>)}
                          {removals.map((key) => <Tag key={key} color="red">− {key}</Tag>)}
                          {!additions.length && !removals.length && <Tag>权限一致</Tag>}
                        </Space>
                      </div>
                    );
                  })()}

                </div>
              ))}
            </div>
          </>
        ) : (
        <ProTable<AdminUserRow>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={{ labelWidth: 80 }}
          locale={emptyLocale}
          toolBarRender={() => [
            <Button key="create" type="primary" disabled={!canManageUsers} onClick={() => setCreateOpen(true)}>
              新建用户
            </Button>,
          ]}
          request={async (params) => {
            const res = await fetchAdminUsers({
              page: params.current,
              pageSize: params.pageSize,
              role: params.role as string | undefined,
              status: params.status as string | undefined,
              keyword: params.displayName as string | undefined,
            });
            return {
              data: res.list || [],
              total: res.pagination?.total || 0,
              success: true,
            };
          }}
        />

        )}
        <Modal
          title={roleEdit ? '编辑角色与权限' : '新增自定义角色'}
          open={roleOpen}
          onCancel={() => setRoleOpen(false)}
          onOk={() => roleForm.submit()}
          destroyOnHidden
          width={780}
        >
          <Form form={roleForm} layout="vertical" onFinish={async (values) => {
            try {
              const input = {
                name: values.name as string,
                description: values.description as string,
                permissions: (values.permissions || []) as string[],
                enabled: values.enabled as boolean,
              };
              if (roleEdit) await updateAdminRole(roleEdit.code, input);
              else await createAdminRole(input);
              setRoleOpen(false);
              message.success('角色及权限已保存');
              await loadRoles();
              actionRef.current?.reload();
            } catch (error) {
              message.error((error as Error)?.message || '角色保存失败');
            }
          }}>
            <Form.Item label="角色名称" name="name" rules={[{ required: true, max: 80 }]}>
              <Input maxLength={80} />
            </Form.Item>
            {roleEdit && <div style={{ marginBottom: 12 }}>当前关联用户：{roleEdit.memberCount} 人。保存权限或停用角色会触发重新鉴权。</div>}
            <Form.Item label="角色描述" name="description">
              <Input.TextArea maxLength={400} />
            </Form.Item>
            <Divider>模块 / 页面 / 按钮 / API 权限</Divider>
            <Form.Item label="模块 / 页面 / 操作 / API 权限" required>
              <Form.Item name="permissions" noStyle>
                <Checkbox.Group style={{ width: '100%' }}>
                  <div style={{ maxHeight: 350, overflowY: 'auto' }}>
                    {permissionGroups.map(([module, keys]) => (
                      <section key={module} style={{ padding: '8px 0', borderBottom: '1px solid #eee' }}>
                        <strong>{module} ({keys.length})</strong>
                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(190px, 1fr))', gap: 8, marginTop: 8 }}>
                          {keys.map((key) => <Checkbox key={key} value={key}>{key}</Checkbox>)}
                        </div>
                      </section>
                    ))}
                  </div>
                </Checkbox.Group>
              </Form.Item>
            </Form.Item>
            <Form.Item name="enabled" label="启用状态" valuePropName="checked">
              <Checkbox>启用此角色</Checkbox>
            </Form.Item>
          </Form>
        </Modal>
        <Modal
          title="一次性临时密码"
          open={temporaryPassword !== null}
          onCancel={() => setTemporaryPassword(null)}
          onOk={() => setTemporaryPassword(null)}
          okText="已安全保存"
          cancelText="关闭"
          destroyOnHidden
        >
          <p>此临时密码仅在此处显示一次。关闭后无法重新查看，切勿发送到公开聊天或日志。</p>
          <Input.Password value={temporaryPassword || ''} readOnly visibilityToggle />
        </Modal>
        <Modal
          title="新建用户"
          open={createOpen}
          onCancel={() => setCreateOpen(false)}
          onOk={() => createForm.submit()}
          destroyOnHidden
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
              <Input placeholder="operator@example.com" />
            </Form.Item>
            <Form.Item name="password" label="初始密码" rules={[{ required: true, min: 12, message: '密码至少 12 位' }]}>
              <Input.Password />
            </Form.Item>
            <Form.Item name="displayName" label="显示名">
              <Input />
            </Form.Item>
            <Form.Item name="role" label="角色" initialValue="operator">
              <Select options={roleOptions} />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title={`分配店铺权限 — ${editUser?.displayName || ''}`}
          open={permOpen}
          width={640}
          onCancel={() => setPermOpen(false)}
          onOk={() => {
            if (!editUser) return;
            confirmAssignStorePermissions(adminUserLabel(editUser), () => permForm.submit());
          }}
          destroyOnHidden
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
