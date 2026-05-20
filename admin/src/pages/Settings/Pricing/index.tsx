import { InfoCircleOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, InputNumber, Select, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const GROUP = 'pricing';

function parseNum(raw: unknown, fallback: number): number {
  const n = typeof raw === 'number' ? raw : parseFloat(String(raw ?? ''));
  if (Number.isNaN(n)) return fallback;
  return n;
}

function boolStr(b: unknown): string {
  return b ? 'true' : 'false';
}

function buildPutItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const rows: Array<{ key: string; val: string }> = [
    { key: 'default_markup_type', val: String(values.default_markup_type ?? 'percent') },
    { key: 'default_markup_percent', val: String(parseNum(values.default_markup_percent, 30)) },
    { key: 'default_markup_amount', val: String(parseNum(values.default_markup_amount, 0)) },
    { key: 'default_rounding_mode', val: String(values.default_rounding_mode ?? '.99') },
    { key: 'default_min_margin_percent', val: String(parseNum(values.default_min_margin_percent, 10)) },
    { key: 'default_currency', val: String(values.default_currency ?? 'CNY') },
    { key: 'enable_platform_pricing_rules', val: boolStr(values.enable_platform_pricing_rules) },
    { key: 'tiktok_markup_percent', val: String(parseNum(values.tiktok_markup_percent, 30)) },
    { key: 'shopee_markup_percent', val: String(parseNum(values.shopee_markup_percent, 30)) },
    { key: 'lazada_markup_percent', val: String(parseNum(values.lazada_markup_percent, 30)) },
    { key: 'amazon_markup_percent', val: String(parseNum(values.amazon_markup_percent, 30)) },
    { key: 'batch_max_size', val: String(parseNum(values.batch_max_size, 500)) },
  ];
  return rows.map((r) => ({
    tenantId,
    groupKey: GROUP,
    itemKey: r.key,
    itemValue: r.val,
    valueType: 'string',
    isEncrypted: false,
    remark: '',
  }));
}

export default function PricingSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        default_markup_type: g.default_markup_type ?? 'percent',
        default_markup_percent: parseNum(g.default_markup_percent, 30),
        default_markup_amount: parseNum(g.default_markup_amount, 0),
        default_rounding_mode: g.default_rounding_mode ?? '.99',
        default_min_margin_percent: parseNum(g.default_min_margin_percent, 10),
        default_currency: g.default_currency ?? 'CNY',
        enable_platform_pricing_rules:
          String(g.enable_platform_pricing_rules ?? 'true').toLowerCase() === 'true',
        tiktok_markup_percent: parseNum(g.tiktok_markup_percent, 30),
        shopee_markup_percent: parseNum(g.shopee_markup_percent, 30),
        lazada_markup_percent: parseNum(g.lazada_markup_percent, 30),
        amazon_markup_percent: parseNum(g.amazon_markup_percent, 30),
        batch_max_size: parseNum(g.batch_max_size, 500),
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <PageContainer title="商品定价 / 发布价格配置" loading={loading}>
      <Alert
        type="info"
        showIcon
        icon={<InfoCircleOutlined />}
        style={{ marginBottom: 16 }}
        message="采集价通常是成本价，不建议直接作为发布价"
        description={
          <Typography.Paragraph style={{ marginBottom: 0 }}>
            在此配置默认加价与尾数规则。在商品详情或列表「应用定价规则」后，仅更新本地 SKU 销售价（
            <Typography.Text code>product_skus.price</Typography.Text>
            ），不会自动发布到平台，也不会调用平台 API。
          </Typography.Paragraph>
        }
      />
      <ProCard>
        <Form form={form} layout="vertical" onFinish={async (vals) => {
          try {
            await saveSettingsItems(buildPutItems(vals));
            message.success('已保存');
            await load();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '保存失败');
          }
        }}>
          <Form.Item name="default_markup_type" label="默认加价方式" rules={[{ required: true }]}>
            <Select
              options={[
                { label: '百分比加价', value: 'percent' },
                { label: '固定金额加价', value: 'fixed' },
                { label: '不加价', value: 'none' },
              ]}
            />
          </Form.Item>
          <Form.Item name="default_markup_percent" label="默认加价比例（%）">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="default_markup_amount" label="默认固定加价金额">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="default_rounding_mode" label="默认尾数规则">
            <Select
              options={[
                { label: '不处理', value: 'none' },
                { label: '取整', value: 'integer' },
                { label: '.9', value: '.9' },
                { label: '.99', value: '.99' },
                { label: '.95', value: '.95' },
              ]}
            />
          </Form.Item>
          <Form.Item name="default_min_margin_percent" label="默认最低利润率（%）">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="default_currency" label="默认币种">
            <Select
              options={[
                { label: 'CNY', value: 'CNY' },
                { label: 'USD', value: 'USD' },
                { label: 'EUR', value: 'EUR' },
              ]}
            />
          </Form.Item>
          <Form.Item name="enable_platform_pricing_rules" label="启用平台覆盖规则" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Typography.Title level={5}>平台加价比例（%）</Typography.Title>
          <Form.Item name="tiktok_markup_percent" label="TikTok">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="shopee_markup_percent" label="Shopee">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="lazada_markup_percent" label="Lazada">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="amazon_markup_percent" label="Amazon">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="batch_max_size" label="单次批量定价最多 SKU 数">
            <InputNumber min={1} max={5000} style={{ width: '100%' }} />
          </Form.Item>
          <Button type="primary" htmlType="submit">
            保存
          </Button>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
