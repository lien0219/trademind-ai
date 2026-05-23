import { Link } from '@umijs/renderer-react';
import { MailOutlined, ReloadOutlined, SaveOutlined, SendOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Col, Form, Input, InputNumber, Radio, Row, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  MAIL_FIELD_LABEL,
  MAIL_FIELD_PLACEHOLDER,
  MAIL_PROVIDER_META,
  MAIL_PROVIDER_OPTIONS,
  smtpPortHint,
} from '@/constants/emailSettings';
import { fetchSettingsList, saveSettingsItems, testEmailConnection, type SettingPutItem } from '@/services/settings';
import { mergeSettingsPrimaryFallback } from '@/utils/settingsForm';

const { Paragraph, Text } = Typography;

/** Primary settings group for SMTP (legacy `email` group merged on load for backward compatibility). */
const GROUP = 'mail';

function buildEmailPutItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const provider = String(values.provider || 'smtp');

  return [
    { tenantId, groupKey: GROUP, itemKey: 'provider', itemValue: provider, isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_host', itemValue: String(values.smtp_host ?? ''), isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_port', itemValue: String(values.smtp_port ?? ''), isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_username', itemValue: String(values.smtp_username ?? ''), isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_password', itemValue: String(values.smtp_password ?? ''), isEncrypted: true, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_from', itemValue: String(values.smtp_from ?? ''), isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_from_name', itemValue: String(values.smtp_from_name ?? ''), isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_use_tls', itemValue: String(values.smtp_use_tls ?? 'false'), isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'smtp_use_ssl', itemValue: String(values.smtp_use_ssl ?? 'false'), isEncrypted: false, remark: '' },
  ];
}

