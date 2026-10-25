import { ArrowLeftOutlined, EditOutlined, ReloadOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import { Button, Descriptions, Form, Input, Modal, Space, Table, Tabs, Tag, message, Result, Spin } from 'antd';
import type { TableColumnsType } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ErrorAlert, SectionCard, StatusTag, TmPageContainer } from '@/components/ui';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import {
  getPlatformTenant,
  getPlatformTenantAudit,
  updatePlatformTenant,
  updatePlatformTenantStatus,
  type OperationLog,
  type Tenant,
  type TenantDetail,
  type TenantMetadataBody,
  type TenantStatus,
} from '@/services/tenants';
import { formatDateTime } from '@/utils/formatTime';
import { confirmSensitiveAction } from '@/utils/sensitiveConfirm';

const STATUS_LABELS: Record<string, string> = {
  pending: '待开通',
  active: '正常',
  suspended: '已暂停',
  archived: '已归档',
  closed: '已关闭',
};

type EditValues = {
  name: string;
  description?: string;
  legalName?: string;
  contactEmail?: string;
  contactPhone?: string;
};

function nextStatus(tenant: Tenant) {
  if (tenant.status === 'active') return { value: 'suspended' as const, label: '暂停企业' };
  if (tenant.status === 'suspended' || tenant.status === 'pending') return { value: 'active' as const, label: '启用企业' };
  return null;
}

