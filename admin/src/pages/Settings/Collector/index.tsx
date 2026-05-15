import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, InputNumber, message, Switch } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'collector';

const FIELDS: Record<string, FieldSpec> = {
  main_service_url: {},
  collector_http_addr: {},
  goto_timeout_ms: {},
  headless: {},
};

export default function CollectorSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        main_service_url: g.main_service_url || 'http://127.0.0.1:8080',
        collector_http_addr: g.collector_http_addr || ':3100',
        goto_timeout_ms: g.goto_timeout_ms ? Number(g.goto_timeout_ms) : 45000,
        headless: g.headless === '0' || g.headless === 'false' ? false : true,
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
    <PageContainer
      title="采集服务"
      subTitle="与独立 Node（Playwright）进程相关的可配置项；此处写入 settings 表便于后台统一查看。"
    >
      <Alert
        type="info"
        showIcon
        message="提示"
        description="实际进程仍读取环境变量；可将常用值记在此处作为团队约定，后续可与任务编排对齐。"
        style={{ marginBottom: 16 }}
      />
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
                goto_timeout_ms: String(values.goto_timeout_ms ?? ''),
                headless: values.headless ? '1' : '0',
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
            label="主服务 URL"
            name="main_service_url"
            rules={[{ required: true }]}
            extra="对应 .env 中 COLLECTOR_MAIN_SERVICE_URL"
          >
            <Input placeholder="http://127.0.0.1:8080" />
          </Form.Item>
          <Form.Item
            label="采集服务监听地址"
            name="collector_http_addr"
            rules={[{ required: true }]}
            extra="对应 COLLECTOR_HTTP_ADDR，例如 :3100"
          >
            <Input placeholder=":3100" />
          </Form.Item>
          <Form.Item label="页面打开超时（毫秒）" name="goto_timeout_ms" rules={[{ required: true }]}>
            <InputNumber min={1000} max={300000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="无头模式" name="headless" valuePropName="checked">
            <Switch />
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
