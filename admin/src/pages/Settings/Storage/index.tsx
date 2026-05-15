import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Form, Input, Radio, Typography } from 'antd';

export default function StorageSettingsPage() {
  return (
    <PageContainer
      title="存储设置"
      subTitle="默认本地存储，可扩展 S3 / COS / OSS / R2 / MinIO，通过 Storage Provider 接入。"
    >
      <Alert
        type="info"
        showIcon
        message="上传路径"
        description="本地模式映射到 data/uploads；业务仅存 object_key 与 public_url。"
        style={{ marginBottom: 16 }}
      />
      <ProCard title="存储方式（占位）" bordered>
        <Form layout="vertical" style={{ maxWidth: 560 }}>
          <Form.Item label="Provider" name="kind" initialValue="local">
            <Radio.Group>
              <Radio.Button value="local">本地磁盘</Radio.Button>
              <Radio.Button value="s3" disabled>
                S3 兼容（预留）
              </Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Form.Item label="本地公开前缀 URL" name="publicBase">
            <Input placeholder="http://127.0.0.1:8080/static" />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            连接测试走&nbsp;
            <Typography.Text code>POST /api/v1/settings/test-storage</Typography.Text>
            ，实现后再绑定 Form 提交。
          </Typography.Paragraph>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