export default function PlatformTenantDetailPage() {
  const { initialState } = useInitialStateModel();
  const params = useParams<{ id?: string }>();
  const tenantId = params.id && /^\d+$/.test(params.id) ? Number(params.id) : undefined;
  const isPlatformOperator = initialState?.currentUser?.platformOperator === true;
  const [detail, setDetail] = useState<TenantDetail | null>(null);
  const [audit, setAudit] = useState<OperationLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditError, setAuditError] = useState<string | null>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const [form] = Form.useForm<EditValues>();
  const routeVersionRef = useRef(0);
  const editContextRef = useRef<{ tenantId: number; version: number; routeVersion: number } | null>(null);
  const isCurrentRoute = (id: number, routeVersion: number) => tenantId === id && routeVersionRef.current === routeVersion;

  const loadDetail = useCallback(async () => {
    if (!tenantId) return;
    const requestTenantId = tenantId;
    const routeVersion = routeVersionRef.current;
    setLoading(true);
    setError(null);
    try {
      const result = await getPlatformTenant(requestTenantId);
      if (isCurrentRoute(requestTenantId, routeVersion)) setDetail(result);
    } catch (requestError) {
      if (isCurrentRoute(requestTenantId, routeVersion)) {
        setError((requestError as Error)?.message || '租户详情加载失败，请重试');
        setDetail(null);
      }
    } finally {
      if (isCurrentRoute(requestTenantId, routeVersion)) setLoading(false);
    }
  }, [tenantId]);

  const loadAudit = useCallback(async () => {
    if (!tenantId) return;
    const requestTenantId = tenantId;
    const routeVersion = routeVersionRef.current;
    setAuditLoading(true);
    setAuditError(null);
    try {
      const result = await getPlatformTenantAudit(requestTenantId);
      if (isCurrentRoute(requestTenantId, routeVersion)) setAudit(result.items || []);
    } catch (requestError) {
      if (isCurrentRoute(requestTenantId, routeVersion)) {
        setAudit([]);
        setAuditError((requestError as Error)?.message || '租户审计加载失败，请重试');
      }
    } finally {
      if (isCurrentRoute(requestTenantId, routeVersion)) setAuditLoading(false);
    }
  }, [tenantId]);

  useEffect(() => {
    routeVersionRef.current += 1;
    setDetail(null);
    setAudit([]);
    setError(null);
    setAuditError(null);
    setEditOpen(false);
    setSubmitting(false);
    submittingRef.current = false;
    editContextRef.current = null;
  }, [form, tenantId]);

  useEffect(() => {
    if (isPlatformOperator && tenantId) {
      void loadDetail();
      void loadAudit();
    }
  }, [isPlatformOperator, loadAudit, loadDetail, tenantId]);

  const openEdit = () => {
    const currentDetail = detail?.tenant.id === tenantId ? detail : null;
    const tenant = currentDetail?.tenant;
    if (!tenant) return;
    form.setFieldsValue({
      name: tenant.name,
      description: tenant.description || '',
      legalName: tenant.legalName || '',
      contactEmail: tenant.contactEmail || '',
      contactPhone: tenant.contactPhone || '',
    });
    editContextRef.current = { tenantId: tenant.id, version: tenant.version, routeVersion: routeVersionRef.current };
    setEditOpen(true);
  };

  const save = async (values: EditValues) => {
    const editContext = editContextRef.current;
    if (!detail || !editContext || !isCurrentRoute(editContext.tenantId, editContext.routeVersion) || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    const body: TenantMetadataBody = {
      name: values.name.trim(),
      description: values.description?.trim() || undefined,
      legalName: values.legalName?.trim() || undefined,
      contactEmail: values.contactEmail?.trim() || undefined,
      contactPhone: values.contactPhone?.trim() || undefined,
      version: editContext.version,
    };
    try {
      await updatePlatformTenant(editContext.tenantId, body);
      if (!isCurrentRoute(editContext.tenantId, editContext.routeVersion)) return;
      message.success('企业资料已保存');
      setEditOpen(false);
      await loadDetail();
    } catch (requestError) {
      if (isCurrentRoute(editContext.tenantId, editContext.routeVersion)) {
        message.error((requestError as Error)?.message || '保存失败，请重试');
      }
    } finally {
      if (isCurrentRoute(editContext.tenantId, editContext.routeVersion)) {
        submittingRef.current = false;
        setSubmitting(false);
      }
    }
  };

  const changeStatus = (tenant: Tenant) => {
    const statusContext = { tenantId: tenant.id, version: tenant.version, routeVersion: routeVersionRef.current };
    const next = nextStatus(tenant);
    if (!next) return;
    confirmSensitiveAction({
      title: `${next.label}「${tenant.name}」？`,
      content: next.value === 'suspended' ? '暂停后企业正常业务访问将被服务端拒绝。' : '启用后企业可以恢复正常业务访问。',
      impacts: [`企业状态：${STATUS_LABELS[tenant.status] || tenant.status} → ${STATUS_LABELS[next.value]}`],
      reversible: true,
      onOk: async () => {
        if (submittingRef.current || !isCurrentRoute(statusContext.tenantId, statusContext.routeVersion)) return;
        submittingRef.current = true;
        setSubmitting(true);
        try {
          await updatePlatformTenantStatus(statusContext.tenantId, { status: next.value, version: statusContext.version });
          message.success(`${next.label}成功`);
          await Promise.all([loadDetail(), loadAudit()]);
        } catch (requestError) {
          if (isCurrentRoute(statusContext.tenantId, statusContext.routeVersion)) {
            message.error((requestError as Error)?.message || '操作失败，请重试');
          }
        } finally {
          if (isCurrentRoute(statusContext.tenantId, statusContext.routeVersion)) {
            submittingRef.current = false;
            setSubmitting(false);
          }
        }
      },
    });
  };

  const auditColumns = useMemo<TableColumnsType<OperationLog>>(() => [
    { title: '操作', dataIndex: 'action', width: 180, ellipsis: true },
    { title: '结果', dataIndex: 'status', width: 100, render: (value?: string) => <StatusTag status={value} /> },
    { title: '说明', dataIndex: 'message', ellipsis: true, render: (value?: string) => value || '—' },
    { title: '时间', dataIndex: 'createdAt', width: 180, render: (value: string) => formatDateTime(value) },
  ], []);

  if (!isPlatformOperator) {
    return <TmPageContainer title="租户详情"><Result status="403" title="无权访问" subTitle="平台租户管理仅对平台运营管理员开放。" /></TmPageContainer>;
  }
  if (!tenantId) {
    return <TmPageContainer title="租户详情"><Result status="404" title="租户标识无效" extra={<Button onClick={() => history.push('/platform/tenants')}>返回租户管理</Button>} /></TmPageContainer>;
  }

  const currentDetail = detail?.tenant.id === tenantId ? detail : null;
  const tenant = currentDetail?.tenant;
  const statusAction = tenant ? nextStatus(tenant) : null;
  return (
    <TmPageContainer
      title={tenant?.name || '租户详情'}
      subTitle={tenant ? `${tenant.code} · 企业状态：${STATUS_LABELS[tenant.status] || tenant.status}` : '查看企业资料和平台操作审计。'}
      extra={[
        <Button key="back" icon={<ArrowLeftOutlined />} onClick={() => history.push('/platform/tenants')}>返回列表</Button>,
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => { void loadDetail(); void loadAudit(); }} loading={loading || auditLoading}>刷新</Button>,
        ...(tenant ? [<Button key="edit" icon={<EditOutlined />} disabled={submitting} onClick={openEdit}>编辑企业</Button>] : []),
        ...(tenant && statusAction ? [<Button key="status" type="primary" danger={statusAction.value === 'suspended'} disabled={submitting} onClick={() => changeStatus(tenant)}>{statusAction.label}</Button>] : []),
      ]}
    >
      {error ? <ErrorAlert title="租户详情加载失败" actionHint={error} action={<Button type="link" onClick={() => void loadDetail()}>重试</Button>} /> : null}
      {loading && !detail ? <SectionCard><Spin tip="正在加载租户详情"><div style={{ minHeight: 96 }} /></Spin></SectionCard> : null}
      {tenant ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <SectionCard title="企业资料" description={tenant.description || '未填写企业说明'}>
            <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small">
              <Descriptions.Item label="企业标识">{tenant.code}</Descriptions.Item>
              <Descriptions.Item label="状态"><StatusTag status={tenant.status} text={STATUS_LABELS[tenant.status] || tenant.status} /></Descriptions.Item>
              <Descriptions.Item label="企业 ID">{tenant.id}</Descriptions.Item>
              <Descriptions.Item label="法定主体">{tenant.legalName || '—'}</Descriptions.Item>
              <Descriptions.Item label="联系邮箱">{tenant.contactEmail || '—'}</Descriptions.Item>
              <Descriptions.Item label="联系电话">{tenant.contactPhone || '—'}</Descriptions.Item>
              <Descriptions.Item label="管理员用户 ID">{tenant.ownerUserId || '—'}</Descriptions.Item>
              <Descriptions.Item label="成员数">{currentDetail?.memberCount ?? 0}</Descriptions.Item>
              <Descriptions.Item label="店铺数">{currentDetail?.shopCount ?? 0}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(tenant.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="更新时间">{formatDateTime(tenant.updatedAt)}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
          <SectionCard title="平台操作审计" description="仅展示租户生命周期相关操作记录。">
            <Tabs activeKey="audit" items={[{
              key: 'audit',
              label: '操作记录',
              children: <>
                {auditError ? <ErrorAlert title="审计记录加载失败" actionHint={auditError} action={<Button type="link" onClick={() => void loadAudit()}>重试</Button>} style={{ marginBottom: 16 }} /> : null}
                {!auditLoading && !auditError && audit.length === 0 ? <div style={{ padding: 24, textAlign: 'center', color: 'var(--ant-color-text-tertiary)' }}>暂无审计记录</div> : <Table<OperationLog> rowKey="id" loading={auditLoading} columns={auditColumns} dataSource={audit} pagination={{ pageSize: 10 }} scroll={{ x: 720 }} />}
              </>,
            }]} />
          </SectionCard>
        </Space>
      ) : null}

      <Modal open={editOpen && Boolean(currentDetail)} title={`编辑企业：${tenant?.name || ''}`} forceRender width={640} confirmLoading={submitting} okText="保存" cancelText="取消" onCancel={() => { if (!submitting) setEditOpen(false); }} onOk={() => { void form.submit(); }}>
        <Form form={form} layout="vertical" onFinish={save} preserve={false}>
          <Form.Item name="name" label="企业名称" rules={[{ required: true, message: '请输入企业名称' }, { max: 120, message: '企业名称不能超过 120 个字符' }]}><Input maxLength={120} /></Form.Item>
          <Form.Item name="legalName" label="法定主体名称"><Input maxLength={160} /></Form.Item>
          <Form.Item name="description" label="企业说明"><Input.TextArea rows={3} maxLength={500} showCount /></Form.Item>
          <Form.Item name="contactEmail" label="联系邮箱" rules={[{ type: 'email', message: '请输入有效邮箱' }]}><Input /></Form.Item>
          <Form.Item name="contactPhone" label="联系电话"><Input maxLength={40} /></Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
