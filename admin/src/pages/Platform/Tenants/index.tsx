import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import {
  EditOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  StopOutlined,
  CheckCircleOutlined,
  LoginOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { history, Link } from '@umijs/max';
import {
  Button,
  Form,
  Input,
  Modal,
  Pagination,
  Result,
  Alert,
  Select,
  Space,
  Table,
  Tag,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { EmptyState, ErrorAlert, SectionCard, StatusTag, TmPageContainer } from '@/components/ui';
import {
  createPlatformTenant,
  createTenantIdempotencyKey,
  deletePlatformTenant,
  queryPlatformTenants,
  queryAuthTenants,
  switchAuthTenant,
  updatePlatformTenant,
  updatePlatformTenantStatus,
  type Tenant,
  type AuthTenant,
  type TenantCreateBody,
  type TenantMetadataBody,
  type TenantStatus,
} from '@/services/tenants';
import { formatDateTime } from '@/utils/formatTime';
import { confirmSensitiveAction } from '@/utils/sensitiveConfirm';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { switchTenantAndReload } from '@/utils/tenantContext';
import { canEnterTenantWorkspace } from '@/utils/tenantWorkspace';
import { passwordError, PASSWORD_HINT } from '@/utils/password';

const STATUS_OPTIONS: { label: string; value: TenantStatus }[] = [
  { get label() { return tr("待开通"); }, value: 'pending' },
  { get label() { return tr("正常"); }, value: 'active' },
  { get label() { return tr("已暂停"); }, value: 'suspended' },
  { get label() { return tr("已归档"); }, value: 'archived' },
  { get label() { return tr("已关闭"); }, value: 'closed' },
  { get label() { return tr("已删除"); }, value: 'deleted' },
];

const statusText: Record<TenantStatus, string> = {
  pending: '待开通',
  active: '正常',
  suspended: '已暂停',
  archived: '已归档',
  closed: '已关闭',
  deleted: '已删除',
};

type TenantFormValues = {
  code: string;
  name: string;
  description?: string;
  legalName?: string;
  contactEmail?: string;
  contactPhone?: string;
  ownerEmail: string;
  ownerPassword?: string;
  ownerDisplayName?: string;
};

function statusAction(tenant: Tenant) {
  if (tenant.status === 'active') return { status: 'suspended' as const, label: tr("暂停企业"), danger: true };
  if (tenant.status === 'suspended' || tenant.status === 'pending') {
    return { status: 'active' as const, label: tr("启用企业"), danger: false };
  }
  return null;
}

function tenantMetadata(values: TenantFormValues, version: number): TenantMetadataBody {
  return {
    name: values.name.trim(),
    description: values.description?.trim() || undefined,
    legalName: values.legalName?.trim() || undefined,
    contactEmail: values.contactEmail?.trim() || undefined,
    contactPhone: values.contactPhone?.trim() || undefined,
    version,
  };
}

export default function PlatformTenantsPage() {
  const language = useLanguage();
  const { initialState } = useInitialStateModel();
  const isPlatformOperator = initialState?.currentUser?.platformOperator === true;
  const [rows, setRows] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<TenantStatus | undefined>();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [authorizedTenants, setAuthorizedTenants] = useState<AuthTenant[]>([]);
  const [accessLoading, setAccessLoading] = useState(true);
  const [accessError, setAccessError] = useState(false);
  const [switchingTenantId, setSwitchingTenantId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Tenant | null>(null);
  const [deleteCode, setDeleteCode] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [form] = Form.useForm<TenantFormValues>();
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Tenant | null>(null);
  const createRequestRef = useRef<{ fingerprint: string; key: string } | null>(null);
  const submittingRef = useRef(false);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!isPlatformOperator) return;
    setLoading(true);
    setError(null);
    try {
      const result = await queryPlatformTenants({ keyword: keyword.trim() || undefined, status, page, pageSize });
      setRows(result.items || []);
      setTotal(result.total || 0);
    } catch (requestError) {
      setRows([]);
      setTotal(0);
      setError((requestError as Error)?.message || tr('租户列表加载失败，请重试'));
    } finally {
      setLoading(false);
    }
  }, [isPlatformOperator, keyword, page, pageSize, status]);

  useEffect(() => {
    if (isPlatformOperator) void load();
  }, [isPlatformOperator, load]);

  const loadAuthorized = useCallback(async () => {
    setAccessLoading(true);
    setAccessError(false);
    try {
      const result = await queryAuthTenants();
      setAuthorizedTenants((result.items || []).filter((item) => item.id > 0));
    } catch {
      setAuthorizedTenants([]);
      setAccessError(true);
    } finally {
      setAccessLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isPlatformOperator) void loadAuthorized();
  }, [isPlatformOperator, loadAuthorized]);

  const enterTenant = async (tenant: Tenant) => {
    if (submittingRef.current || switchingTenantId !== null ||
        !canEnterTenantWorkspace(tenant, authorizedTenants)) return;
    if (initialState?.currentUser?.tenantId === tenant.id) {
      history.push('/dashboard/product-operations');
      return;
    }
    setSwitchingTenantId(tenant.id);
    try {
      await switchTenantAndReload(tenant.id, switchAuthTenant, '/dashboard/product-operations');
    } catch (error) {
      setSwitchingTenantId(null);
      message.error((error as Error)?.message || tr('进入企业失败，请重试'));
      // A suspended membership may have changed since the authorization list
      // was fetched. Re-check server-authoritative membership data.
      void loadAuthorized();
    }
  };

  const openCreate = () => {
    createRequestRef.current = null;
    setEditing(null);
    setModalOpen(true);
  };

  const openEdit = (tenant: Tenant) => {
    createRequestRef.current = null;
    setEditing(tenant);
    setModalOpen(true);
  };

  const runSubmit = async (values: TenantFormValues) => {
    if (submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      if (editing) {
        await updatePlatformTenant(editing.id, tenantMetadata(values, editing.version));
        message.success(tr('企业信息已保存'));
      } else {
        const createFields = {
          name: values.name.trim(),
          description: values.description?.trim() || undefined,
          legalName: values.legalName?.trim() || undefined,
          contactEmail: values.contactEmail?.trim() || undefined,
          contactPhone: values.contactPhone?.trim() || undefined,
          code: values.code.trim(),
          ownerEmail: values.ownerEmail.trim(),
          // Passwords must be validated as entered. Trimming them would
          // silently change the credential supplied by the operator.
          ownerPassword: values.ownerPassword || undefined,
          ownerDisplayName: values.ownerDisplayName?.trim() || undefined,
        };
        const fingerprint = JSON.stringify(createFields);
        const previousRequest = createRequestRef.current;
        const idempotencyKey = previousRequest?.fingerprint === fingerprint
          ? previousRequest.key
          : createTenantIdempotencyKey();
        createRequestRef.current = { fingerprint, key: idempotencyKey };
        const body: TenantCreateBody = { ...createFields, idempotencyKey };
        await createPlatformTenant(body);
        createRequestRef.current = null;
        message.success(tr('企业已创建'));
      }
      setModalOpen(false);
      await Promise.all([load(), loadAuthorized()]);
    } catch (requestError) {
      message.error((requestError as Error)?.message || tr('保存失败，请检查后重试'));
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const mutateStatus = (tenant: Tenant, nextStatus: TenantStatus, label: string) => {
    confirmSensitiveAction({
      title: () => trFormat('{action}「{name}」？', { action: tr(label), name: tenant.name }),
      content: () => nextStatus === 'suspended'
        ? tr('暂停后该企业不能继续执行正常业务请求，已创建任务的处理由服务端状态规则决定。')
        : tr('启用后该企业可恢复正常业务访问。'),
      impacts: () => [trFormat('企业：{name}', { name: tenant.name }), trFormat('状态：{from} → {to}', { from: tr(statusText[tenant.status]), to: tr(statusText[nextStatus]) })],
      reversible: true,
      onOk: async () => {
        if (submittingRef.current) return;
        submittingRef.current = true;
        setSubmitting(true);
        try {
          await updatePlatformTenantStatus(tenant.id, { status: nextStatus, version: tenant.version });
          message.success(trFormat('{action}成功', { action: tr(label) }));
          await load();
        } catch (requestError) {
          message.error((requestError as Error)?.message || trFormat('{action}失败，请重试', { action: tr(label) }));
        } finally {
          submittingRef.current = false;
          setSubmitting(false);
        }
      },
    });
  };

  const deleteTenant = async () => {
    const target = deleteTarget;
    if (!target || deleteCode !== target.code || deleting || submittingRef.current) return;
    if (!['suspended', 'archived', 'closed'].includes(target.status)) return;
    submittingRef.current = true;
    setDeleting(true);
    try {
      await deletePlatformTenant(target.id, { code: target.code, version: target.version });
      message.success(tr('企业已删除，历史数据及审计记录已保留'));
      setDeleteTarget(null);
      setDeleteCode('');
      if (status !== 'deleted' && page > 1 && rows.length === 1) setPage(page - 1);
      else await Promise.all([load(), loadAuthorized()]);
    } catch (error) {
      message.error((error as Error)?.message || tr('删除失败，请刷新列表后重试'));
      await load();
    } finally {
      submittingRef.current = false;
      setDeleting(false);
    }
  };

  const columns = useMemo<TableColumnsType<Tenant>>(() => [
    {
      title: tr('企业标识'),
      dataIndex: 'code',
      width: 160,
      render: (value: string, row) => <Link to={`/platform/tenants/${row.id}`}>{value}</Link>,
    },
    { title: tr('企业名称'), dataIndex: 'name', ellipsis: true, width: 220 },
    {
      title: tr('状态'),
      dataIndex: 'status',
      width: 110,
      render: (value: TenantStatus) => <StatusTag status={value} text={statusText[value] ? tr(statusText[value]) : value} />,
    },
    { title: tr('联系人'), dataIndex: 'contactEmail', width: 220, ellipsis: true, render: (value?: string) => value || '—' },
    { title: tr('创建时间'), dataIndex: 'createdAt', width: 180, render: (value: string) => formatDateTime(value) },
    { title: tr('更新时间'), dataIndex: 'updatedAt', width: 180, render: (value: string) => formatDateTime(value) },
    {
      title: tr('操作'),
      key: 'actions',
      fixed: 'right',
      width: 410,
      render: (_value, row) => {
        const action = statusAction(row);
        const authorized = canEnterTenantWorkspace(row, authorizedTenants);
        return (
          <Space wrap size={4}>
            {authorized ? (
              <Button
                type="link"
                size="small"
                icon={<LoginOutlined />}
                disabled={submitting || switchingTenantId !== null}
                loading={switchingTenantId === row.id}
                onClick={() => void enterTenant(row)}
              >{tr("进入企业工作台")}</Button>
            ) : (
              <span title={accessError ? tr('成员授权暂时无法核验') : tr('当前账号不是该企业的有效成员，需要获得企业授权')} style={{ color: 'var(--ant-color-text-tertiary)' }}>
                {accessLoading ? tr('检查企业授权…') : (accessError ? tr('授权校验失败') : tr('需企业成员授权'))}
              </span>
            )}
            <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => history.push(`/platform/tenants/${row.id}`)}>{tr("详情")}</Button>
            <Button type="link" size="small" icon={<EditOutlined />} disabled={submitting} onClick={() => openEdit(row)}>{tr("编辑")}</Button>
            {row.status !== 'deleted' ? (
              <Button
                type="link"
                danger
                size="small"
                icon={<DeleteOutlined />}
                disabled={submitting || deleting || !['suspended', 'archived', 'closed'].includes(row.status)}
                title={['active', 'pending'].includes(row.status) ? tr('请先暂停或归档企业') : tr('逻辑删除，保留历史记录')}
                onClick={() => { setDeleteTarget(row); setDeleteCode(''); }}
              >{tr("删除")}</Button>
            ) : null}
            {action ? (
              <Button
                type="link"
                size="small"
                danger={action.danger}
                icon={action.danger ? <StopOutlined /> : <CheckCircleOutlined />}
                disabled={submitting}
                onClick={() => mutateStatus(row, action.status, action.label)}
              >
                {tr(action.label)}
              </Button>
            ) : null}
          </Space>
        );
      },
    },
  ], [submitting, deleting, load, authorizedTenants, accessLoading, accessError, switchingTenantId, enterTenant, language]);

  if (!isPlatformOperator) {
    return <TmPageContainer title={tr("租户管理")}><Result status="403" title={tr("无权访问")} subTitle={tr("平台租户管理仅对平台运营管理员开放。")} /></TmPageContainer>;
  }

  return (
    <TmPageContainer
      title={tr("租户管理")}
      subTitle={tr("管理企业开通、状态、联系人与平台审计。")}
      extra={[
        <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>{tr("创建企业")}</Button>,
      ]}
    >
      <Alert
        type="info"
        showIcon
        message={tr("平台控制台与企业工作台权限独立")}
        description={tr("平台管理员可管理全部企业资料；只有当前账号具备有效企业成员资格时才能进入业务工作台。创建企业可指定企业管理员，切换企业将核验授权并记录审计。")}
        style={{ marginBottom: 16 }}
      />
      <SectionCard
        title={tr("企业列表")}
        description={tr("平台管理员可查看跨企业的管理元数据；业务数据仍需通过显式企业上下文访问。")}
        headerExtra={<Button type="text" icon={<ReloadOutlined />} aria-label={tr("刷新")} onClick={() => { void load(); void loadAuthorized(); }} loading={loading || accessLoading}>{tr("刷新")}</Button>}
      >
        <Space wrap size={[8, 8]} style={{ marginBottom: 16 }}>
          <Input.Search
            allowClear
            value={keyword}
            placeholder={tr("搜索企业名称、标识")}
            aria-label={tr("搜索企业名称或标识")}
            onChange={(event) => { setKeyword(event.target.value); setPage(1); }}
            onSearch={() => setPage(1)}
            style={{ width: 280 }}
          />
          <Select
            allowClear
            value={status}
            placeholder={tr("按状态筛选")}
            aria-label={tr("按企业状态筛选")}
            options={STATUS_OPTIONS.map(option => ({ ...option, label: tr(option.label) }))}
            onChange={(value: TenantStatus | undefined) => { setStatus(value); setPage(1); }}
            style={{ width: 160 }}
          />
        </Space>
        {error ? <ErrorAlert title={tr("租户列表加载失败")} actionHint={error} action={<Button type="link" onClick={() => void load()}>{tr("重试")}</Button>} style={{ marginBottom: 16 }} /> : null}
        {!loading && !error && rows.length === 0 ? (
          <EmptyState title={tr("暂无企业")} description={tr("创建第一家企业后，企业会显示在这里。")} actionLabel={tr("创建企业")} onAction={openCreate} />
        ) : (
          <Table<Tenant>
            rowKey="id"
            loading={loading}
            columns={columns}
            dataSource={rows}
            scroll={{ x: 1200 }}
            pagination={false}
          />
        )}
        <Pagination
          current={page}
          pageSize={pageSize}
          total={total}
          showSizeChanger
          showTotal={(value) => trFormat('共 {count} 家企业', { count: value })}
          onChange={(nextPage, nextPageSize) => { setPage(nextPage); if (nextPageSize !== pageSize) setPageSize(nextPageSize); }}
          style={{ marginTop: 16, textAlign: 'right' }}
        />
      </SectionCard>

      <Modal
        open={Boolean(deleteTarget)}
        title={deleteTarget ? trFormat('删除企业：{name}', { name: deleteTarget.name }) : tr('删除企业')}
        okText={tr("确认删除")}
        okButtonProps={{ danger: true, disabled: !deleteTarget || deleteCode !== deleteTarget.code || deleting }}
        confirmLoading={deleting}
        onCancel={() => { if (!deleting) { setDeleteTarget(null); setDeleteCode(''); } }}
        onOk={() => { void deleteTenant(); }}
        destroyOnHidden
      >
        <Alert
          type="warning"
          showIcon
          message={tr("删除后将从默认企业列表移除")}
          description={tr("删除会停用企业，撤销其会话及待处理邀请。订单、店铺、文件和审计记录不会物理删除；无法在普通后台恢复，原企业标识仍会保留。")}
          style={{ marginBottom: 16 }}
        />
        <p>{tr('请输入企业标识以确认删除。')} <strong>{deleteTarget?.code}</strong></p>
        <Input
          aria-label={tr("输入企业标识确认删除")}
          value={deleteCode}
          onChange={(e) => setDeleteCode(e.target.value)}
          autoComplete="off"
          disabled={deleting}
        />
      </Modal>
      <Modal
        open={modalOpen}
        title={editing ? trFormat('编辑企业：{name}', { name: editing.name }) : tr('创建企业')}
        width={640}
        destroyOnHidden
        confirmLoading={submitting}
        okText={editing ? tr('保存') : tr('创建企业')}
        cancelText={tr("取消")}
        onCancel={() => {
          if (!submitting) {
            createRequestRef.current = null;
            setModalOpen(false);
          }
        }}
        onOk={() => { void form.submit(); }}
      >
        <Form
          key={editing?.id ?? 'create'}
          form={form}
          layout="vertical"
          onFinish={runSubmit}
          preserve={false}
          initialValues={editing ? {
            code: editing.code,
            name: editing.name,
            description: editing.description,
            legalName: editing.legalName,
            contactEmail: editing.contactEmail,
            contactPhone: editing.contactPhone,
            ownerEmail: editing.contactEmail || '',
          } : undefined}
        >
          {!editing ? (
            <Form.Item name="code" label={tr("企业标识")} rules={[
              { required: true, message: tr('请输入企业标识') },
              { pattern: /^[a-z][a-z0-9-]{2,47}$/, message: tr('使用 3-48 位小写字母、数字和连字符，并以小写字母开头') },
            ]}>
              <Input placeholder={tr("例如 acme-cn")} maxLength={64} />
            </Form.Item>
          ) : null}
          <Form.Item name="name" label={tr("企业名称")} rules={[
            { required: true, whitespace: true, message: tr('企业名称不能为空或仅包含空格') },
            { max: 120, message: tr('企业名称不能超过 120 个字符') },
          ]}>
            <Input placeholder={tr("请输入企业名称")} maxLength={120} />
          </Form.Item>
          <Form.Item name="legalName" label={tr("法定主体名称")}><Input maxLength={160} /></Form.Item>
          <Form.Item name="description" label={tr("企业说明")}><Input.TextArea rows={3} maxLength={500} showCount /></Form.Item>
          <Space wrap style={{ width: '100%' }}>
            <Form.Item name="contactEmail" label={tr("联系邮箱")} rules={[{ type: 'email', message: tr('请输入有效邮箱') }]} style={{ minWidth: 260 }}><Input /></Form.Item>
            <Form.Item name="contactPhone" label={tr("联系电话")} style={{ minWidth: 220 }}><Input /></Form.Item>
          </Space>
          {!editing ? (
            <>
              <Form.Item name="ownerEmail" label={tr("企业管理员邮箱")} rules={[{ required: true, message: tr('请输入企业管理员邮箱') }, { type: 'email', message: tr('请输入有效邮箱') }]}><Input /></Form.Item>
              <Form.Item name="ownerDisplayName" label={tr("管理员显示名")}><Input maxLength={80} /></Form.Item>
              <Form.Item
                name="ownerPassword"
                label={tr("初始密码")}
                extra={trFormat('首次创建管理员账号必须填写：{policy} 如果邮箱已注册，留空将保留原密码。', { policy: tr(PASSWORD_HINT) })}
                rules={[{
                  validator: async (_, value: string | undefined) => {
                    if (!value) return;
                    const error = passwordError(value);
                    if (error) throw new Error(tr(error));
                  },
                }]}
              >
                <Input.Password autoComplete="new-password" />
              </Form.Item>
            </>
          ) : null}
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
