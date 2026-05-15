import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Form, Input, Typography } from 'antd';

export default function SystemSettingsPage() {
  return (
    <PageContainer
      title="系统设置"
      subTitle="站点、安全与队列等基础参数将放在配置中心（settings 表）。"
    >
      <Alert
        type="info"
        showIcon
        message="对接说明"
        description="保存与脱敏展示需调用后端 PUT /api/v1/settings；密钥类字段仅展示 sk-**** 形式。"
        style={{ marginBottom: 16 }}
      />
      <ProCard title="站点（占位表单）" bordered>
        <Form layout="vertical" style={{ maxWidth: 480 }}>
          <Form.Item label="站点名称" name="siteName" rules={[{ required: true, message: '请输入站点名称' }]}>
            <Input placeholder="贸灵 TradeMind" />
          </Form.Item>
          <Form.Item label="时区" name="timezone">
            <Input placeholder="Asia/Shanghai" disabled />
          </Form.Item>
          <Typography.Text type="secondary">
            提交按钮将在接通配置 API 后启用，避免散写 fetch。
          </Typography.Text>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
