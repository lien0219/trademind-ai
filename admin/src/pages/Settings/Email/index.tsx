import { MailOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Button,
  Form,
  Input,
  Radio,
  Space,
  InputNumber,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, testEmailConnection, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const GROUP = 'email';

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

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        provider: g.provider || 'smtp',
        smtp_host: g.smtp_host || '',
        smtp_port: g.smtp_port || 465,
        smtp_username: g.smtp_username || '',
        smtp_password: g.smtp_password || '',
        smtp_from: g.smtp_from || '',
        smtp_from_name: g.smtp_from_name || '',
        smtp_use_tls: g.smtp_use_tls === 'true',
        smtp_use_ssl: g.smtp_use_ssl !== 'false',
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
      message.success('测试邮件已发送');
    } catch (e: unknown) {
      if ((e as any).errorFields) return; // validate error
      message.error((e as Error)?.message || '发送失败');
    } finally {
      setTesting(false);
    }
  };

  return (
    <PageContainer title="邮箱设置">
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
          style={{ maxWidth: 600 }}
          onFinish={async (values) => {
            try {
              // Convert booleans to strings
              const v = { ...values };
              v.smtp_use_tls = v.smtp_use_tls ? 'true' : 'false';
              v.smtp_use_ssl = v.smtp_use_ssl ? 'true' : 'false';
              await saveSettingsItems(buildEmailPutItems(v));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Form.Item label="服务提供商" name="provider">
            <Radio.Group>
              <Radio.Button value="smtp">SMTP</Radio.Button>
            </Radio.Group>
          </Form.Item>
          
          <Form.Item label="SMTP 服务器 (Host)" name="smtp_host" rules={[{ required: true }]}>
            <Input placeholder="smtp.example.com" />
          </Form.Item>
          
          <Form.Item label="SMTP 端口 (Port)" name="smtp_port" rules={[{ required: true }]}>
            <InputNumber style={{ width: 200 }} />
          </Form.Item>
          
          <Form.Item label="邮箱账号 (Username)" name="smtp_username">
            <Input placeholder="通常为你的邮箱地址" />
          </Form.Item>
          
          <Form.Item label="邮箱密码 (Password / Auth Code)" name="smtp_password">
            <Input.Password autoComplete="new-password" placeholder="密码或应用授权码" />
          </Form.Item>
          
          <Form.Item label="发件人邮箱 (From Email)" name="smtp_from" rules={[{ required: true, type: 'email' }]}>
            <Input placeholder="noreply@example.com" />
          </Form.Item>
          
          <Form.Item label="发件人名称 (From Name)" name="smtp_from_name">
            <Input placeholder="TradeMind" />
          </Form.Item>
          
          <Form.Item label="连接安全方式">
            <Space>
              <Form.Item name="smtp_use_ssl" valuePropName="checked" noStyle>
                <Radio value={true}>SSL</Radio>
              </Form.Item>
              <Form.Item name="smtp_use_tls" valuePropName="checked" noStyle>
                <Radio value={true}>TLS (STARTTLS)</Radio>
              </Form.Item>
            </Space>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </ProCard>
      
      <ProCard title="发送测试" bordered style={{ marginTop: 16 }}>
        <Form form={testForm} layout="inline" style={{ maxWidth: 600 }}>
          <Form.Item name="to" label="接收邮箱" rules={[{ required: true, type: 'email' }]}>
            <Input placeholder="test@example.com" style={{ width: 300 }} />
          </Form.Item>
          <Form.Item>
            <Button icon={<MailOutlined />} onClick={handleTest} loading={testing}>
              测试发送
            </Button>
          </Form.Item>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
