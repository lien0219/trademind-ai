import { Link } from '@umijs/renderer-react';
import { BellOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
  Divider,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import {
  NOTIFICATION_CHANNEL_META,
  NOTIFICATION_CHANNEL_OPTIONS,
  NOTIFICATION_SEVERITY_OPTIONS,
  WEBHOOK_METHOD_OPTIONS,
  parseNotificationChannels,
  stringifyNotificationChannels,
} from '@/constants/alertNotify';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const { Paragraph, Text } = Typography;

const GROUP_TC = 'taskcenter';
const GROUP_AN = 'alert_notify';

function boolStr(b: unknown) {
  return b ? 'true' : 'false';
}

function truthyStored(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function buildTcNotifyItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const channels = Array.isArray(values.notification_channels)
    ? stringifyNotificationChannels(values.notification_channels as string[])
    : String(values.notification_channels ?? '[]');
  return [
    {
      tenantId,
      groupKey: GROUP_TC,
      itemKey: 'enable_external_notifications',
      itemValue: boolStr(values.enable_external_notifications),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP_TC,
      itemKey: 'notification_min_severity',
      itemValue: String(values.notification_min_severity ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP_TC,
      itemKey: 'notify_on_alert_generated',
      itemValue: boolStr(values.notify_on_alert_generated),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP_TC,
      itemKey: 'notify_on_repeated_alert',
      itemValue: boolStr(values.notify_on_repeated_alert),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP_TC,
      itemKey: 'notification_channels',
      itemValue: channels,
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP_TC,
      itemKey: 'alert_detail_public_base',
      itemValue: String(values.alert_detail_public_base ?? '').trim(),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
  ];
}

function buildAlertNotifyItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const g = GROUP_AN;
  const displayChannels = Array.isArray(values.an_channels)
    ? stringifyNotificationChannels(values.an_channels as string[])
    : String(values.an_channels ?? '[]');
  return [
    { tenantId, groupKey: g, itemKey: 'enabled', itemValue: boolStr(values.an_enabled), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'channels', itemValue: displayChannels, valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'mail_enabled', itemValue: boolStr(values.mail_enabled), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'mail_to', itemValue: String(values.mail_to ?? ''), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'mail_cc', itemValue: String(values.mail_cc ?? ''), valueType: 'string', isEncrypted: false, remark: '' },
    {
      tenantId,
      groupKey: g,
      itemKey: 'mail_subject_prefix',
      itemValue: String(values.mail_subject_prefix ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    { tenantId, groupKey: g, itemKey: 'webhook_enabled', itemValue: boolStr(values.webhook_enabled), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'webhook_url', itemValue: String(values.webhook_url ?? ''), valueType: 'string', isEncrypted: true, remark: '' },
    {
      tenantId,
      groupKey: g,
      itemKey: 'webhook_method',
      itemValue: String(values.webhook_method ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    { tenantId, groupKey: g, itemKey: 'webhook_secret', itemValue: String(values.webhook_secret ?? ''), valueType: 'string', isEncrypted: true, remark: '' },
    {
      tenantId,
      groupKey: g,
      itemKey: 'webhook_timeout_seconds',
      itemValue:
        values.webhook_timeout_seconds === undefined || values.webhook_timeout_seconds === null || values.webhook_timeout_seconds === ''
          ? ''
          : String(values.webhook_timeout_seconds),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    { tenantId, groupKey: g, itemKey: 'webhook_template', itemValue: String(values.webhook_template ?? ''), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'feishu_enabled', itemValue: boolStr(values.feishu_enabled), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'feishu_webhook_url', itemValue: String(values.feishu_webhook_url ?? ''), valueType: 'string', isEncrypted: true, remark: '' },
    { tenantId, groupKey: g, itemKey: 'feishu_secret', itemValue: String(values.feishu_secret ?? ''), valueType: 'string', isEncrypted: true, remark: '' },
    { tenantId, groupKey: g, itemKey: 'wecom_enabled', itemValue: boolStr(values.wecom_enabled), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'wecom_webhook_url', itemValue: String(values.wecom_webhook_url ?? ''), valueType: 'string', isEncrypted: true, remark: '' },
  ];
}

function ChannelPanel({
  title,
  desc,
  planned,
  switchName,
  enabled,
  groupEnabled,
  children,
}: {
  title: string;
  desc: string;
  planned?: boolean;
  switchName: string;
  enabled: boolean;
  groupEnabled: boolean;
  children: ReactNode;
}) {
  const active = groupEnabled && enabled;
  return (
    <div className="tm-alert-notify__channel-panel">
      <div className="tm-alert-notify__channel-head">
        <div>
          <Space size={8} wrap>
            <Text className="tm-alert-notify__channel-title">{title}</Text>
            {planned ? <Tag>预留</Tag> : null}
          </Space>
          <Text type="secondary" className="tm-alert-notify__channel-desc">
            {desc}
          </Text>
        </div>
        <Form.Item name={switchName} valuePropName="checked" style={{ marginBottom: 0, flexShrink: 0 }}>
          <Switch disabled={!groupEnabled} />
        </Form.Item>
      </div>
      <div className={active ? undefined : 'tm-alert-notify__channel-fields--dim'}>{children}</div>
    </div>
  );
}

export default function AlertNotifySettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const tc = pickGroup(items, GROUP_TC);
      const an = pickGroup(items, GROUP_AN);
      form.setFieldsValue({
        enable_external_notifications: truthyStored(tc.enable_external_notifications),
        notification_min_severity: tc.notification_min_severity || undefined,
        notify_on_alert_generated: truthyStored(tc.notify_on_alert_generated),
        notify_on_repeated_alert: truthyStored(tc.notify_on_repeated_alert),
        notification_channels: parseNotificationChannels(tc.notification_channels),
        alert_detail_public_base: tc.alert_detail_public_base || '',
        an_enabled: truthyStored(an.enabled),
        an_channels: parseNotificationChannels(an.channels),
        mail_enabled: truthyStored(an.mail_enabled),
        mail_to: an.mail_to || '',
        mail_cc: an.mail_cc || '',
        mail_subject_prefix: an.mail_subject_prefix || '',
        webhook_enabled: truthyStored(an.webhook_enabled),
        webhook_url: an.webhook_url || '',
        webhook_method: an.webhook_method || undefined,
        webhook_secret: an.webhook_secret || '',
        webhook_timeout_seconds:
          an.webhook_timeout_seconds === '' || an.webhook_timeout_seconds === undefined
            ? undefined
            : parseInt(String(an.webhook_timeout_seconds), 10) || undefined,
        webhook_template: an.webhook_template || '',
        feishu_enabled: truthyStored(an.feishu_enabled),
        feishu_webhook_url: an.feishu_webhook_url || '',
        feishu_secret: an.feishu_secret || '',
        wecom_enabled: truthyStored(an.wecom_enabled),
        wecom_webhook_url: an.wecom_webhook_url || '',
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

  return (
    <PageContainer title="告警通知配置" subTitle="配置任务告警的外部通知策略与邮件、Webhook 等出站通道">
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <BellOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">
                出站告警通知
              </Typography.Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                站内告警生成规则在「系统设置」中配置；SMTP 发信服务器在「邮箱设置」中配置。本页负责选择通知通道、触发条件，以及各通道的收件人 /
                Webhook 参数。敏感信息加密存库，接口脱敏展示。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <Alert
          type="info"
          showIcon
          message="配置说明"
          description={
            <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
              <li>
                需同时开启「启用外部通知」与「通道总开关」，并配置至少一个可用通道后，系统才会自动向外发送。
              </li>
              <li>
                告警扫描与站内策略见 <Link to="/settings/system">系统设置</Link>；SMTP 见{' '}
                <Link to="/settings/email">邮箱设置</Link>。
              </li>
              <li>飞书 / 企业微信当前为预留能力，可保存配置，实际发送结果为 skipped。</li>
              <li>通知正文与 Webhook 负载均经裁剪，不会包含完整平台响应、客户消息全文或密钥。</li>
            </ul>
          }
        />

        <Form
          form={form}
          layout="vertical"
          onFinish={async (values) => {
            try {
              await saveSettingsItems(buildTcNotifyItems(values).concat(buildAlertNotifyItems(values)));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <ProCard
            variant="outlined"
            title="通知策略"
            className="tm-system-settings__panel"
            style={{ marginTop: 16 }}
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Form.Item label="启用外部通知" name="enable_external_notifications" valuePropName="checked" extra="需与下方「通道总开关」同时开启">
              <Switch />
            </Form.Item>

            <Form.Item
              noStyle
              shouldUpdate={(prev, next) => prev.enable_external_notifications !== next.enable_external_notifications}
            >
              {({ getFieldValue }) => {
                const externalEnabled = !!getFieldValue('enable_external_notifications');
                return (
                  <>
                    <Row gutter={[24, 0]}>
                      <Col xs={24} md={12} lg={8}>
                        <Form.Item
                          label="通知最低严重等级"
                          name="notification_min_severity"
                          tooltip="留空则不自动向外部渠道发送（仍可在告警列表手动触发）"
                        >
                          <Select
                            allowClear
                            placeholder="未设置（不自动外发）"
                            options={NOTIFICATION_SEVERITY_OPTIONS}
                            disabled={!externalEnabled}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12} lg={8}>
                        <Form.Item label="新告警时通知" name="notify_on_alert_generated" valuePropName="checked">
                          <Switch disabled={!externalEnabled} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12} lg={8}>
                        <Form.Item
                          label="告警重复出现时通知"
                          name="notify_on_repeated_alert"
                          valuePropName="checked"
                          tooltip="同一告警重复触发且计数递增时再次通知"
                        >
                          <Switch disabled={!externalEnabled} />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Row gutter={[24, 0]}>
                      <Col xs={24} md={12}>
                        <Form.Item
                          label="自动通知通道"
                          name="notification_channels"
                          rules={externalEnabled ? [{ required: true, message: '请至少选择一个通知通道' }] : []}
                          tooltip="决定系统自动外发时尝试哪些通道；各通道细节在下方配置"
                        >
                          <Select
                            mode="multiple"
                            allowClear
                            placeholder="选择邮件、Webhook 等"
                            options={NOTIFICATION_CHANNEL_OPTIONS}
                            disabled={!externalEnabled}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12}>
                        <Form.Item
                          label="详情链接公开前缀"
                          name="alert_detail_public_base"
                          tooltip="拼接到站内路径前，用于邮件 / Webhook 中可点击的管理端地址"
                        >
                          <Input placeholder="https://ops.example.com" disabled={!externalEnabled} />
                        </Form.Item>
                      </Col>
                    </Row>
                    {!externalEnabled ? (
                      <Text type="secondary" style={{ display: 'block', marginTop: -8, fontSize: 12 }}>
                        开启外部通知后可配置等级、触发条件与通道
                      </Text>
                    ) : null}
                  </>
                );
              }}
            </Form.Item>
          </ProCard>

          <ProCard variant="outlined" title="通道配置" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Form.Item
              label="通道总开关"
              name="an_enabled"
              valuePropName="checked"
              extra="关闭后所有通道配置保留，但不会实际发送"
            >
              <Switch />
            </Form.Item>

            <Form.Item label="管理端展示通道（可选）" name="an_channels" tooltip="仅影响后台展示，不影响实际发送逻辑">
              <Select mode="multiple" allowClear placeholder="默认展示全部已配置通道" options={NOTIFICATION_CHANNEL_OPTIONS} />
            </Form.Item>

            <Divider plain>各通道参数</Divider>

            <Form.Item
              noStyle
              shouldUpdate={(prev, next) =>
                prev.an_enabled !== next.an_enabled ||
                prev.mail_enabled !== next.mail_enabled ||
                prev.webhook_enabled !== next.webhook_enabled ||
                prev.feishu_enabled !== next.feishu_enabled ||
                prev.wecom_enabled !== next.wecom_enabled
              }
            >
              {({ getFieldValue }) => {
                const groupEnabled = !!getFieldValue('an_enabled');
                return (
                  <Row gutter={[16, 16]}>
                    <Col xs={24} lg={12}>
                      <ChannelPanel
                        title={NOTIFICATION_CHANNEL_META.mail.label}
                        desc={NOTIFICATION_CHANNEL_META.mail.desc}
                        switchName="mail_enabled"
                        enabled={!!getFieldValue('mail_enabled')}
                        groupEnabled={groupEnabled}
                      >
                        <Form.Item label="收件人" name="mail_to" extra="多个地址用英文逗号分隔">
                          <Input placeholder="ops@example.com, oncall@example.com" />
                        </Form.Item>
                        <Form.Item label="抄送" name="mail_cc">
                          <Input placeholder="可选" />
                        </Form.Item>
                        <Form.Item label="主题前缀" name="mail_subject_prefix" extra="留空则仅使用 [等级][分类] 标题">
                          <Input placeholder="例如 [TradeMind]" />
                        </Form.Item>
                      </ChannelPanel>
                    </Col>
                    <Col xs={24} lg={12}>
                      <ChannelPanel
                        title={NOTIFICATION_CHANNEL_META.webhook.label}
                        desc={NOTIFICATION_CHANNEL_META.webhook.desc}
                        switchName="webhook_enabled"
                        enabled={!!getFieldValue('webhook_enabled')}
                        groupEnabled={groupEnabled}
                      >
                        <Form.Item label="请求地址" name="webhook_url">
                          <Input.Password placeholder="https://hooks.example.com/alerts" autoComplete="off" />
                        </Form.Item>
                        <Row gutter={12}>
                          <Col span={12}>
                            <Form.Item label="HTTP 方法" name="webhook_method">
                              <Select allowClear placeholder="POST" options={WEBHOOK_METHOD_OPTIONS} />
                            </Form.Item>
                          </Col>
                          <Col span={12}>
                            <Form.Item label="超时（秒）" name="webhook_timeout_seconds" tooltip="留空使用系统默认值">
                              <InputNumber min={1} max={300} style={{ width: '100%' }} placeholder="可选" />
                            </Form.Item>
                          </Col>
                        </Row>
                        <Form.Item
                          label="签名密钥"
                          name="webhook_secret"
                          extra="可选；设置后请求头携带 X-TradeMind-Signature（HMAC-SHA256）"
                        >
                          <Input.Password autoComplete="off" placeholder="留空则不签名" />
                        </Form.Item>
                      </ChannelPanel>
                    </Col>
                    <Col xs={24} lg={12}>
                      <ChannelPanel
                        title={NOTIFICATION_CHANNEL_META.feishu.label}
                        desc={NOTIFICATION_CHANNEL_META.feishu.desc}
                        planned
                        switchName="feishu_enabled"
                        enabled={!!getFieldValue('feishu_enabled')}
                        groupEnabled={groupEnabled}
                      >
                        <Form.Item label="Webhook 地址" name="feishu_webhook_url">
                          <Input.Password autoComplete="off" placeholder="预留，后续版本启用" />
                        </Form.Item>
                        <Form.Item label="签名 Secret" name="feishu_secret">
                          <Input.Password autoComplete="off" />
                        </Form.Item>
                      </ChannelPanel>
                    </Col>
                    <Col xs={24} lg={12}>
                      <ChannelPanel
                        title={NOTIFICATION_CHANNEL_META.wecom.label}
                        desc={NOTIFICATION_CHANNEL_META.wecom.desc}
                        planned
                        switchName="wecom_enabled"
                        enabled={!!getFieldValue('wecom_enabled')}
                        groupEnabled={groupEnabled}
                      >
                        <Form.Item label="Webhook 地址" name="wecom_webhook_url">
                          <Input.Password autoComplete="off" placeholder="预留，后续版本启用" />
                        </Form.Item>
                      </ChannelPanel>
                    </Col>
                  </Row>
                );
              }}
            </Form.Item>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer" style={{ marginTop: 16 }}>
            <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
              保存配置
            </Button>
          </ProCard>
        </Form>
      </div>
    </PageContainer>
  );
}
