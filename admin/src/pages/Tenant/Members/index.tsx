import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  MailOutlined,
  PlusOutlined,
  ReloadOutlined,
  UserAddOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  message,
  Result,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { EmptyState, ErrorAlert, SectionCard, StatusTag, TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  cancelTenantInvitation,
  createTenantInvitation,
  getTenantInvitations,
  getTenantMembers,
  leaveTenant,
  updateTenantMember,
  type TenantInvitation,
  type TenantMember,
  type TenantMemberStorePermission,
} from '@/services/tenants';
import { fetchAdminRoles, type AdminRoleRow } from '@/services/adminUsers';
import { queryShops, type ShopListRow } from '@/services/shops';
import { confirmSensitiveAction } from '@/utils/sensitiveConfirm';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { useTenantLeaveEligibility } from '@/hooks/useTenantLeaveEligibility';
import { captureTenantContext, isTenantContextCurrent } from '@/utils/tenantContext';

const MEMBER_STATUS_OPTIONS = [
  { label: '正常', value: 'active' },
  { label: '禁用', value: 'disabled' },
  { label: '移除', value: 'removed' },
];

const STANDARD_ROLE_OPTIONS = [
  { label: '管理员', value: 'admin' },
  { label: '运营', value: 'operator' },
  { label: '只读', value: 'readonly' },
  { label: '审核', value: 'reviewer' },
];

const INVITATION_EXPIRY_OPTIONS = [
  { label: '24 小时', value: 24 },
  { label: '3 天', value: 72 },
  { label: '7 天', value: 168 },
  { label: '14 天', value: 336 },
  { label: '30 天', value: 720 },
];

const SCOPE_OPTIONS = [
  { label: '只读', value: 'view' },
  { label: '运营', value: 'operate' },
  { label: '管理', value: 'manage' },
];

function roleLabel(role: string, roles: AdminRoleRow[]) {
  return roles.find((item) => item.code === role)?.name || STANDARD_ROLE_OPTIONS.find((item) => item.value === role)?.label || role;
}

function memberName(member: TenantMember) {
  return member.displayName?.trim() || member.email || member.userId;
}

type StorePermissionFormValue = {
  storeId: string;
  permissionScope: string;
};

type InviteFormValues = {
  email: string;
  role: string;
  storePermissions?: StorePermissionFormValue[];
  expiresInHours: number;
};

type MemberFormValues = {
  role: string;
  status: string;
  storePermissions?: StorePermissionFormValue[];
};

function storesToPayload(items: StorePermissionFormValue[] | undefined, shops: ShopListRow[]): TenantMemberStorePermission[] {
  return (items || []).filter((item) => item.storeId).map(({ storeId, permissionScope }) => {
    const shop = shops.find((item) => item.id === storeId);
    return { storeId, platform: shop?.platform, permissionScope: permissionScope || 'operate' };
  });
}

function StorePermissionList({ shops }: { shops: ShopListRow[] }) {
  return (
    <Form.List
      name="storePermissions"
      rules={[{
        validator: async (_rule, value: StorePermissionFormValue[] | undefined) => {
          const ids = (value || []).map((item) => item?.storeId).filter(Boolean);
          if (new Set(ids).size !== ids.length) throw new Error('同一家店铺只能配置一次权限');
        },
      }]}
    >
      {(fields, { add, remove }, { errors }) => (
        <>
          {fields.map((field) => (
            <Space key={field.key} wrap align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
              <Form.Item
                name={[field.name, 'storeId']}
                rules={[{ required: true, message: '请选择店铺' }]}
              >
                <Select
                  style={{ width: 260, maxWidth: '100%' }}
                  placeholder="选择店铺"
                  aria-label={`店铺授权 ${Number(field.name) + 1}`}
                  options={shops.map((shop) => ({
                    label: `${shop.shopName}（${shop.platform}）`,
                    value: shop.id,
                  }))}
                />
              </Form.Item>
              <Form.Item
                name={[field.name, 'permissionScope']}
                rules={[{ required: true, message: '请选择权限级别' }]}
              >
                <Select style={{ width: 120 }} options={SCOPE_OPTIONS} aria-label={`店铺权限级别 ${Number(field.name) + 1}`} />
              </Form.Item>
              <Button type="link" danger icon={<DeleteOutlined />} aria-label="移除店铺权限" onClick={() => remove(field.name)} />
            </Space>
          ))}
          <Form.ErrorList errors={errors} />
          <Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ permissionScope: 'operate' })} block>
            添加店铺权限
          </Button>
        </>
      )}
    </Form.List>
  );
}

