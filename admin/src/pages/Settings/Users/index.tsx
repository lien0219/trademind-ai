import SettingsMessage from '../SettingsMessage';
import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PAGE_COPY } from '@/constants/copywriting';
import { layoutTokens } from '@/constants/layoutTokens';
import { ROLE_OPTIONS } from '@/constants/permissionLabels';
import { localizedRoleLabel as roleLabel } from './displayLabels';
import {
  confirmAssignStorePermissions,
  confirmChangeUserRole,
} from '@/constants/sensitiveActions';
import { formatDateTime } from '@/utils/formatTime';
import {
  createAdminUser,
  createAdminRole,
  deleteAdminRole,
  deleteAdminUser,
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
import { Alert, Button, Checkbox, Form, Input, Modal, Select, Space, Tag, Tabs, Tooltip, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { PERMISSIONS } from '@/utils/permission';
import { PASSWORD_HINT, passwordError } from '@/utils/password';
import { confirmDisableUser } from './disableUserConfirm';
import RolePermissionPicker from './RolePermissionPicker';
import RolePermissionsPanel from './RolePermissionsPanel';
import PasswordInput from './PasswordInput';
import DeleteAccessDialog, { type DeleteAccessTarget } from './DeleteAccessDialog';

const STATUS_OPTIONS = [
  { get label() { return tr("正常"); }, value: 'active' },
  { get label() { return tr("禁用"); }, value: 'disabled' },
];

const SCOPE_OPTIONS = [
  { get label() { return tr("只读"); }, value: 'view' },
  { get label() { return tr("运营"); }, value: 'operate' },
  { get label() { return tr("管理"); }, value: 'manage' },
];

function roleTag(role: string) {
  const r = (role || '').toLowerCase();
  if (r === 'admin') return <Tag color="blue">{tr("管理员")}</Tag>;
  if (r === 'operator') return <Tag color="cyan">{tr("运营")}</Tag>;
  if (r === 'readonly') return <Tag>{tr("只读")}</Tag>;
  if (r === 'reviewer') return <Tag color="purple">{tr("审核")}</Tag>;
  return <Tag>{role}</Tag>;
}

function adminUserLabel(row: Pick<AdminUserRow, 'displayName' | 'email' | 'username'>): string {
  return (row.displayName || '').trim() || (row.email || '').trim() || row.username || tr("该用户");
}

export default function SettingsUsersPage() {
  const language = useLanguage();
  const actionRef = useRef<ActionType>();
  const { canManageUsers, user: currentUser } = usePermission();
  const submittingRef = useRef(false);
  const [submitting, setSubmitting] = useState(false);
  const [profileEdit, setProfileEdit] = useState<AdminUserRow | null>(null);
  const [profileForm] = Form.useForm<{ displayName: string }>();
  const runMutation = async (mutate: () => Promise<void>) => {
    if (submittingRef.current || !canManageUsers) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      await mutate();
    } catch (error) {
      message.error((error as Error)?.message || tr("操作失败，请重试"));
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };
  const [createOpen, setCreateOpen] = useState(false);
  const [temporaryPassword, setTemporaryPassword] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<DeleteAccessTarget | null>(null);
  const [roles, setRoles] = useState<AdminRoleRow[]>([]);
  const [permissionCatalog, setPermissionCatalog] = useState<string[]>([]);
  const [rolesLoading, setRolesLoading] = useState(false);
  const [rolesReady, setRolesReady] = useState(false);
  const [roleLoadError, setRoleLoadError] = useState<string | null>(null);
  const [roleEdit, setRoleEdit] = useState<AdminRoleRow | null>(null);
  const [roleOpen, setRoleOpen] = useState(false);
  const [roleInitialValues, setRoleInitialValues] = useState<{
    name?: string;
    description?: string;
    permissions: string[];
    enabled: boolean;
  }>({ permissions: [], enabled: true });
  const [roleForm] = Form.useForm();
  const [activeTab, setActiveTab] = useState('users');
  useEffect(() => {
    if (!roleOpen) return;
    roleForm.resetFields();
    roleForm.setFieldsValue(roleInitialValues);
  }, [roleOpen, roleInitialValues, roleForm]);
  const loadRoles = useCallback(async () => {
    setRolesLoading(true);
    setRoleLoadError(null);
    try {
      const [result, permissions] = await Promise.all([fetchAdminRoles(), fetchRolePermissionCatalog()]);
      setRoles(result.list || []);
      setPermissionCatalog(permissions.permissions || []);
      setRolesReady(true);
    } catch (error) {
      setRoles([]);
      setPermissionCatalog([]);
      setRolesReady(false);
      setRoleLoadError((error as Error)?.message || tr("角色权限加载失败，请重试"));
    } finally {
      setRolesLoading(false);
    }
  }, []);
  useEffect(() => {
    if (canManageUsers) void loadRoles();
  }, [canManageUsers, loadRoles]);
  const roleDirectoryReady = rolesReady && !rolesLoading && !roleLoadError;
  const roleOptions = useMemo(() => roles
    .filter((r) => r.enabled)
    .map((r) => ({ label: roleLabel(r.code, r.name), value: r.code })), [roles, language]);
  const emptyLocale = useListEmptyLocale('usersSettings', {
    onAction: () => setCreateOpen(true),
    actionLabel: tr("创建用户"),
  });
  const [permOpen, setPermOpen] = useState(false);
  const [editUser, setEditUser] = useState<AdminUserRow | null>(null);
  const [shops, setShops] = useState<ShopListRow[]>([]);
	const shopsLoadingRef = useRef(false);
	const [shopsLoading, setShopsLoading] = useState(false);
  const [createForm] = Form.useForm();
  const [permForm] = Form.useForm();
  useEffect(() => {
    if (!permOpen || !editUser) return;
    permForm.resetFields();
    permForm.setFieldsValue({
      items: (editUser.storePermissions || []).map((p) => ({
        storeId: p.storeId,
        permissionScope: p.permissionScope || 'operate',
      })),
    });
  }, [editUser, permForm, permOpen]);
  useEffect(() => {
    if (!profileEdit) return;
    profileForm.resetFields();
    profileForm.setFieldsValue({ displayName: profileEdit.displayName || '' });
  }, [profileEdit, profileForm]);

  const loadShops = useCallback(async () => {
    try {
      const res = await queryShops({ page: 1, pageSize: 200 });
      setShops(res.list || []);
      return true;
    } catch (error) {
      message.error((error as Error)?.message || tr("店铺加载失败，请重试"));
      return false;
    }
  }, []);

  const columns: ProColumns<AdminUserRow>[] = [
    { title: tr("显示名"), dataIndex: 'displayName', width: 140, ellipsis: true },
    { title: tr("邮箱"), dataIndex: 'email', width: 180, ellipsis: true, search: false },
    { title: tr("手机"), dataIndex: 'phone', width: 120, search: false },
    {
      title: tr("角色"),
      dataIndex: 'role',
      width: 100,
      valueType: 'select',
      fieldProps: { options: roleOptions.length ? roleOptions : ROLE_OPTIONS },
      render: (_, row) => {
        const role = roles.find((r) => r.code === row.role);
        return role ? <Tag>{roleLabel(role.code, role.name)}</Tag> : roleTag(row.role);
      },
    },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 90,
      valueType: 'select',
      fieldProps: { options: STATUS_OPTIONS },
      render: (_, row) =>
        row.status === 'disabled' ? <Tag color="error">{tr("禁用")}</Tag> : <Tag color="success">{tr("正常")}</Tag>,
    },
    {
      title: tr("授权店铺"),
      dataIndex: 'storePermissions',
      search: false,
      ellipsis: true,
      render: (_, row) =>
        row.role === 'admin'
          ? tr("全部")
          : (row.storePermissions || []).map((p) => p.storeName || p.storeId).join('、') || '—',
    },
    {
      title: tr("最近操作"),
      dataIndex: 'lastOperationAt',
      width: 168,
      search: false,
      render: (_, row) => formatDateTime(row.lastOperationAt),
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 280,
      render: (_, row) => {
        const isCurrentAccount = row.id === currentUser?.id;
        return (
          <Space wrap size={[0, 4]} style={{ width: '100%' }}>
            <Button key="edit" type="link" size="small" disabled={!canManageUsers || submitting}
              onClick={() => setProfileEdit(row)}>{tr("编辑用户")}</Button>
            <Button
              key="role"
              type="link"
              size="small"
              disabled={!canManageUsers || submitting}
              onClick={() => {
                let selectedRole = row.role;
                Modal.confirm({
                  title: <SettingsMessage source={"修改角色"} />,
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
                  okText: <SettingsMessage source={"确认修改"} />,
                  cancelText: <SettingsMessage source={"取消"} />,
                  onOk: () => {
                    if (selectedRole === row.role) return;
                    const roleLabel = roleOptions.find((o) => o.value === selectedRole)?.label || selectedRole;
                    confirmChangeUserRole(adminUserLabel(row), roleLabel, () => runMutation(async () => {
                      await updateAdminUser(row.id, { role: selectedRole });
                      message.success(tr("角色已更新"));
                      actionRef.current?.reload();
                    }));
                  },
                });
              }}
            >{tr("改角色")}</Button>
            <Tooltip title={row.role === 'admin' ? tr("管理员默认拥有全部店铺权限") : undefined}>
              <span>
                <Button
                  key="perm"
                  disabled={row.role === 'admin' || !canManageUsers || submitting || shopsLoading}
                  type="link"
                  size="small"
                  onClick={async () => {
                    if (shopsLoadingRef.current) return;
                    shopsLoadingRef.current = true;
                    setShopsLoading(true);
                    try {
                      if (!(await loadShops())) return;
                      setEditUser(row);
                      setPermOpen(true);
                    } finally {
                      shopsLoadingRef.current = false;
                      setShopsLoading(false);
                    }
                  }}
                >{tr("店铺权限")}</Button>
              </span>
            </Tooltip>
            <Tooltip title={isCurrentAccount ? tr("不能重置当前登录账号的密码") : undefined}>
              <span>
                <Button
                  key="resetPassword"
                  type="link"
                  disabled={!canManageUsers || submitting || isCurrentAccount}
                  size="small"
                  onClick={() => Modal.confirm({
                    title: <SettingsMessage source={"重置用户密码"} />,
                    okText: <SettingsMessage source={"确认重置"} />,
                    cancelText: <SettingsMessage source={"取消"} />,
                    content: <SettingsMessage source={"将重置「{value0}」的登录密码并使旧登录令牌失效。请通过安全渠道交付临时密码。"} values={{ value0: adminUserLabel(row) }} />,
                    onOk: () => runMutation(async () => {
                      const result = await resetAdminUserPassword(row.id);
                      setTemporaryPassword(result.temporaryPassword);
                      message.success(tr("密码已重置，请妥善保存临时密码"));
                    }),
                  })}
                >{tr("重置密码")}</Button>
              </span>
            </Tooltip>
            <Tooltip title={isCurrentAccount ? tr("不能修改当前登录账号的状态") : undefined}>
              <span>
                <Button
                  key="disable"
                  disabled={!canManageUsers || submitting || isCurrentAccount}
                  type="link"
                  size="small"
                  danger={row.status !== 'disabled'}
                  onClick={() => {
                    const next = row.status === 'disabled' ? 'active' : 'disabled';
                    if (next === 'disabled') {
                      confirmDisableUser(adminUserLabel(row), () => runMutation(async () => {
                        await updateAdminUser(row.id, { status: next });
                        message.success(tr("已更新"));
                        actionRef.current?.reload();
                      }));
                    } else {
                      void runMutation(async () => {
                        await updateAdminUser(row.id, { status: next });
                        message.success(tr("已更新"));
                        actionRef.current?.reload();
                      });
                    }
                  }}
                >
                  {row.status === 'disabled' ? tr("启用") : tr("禁用")}
                </Button>
              </span>
            </Tooltip>
            <Tooltip title={isCurrentAccount ? tr("不能删除当前登录账号") : undefined}>
              <span><Button type="link" size="small" danger disabled={!canManageUsers || submitting || isCurrentAccount}
                onClick={() => setDeleteTarget({ kind: 'user', id: row.id, name: adminUserLabel(row) })}>{tr("删除用户")}</Button></span>
            </Tooltip>
          </Space>
        );
      },
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.USER_MANAGE} showForbiddenPage>
      <TmPageContainer title={PAGE_COPY.usersSettings.title} subTitle={PAGE_COPY.usersSettings.description}>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          { key: 'users', label: tr("用户管理") },
          { key: 'roles', label: tr("角色与权限矩阵") },
        ]} />
        {activeTab === 'roles' ? (
          <>
            {roleLoadError && (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 16 }}
                message={tr("角色与权限目录加载失败")}
                description={roleLoadError}
                action={<Button size="small" loading={rolesLoading} onClick={() => void loadRoles()}>{tr("重试角色目录")}</Button>}
              />
            )}
            <RolePermissionsPanel roles={roles} loading={rolesLoading} ready={roleDirectoryReady} hasError={!!roleLoadError}
              canManage={canManageUsers} submitting={submitting} onRefresh={() => void loadRoles()} onCreate={() => {
                setRoleEdit(null);
                setRoleInitialValues({ permissions: [], enabled: true });
                setRoleOpen(true);
              }} onEdit={(r) => {
                setRoleEdit(r);
                setRoleInitialValues({
                  name: r.name,
                  description: r.description,
                  permissions: r.permissions,
                  enabled: r.enabled,
                });
                setRoleOpen(true);
              }} onDelete={(r) => setDeleteTarget({ kind: 'role', id: r.code, name: roleLabel(r.code, r.name) })} onToggle={(r) => {
                Modal.confirm({
                  title: r.enabled ? <SettingsMessage source={"停用角色并立即撤权？"} /> : <SettingsMessage source={"重新启用角色？"} />,
                  content: <SettingsMessage source={"角色「{value0}」现有 {value1} 位成员，变更将影响其访问权限。"} values={{ value0: roleLabel(r.code, r.name), value1: r.memberCount }} />,
                  okText: r.enabled ? <SettingsMessage source={"确认停用"} /> : <SettingsMessage source={"确认启用"} />,
                  cancelText: <SettingsMessage source={"取消"} />,
                  onOk: () => runMutation(async () => {
                    await updateAdminRole(r.code, { enabled: !r.enabled });
                    message.success(tr("角色状态已更新"));
                    await loadRoles();
                  }),
                });
              }} />
          </>
        ) : (
        <ProTable<AdminUserRow>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={{ labelWidth: 80 }}
          locale={emptyLocale}
          toolBarRender={() => [
            <Button key="create" type="primary" disabled={!canManageUsers} onClick={() => setCreateOpen(true)}>{tr("新建用户")}</Button>,
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
          title={roleEdit ? tr("编辑角色与权限") : tr("新增自定义角色")}
          open={roleOpen}
          onCancel={() => { if (!submitting) setRoleOpen(false); }}
          onOk={() => { if (roleDirectoryReady) roleForm.submit(); }}
          confirmLoading={submitting}
          okButtonProps={{ disabled: submitting || !roleDirectoryReady, 'aria-label': roleEdit ? tr("保存修改") : tr("创建角色") }}
          okText={roleEdit ? tr("保存修改") : tr("创建角色")}
          cancelText={tr("取消")}
          destroyOnHidden
          centered
          width={layoutTokens.modalWidthLg}
          className="tm-role-editor"
          styles={{ body: { maxHeight: 'calc(100dvh - 200px)', overflowY: 'auto' } }}
        >
          <Form key={roleEdit?.code || 'new-role'} name="admin-role" form={roleForm} preserve={false} initialValues={roleInitialValues} layout="vertical" onFinish={(values) => runMutation(async () => {
              if (!roleDirectoryReady) return;
              const input = {
                name: values.name as string,
                description: values.description as string,
                permissions: (values.permissions || []) as string[],
                enabled: values.enabled as boolean,
              };
              if (roleEdit) await updateAdminRole(roleEdit.code, input);
              else await createAdminRole(input);
              setRoleOpen(false);
              message.success(tr("角色及权限已保存"));
              await loadRoles();
              actionRef.current?.reload();
          })}>
            <p className="tm-role-editor__intro">{tr("按岗位配置权限，创建后可分配给用户。")}</p>
            {roleEdit && <Alert className="tm-role-editor__notice" type="info" showIcon message={trFormat("当前关联用户：{value0} 人。保存权限或停用角色会触发重新鉴权。", { value0: roleEdit.memberCount })} />}
            <div className="tm-role-editor__fields">
              <Form.Item label={tr("角色名称")} name="name" rules={[{ required: true, message: tr("请输入角色名称") }, { max: 80, message: tr("角色名称不能超过 80 个字符") }]}>
                <Input maxLength={80} placeholder={tr("例如：商品运营、财务审核")} />
              </Form.Item>
              <Form.Item name="enabled" label={tr("启用状态")} valuePropName="checked">
                <Checkbox>{tr("启用此角色")}</Checkbox>
              </Form.Item>
            </div>
            <Form.Item label={tr("角色描述")} name="description">
              <Input.TextArea maxLength={400} rows={2} showCount placeholder={tr("说明此角色适用的岗位和职责（选填）")} />
            </Form.Item>
            <Form.Item name="permissions" label={tr("权限范围")} extra={tr("所选权限决定可访问的页面与操作；未选择的功能不授予权限。")}>
              <RolePermissionPicker catalog={permissionCatalog} disabled={submitting || !roleDirectoryReady} />
            </Form.Item>
          </Form>
        </Modal>
        <Modal
          title={tr("一次性临时密码")}
          open={temporaryPassword !== null}
          onCancel={() => setTemporaryPassword(null)}
          onOk={() => setTemporaryPassword(null)}
          okText={tr("已安全保存")}
          cancelText={tr("关闭")}
          destroyOnHidden
        >
          <p>{tr("此临时密码仅在此处显示一次。关闭后无法重新查看，切勿发送到公开聊天或日志。")}</p>
          <PasswordInput value={temporaryPassword || ''} readOnly />
        </Modal>
        <Modal
          title={tr("编辑用户")}
          open={profileEdit !== null}
          onCancel={() => { if (!submitting) setProfileEdit(null); }}
          onOk={() => profileForm.submit()}
          okText={tr("保存")}
          confirmLoading={submitting}
          okButtonProps={{ disabled: submitting }}
          destroyOnHidden
        >
          <Form name="admin-user-edit" form={profileForm} preserve={false} initialValues={{ displayName: profileEdit?.displayName || '' }} layout="vertical" onFinish={(values) => runMutation(async () => {
            if (!profileEdit) return;
            await updateAdminUser(profileEdit.id, { displayName: values.displayName });
            message.success(tr("用户已更新"));
            setProfileEdit(null);
            actionRef.current?.reload();
          })}>
            <Form.Item name="displayName" label={tr("显示名")} rules={[{ max: 128 }]}><Input maxLength={128} /></Form.Item>
          </Form>
        </Modal>
        <Modal
          title={tr("新建用户")}
          open={createOpen}
          onCancel={() => {
            if (submitting) return;
            createForm.resetFields();
            setCreateOpen(false);
          }}
          onOk={() => createForm.submit()}
          confirmLoading={submitting}
          okButtonProps={{ disabled: submitting }}
          destroyOnHidden
        >
          <Form
            name="admin-user-create"
            form={createForm}
            preserve={false}
            layout="vertical"
            onFinish={(v) => runMutation(async () => {
              await createAdminUser(v);
              message.success(tr("用户已创建"));
              setCreateOpen(false);
              createForm.resetFields();
              actionRef.current?.reload();
            })}
          >
            <Form.Item name="email" label={tr("邮箱")} rules={[{ required: true }]}>
              <Input placeholder="operator@example.com" />
            </Form.Item>
            <Form.Item name="password" label={tr("初始密码")} extra={tr(PASSWORD_HINT)} rules={[
              { required: true, message: tr("请输入初始密码") },
              { validator: (_, value) => {
                const error = passwordError(value || '');
                return error ? Promise.reject(new Error(tr(error))) : Promise.resolve();
              } },
            ]}>
              <PasswordInput disabled={submitting} />
            </Form.Item>
            <Form.Item name="displayName" label={tr("显示名")}>
              <Input />
            </Form.Item>
            <Form.Item name="role" label={tr("角色")} initialValue="operator">
              <Select options={roleOptions} />
            </Form.Item>
          </Form>
        </Modal>

        <DeleteAccessDialog target={deleteTarget} submitting={submitting}
          onCancel={() => { if (!submitting) setDeleteTarget(null); }}
          onConfirm={() => void runMutation(async () => {
            if (!deleteTarget) return;
            if (deleteTarget.kind === 'user') await deleteAdminUser(deleteTarget.id);
            else await deleteAdminRole(deleteTarget.id);
            message.success(deleteTarget.kind === 'user' ? tr("用户已删除") : tr("角色已删除"));
            setDeleteTarget(null);
            await loadRoles();
            actionRef.current?.reload();
          })} />

        <Modal
          title={trFormat("分配店铺权限 — {value0}", { value0: editUser?.displayName || '' })}
          open={permOpen}
          width={640}
          onCancel={() => { if (!submitting) setPermOpen(false); }}
          confirmLoading={submitting}
          okButtonProps={{ disabled: submitting }}
          onOk={() => {
            if (!editUser) return;
            confirmAssignStorePermissions(adminUserLabel(editUser), () => permForm.submit());
          }}
          destroyOnHidden
        >
          <Form
            name="admin-user-stores"
            form={permForm}
            preserve={false}
            initialValues={{ items: (editUser?.storePermissions || []).map((p) => ({
              storeId: p.storeId, permissionScope: p.permissionScope || 'operate',
            })) }}
            layout="vertical"
            onFinish={(v) => runMutation(async () => {
              if (!editUser) return;
              await setAdminUserStorePermissions(editUser.id, v.items || []);
              message.success(tr("店铺权限已保存"));
              setPermOpen(false);
              actionRef.current?.reload();
            })}
          >
            <Form.List name="items">
              {(fields, { add, remove }) => (
                <>
                  {fields.map((field) => (
                    <Space wrap key={field.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                      <Form.Item
                        {...field}
                        name={[field.name, 'storeId']}
                        rules={[{ required: true, message: tr("选择店铺") }]}
                      >
                        <Select
                          style={{ width: 240, maxWidth: '100%' }}
                          placeholder={tr("选择店铺")}
                          aria-label={trFormat("店铺授权 {value0}", { value0: Number(field.name) + 1 })}
                          options={shops.map((s) => ({
                            label: `${s.shopName || s.id} (${s.platform})`,
                            value: s.id,
                          }))}
                        />
                      </Form.Item>
                      <Form.Item name={[field.name, 'permissionScope']}>
                        <Select style={{ width: 120 }} options={SCOPE_OPTIONS} />
                      </Form.Item>
                      <Button type="link" onClick={() => remove(field.name)}>{tr("移除")}</Button>
                    </Space>
                  ))}
                  <Button type="dashed" onClick={() => add({ permissionScope: 'operate' })} block>{tr("添加店铺")}</Button>
                </>
              )}
            </Form.List>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
