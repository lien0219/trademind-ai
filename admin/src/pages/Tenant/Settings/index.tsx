import { CodeOutlined, ReloadOutlined, SaveOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import { Alert, Button, Descriptions, Form, Input, Modal, Popconfirm, Space, Spin, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ErrorAlert, SectionCard, StatusTag, TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { getCurrentTenant, updateCurrentTenant, type TenantDetail, type TenantMetadataBody } from '@/services/tenants';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { TENANT_SETTINGS_EXAMPLE, formatTenantSettings, inspectTenantSettings, summarizeTenantSettings } from '@/utils/tenantSettings';

const { Text } = Typography;

type TenantProfileForm = {
  name: string;
  legalName?: string;
  description?: string;
  contactEmail?: string;
  contactPhone?: string;
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
  const [form] = Form.useForm<TenantProfileForm>();
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [advancedText, setAdvancedText] = useState('{}');
  const [advancedError, setAdvancedError] = useState<string | null>(null);
  const [showExample, setShowExample] = useState(false);

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
      });
      setAdvancedOpen(false);
      setAdvancedError(null);
      setShowExample(false);
    } catch (requestError) {
      setError((requestError as Error)?.message || '企业资料加载失败，请重试');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  // Profile saves deliberately omit settings. A stale, unopened JSON editor
  // must never overwrite independent changes to the tenant's advanced options.
  const saveProfile = async (values: TenantProfileForm) => {
    if (!canEdit || !detail || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    const body: TenantMetadataBody = {
      name: values.name.trim(),
      legalName: values.legalName?.trim() || undefined,
      description: values.description?.trim() || undefined,
      contactEmail: values.contactEmail?.trim() || undefined,
      contactPhone: values.contactPhone?.trim() || undefined,
      version: detail.tenant.version,
    };
    try {
      const tenant = await updateCurrentTenant(body);
      setDetail((current) => current ? { ...current, tenant } : current);
      message.success('企业资料已保存');
    } catch (requestError) {
      message.error((requestError as Error)?.message || '保存失败，请刷新后重试');
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const openAdvanced = () => {
    if (!canEdit || !detail || submittingRef.current) return;
    setAdvancedText(formatTenantSettings(detail.tenant.settings || '{}'));
    setAdvancedError(null);
    setShowExample(false);
    setAdvancedOpen(true);
  };

  const updateAdvancedText = (value: string) => {
    setAdvancedText(value);
    setAdvancedError(null);
  };

  const formatAdvanced = () => {
    const result = inspectTenantSettings(advancedText);
    if (!result.ok) {
      setAdvancedError(result.error);
      return;
    }
    setAdvancedText(JSON.stringify(result.value, null, 2));
    setAdvancedError(null);
  };

  const saveAdvanced = async () => {
    if (!canEdit || !detail || submittingRef.current) return;
    const result = inspectTenantSettings(advancedText);
    if (!result.ok) {
      setAdvancedError(result.error);
      return;
    }
    submittingRef.current = true;
    setSubmitting(true);
    const tenant = detail.tenant;
    // The backend update contract writes profile fields on every call.
    // Use the last loaded values, not unsaved edits in the separate form.
    const body: TenantMetadataBody = {
      name: tenant.name,
      legalName: tenant.legalName,
      description: tenant.description,
      contactEmail: tenant.contactEmail,
      contactPhone: tenant.contactPhone,
      settings: JSON.stringify(result.value),
      version: tenant.version,
    };
    try {
      const updated = await updateCurrentTenant(body);
      setDetail((current) => current ? { ...current, tenant: updated } : current);
      setAdvancedOpen(false);
      setAdvancedError(null);
      message.success('企业高级配置已保存');
    } catch (requestError) {
      message.error((requestError as Error)?.message || '保存失败；请刷新以避免覆盖其他管理员的更改');
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const rootWorkspace = detail?.tenant.code === 'root-workspace';
  const configSummary = summarizeTenantSettings(detail?.tenant.settings || '{}');

  return (
    <TmPageContainer
      title={rootWorkspace ? 'Root 工作台信息' : '企业信息与设置'}
      subTitle={rootWorkspace ? '系统自动维护的业务工作空间，不等同于平台全局配置。' : '查看当前企业的资料、成员与店铺统计，并管理企业级扩展选项。'}
      extra={[<Button key="refresh" icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>]}
    >
      {error ? <ErrorAlert title="企业资料加载失败" actionHint={error} action={<Button type="link" onClick={() => void load()}>重试</Button>} /> : null}
      {loading && !detail ? (
        <SectionCard><Spin tip="正在加载企业资料"><div style={{ minHeight: 96 }} /></Spin></SectionCard>
      ) : detail ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {rootWorkspace ? (
            <Alert
              type="info"
              showIcon
              message="这是 Root 专属业务工作空间"
              description="企业级资料和扩展选项只作用于这个工作空间；全平台运行参数请到「设置 → 系统设置」中管理。此处 JSON 不控制 Root 权限。"
            />
          ) : null}
          <SectionCard title={detail.tenant.name} description={detail.tenant.description || (rootWorkspace ? '系统内部业务空间' : '未填写企业说明')}>
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
            title={rootWorkspace ? '工作台资料' : '企业基本资料'}
            description={canEdit ? '常用资料直接填写；保存不会修改高级 JSON 配置。并发修改需刷新后重试。' : '当前账号仅可查看资料，不能修改。'}
            headerExtra={canEdit ? <Tag color="blue">可编辑</Tag> : <Tag>只读</Tag>}
          >
            <Form form={form} layout="vertical" onFinish={saveProfile} disabled={!canEdit || submitting}>
              <Form.Item name="name" label={rootWorkspace ? '工作台名称' : '企业名称'} rules={[{ required: true, message: '请输入名称' }, { max: 120, message: '名称不能超过 120 个字符' }]}>
                <Input maxLength={120} />
              </Form.Item>
              <Form.Item name="legalName" label="法定主体名称"><Input maxLength={160} placeholder="选填" /></Form.Item>
              <Form.Item name="description" label="企业说明"><Input.TextArea rows={3} maxLength={500} showCount placeholder="描述该企业的用途或经营范围（选填）" /></Form.Item>
              <Space wrap style={{ width: '100%' }}>
                <Form.Item name="contactEmail" label="联系邮箱" rules={[{ type: 'email', message: '请输入有效邮箱' }]} style={{ minWidth: 260 }}><Input placeholder="选填" /></Form.Item>
                <Form.Item name="contactPhone" label="联系电话" style={{ minWidth: 220 }}><Input maxLength={40} placeholder="选填" /></Form.Item>
              </Space>
              {canEdit ? (
                <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={submitting} disabled={submitting}>保存基本资料</Button>
              ) : null}
            </Form>
          </SectionCard>

          <SectionCard
            title="高级配置"
            description="企业级扩展数据；普通使用无需配置，默认保持 {} 即可。"
            headerExtra={canEdit ? <Button icon={<CodeOutlined />} onClick={openAdvanced} disabled={submitting}>编辑高级配置</Button> : <Tag>仅管理员可编辑</Tag>}
          >
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Space wrap>
                <Tag color={configSummary.valid ? 'default' : 'warning'}>{configSummary.label}</Tag>
                <Text type="secondary">当前设置项仅用于存储企业扩展数据，不会自动启用功能、改变权限或覆盖 .env。</Text>
              </Space>
              <Alert
                type="info"
                showIcon
                message="无需手工编写 JSON"
                description="系统尚未为这些扩展字段提供统一的业务开关。请通过各功能页面配置已经支持的选项；仅在明确知道字段用途时使用高级编辑。"
              />
            </Space>
          </SectionCard>

          <Modal
            title="编辑企业高级配置"
            open={advancedOpen && canEdit}
            onCancel={() => { if (!submitting) { setAdvancedOpen(false); setAdvancedError(null); } }}
            onOk={() => { void saveAdvanced(); }}
            okText="保存高级配置"
            okButtonProps={{ disabled: submitting || !canEdit }}
            confirmLoading={submitting}
            maskClosable={false}
            width={720}
            destroyOnHidden
          >
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Alert
                type="warning"
                showIcon
                message="仅供高级管理员使用"
                description="这里只允许普通企业扩展选项，不允许密码、API Key、Token、Cookie 或私钥。Root 权限和平台配置不由此控制。保存会增加企业版本号。"
              />
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
                <label htmlFor="tenant-advanced-json">扩展设置（JSON 对象）</label>
                <Button
                  type="link"
                  size="small"
                  icon={<QuestionCircleOutlined />}
                  aria-expanded={showExample}
                  aria-controls="tenant-settings-example"
                  onClick={() => setShowExample((previous) => !previous)}
                >
                  {showExample ? '收起填写示例' : '查看填写示例'}
                </Button>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                没有额外配置请保留 {'{}'}。键名和文本必须使用英文双引号；不能写注释、尾随逗号。
              </Text>
              {showExample ? (
                <div
                  id="tenant-settings-example"
                  style={{
                    border: '1px solid var(--ant-color-border-secondary)',
                    background: 'var(--ant-color-fill-quaternary)',
                    borderRadius: 8,
                    padding: 12,
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8, marginBottom: 8, flexWrap: 'wrap' }}>
                    <Text strong>JSON 结构示例</Text>
                    <Popconfirm
                      title="用示例替换当前编辑框内容？"
                      description="这会覆盖当前未保存的编辑内容，但不会自动保存到服务器。"
                      okText="替换"
                      cancelText="取消"
                      onConfirm={() => updateAdvancedText(TENANT_SETTINGS_EXAMPLE)}
                    >
                      <Button size="small" disabled={submitting}>填入示例</Button>
                    </Popconfirm>
                  </div>
                  <pre
                    style={{
                      whiteSpace: 'pre-wrap',
                      overflowWrap: 'anywhere',
                      margin: '0 0 8px',
                      fontSize: 12,
                      lineHeight: 1.6,
                      fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace',
                    }}
                  >{TENANT_SETTINGS_EXAMPLE}</pre>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    custom、department、remark、labels 仅用于演示对象、字符串和数组格式；它们只是保存的元数据，
                    不会自动改变系统功能、权限、语言或业务规则。正式功能请到对应的功能设置页面配置。
                  </Text>
                </div>
              ) : null}
              <Input.TextArea
                id="tenant-advanced-json"
                aria-label="企业扩展设置 JSON"
                value={advancedText}
                onChange={(event) => updateAdvancedText(event.target.value)}
                autoSize={{ minRows: 8, maxRows: 18 }}
                spellCheck={false}
                disabled={submitting}
                style={{ fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace' }}
              />
              {advancedError ? <Alert type="error" showIcon message={advancedError} /> : null}
              <Space wrap>
                <Button onClick={formatAdvanced} disabled={submitting}>校验并格式化</Button>
                <Popconfirm title="清空全部扩展选项？" description="只会改变编辑框内容，点击保存后才会提交。" onConfirm={() => updateAdvancedText('{}')} okText="清空" cancelText="取消">
                  <Button disabled={submitting}>恢复为空对象</Button>
                </Popconfirm>
              </Space>
              <Text type="secondary">最大 64 KiB；原始 JSON 不会随基本资料一同保存。修改前请先确认已有选项的用途。</Text>
            </Space>
          </Modal>
        </Space>
      ) : null}
    </TmPageContainer>
  );
}
