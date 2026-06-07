import { Link } from '@umijs/renderer-react';
import { LockOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Col, Form, Input, InputNumber, Row, Select, Space, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  SECURITY_FIELD_HELP,
  SECURITY_FIELD_LABEL,
  SECURITY_FIELD_PLACEHOLDER,
  SECURITY_SESSION_TIMEOUT_PRESETS,
} from '@/constants/securitySettings';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const { Paragraph, Text, Title } = Typography;

const GROUP = 'security';

const FIELDS: Record<string, FieldSpec> = {
  session_idle_timeout_min: {},
  force_https: {},
  ops_webhook_secret: { encrypted: true },
};

function truthyStored(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function SecurityToggleCard({
  name,
  label,
  extra,
}: {
  name: string;
  label: string;
  extra: string;
}) {
  return (
    <div className="tm-system-settings__toggle-card">
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Text className="tm-system-settings__toggle-label">{label}</Text>
          <Text type="secondary" className="tm-system-settings__toggle-extra">
            {extra}
          </Text>
        </div>
        <Form.Item name={name} valuePropName="checked" style={{ marginBottom: 0, flexShrink: 0 }}>
          <Switch />
        </Form.Item>
      </div>
    </div>
  );
}

export default function SecuritySettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        session_idle_timeout_min: g.session_idle_timeout_min ? Number(g.session_idle_timeout_min) : 60,
        force_https: truthyStored(g.force_https),
        ops_webhook_secret: g.ops_webhook_secret || '',
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  const onFinish = async (values: Record<string, unknown>) => {
    try {
      const payload = {
        session_idle_timeout_min: String(values.session_idle_timeout_min ?? ''),
        force_https: values.force_https ? 'true' : 'false',
        ops_webhook_secret: values.ops_webhook_secret ?? '',
      };
      await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
      message.success('已保存');
      await load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存失败');
    }
  };

  return (
    <PageContainer title="安全设置" subTitle="会话超时、HTTPS 策略与运维回调签名校验">
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <LockOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Title level={5} className="tm-system-settings__hero-title">
                访问与传输安全
              </Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                控制后台登录会话的有效时长、是否强制 HTTPS，以及外部运维 Webhook 的签名校验密钥。敏感项加密存库、接口脱敏，日志不输出明文密钥。
                业务告警 Webhook 请在 <Link to="/settings/alert-notify">告警通知配置</Link> 中单独设置。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <Form form={form} layout="vertical" onFinish={onFinish}>
          <ProCard
            variant="outlined"
            title="会话管理"
            className="tm-system-settings__panel"
            style={{ marginTop: 16 }}
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="空闲超时说明"
              description="仅统计无操作时间；关闭浏览器标签不会立即失效，下次请求时会校验会话是否过期。"
            />
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={10}>
                <Form.Item
                  label={SECURITY_FIELD_LABEL.sessionIdleTimeoutMin}
                  name="session_idle_timeout_min"
                  rules={[
                    { required: true, message: '请填写会话空闲超时' },
                    { type: 'number', min: 5, max: 10080, message: '范围 5–10080 分钟' },
                  ]}
                  extra={SECURITY_FIELD_HELP.sessionIdleTimeoutMin}
                >
                  <InputNumber min={5} max={10080} style={{ width: '100%' }} suffix="分钟" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={10}>
                <Form.Item label="快捷选择" style={{ marginBottom: 0 }}>
                  <Select
                    placeholder="选择常用时长"
                    allowClear
                    options={SECURITY_SESSION_TIMEOUT_PRESETS.map((p) => ({ label: p.label, value: p.value }))}
                    onChange={(v) => {
                      if (typeof v === 'number') {
                        form.setFieldValue('session_idle_timeout_min', v);
                      }
                    }}
                  />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="传输安全" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={16} lg={14}>
                <SecurityToggleCard
                  name="force_https"
                  label={SECURITY_FIELD_LABEL.forceHttps}
                  extra={SECURITY_FIELD_HELP.forceHttps}
                />
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="Webhook 签名校验" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Paragraph type="secondary" style={{ marginBottom: 16, fontSize: 13 }}>
              用于验证外部系统向贸灵发起的运维类回调请求，与告警通知中的 Webhook 密钥相互独立。
            </Paragraph>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={14} lg={12}>
                <Form.Item
                  label={SECURITY_FIELD_LABEL.opsWebhookSecret}
                  name="ops_webhook_secret"
                  extra={SECURITY_FIELD_HELP.opsWebhookSecret}
                >
                  <Input.Password autoComplete="new-password" placeholder={SECURITY_FIELD_PLACEHOLDER.opsWebhookSecret} />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer" style={{ marginTop: 16 }}>
            <Space wrap>
              <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
                保存配置
              </Button>
              <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            </Space>
          </ProCard>
        </Form>
      </div>
    </PageContainer>
  );
}
