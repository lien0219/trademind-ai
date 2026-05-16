import { Link } from '@umijs/renderer-react';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, InputNumber, message, Select, Space } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, testAIConnection } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'ai';

const FIELDS: Record<string, FieldSpec> = {
  provider: {},
  base_url: {},
  model: {},
  api_key: { encrypted: true },
  temperature: {},
  max_tokens: {},
  timeout_sec: {},
};

export default function AISettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        provider: g.provider || 'openai_compatible',
        base_url: g.base_url || '',
        model: g.model || '',
        api_key: g.api_key || '',
        temperature: g.temperature !== undefined && g.temperature !== '' ? Number(g.temperature) : 0.7,
        max_tokens: g.max_tokens !== undefined && g.max_tokens !== '' ? Number(g.max_tokens) : 512,
        timeout_sec: g.timeout_sec ? Number(g.timeout_sec) : 60,
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
    <PageContainer title="AI 设置">
      <ProCard bordered style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="自备大模型 API"
          description={
            <>
              请在 OpenAI / DeepSeek / 通义 / Ollama（OpenAI 兼容）等渠道自行申请 Key 与 Base URL。贸灵前端不会请求模型接口，仅后端通过 AI Gateway 调用。摘要见{' '}
              <Link to="/settings/integrations">第三方集成总览</Link>。
            </>
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
          style={{ maxWidth: 560 }}
          onFinish={async (values) => {
            try {
              const payload = {
                ...values,
                timeout_sec: String(values.timeout_sec ?? ''),
                temperature: String(values.temperature ?? ''),
                max_tokens: String(values.max_tokens ?? ''),
              };
              await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Form.Item label="Provider 类型" name="provider" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'OpenAI 兼容', value: 'openai_compatible' },
                { label: 'DeepSeek（预留）', value: 'deepseek', disabled: true },
                { label: '通义（预留）', value: 'qwen', disabled: true },
              ]}
            />
          </Form.Item>
          <Form.Item
            label="Base URL"
            name="base_url"
            rules={[{ required: true, message: '请输入 Base URL' }]}
            extra="OpenAI 兼容接口根路径，不含 /chat/completions"
          >
            <Input placeholder="https://api.openai.com/v1" />
          </Form.Item>
          <Form.Item label="模型" name="model" rules={[{ required: true, message: '请输入模型名' }]}>
            <Input placeholder="gpt-4o-mini" />
          </Form.Item>
          <Form.Item label="API Key" name="api_key" rules={[{ required: true, message: '请输入 API Key' }]}>
            <Input.Password placeholder="敏感；保存后显示为 ****，留空不覆盖则需在列表里保留原占位" autoComplete="new-password" />
          </Form.Item>
          <Form.Item label="Temperature" name="temperature" extra="默认 0.7；与网关缺省一致">
            <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="Max tokens" name="max_tokens" extra="默认 512">
            <InputNumber min={1} max={32000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="超时（秒）" name="timeout_sec" rules={[{ required: true }]}>
            <InputNumber min={5} max={600} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>
                保存
              </Button>
              <Button
                loading={testing}
                onClick={async () => {
                  setTesting(true);
                  try {
                    await testAIConnection();
                    message.success('连接成功');
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '连接失败');
                  } finally {
                    setTesting(false);
                  }
                }}
              >
                测试连接
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
