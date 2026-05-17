import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, InputNumber, Select, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

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
      itemValue: String(values.notification_channels ?? '[]'),
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
  return [
    { tenantId, groupKey: g, itemKey: 'enabled', itemValue: boolStr(values.an_enabled), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: g, itemKey: 'channels', itemValue: String(values.an_channels ?? '[]'), valueType: 'string', isEncrypted: false, remark: '' },
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
        notification_channels: tc.notification_channels || '[]',
        alert_detail_public_base: tc.alert_detail_public_base || '',
        an_enabled: truthyStored(an.enabled),
        an_channels: an.channels || '[]',
        mail_enabled: truthyStored(an.mail_enabled),
        mail_to: an.mail_to || '',
        mail_cc: an.mail_cc || '',
        mail_subject_prefix: an.mail_subject_prefix || '',
        webhook_enabled: truthyStored(an.webhook_enabled),
        webhook_url: an.webhook_url || '',
        webhook_method: an.webhook_method || '',
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
    <PageContainer title="告警通知配置">
      <ProCard bordered style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="配置原则"
          description={
            <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
              <li>
                告警扫描策略见 <Typography.Text code>settings.taskcenter</Typography.Text>（系统设置）；出站通道见{' '}
                <Typography.Text code>settings.alert_notify</Typography.Text>；SMTP 仅使用 <Typography.Text code>settings.mail</Typography.Text>（邮件设置）。
              </li>
              <li>不在环境变量或仓库内填写收件邮箱、Webhook、阈值或通道开关；敏感项仅存库（AES-GCM），接口以 **** 脱敏。</li>
              <li>飞书 / 企业微信首版为预留（planned），可保存配置，发送结果为 skipped。</li>
              <li>
                部署级 <Typography.Text code>TASK_ALERT_SCAN_ENABLED</Typography.Text> 与页面「启用告警定时扫描 Worker」同时开启后，进程内才会定时扫描。
              </li>
              <li>通知正文与 Webhook 负载均经裁剪；不会包含完整平台响应、客户消息全文或密钥。</li>
            </ul>
          }
        />
      </ProCard>
      <ProCard
        bordered
        extra={
          <Button type="link" onClick={load} disabled={loading}>
            重新加载
          </Button>
        }
      >
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 720 }}
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
          <ProCard title="策略（settings.taskcenter）" bordered type="inner" style={{ marginBottom: 16 }}>
            <Form.Item label="启用外部通知（与下方分组总开关同时生效）" name="enable_external_notifications" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item
              label="通知最低严重等级（外部）"
              name="notification_min_severity"
              tooltip="留空则不自动向外部渠道发送（仍可在告警列表手动触发，需配置通道）"
            >
              <Select
                allowClear
                placeholder="未配置"
                options={[
                  { value: 'low', label: 'low' },
                  { value: 'medium', label: 'medium' },
                  { value: 'high', label: 'high' },
                  { value: 'critical', label: 'critical' },
                ]}
              />
            </Form.Item>
            <Form.Item label="新告警时通知" name="notify_on_alert_generated" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="告警重复出现时通知（alert_count 递增）" name="notify_on_repeated_alert" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item
              label="通知通道（JSON 数组）"
              name="notification_channels"
              rules={[{ required: true, message: '请填写 JSON 数组，如 ["mail","webhook"]' }]}
            >
              <Input.TextArea rows={2} placeholder='例如 ["mail","webhook"]' />
            </Form.Item>
            <Form.Item
              label="详情链接公开前缀（可选）"
              name="alert_detail_public_base"
              tooltip="拼接到站内路径前，用于邮件/Webhook 中可点击的管理端地址，如 https://ops.example.com"
            >
              <Input placeholder="https://your-admin.example.com" />
            </Form.Item>
          </ProCard>

          <ProCard title="通道配置（settings.alert_notify）" bordered type="inner" style={{ marginBottom: 16 }}>
            <Form.Item label="启用本分组总开关" name="an_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="通道展示列表（JSON，可选）" name="an_channels">
              <Input.TextArea rows={1} placeholder='例如 ["mail","webhook","feishu","wecom"]' />
            </Form.Item>
            <Typography.Title level={5}>邮件</Typography.Title>
            <Form.Item label="启用邮件" name="mail_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="收件人 mail_to（逗号分隔）" name="mail_to">
              <Input placeholder="在页面配置实际收件人" />
            </Form.Item>
            <Form.Item label="抄送 mail_cc" name="mail_cc">
              <Input />
            </Form.Item>
            <Form.Item label="主题前缀（可选）" name="mail_subject_prefix">
              <Input placeholder="留空则仅使用 [等级][分类] 标题" />
            </Form.Item>
            <Typography.Title level={5}>Webhook</Typography.Title>
            <Form.Item label="启用 Webhook" name="webhook_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="URL（加密）" name="webhook_url">
              <Input.Password placeholder="在页面配置 HTTPS 地址" autoComplete="off" />
            </Form.Item>
            <Form.Item label="HTTP 方法" name="webhook_method" tooltip="留空则发送时使用 POST">
              <Input placeholder="POST" />
            </Form.Item>
            <Form.Item label="签名密钥（加密）" name="webhook_secret" extra="可选；设置后请求带 X-TradeMind-Signature: HMAC-SHA256(body)">
              <Input.Password autoComplete="off" />
            </Form.Item>
            <Form.Item label="HTTP 超时（秒）" name="webhook_timeout_seconds" tooltip="留空则使用后端安全默认值">
              <InputNumber min={1} max={300} style={{ width: '100%' }} placeholder="可选" />
            </Form.Item>
            <Form.Item label="模板（预留）" name="webhook_template">
              <Input.TextArea rows={2} disabled />
            </Form.Item>
            <Typography.Title level={5}>飞书（planned / 预留）</Typography.Title>
            <Form.Item label="启用" name="feishu_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="Webhook URL（加密）" name="feishu_webhook_url">
              <Input.Password autoComplete="off" />
            </Form.Item>
            <Form.Item label="Secret（加密）" name="feishu_secret">
              <Input.Password autoComplete="off" />
            </Form.Item>
            <Typography.Title level={5}>企业微信（planned / 预留）</Typography.Title>
            <Form.Item label="启用" name="wecom_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="Webhook URL（加密）" name="wecom_webhook_url">
              <Input.Password autoComplete="off" />
            </Form.Item>
          </ProCard>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