export default function TenantMembersPage() {
  const { can, user } = usePermission();
  const canManage = can(PERMISSIONS.USER_MANAGE);
  const canLeave = useTenantLeaveEligibility(user);
  const [activeTab, setActiveTab] = useState('members');
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [invitations, setInvitations] = useState<TenantInvitation[]>([]);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [roles, setRoles] = useState<AdminRoleRow[]>([]);
  const [rolesLoading, setRolesLoading] = useState(true);
  const [rolesError, setRolesError] = useState<string | null>(null);
  const [shopsLoading, setShopsLoading] = useState(true);
  const [shopsError, setShopsError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [memberOpen, setMemberOpen] = useState(false);
  const [selectedMember, setSelectedMember] = useState<TenantMember | null>(null);
  const [inviteToken, setInviteToken] = useState<string | null>(null);
  const submittingRef = useRef(false);
  const [submitting, setSubmitting] = useState(false);
  const [inviteForm] = Form.useForm<InviteFormValues>();
  const [memberForm] = Form.useForm<MemberFormValues>();
  const membersRead = useRef(0);
  const directoriesRead = useRef(0);

  const load = useCallback(async () => {
    if (!canManage) return;
    const sequence = ++membersRead.current;
    const context = captureTenantContext();
    setLoading(true);
    setError(null);
    try {
      const [memberResult, invitationResult] = await Promise.all([getTenantMembers(), getTenantInvitations()]);
      if (sequence !== membersRead.current || !isTenantContextCurrent(context)) return;
      setMembers(memberResult.items || []);
      setInvitations(invitationResult.items || []);
    } catch (requestError) {
      if (sequence === membersRead.current && isTenantContextCurrent(context))
        setError((requestError as Error)?.message || '企业成员数据加载失败，请重试');
    } finally {
      if (sequence === membersRead.current) setLoading(false);
    }
  }, [canManage]);

  const loadDirectories = useCallback(async () => {
    if (!canManage) return;
    const sequence = ++directoriesRead.current;
    const context = captureTenantContext();
    const isCurrent = () => sequence === directoriesRead.current && isTenantContextCurrent(context);
    setRolesLoading(true);
    setShopsLoading(true);
    setRolesError(null);
    setShopsError(null);
    const roleResult = fetchAdminRoles()
      .then((result) => { if (isCurrent()) setRoles(result.list || []); })
      .catch((requestError) => {
        if (!isCurrent()) return;
        setRoles([]);
        setRolesError((requestError as Error)?.message || '企业角色加载失败');
      })
      .finally(() => { if (isCurrent()) setRolesLoading(false); });
    const shopResult = queryShops({ page: 1, pageSize: 200 })
      .then((result) => { if (isCurrent()) setShops(result.list || []); })
      .catch((requestError) => {
        if (!isCurrent()) return;
        setShops([]);
        setShopsError((requestError as Error)?.message || '企业店铺加载失败');
      })
      .finally(() => { if (isCurrent()) setShopsLoading(false); });
    await Promise.all([roleResult, shopResult]);
  }, [canManage]);

  useEffect(() => {
    if (!canManage) return;
    void load();
    void loadDirectories();
    return () => {
      ++membersRead.current;
      ++directoriesRead.current;
    };
  }, [canManage, load, loadDirectories]);

  const roleOptions = useMemo(() => {
    return roles.filter((role) => role.enabled).map((role) => ({ label: role.name, value: role.code }));
  }, [roles]);
  const directoryReady = !rolesLoading && !shopsLoading && !rolesError && !shopsError && roleOptions.length > 0;

  const runMutation = async (
    mutate: () => Promise<void>,
    successMessage: string,
    allowWithoutManage = false,
    refreshAfterMutation = true,
  ): Promise<boolean> => {
    if ((!canManage && !allowWithoutManage) || submittingRef.current) return false;
    submittingRef.current = true;
    setSubmitting(true);
    const context = captureTenantContext();
    try {
      await mutate();
      if (!isTenantContextCurrent(context)) return false;
      // Leaving an organization revokes its read access immediately. Never
      // re-fetch the departed tenant before navigating away.
      if (refreshAfterMutation) await load();
      if (!isTenantContextCurrent(context)) return false;
      message.success(successMessage);
      return true;
    } catch (requestError) {
      if (isTenantContextCurrent(context)) message.error((requestError as Error)?.message || '操作失败，请重试');
      return false;
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const openMember = (member: TenantMember) => {
    if (!canManage || !directoryReady) return;
    setSelectedMember(member);
    setMemberOpen(true);
  };

  const submitMember = async (values: MemberFormValues) => {
    if (!selectedMember || !directoryReady) return;
    const succeeded = await runMutation(
      () => updateTenantMember(selectedMember.id, {
        role: values.role,
        status: values.status,
        storePermissions: values.role === 'admin' ? [] : storesToPayload(values.storePermissions, shops),
      }).then(() => undefined),
      '成员信息已更新',
    );
    if (succeeded) setMemberOpen(false);
  };

  const confirmMemberStatus = (member: TenantMember, status: 'disabled' | 'removed') => {
    const label = status === 'removed' ? '移除成员' : '禁用成员';
    confirmSensitiveAction({
      title: `${label}「${memberName(member)}」？`,
      content: status === 'removed' ? '移除后成员将失去当前企业访问权，历史审计记录会保留。' : '禁用后成员暂时不能访问当前企业。',
      impacts: [`成员：${memberName(member)}`, `状态：${status === 'removed' ? '移除' : '禁用'}`],
      reversible: status !== 'removed',
      onOk: async () => {
        await runMutation(
          () => updateTenantMember(member.id, { role: member.role, status, storePermissions: member.storePermissions || [] }).then(() => undefined),
          `${label}成功`,
        );
      },
    });
  };

  const submitInvite = async (values: InviteFormValues) => {
    if (!directoryReady) return;
    const succeeded = await runMutation(
      async () => {
        const result = await createTenantInvitation({
          email: values.email.trim(),
          role: values.role,
          storePermissions: values.role === 'admin' ? [] : storesToPayload(values.storePermissions, shops),
          expiresInHours: values.expiresInHours,
        });
        setInviteToken(result.token);
        setInviteOpen(false);
      },
      '邀请已创建，请复制一次性邀请 Token',
    );
    if (succeeded) inviteForm.resetFields();
  };

  const copyToken = async () => {
    if (!inviteToken) return;
    try {
      await navigator.clipboard.writeText(inviteToken);
      message.success('邀请 Token 已复制');
    } catch {
      message.error('复制失败，请手动复制');
    }
  };

  const cancelInvitation = (invitation: TenantInvitation) => {
    confirmSensitiveAction({
      title: '取消这条邀请？',
      content: '取消后该邀请 Token 将不能再用于接受企业邀请。',
      impacts: [`受邀邮箱：${invitation.email}`],
      reversible: false,
      onOk: async () => {
        await runMutation(
          () => cancelTenantInvitation(invitation.id).then(() => undefined),
          '邀请已取消',
        );
      },
    });
  };

  const leave = () => {
    if (!canLeave) return;
    confirmSensitiveAction({
      title: '退出当前企业？',
      content: '退出后将立即失去当前企业访问权，需要重新获得邀请才能加入。',
      impacts: ['当前会话将刷新', '未保存的表单内容会丢失'],
      reversible: true,
      onOk: async () => {
        const succeeded = await runMutation(() => leaveTenant().then(() => undefined), '已退出企业', true, false);
        if (succeeded) window.location.reload();
      },
    });
  };

  const openInvite = () => {
    if (!canManage || !directoryReady) return;
    setInviteOpen(true);
  };

  const memberColumns = useMemo<TableColumnsType<TenantMember>>(() => [
    { title: '成员', key: 'member', width: 220, render: (_value, row) => <Space direction="vertical" size={0}><strong>{memberName(row)}</strong><span style={{ color: 'var(--ant-color-text-tertiary)', fontSize: 12 }}>{row.email || row.userId}</span></Space> },
    { title: '角色', dataIndex: 'role', width: 110, render: (value: string) => <Tag>{roleLabel(value, roles)}</Tag> },
    { title: '状态', dataIndex: 'status', width: 100, render: (value: string) => <StatusTag status={value} text={{ active: '正常', disabled: '禁用', removed: '已移除', invited: '待接受' }[value] || value} /> },
    { title: '店铺权限', key: 'stores', ellipsis: true, render: (_value, row) => row.role === 'admin' ? '全部店铺' : row.storePermissions?.map((item) => shops.find((shop) => shop.id === item.storeId)?.shopName || item.storeId).join('、') || '未分配' },
    { title: '操作', key: 'actions', width: 250, render: (_value, row) => <Space wrap size={4}><Button type="link" size="small" icon={<EditOutlined />} disabled={!canManage || !directoryReady || submitting || row.status === 'removed'} onClick={() => openMember(row)}>编辑</Button>{row.status === 'active' ? <Button type="link" size="small" danger disabled={!canManage || submitting || row.userId === user?.id} onClick={() => confirmMemberStatus(row, 'disabled')}>禁用</Button> : null}{row.status !== 'removed' ? <Button type="link" size="small" danger disabled={!canManage || submitting || row.userId === user?.id} onClick={() => confirmMemberStatus(row, 'removed')}>移除</Button> : null}</Space> },
  ], [canManage, directoryReady, roles, shops, submitting, user?.id]);

  const invitationColumns = useMemo<TableColumnsType<TenantInvitation>>(() => [
    { title: '受邀邮箱', dataIndex: 'email', width: 260, ellipsis: true },
    { title: '角色', dataIndex: 'role', width: 120, render: (value: string) => <Tag>{roleLabel(value, roles)}</Tag> },
    { title: '状态', dataIndex: 'status', width: 110, render: (value: string) => <StatusTag status={value} text={{ pending: '待接受', accepted: '已接受', cancelled: '已取消', expired: '已过期' }[value] || value} /> },
    { title: '有效期至', dataIndex: 'expiresAt', width: 180, render: (value: string) => formatDateTime(value) },
    { title: '创建时间', dataIndex: 'createdAt', width: 180, render: (value: string) => formatDateTime(value) },
    { title: '操作', key: 'actions', width: 110, render: (_value, row) => row.status === 'pending' ? <Button type="link" danger size="small" icon={<DeleteOutlined />} disabled={!canManage || submitting} onClick={() => cancelInvitation(row)}>取消邀请</Button> : null },
  ], [canManage, directoryReady, roles, submitting]);

  if (!canManage) {
    return (
      <TmPageContainer title="企业成员与邀请">
        <Result
          status="403"
          title="无权访问"
          subTitle="企业成员和邀请管理仅对拥有 user.manage 权限的成员开放。"
          extra={canLeave ? <Button danger icon={<DeleteOutlined />} disabled={submitting} onClick={leave}>退出当前企业</Button> : undefined}
        />
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer
      title="企业成员与邀请"
      subTitle="管理当前企业成员、角色和店铺授权；平台角色不会由此页面创建。"
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>,
        <Button key="invite" type="primary" icon={<UserAddOutlined />} disabled={!directoryReady} onClick={openInvite}>邀请成员</Button>,
      ]}
    >
      {error ? <ErrorAlert title="成员数据加载失败" actionHint={error} action={<Button type="link" onClick={() => void load()}>重试</Button>} style={{ marginBottom: 16 }} /> : null}
      {rolesError || shopsError ? <Alert type="error" showIcon message="角色或店铺授权数据加载失败" description={[rolesError, shopsError].filter(Boolean).join('；')} action={<Button type="link" onClick={() => void loadDirectories()}>重试</Button>} style={{ marginBottom: 16 }} /> : null}
      {!rolesError && !shopsError && !rolesLoading && !shopsLoading && roleOptions.length === 0 ? <Alert type="error" showIcon message="当前企业没有可分配角色" description="请在现有角色管理中启用至少一个角色后再邀请或编辑成员。" style={{ marginBottom: 16 }} /> : null}
      <SectionCard>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[{ key: 'members', label: `成员（${members.length}）`, children: !loading && !error && members.length === 0 ? <EmptyState title="暂无企业成员" description="企业管理员可以邀请成员加入。" actionLabel="邀请成员" onAction={openInvite} /> : <Table rowKey="id" loading={loading} columns={memberColumns} dataSource={members} pagination={false} scroll={{ x: 960 }} /> }, { key: 'invitations', label: `邀请（${invitations.length}）`, children: !loading && !error && invitations.length === 0 ? <EmptyState title="暂无邀请记录" description="创建邀请后，一次性 Token 只会在创建成功时显示。" actionLabel="邀请成员" onAction={openInvite} /> : <Table rowKey="id" loading={loading} columns={invitationColumns} dataSource={invitations} pagination={false} scroll={{ x: 900 }} /> }]} />
      </SectionCard>

      {canLeave ? (
        <div
          role="region"
          aria-label="退出当前企业"
          style={{
            marginTop: 16,
            padding: '14px 16px',
            border: '1px solid var(--ant-color-border-secondary)',
            borderRadius: 12,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 16,
            background: 'var(--ant-color-bg-container)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, flex: '1 1 280px' }}>
            <span style={{
              display: 'inline-flex', padding: 9, borderRadius: 10,
              background: 'var(--ant-color-warning-bg)',
              color: 'var(--ant-color-warning)',
            }}>
              <DeleteOutlined aria-hidden="true" />
            </span>
            <div>
              <strong style={{ display: 'block', marginBottom: 4 }}>退出当前企业</strong>
              <span style={{ color: 'var(--ant-color-text-secondary)', fontSize: 13 }}>
                退出后将失去此企业的访问权限，需重新接受邀请才能加入。
              </span>
            </div>
          </div>
          <Button danger icon={<DeleteOutlined />} disabled={submitting} onClick={leave}>
            退出企业
          </Button>
        </div>
      ) : null}

      <Modal open={inviteOpen} title="邀请成员" destroyOnHidden width={560} confirmLoading={submitting} okButtonProps={{ disabled: !directoryReady }} okText="创建邀请" cancelText="取消" onCancel={() => { if (!submitting) setInviteOpen(false); }} onOk={() => { void inviteForm.submit(); }}>
        <Form key="invite" form={inviteForm} layout="vertical" onFinish={submitInvite} preserve={false} initialValues={{ role: 'operator', expiresInHours: 168 }}>
          <Form.Item name="email" label="成员邮箱" rules={[{ required: true, message: '请输入成员邮箱' }, { type: 'email', message: '请输入有效邮箱' }]}><Input prefix={<MailOutlined />} autoComplete="off" /></Form.Item>
          <Form.Item name="role" label="企业角色" rules={[{ required: true, message: '请选择企业角色' }]}><Select options={roleOptions} /></Form.Item>
          <Form.Item name="expiresInHours" label="邀请有效期" rules={[{ required: true, message: '请选择邀请有效期' }]}><Select options={INVITATION_EXPIRY_OPTIONS} /></Form.Item>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.role !== current.role}>{({ getFieldValue }) => getFieldValue('role') === 'admin' ? <Alert type="info" showIcon message="管理员默认拥有当前企业全部店铺范围。" /> : <Form.Item label="店铺授权"><StorePermissionList shops={shops} /></Form.Item>}</Form.Item>
        </Form>
      </Modal>

      <Modal open={memberOpen} title={selectedMember ? `编辑成员：${memberName(selectedMember)}` : '编辑成员'} destroyOnHidden width={560} confirmLoading={submitting} okButtonProps={{ disabled: !directoryReady }} okText="保存" cancelText="取消" onCancel={() => { if (!submitting) setMemberOpen(false); }} onOk={() => { void memberForm.submit(); }}>
        <Form
          key={selectedMember?.id ?? 'member'}
          form={memberForm}
          layout="vertical"
          onFinish={submitMember}
          preserve={false}
          initialValues={selectedMember ? {
            role: selectedMember.role,
            status: selectedMember.status,
            storePermissions: selectedMember.storePermissions?.map((item) => ({
              storeId: item.storeId,
              permissionScope: item.permissionScope || 'operate',
            })) || [],
          } : undefined}
        >
          <Form.Item name="role" label="企业角色" rules={[{ required: true, message: '请选择企业角色' }]}><Select options={roleOptions} /></Form.Item>
          <Form.Item name="status" label="成员状态" rules={[{ required: true, message: '请选择成员状态' }]}><Select options={MEMBER_STATUS_OPTIONS} /></Form.Item>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.role !== current.role}>{({ getFieldValue }) => getFieldValue('role') === 'admin' ? <Alert type="info" showIcon message="管理员默认拥有当前企业全部店铺范围。" /> : <Form.Item label="店铺授权"><StorePermissionList shops={shops} /></Form.Item>}</Form.Item>
        </Form>
      </Modal>

      <Modal open={Boolean(inviteToken)} title="邀请已创建" footer={<Button onClick={() => setInviteToken(null)}>关闭</Button>} onCancel={() => setInviteToken(null)}>
        <Alert type="warning" showIcon message="Token 只在本次显示，请通过安全渠道发送给受邀成员。" description="系统不会代发邮件，也不会再次展示完整 Token。" />
        <Input.TextArea value={inviteToken || ''} readOnly autoSize={{ minRows: 3, maxRows: 6 }} style={{ marginTop: 16 }} aria-label="一次性邀请 Token" />
        <Button icon={<CopyOutlined />} style={{ marginTop: 12 }} onClick={() => void copyToken()}>复制 Token</Button>
      </Modal>
    </TmPageContainer>
  );
}
