import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Button, Form, Input, InputNumber, message, Switch } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'collector';

const FIELDS: Record<string, FieldSpec> = {
  main_service_url: {},
  collector_http_addr: {},
  goto_timeout_ms: {},
  headless: {},
  collect_batch_concurrency_1688: {},
  collect_batch_delay_min_ms_1688: {},
  collect_batch_delay_max_ms_1688: {},
  collect_batch_retry_on_blocked: {},
  collect_batch_retry_on_timeout: {},
  collect_batch_max_retries_1688: {},
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
        collect_batch_concurrency_1688: g.collect_batch_concurrency_1688
          ? Number(g.collect_batch_concurrency_1688)
          : 1,
        collect_batch_delay_min_ms_1688: g.collect_batch_delay_min_ms_1688
          ? Number(g.collect_batch_delay_min_ms_1688)
          : 1500,
        collect_batch_delay_max_ms_1688: g.collect_batch_delay_max_ms_1688
          ? Number(g.collect_batch_delay_max_ms_1688)
          : 5000,
        collect_batch_retry_on_blocked:
          g.collect_batch_retry_on_blocked === undefined ||
          g.collect_batch_retry_on_blocked === '' ||
          g.collect_batch_retry_on_blocked === '1' ||
          g.collect_batch_retry_on_blocked === 'true',
        collect_batch_retry_on_timeout:
          g.collect_batch_retry_on_timeout === undefined ||
          g.collect_batch_retry_on_timeout === '' ||
          g.collect_batch_retry_on_timeout === '1' ||
          g.collect_batch_retry_on_timeout === 'true',
        collect_batch_max_retries_1688: g.collect_batch_max_retries_1688
          ? Number(g.collect_batch_max_retries_1688)
          : 2,
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
    <PageContainer title="采集服务">
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
                collect_batch_concurrency_1688: String(values.collect_batch_concurrency_1688 ?? 1),
                collect_batch_delay_min_ms_1688: String(values.collect_batch_delay_min_ms_1688 ?? 1500),
                collect_batch_delay_max_ms_1688: String(values.collect_batch_delay_max_ms_1688 ?? 5000),
                collect_batch_retry_on_blocked: values.collect_batch_retry_on_blocked ? '1' : '0',
                collect_batch_retry_on_timeout: values.collect_batch_retry_on_timeout ? '1' : '0',
                collect_batch_max_retries_1688: String(values.collect_batch_max_retries_1688 ?? 2),
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
          >
            <Input placeholder="http://127.0.0.1:8080" />
          </Form.Item>
          <Form.Item
            label="采集服务监听地址"
            name="collector_http_addr"
            rules={[{ required: true }]}
          >
            <Input placeholder=":3100" />
          </Form.Item>
          <Form.Item label="页面打开超时（毫秒）" name="goto_timeout_ms" rules={[{ required: true }]}>
            <InputNumber min={1000} max={300000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="无头模式" name="headless" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            label="1688 批量并发上限"
            name="collect_batch_concurrency_1688"
            tooltip="仅批量采集生效；建议 1–2，过高易触发 1688 风控导致整批失败。"
          >
            <InputNumber min={1} max={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="1688 批量随机间隔最小（毫秒）" name="collect_batch_delay_min_ms_1688">
            <InputNumber min={0} max={120000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="1688 批量随机间隔最大（毫秒）" name="collect_batch_delay_max_ms_1688">
            <InputNumber min={0} max={120000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="批量遇风控/验证页时自动重试" name="collect_batch_retry_on_blocked" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="批量遇超时/导航失败时自动重试" name="collect_batch_retry_on_timeout" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="1688 批量任务最大自动重试次数" name="collect_batch_max_retries_1688">
            <InputNumber min={0} max={5} style={{ width: '100%' }} />
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
