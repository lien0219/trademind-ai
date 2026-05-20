import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Button, Form, Input, InputNumber, message, Switch } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'security';

const FIELDS: Record<string, FieldSpec> = {
  session_idle_timeout_min: {},
  force_https: {},
  ops_webhook_secret: { encrypted: true },
};

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
        force_https: g.force_https === '1' || g.force_https === 'true',
        ops_webhook_secret: g.ops_webhook_secret || '',
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
    <PageContainer title="安全设置">
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
          style={{ maxWidth: 520 }}
          onFinish={async (values) => {
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
          }}
        >
          <Form.Item
            label="会话空闲超时（分钟）"
            name="session_idle_timeout_min"
            rules={[{ required: true }]}
          >
            <InputNumber min={5} max={10080} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="强制 HTTPS（反向代理场景）" name="force_https" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            label="运维通知密钥（Webhook）"
            name="ops_webhook_secret"
          >
            <Input.Password autoComplete="new-password" placeholder="无需回调可留空" />
          </Form.Item>
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