export default function EmailSettingsPage() {
  const [form] = Form.useForm();
  const [testForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const smtpPort = Form.useWatch('smtp_port', form);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = mergeSettingsPrimaryFallback(items, GROUP, 'email');
      form.setFieldsValue({
        provider: g.provider || 'smtp',
        smtp_host: g.smtp_host || '',
        smtp_port: g.smtp_port ? Number(g.smtp_port) : 465,
        smtp_username: g.smtp_username || '',
        smtp_password: g.smtp_password || '',
        smtp_from: g.smtp_from || '',
        smtp_from_name: g.smtp_from_name || '',
        smtp_use_tls: g.smtp_use_tls === 'true',
        smtp_use_ssl: g.smtp_use_ssl === 'true',
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    load();
  }, [load]);

  const handleTest = async () => {
    try {
      const values = await testForm.validateFields();
      setTesting(true);
      await testEmailConnection(values.to);
      message.success('测试邮件已发送，请查收收件箱');
    } catch (e: unknown) {
      if ((e as { errorFields?: unknown }).errorFields) return;
      message.error((e as Error)?.message || '发送失败');
    } finally {
      setTesting(false);
    }
  };

  const onFinish = async (values: Record<string, unknown>) => {
    try {
      const v = { ...values };
      v.smtp_use_tls = v.smtp_use_tls ? 'true' : 'false';
      v.smtp_use_ssl = v.smtp_use_ssl ? 'true' : 'false';
      v.smtp_port = v.smtp_port != null ? String(v.smtp_port) : '';
      await saveSettingsItems(buildEmailPutItems(v));
      message.success('已保存');
      await load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存失败');
    }
  };

  return (
    <PageContainer title="邮箱设置" subTitle="配置 SMTP 发信服务器，供告警通知与系统邮件使用">
      <div className="tm-system-settings">
        <ProCard bordered className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <MailOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">
                自备 SMTP 服务
              </Typography.Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                贸灵不提供邮件代发账号。请使用企业邮箱、QQ/网易客户端授权码、云邮件推送或 SendGrid /
                Mailgun 等 SMTP。密码加密存库、接口脱敏，日志不记录明文密码。告警收件人请在{' '}
                <Link to="/settings/alert-notify">告警通知配置</Link> 中设置。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <Form form={form} layout="vertical" onFinish={onFinish}>
          <ProCard
            bordered
            title="服务提供商"
            className="tm-system-settings__panel"
            style={{ marginTop: 16 }}
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Form.Item name="provider" style={{ marginBottom: 0 }}>
              <Radio.Group>
                {MAIL_PROVIDER_OPTIONS.map((opt) => (
                  <Radio.Button key={opt.value} value={opt.value}>
                    {opt.label}
                  </Radio.Button>
                ))}
              </Radio.Group>
            </Form.Item>
            <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
              {MAIL_PROVIDER_META.smtp.desc}
            </Text>
          </ProCard>

          <ProCard bordered title="SMTP 连接" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={14}>
                <Form.Item
                  label={MAIL_FIELD_LABEL.host}
                  name="smtp_host"
                  rules={[{ required: true, message: '请输入 SMTP 服务器地址' }]}
                >
                  <Input placeholder={MAIL_FIELD_PLACEHOLDER.host} />
                </Form.Item>
              </Col>
              <Col xs={24} md={10}>
                <Form.Item
                  label={MAIL_FIELD_LABEL.port}
                  name="smtp_port"
                  rules={[{ required: true, message: '请输入 SMTP 端口' }]}
                  extra={smtpPortHint(typeof smtpPort === 'number' ? smtpPort : Number(smtpPort))}
                >
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder={MAIL_FIELD_PLACEHOLDER.port} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={MAIL_FIELD_LABEL.username} name="smtp_username">
                  <Input placeholder={MAIL_FIELD_PLACEHOLDER.username} autoComplete="off" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={MAIL_FIELD_LABEL.password} name="smtp_password">
                  <Input.Password autoComplete="new-password" placeholder={MAIL_FIELD_PLACEHOLDER.password} />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard bordered title="发件人信息" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12}>
                <Form.Item
                  label={MAIL_FIELD_LABEL.from}
                  name="smtp_from"
                  rules={[{ required: true, type: 'email', message: '请输入有效的发件人邮箱' }]}
                >
                  <Input placeholder={MAIL_FIELD_PLACEHOLDER.from} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={MAIL_FIELD_LABEL.fromName} name="smtp_from_name">
                  <Input placeholder={MAIL_FIELD_PLACEHOLDER.fromName} />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard bordered title="加密方式" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="常见组合"
              description={
                <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
                  <li>端口 465 → 开启 SSL（SMTPS）</li>
                  <li>端口 587 → 开启 STARTTLS</li>
                  <li>请按邮件服务商文档选择，勿盲目同时开启两种方式</li>
                </ul>
              }
            />
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label={MAIL_FIELD_LABEL.ssl} name="smtp_use_ssl" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={MAIL_FIELD_LABEL.tls} name="smtp_use_tls" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard bordered title="发送测试" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Paragraph type="secondary" style={{ marginBottom: 12 }}>
              保存 SMTP 配置后，可向指定邮箱发送测试邮件以验证连通性。
            </Paragraph>
            <Form form={testForm} layout="vertical">
              <Row gutter={[16, 0]} align="bottom">
                <Col xs={24} md={12} lg={10}>
                  <Form.Item
                    name="to"
                    label="测试收件邮箱"
                    rules={[{ required: true, type: 'email', message: '请输入有效的收件邮箱' }]}
                    style={{ marginBottom: 0 }}
                  >
                    <Input placeholder={MAIL_FIELD_PLACEHOLDER.testTo} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12} lg={8}>
                  <Form.Item style={{ marginBottom: 0 }}>
                    <Button type="default" icon={<SendOutlined />} onClick={() => void handleTest()} loading={testing}>
                      发送测试邮件
                    </Button>
                  </Form.Item>
                </Col>
              </Row>
            </Form>
          </ProCard>

          <ProCard bordered className="tm-system-settings__footer" style={{ marginTop: 16 }}>
            <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
              保存配置
            </Button>
          </ProCard>
        </Form>
      </div>
    </PageContainer>
  );
}
