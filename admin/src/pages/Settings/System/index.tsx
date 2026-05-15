import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'system';

const FIELDS: Record<string, FieldSpec> = {
  site_name: {},
  timezone: {},
};

export default function SystemSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      form.setFieldsValue(pickGroup(items, GROUP));
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
    <PageContainer title="系统设置" subTitle="站点等基础参数保存在 settings 表（group: system）。">
      <Alert
        type="info"
        showIcon
        message="说明"
        description="列表接口已对加密项脱敏；未改动的密钥字段可保持 **** 占位，保存时不会覆盖原值。"
        style={{ marginBottom: 16 }}
      />
      <ProCard
        title="站点"
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
          style={{ maxWidth: 480 }}
          onFinish={async (values) => {
            try {
              await saveSettingsItems(toPutItems(GROUP, FIELDS, values));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Form.Item label="站点名称" name="site_name" rules={[{ required: true, message: '请输入站点名称' }]}>
            <Input placeholder="贸灵 TradeMind" />
          </Form.Item>
          <Form.Item label="时区" name="timezone" rules={[{ required: true, message: '请输入时区' }]}>
            <Input placeholder="Asia/Shanghai" />
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
