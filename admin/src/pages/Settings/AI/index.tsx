import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Form, Input, InputNumber, Select, Typography } from 'antd';

export default function AISettingsPage() {
  return (
    <PageContainer
      title="AI 设置"
      subTitle="通过 AI Provider 抽象接入 OpenAI 兼容与多家模型，不在前端暴露 API Key。"
    >
      <Alert
        type="warning"
        showIcon
        message="安全提示"
        description="API Key、Base URL 仅在后端加密存储；此处表单仅为交互占位。"
        style={{ marginBottom: 16 }}
      />
      <ProCard title="Provider（占位）" bordered>
        <Form layout="vertical" style={{ maxWidth: 560 }}>
          <Form.Item label="Provider 类型" name="provider" initialValue="openai_compatible">
            <Select
              options={[
                { label: 'OpenAI 兼容', value: 'openai_compatible' },
                { label: 'DeepSeek（预留）', value: 'deepseek', disabled: true },
                { label: '通义（预留）', value: 'qwen', disabled: true },
              ]}
            />
          </Form.Item>
          <Form.Item label="Base URL" name="baseUrl">
            <Input placeholder="https://api.openai.com/v1" />
          </Form.Item>
          <Form.Item label="模型" name="model">
            <Input placeholder="gpt-4o-mini" />
          </Form.Item>
          <Form.Item label="API Key" name="apiKey">
            <Input.Password placeholder="仅在后端保存；此处为占位" autoComplete="new-password" />
          </Form.Item>
          <Form.Item label="超时（秒）" name="timeoutSec">
            <InputNumber min={5} max={300} style={{ width: '100%' }} placeholder="60" />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            「测试连接」将调用后端&nbsp;
            <Typography.Text code>POST /api/v1/settings/test-ai</Typography.Text>（实现后）。
          </Typography.Paragraph>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
