import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
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
  if (tenant.status === 'active') return { value: 'suspended' as const, label: tr("暂停企业") };
  if (tenant.status === 'suspended' || tenant.status === 'pending') return { value: 'active' as const, label: tr("启用企业") };
  return null;
}

export default function PlatformTenantDetailPage() {
  const language = useLanguage();
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
        setError((requestError as Error)?.message || tr('租户详情加载失败，请重试'));
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
        setAuditError((requestError as Error)?.message || tr('租户审计加载失败，请重试'));
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
      message.success(tr('企业资料已保存'));
      setEditOpen(false);
      await loadDetail();
    } catch (requestError) {
      if (isCurrentRoute(editContext.tenantId, editContext.routeVersion)) {
        message.error((requestError as Error)?.message || tr('保存失败，请重试'));
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
      title: () => trFormat('{action}「{name}」？', { action: tr(next.label), name: tenant.name }),
      content: () => next.value === 'suspended' ? tr('暂停后企业正常业务访问将被服务端拒绝。') : tr('启用后企业可以恢复正常业务访问。'),
      impacts: () => [trFormat('企业状态：{from} → {to}', { from: STATUS_LABELS[tenant.status] ? tr(STATUS_LABELS[tenant.status]) : tenant.status, to: tr(STATUS_LABELS[next.value]) })],
      reversible: true,
      onOk: async () => {
        if (submittingRef.current || !isCurrentRoute(statusContext.tenantId, statusContext.routeVersion)) return;
        submittingRef.current = true;
        setSubmitting(true);
        try {
          await updatePlatformTenantStatus(statusContext.tenantId, { status: next.value, version: statusContext.version });
          message.success(trFormat('{action}成功', { action: tr(next.label) }));
          await Promise.all([loadDetail(), loadAudit()]);
        } catch (requestError) {
          if (isCurrentRoute(statusContext.tenantId, statusContext.routeVersion)) {
            message.error((requestError as Error)?.message || tr('操作失败，请重试'));
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
    { title: tr('操作'), dataIndex: 'action', width: 180, ellipsis: true },
    { title: tr('结果'), dataIndex: 'status', width: 100, render: (value?: string) => <StatusTag status={value} /> },
    { title: tr('说明'), dataIndex: 'message', ellipsis: true, render: (value?: string) => value || '—' },
    { title: tr('时间'), dataIndex: 'createdAt', width: 180, render: (value: string) => formatDateTime(value) },
  ], [language]);

  if (!isPlatformOperator) {
    return <TmPageContainer title={tr("租户详情")}><Result status="403" title={tr("无权访问")} subTitle={tr("平台租户管理仅对平台运营管理员开放。")} /></TmPageContainer>;
  }
  if (!tenantId) {
    return <TmPageContainer title={tr("租户详情")}><Result status="404" title={tr("租户标识无效")} extra={<Button onClick={() => history.push('/platform/tenants')}>{tr("返回租户管理")}</Button>} /></TmPageContainer>;
  }

  const currentDetail = detail?.tenant.id === tenantId ? detail : null;
  const tenant = currentDetail?.tenant;
  const statusAction = tenant ? nextStatus(tenant) : null;
  return (
    <TmPageContainer
      title={tenant ? <span>{tenant.name}</span> : tr('租户详情')}
      subTitle={tenant ? trFormat('{code} · 企业状态：{status}', { code: tenant.code, status: STATUS_LABELS[tenant.status] ? tr(STATUS_LABELS[tenant.status]) : tenant.status }) : tr('查看企业资料和平台操作审计。')}
      extra={[
        <Button key="back" icon={<ArrowLeftOutlined />} onClick={() => history.push('/platform/tenants')}>{tr("返回列表")}</Button>,
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => { void loadDetail(); void loadAudit(); }} loading={loading || auditLoading}>{tr("刷新")}</Button>,
        ...(tenant ? [<Button key="edit" icon={<EditOutlined />} disabled={submitting} onClick={openEdit}>{tr("编辑企业")}</Button>] : []),
        ...(tenant && statusAction ? [<Button key="status" type="primary" danger={statusAction.value === 'suspended'} disabled={submitting} onClick={() => changeStatus(tenant)}>{tr(statusAction.label)}</Button>] : []),
      ]}
    >
      {error ? <ErrorAlert title={tr("租户详情加载失败")} actionHint={error} action={<Button type="link" onClick={() => void loadDetail()}>{tr("重试")}</Button>} /> : null}
      {loading && !detail ? <SectionCard><Spin tip={tr("正在加载租户详情")}><div style={{ minHeight: 96 }} /></Spin></SectionCard> : null}
      {tenant ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <SectionCard title={tr("企业资料")} description={tenant.description ? <span>{tenant.description}</span> : tr('未填写企业说明')}>
            <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small">
              <Descriptions.Item label={tr("企业标识")}>{tenant.code}</Descriptions.Item>
              <Descriptions.Item label={tr("状态")}><StatusTag status={tenant.status} text={STATUS_LABELS[tenant.status] ? tr(STATUS_LABELS[tenant.status]) : tenant.status} /></Descriptions.Item>
              <Descriptions.Item label={tr("企业 ID")}>{tenant.id}</Descriptions.Item>
              <Descriptions.Item label={tr("法定主体")}>{tenant.legalName || '—'}</Descriptions.Item>
              <Descriptions.Item label={tr("联系邮箱")}>{tenant.contactEmail || '—'}</Descriptions.Item>
              <Descriptions.Item label={tr("联系电话")}>{tenant.contactPhone || '—'}</Descriptions.Item>
              <Descriptions.Item label={tr("管理员用户 ID")}>{tenant.ownerUserId || '—'}</Descriptions.Item>
              <Descriptions.Item label={tr("成员数")}>{currentDetail?.memberCount ?? 0}</Descriptions.Item>
              <Descriptions.Item label={tr("店铺数")}>{currentDetail?.shopCount ?? 0}</Descriptions.Item>
              <Descriptions.Item label={tr("创建时间")}>{formatDateTime(tenant.createdAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("更新时间")}>{formatDateTime(tenant.updatedAt)}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
          <SectionCard title={tr("平台操作审计")} description={tr("仅展示租户生命周期相关操作记录。")}>
            <Tabs activeKey="audit" items={[{
              key: 'audit',
              label: tr('操作记录'),
              children: <>
                {auditError ? <ErrorAlert title={tr("审计记录加载失败")} actionHint={auditError} action={<Button type="link" onClick={() => void loadAudit()}>{tr("重试")}</Button>} style={{ marginBottom: 16 }} /> : null}
                {!auditLoading && !auditError && audit.length === 0 ? <div style={{ padding: 24, textAlign: 'center', color: 'var(--ant-color-text-tertiary)' }}>{tr("暂无审计记录")}</div> : <Table<OperationLog> rowKey="id" loading={auditLoading} columns={auditColumns} dataSource={audit} pagination={{ pageSize: 10 }} scroll={{ x: 720 }} />}
              </>,
            }]} />
          </SectionCard>
        </Space>
      ) : null}

      <Modal open={editOpen && Boolean(currentDetail)} title={trFormat('编辑企业：{name}', { name: tenant?.name || '' })} forceRender width={640} confirmLoading={submitting} okText={tr("保存")} cancelText={tr("取消")} onCancel={() => { if (!submitting) setEditOpen(false); }} onOk={() => { void form.submit(); }}>
        <Form form={form} layout="vertical" onFinish={save} preserve={false}>
          <Form.Item name="name" label={tr("企业名称")} rules={[{ required: true, message: tr('请输入企业名称') }, { max: 120, message: tr('企业名称不能超过 120 个字符') }]}><Input maxLength={120} /></Form.Item>
          <Form.Item name="legalName" label={tr("法定主体名称")}><Input maxLength={160} /></Form.Item>
          <Form.Item name="description" label={tr("企业说明")}><Input.TextArea rows={3} maxLength={500} showCount /></Form.Item>
          <Form.Item name="contactEmail" label={tr("联系邮箱")} rules={[{ type: 'email', message: tr('请输入有效邮箱') }]}><Input /></Form.Item>
          <Form.Item name="contactPhone" label={tr("联系电话")}><Input maxLength={40} /></Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
