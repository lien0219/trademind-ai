import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { DollarOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import { Button, Col, Divider, Form, InputNumber, Row, Select, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  MARKUP_TYPE_OPTIONS,
  PRICING_CURRENCY_OPTIONS,
  PRICING_PLATFORM_MARKUPS,
  ROUNDING_MODE_OPTIONS,
} from '@/constants/pricingSettings';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const { Paragraph, Text } = Typography;

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
    { key: 'default_markup_multiplier', val: String(parseNum(values.default_markup_multiplier, 1.5)) },
    { key: 'default_shipping_cost', val: String(parseNum(values.default_shipping_cost, 0)) },
    { key: 'default_shipping_cost_per_weight', val: String(parseNum(values.default_shipping_cost_per_weight, 0)) },
    { key: 'default_platform_commission_percent', val: String(parseNum(values.default_platform_commission_percent, 0)) },
    { key: 'default_min_profit', val: String(parseNum(values.default_min_profit, 0)) },
    { key: 'default_exchange_rate', val: String(parseNum(values.default_exchange_rate, 1)) },
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
  useLanguage();
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
        default_markup_multiplier: parseNum(g.default_markup_multiplier, 1.5),
        default_shipping_cost: parseNum(g.default_shipping_cost, 0),
        default_shipping_cost_per_weight: parseNum(g.default_shipping_cost_per_weight, 0),
        default_platform_commission_percent: parseNum(g.default_platform_commission_percent, 0),
        default_min_profit: parseNum(g.default_min_profit, 0),
        default_exchange_rate: parseNum(g.default_exchange_rate, 1),
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
      message.error((e as Error)?.message || tr("加载失败"));
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer title={tr("商品定价")} subTitle={tr("配置默认加价、尾数与平台覆盖规则，用于本地销售价计算。")}>
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <DollarOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">{tr("成本价 → 发布价")}</Typography.Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">{tr("采集价通常是成本价，不建议直接作为发布价。在此配置默认加价与尾数规则；在商品详情或列表「应用定价规则」后，仅更新本地商品规格的销售价，不会自动发布到平台店铺。")}</Paragraph>
            </div>
          </div>
        </ProCard>

        <Form
          form={form}
          layout="vertical"
          onFinish={async (vals) => {
            try {
              await saveSettingsItems(buildPutItems(vals));
              message.success(tr("已保存"));
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || tr("保存失败"));
            }
          }}
        >
          <ProCard
            variant="outlined"
            title={tr("默认定价规则")}
            className="tm-system-settings__panel"
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>{tr("重新加载")}</Button>
            }
          >
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={8}>
                <Form.Item name="default_markup_type" label={tr("加价方式")} rules={[{ required: true, message: tr("请选择加价方式") }]}>
                  <Select options={MARKUP_TYPE_OPTIONS.map((option) => ({ ...option, label: tr(option.label) }))} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item name="default_currency" label={tr("默认币种")}>
                  <Select options={PRICING_CURRENCY_OPTIONS.map((option) => ({ ...option, label: tr(option.label) }))} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item name="default_rounding_mode" label={tr("尾数规则")} extra={tr("例如 .99 使价格显示为 19.99")}>
                  <Select options={ROUNDING_MODE_OPTIONS.map((option) => ({ ...option, label: tr(option.label) }))} />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item noStyle shouldUpdate={(prev, next) => prev.default_markup_type !== next.default_markup_type}>
              {({ getFieldValue }) => {
                const markupType = String(getFieldValue('default_markup_type') || 'percent');
                return (
                  <Row gutter={[24, 0]}>
                    <Col xs={24} md={12} lg={8}>
                      <Form.Item
                        label={tr("加价比例（%）")}
                        name="default_markup_percent"
                        extra={markupType === 'percent' ? tr("在成本价基础上按百分比加价") : tr("仅「百分比加价」时生效")}
                      >
                        <InputNumber
                          min={0}
                          max={1000}
                          style={{ width: '100%' }}
                          placeholder="30"
                          disabled={markupType !== 'percent'}
                        />
                      </Form.Item>
                    </Col>
                    <Col xs={24} md={12} lg={8}>
                      <Form.Item
                        label={tr("固定加价金额")}
                        name="default_markup_amount"
                        extra={markupType === 'fixed' ? tr("在成本价基础上加固定金额") : tr("仅「固定金额加价」时生效")}
                      >
                        <InputNumber
                          min={0}
                          style={{ width: '100%' }}
                          placeholder="0"
                          disabled={markupType !== 'fixed'}
                        />
                      </Form.Item>
                    </Col>
                    <Col xs={24} md={12} lg={8}>
                      <Form.Item
                        label={tr("加价倍率")}
                        name="default_markup_multiplier"
                        extra={markupType === 'multiplier' ? tr("例如 1.5 表示成本 x 1.5") : tr("仅「倍率加价」时生效")}
                      >
                        <InputNumber
                          min={0}
                          step={0.1}
                          precision={2}
                          style={{ width: '100%' }}
                          placeholder="1.5"
                          disabled={markupType !== 'multiplier'}
                        />
                      </Form.Item>
                    </Col>
                    <Col xs={24} md={12} lg={8}>
                      <Form.Item
                        label={tr("最低利润率（%）")}
                        name="default_min_margin_percent"
                        tooltip={tr("用于校验或提示；具体行为取决于定价引擎实现")}
                      >
                        <InputNumber min={0} max={1000} style={{ width: '100%' }} placeholder="10" />
                      </Form.Item>
                    </Col>
                  </Row>
                );
              }}
            </Form.Item>
            <Divider plain>{tr("成本、佣金与汇率")}</Divider>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label={tr("固定运费成本")} name="default_shipping_cost">
                  <InputNumber min={0} precision={2} style={{ width: '100%' }} placeholder="0" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label={tr("按重量运费单价")} name="default_shipping_cost_per_weight">
                  <InputNumber min={0} precision={2} style={{ width: '100%' }} placeholder="0" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label={tr("平台佣金（%）")} name="default_platform_commission_percent">
                  <InputNumber min={0} max={95} precision={2} style={{ width: '100%' }} placeholder="0" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label={tr("最低利润保护")} name="default_min_profit">
                  <InputNumber min={0} precision={2} style={{ width: '100%' }} placeholder="0" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label={tr("汇率（CNY → 目标币种）")} name="default_exchange_rate">
                  <InputNumber min={0.0001} precision={6} style={{ width: '100%' }} placeholder="1" />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title={tr("平台覆盖规则")} className="tm-system-settings__panel">
            <Form.Item
              label={tr("启用平台覆盖")}
              name="enable_platform_pricing_rules"
              valuePropName="checked"
              extra={tr("开启后，各平台可单独设置加价比例，覆盖默认定价")}
            >
              <Switch />
            </Form.Item>
            <Form.Item
              noStyle
              shouldUpdate={(prev, next) => prev.enable_platform_pricing_rules !== next.enable_platform_pricing_rules}
            >
              {({ getFieldValue }) => {
                const platformRulesEnabled = !!getFieldValue('enable_platform_pricing_rules');
                return (
                  <>
                    <Divider plain>{tr("平台加价比例（%）")}</Divider>
                    <Row gutter={[16, 0]}>
                      {PRICING_PLATFORM_MARKUPS.map((item) => (
                        <Col xs={24} sm={12} md={6} key={item.name}>
                          <Form.Item label={tr(item.label)} name={item.name}>
                            <InputNumber
                              min={0}
                              style={{ width: '100%' }}
                              placeholder="30"
                              disabled={!platformRulesEnabled}
                            />
                          </Form.Item>
                        </Col>
                      ))}
                    </Row>
                    {!platformRulesEnabled ? (
                      <Text type="secondary" style={{ display: 'block', marginTop: -8, fontSize: 12 }}>{tr("启用平台覆盖后可配置各平台加价比例")}</Text>
                    ) : null}
                  </>
                );
              }}
            </Form.Item>
          </ProCard>

          <ProCard variant="outlined" title={tr("批量操作")} className="tm-system-settings__panel">
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={8}>
                <Form.Item
                  label={tr("单次批量定价上限")}
                  name="batch_max_size"
                  extra={tr("商品列表批量「应用定价规则」时，单次最多处理的规格数量")}
                >
                  <InputNumber min={1} max={5000} style={{ width: '100%' }} placeholder="500" />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer">
            <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>{tr("保存配置")}</Button>
          </ProCard>
        </Form>
      </div>
    </TmPageContainer>
  );
}
