import { SaveOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Descriptions, Form, Input, Space, Spin, Tag, message } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ErrorAlert, SectionCard, StatusTag, TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { getCurrentTenant, updateCurrentTenant, type TenantDetail, type TenantMetadataBody } from '@/services/tenants';
import { PERMISSIONS } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';

type TenantSettingsForm = {
  name: string;
  legalName?: string;
  description?: string;
  contactEmail?: string;
  contactPhone?: string;
  settings: string;
};

const TENANT_STATUS_LABELS: Record<string, string> = {
  pending: '待开通',
  active: '正常',
  suspended: '已暂停',
  archived: '已归档',
  closed: '已关闭',
};

export default function TenantSettingsPage() {
  const { can } = usePermission();
  const canEdit = can(PERMISSIONS.SETTINGS_MANAGE);
  const [detail, setDetail] = useState<TenantDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const [form] = Form.useForm<TenantSettingsForm>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await getCurrentTenant();
      setDetail(result);
      form.setFieldsValue({
        name: result.tenant.name,
        legalName: result.tenant.legalName || '',
        description: result.tenant.description || '',
        contactEmail: result.tenant.contactEmail || '',
        contactPhone: result.tenant.contactPhone || '',
        settings: result.tenant.settings || '{}',
      });
    } catch (requestError) {
      setError((requestError as Error)?.message || '企业资料加载失败，请重试');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = async (values: TenantSettingsForm) => {
    if (!canEdit || !detail || submittingRef.current) return;
    let parsedSettings: unknown;
    try {
      parsedSettings = JSON.parse(values.settings || '{}') as unknown;
    } catch {
      form.setFields([{ name: 'settings', errors: ['请输入有效的 JSON 对象'] }]);
      return;
    }
    if (!parsedSettings || Array.isArray(parsedSettings) || typeof parsedSettings !== 'object') {
      form.setFields([{ name: 'settings', errors: ['设置必须是 JSON 对象'] }]);
      return;
    }

    submittingRef.current = true;
    setSubmitting(true);
    const body: TenantMetadataBody = {
      name: values.name.trim(),
      legalName: values.legalName?.trim() || undefined,
      description: values.description?.trim() || undefined,
      contactEmail: values.contactEmail?.trim() || undefined,
      contactPhone: values.contactPhone?.trim() || undefined,
      settings: JSON.stringify(parsedSettings),
      version: detail.tenant.version,
    };
    try {
      const tenant = await updateCurrentTenant(body);
      setDetail((current) => current ? { ...current, tenant } : current);
      message.success('企业资料已保存');
    } catch (requestError) {
      message.error((requestError as Error)?.message || '保存失败，请重试');
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  return (
    <TmPageContainer
      title="企业信息与设置"
      subTitle="查看当前企业资料、状态与成员和店铺统计。"
      extra={[<Button key="refresh" icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>]}
    >
      {error ? <ErrorAlert title="企业资料加载失败" actionHint={error} action={<Button type="link" onClick={() => void load()}>重试</Button>} /> : null}
      {loading && !detail ? (
        <SectionCard><Spin tip="正在加载企业资料"><div style={{ minHeight: 96 }} /></Spin></SectionCard>
      ) : detail ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <SectionCard title={detail.tenant.name} description={detail.tenant.description || '未填写企业说明'}>
            <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small">
              <Descriptions.Item label="企业标识">{detail.tenant.code}</Descriptions.Item>
              <Descriptions.Item label="企业状态"><StatusTag status={detail.tenant.status} text={TENANT_STATUS_LABELS[detail.tenant.status] || detail.tenant.status} /></Descriptions.Item>
              <Descriptions.Item label="企业 ID">{detail.tenant.id}</Descriptions.Item>
              <Descriptions.Item label="成员数">{detail.memberCount}</Descriptions.Item>
              <Descriptions.Item label="店铺数">{detail.shopCount}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(detail.tenant.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="法定主体">{detail.tenant.legalName || '—'}</Descriptions.Item>
              <Descriptions.Item label="联系邮箱">{detail.tenant.contactEmail || '—'}</Descriptions.Item>
              <Descriptions.Item label="联系电话">{detail.tenant.contactPhone || '—'}</Descriptions.Item>
            </Descriptions>
          </SectionCard>

          <SectionCard
            title="企业资料与设置"
            description={canEdit ? '保存时将校验版本号；并发修改需要刷新后重试。' : '当前账号可以查看企业资料，但没有修改权限。'}
            headerExtra={canEdit ? <Tag color="blue">可编辑</Tag> : <Tag>只读</Tag>}
          >
            <Form form={form} layout="vertical" onFinish={save} disabled={!canEdit || submitting}>
              <Form.Item name="name" label="企业名称" rules={[{ required: true, message: '请输入企业名称' }, { max: 120, message: '企业名称不能超过 120 个字符' }]}>
                <Input maxLength={120} />
              </Form.Item>
              <Form.Item name="legalName" label="法定主体名称"><Input maxLength={160} /></Form.Item>
              <Form.Item name="description" label="企业说明"><Input.TextArea rows={3} maxLength={500} showCount /></Form.Item>
              <Space wrap style={{ width: '100%' }}>
                <Form.Item name="contactEmail" label="联系邮箱" rules={[{ type: 'email', message: '请输入有效邮箱' }]} style={{ minWidth: 260 }}><Input /></Form.Item>
                <Form.Item name="contactPhone" label="联系电话" style={{ minWidth: 220 }}><Input maxLength={40} /></Form.Item>
              </Space>
              <Form.Item name="settings" label="企业设置（JSON）" extra="仅保存企业级选项；请勿在此存放密码、Token 或平台密钥。" rules={[{ required: true, message: '设置必须是 JSON 对象' }]}>
                <Input.TextArea rows={8} spellCheck={false} autoSize={{ minRows: 5, maxRows: 16 }} />
              </Form.Item>
              {canEdit ? <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={submitting} disabled={submitting}>保存企业资料</Button> : null}
            </Form>
          </SectionCard>
        </Space>
      ) : null}
    </TmPageContainer>
  );
}
