import { Link } from '@umijs/renderer-react';
import { InboxOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Col, Divider, Form, InputNumber, Row, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  INVENTORY_ALERT_TOGGLES,
  INVENTORY_ORDER_TOGGLES,
  INVENTORY_PLATFORM_RATE_LIMITS,
} from '@/constants/inventorySettings';
import { INVENTORY_SYNC_BATCH_MAX_SIZE_LABEL } from '@/constants/userFriendly';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const { Paragraph, Text } = Typography;

const GROUP = 'inventory';

function parseIntField(raw: unknown, fallback: number): number {
  const n = typeof raw === 'number' ? raw : parseInt(String(raw ?? ''), 10);
  if (Number.isNaN(n) || n < 0) return fallback;
  return n;
}

function truthyStored(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function buildPutItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const boolStr = (b: unknown) => (b ? 'true' : 'false');
  const syncAfter = boolStr(values.inventory_sync_after_deduct);
  const defWarn = parseIntField(values.default_warning_stock, 5);
  const defSafe = parseIntField(values.default_safety_stock, 0);
  const mismatchTh = parseIntField(values.platform_stock_mismatch_threshold, 0);
  const batchMax = parseIntField(values.inventory_sync_batch_max_size, 500);
  const batchWarn = parseIntField(values.inventory_stock_settings_batch_max_size, 500);
  const rlTiktok = parseIntField(values.inventory_sync_platform_rate_limit_per_minute_tiktok, 60);
  const rlShopee = parseIntField(values.inventory_sync_platform_rate_limit_per_minute_shopee, 60);
  const rlLazada = parseIntField(values.inventory_sync_platform_rate_limit_per_minute_lazada, 60);
  const rlAmazon = parseIntField(values.inventory_sync_platform_rate_limit_per_minute_amazon, 30);
  const rows: SettingPutItem[] = [
    { tenantId, groupKey: GROUP, itemKey: 'default_warning_stock', itemValue: String(defWarn), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'default_safety_stock', itemValue: String(defSafe), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'enable_inventory_alerts', itemValue: boolStr(values.enable_inventory_alerts), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'alert_out_of_stock', itemValue: boolStr(values.alert_out_of_stock), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'alert_platform_stock_mismatch', itemValue: boolStr(values.alert_platform_stock_mismatch), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'platform_stock_mismatch_threshold', itemValue: String(mismatchTh), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_match_order_skus', itemValue: boolStr(values.auto_match_order_skus), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_deduct_after_sku_match', itemValue: boolStr(values.auto_deduct_after_sku_match), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_deduct_manual_orders', itemValue: boolStr(values.auto_deduct_manual_orders), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_deduct_platform_orders', itemValue: boolStr(values.auto_deduct_platform_orders), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_restore_cancelled_orders', itemValue: boolStr(values.auto_restore_cancelled_orders), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_sync_inventory_after_order_deduct', itemValue: syncAfter, valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'auto_sync_platform_inventory_after_deduct', itemValue: syncAfter, valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'allow_manual_sku_bind_after_deduct', itemValue: boolStr(values.allow_manual_sku_bind_after_deduct), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'allow_negative_stock', itemValue: boolStr(values.allow_negative_stock), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: GROUP, itemKey: 'inventory_sync_batch_max_size', itemValue: String(batchMax), valueType: 'string', isEncrypted: false, remark: '' },
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'inventory_stock_settings_batch_max_size',
      itemValue: String(batchWarn),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'inventory_sync_platform_rate_limit_enabled',
      itemValue: boolStr(values.inventory_sync_platform_rate_limit_enabled),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'inventory_sync_platform_rate_limit_per_minute_tiktok',
      itemValue: String(rlTiktok),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'inventory_sync_platform_rate_limit_per_minute_shopee',
      itemValue: String(rlShopee),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'inventory_sync_platform_rate_limit_per_minute_lazada',
      itemValue: String(rlLazada),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'inventory_sync_platform_rate_limit_per_minute_amazon',
      itemValue: String(rlAmazon),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
  ];
  return rows;
}

function SettingsToggleCard({
  name,
  label,
  extra,
  link,
}: {
  name: string;
  label: string;
  extra: string;
  link?: { href: string; text: string };
}) {
  return (
    <div className="tm-system-settings__toggle-card">
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Text className="tm-system-settings__toggle-label">{label}</Text>
          <Text type="secondary" className="tm-system-settings__toggle-extra">
            {extra}
            {link ? (
              <>
                {' '}
                <Link to={link.href}>{link.text}</Link>
              </>
            ) : null}
          </Text>
        </div>
        <Form.Item name={name} valuePropName="checked" style={{ marginBottom: 0, flexShrink: 0 }}>
          <Switch />
        </Form.Item>
      </div>
    </div>
  );
}

export default function InventorySettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      const syncAfter =
        truthyStored(g.auto_sync_inventory_after_order_deduct) ||
        truthyStored(g.auto_sync_platform_inventory_after_deduct);
      const parseN = (s: string | undefined, d: number) => {
        const n = parseInt(String(s ?? '').trim(), 10);
        return Number.isNaN(n) || n < 0 ? d : n;
      };
      form.setFieldsValue({
        default_warning_stock: parseN(g.default_warning_stock, 5),
        default_safety_stock: parseN(g.default_safety_stock, 0),
        enable_inventory_alerts: truthyStored(g.enable_inventory_alerts) || g.enable_inventory_alerts === '' || g.enable_inventory_alerts === undefined,
        alert_out_of_stock: truthyStored(g.alert_out_of_stock) || g.alert_out_of_stock === '' || g.alert_out_of_stock === undefined,
        alert_platform_stock_mismatch:
          truthyStored(g.alert_platform_stock_mismatch) ||
          g.alert_platform_stock_mismatch === '' ||
          g.alert_platform_stock_mismatch === undefined,
        platform_stock_mismatch_threshold: parseN(g.platform_stock_mismatch_threshold, 0),
        auto_match_order_skus:
          g.auto_match_order_skus === '' ? true : truthyStored(g.auto_match_order_skus),
        auto_deduct_after_sku_match: truthyStored(g.auto_deduct_after_sku_match),
        auto_deduct_manual_orders: truthyStored(g.auto_deduct_manual_orders),
        auto_deduct_platform_orders: truthyStored(g.auto_deduct_platform_orders),
        auto_restore_cancelled_orders:
          g.auto_restore_cancelled_orders === '' ? true : truthyStored(g.auto_restore_cancelled_orders),
        inventory_sync_after_deduct: syncAfter,
        allow_manual_sku_bind_after_deduct:
          g.allow_manual_sku_bind_after_deduct === '' ? true : truthyStored(g.allow_manual_sku_bind_after_deduct),
        allow_negative_stock: truthyStored(g.allow_negative_stock),
        inventory_sync_batch_max_size: parseN(g.inventory_sync_batch_max_size, 500),
        inventory_stock_settings_batch_max_size: parseN(g.inventory_stock_settings_batch_max_size, 500),
        inventory_sync_platform_rate_limit_enabled:
          g.inventory_sync_platform_rate_limit_enabled === ''
            ? true
            : truthyStored(g.inventory_sync_platform_rate_limit_enabled),
        inventory_sync_platform_rate_limit_per_minute_tiktok: parseN(g.inventory_sync_platform_rate_limit_per_minute_tiktok, 60),
        inventory_sync_platform_rate_limit_per_minute_shopee: parseN(g.inventory_sync_platform_rate_limit_per_minute_shopee, 60),
        inventory_sync_platform_rate_limit_per_minute_lazada: parseN(g.inventory_sync_platform_rate_limit_per_minute_lazada, 60),
        inventory_sync_platform_rate_limit_per_minute_amazon: parseN(g.inventory_sync_platform_rate_limit_per_minute_amazon, 30),
      });
      if (!Object.keys(g).length) {
        form.setFieldsValue({
          default_warning_stock: 5,
          default_safety_stock: 0,
          enable_inventory_alerts: true,
          alert_out_of_stock: true,
          alert_platform_stock_mismatch: true,
          platform_stock_mismatch_threshold: 0,
          auto_match_order_skus: true,
          auto_deduct_after_sku_match: false,
          auto_deduct_manual_orders: false,
          auto_deduct_platform_orders: false,
          auto_restore_cancelled_orders: true,
          inventory_sync_after_deduct: false,
          allow_manual_sku_bind_after_deduct: true,
          allow_negative_stock: false,
          inventory_sync_batch_max_size: 500,
          inventory_stock_settings_batch_max_size: 500,
          inventory_sync_platform_rate_limit_enabled: true,
          inventory_sync_platform_rate_limit_per_minute_tiktok: 60,
          inventory_sync_platform_rate_limit_per_minute_shopee: 60,
          inventory_sync_platform_rate_limit_per_minute_lazada: 60,
          inventory_sync_platform_rate_limit_per_minute_amazon: 30,
        });
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  const onFinish = async (values: Record<string, unknown>) => {
    try {
      await saveSettingsItems(buildPutItems(values));
      message.success('已保存');
      await load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存失败');
    }
  };

  return (
    <PageContainer title="库存 / 订单" subTitle="本地库存预警、订单扣减与平台库存同步策略">
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <InboxOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">
                本地库存与订单联动
              </Typography.Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                策略仅影响本地商品规格库存。平台订单同步后再按规则尝试扣减；扣库失败或跳过不会自动回滚平台侧数据。相关审计：
                <Link to="/inventory/effects"> 订单库存影响</Link>、
                <Link to="/inventory/logs"> 库存流水</Link>、
                <Link to="/inventory/alerts"> 库存预警</Link>。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <Form form={form} layout="vertical" onFinish={onFinish}>
          <ProCard
            variant="outlined"
            title="库存预警"
            className="tm-system-settings__panel"
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Alert
              showIcon
              type="info"
              style={{ marginBottom: 16 }}
              message="默认值说明"
              description="「默认预警线 / 安全线」只影响新创建的商品规格，不会批量改写已有规格；已有规格可在商品详情或「库存预警」中单独设置。"
            />
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label="新建规格默认预警线" name="default_warning_stock" extra="库存低于此值触发预警">
                  <InputNumber min={0} style={{ width: '100%' }} placeholder="5" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label="新建规格默认安全线" name="default_safety_stock" extra="0 表示不设置安全库存下限">
                  <InputNumber min={0} style={{ width: '100%' }} placeholder="0" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item
                  label="平台库存差异阈值"
                  name="platform_stock_mismatch_threshold"
                  tooltip="平台与本地库存差值超过此数视为不一致；0 表示必须完全一致"
                >
                  <InputNumber min={0} style={{ width: '100%' }} placeholder="0" />
                </Form.Item>
              </Col>
            </Row>
            <Divider plain>预警开关</Divider>
            <Row gutter={[12, 12]}>
              {INVENTORY_ALERT_TOGGLES.map((item) => (
                <Col xs={24} sm={12} lg={8} key={item.name}>
                  <SettingsToggleCard name={item.name} label={item.label} extra={item.extra} />
                </Col>
              ))}
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="订单与库存联动" className="tm-system-settings__panel">
            <Row gutter={[12, 12]}>
              {INVENTORY_ORDER_TOGGLES.map((item) => (
                <Col xs={24} sm={12} lg={8} key={item.name}>
                  <SettingsToggleCard
                    name={item.name}
                    label={item.label}
                    extra={item.extra}
                    link={'link' in item ? item.link : undefined}
                  />
                </Col>
              ))}
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="批量同步与限流" className="tm-system-settings__panel">
            <Paragraph type="secondary" style={{ marginBottom: 16 }}>
              控制单次批量任务数量与各平台每分钟请求上限；超限时任务会稍后自动重试。
            </Paragraph>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label={INVENTORY_SYNC_BATCH_MAX_SIZE_LABEL} name="inventory_sync_batch_max_size">
                  <InputNumber min={1} max={5000} style={{ width: '100%' }} placeholder="500" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item
                  label="单次批量设置预警线上限"
                  name="inventory_stock_settings_batch_max_size"
                  extra="仅限制批量修改预警线 / 安全线，与上方库存同步批量无关"
                >
                  <InputNumber min={1} max={5000} style={{ width: '100%' }} placeholder="500" />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="启用平台节流" name="inventory_sync_platform_rate_limit_enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item
              noStyle
              shouldUpdate={(prev, next) =>
                prev.inventory_sync_platform_rate_limit_enabled !== next.inventory_sync_platform_rate_limit_enabled
              }
            >
              {({ getFieldValue }) => {
                const rateLimitEnabled = !!getFieldValue('inventory_sync_platform_rate_limit_enabled');
                return (
                  <>
                    <Row gutter={[16, 0]}>
                      {INVENTORY_PLATFORM_RATE_LIMITS.map((item) => (
                        <Col xs={24} sm={12} md={6} key={item.name}>
                          <Form.Item label={`${item.label} 每分钟配额`} name={item.name}>
                            <InputNumber min={1} style={{ width: '100%' }} disabled={!rateLimitEnabled} placeholder="60" />
                          </Form.Item>
                        </Col>
                      ))}
                    </Row>
                    {!rateLimitEnabled ? (
                      <Text type="secondary" style={{ display: 'block', marginTop: -8, fontSize: 12 }}>
                        启用平台节流后可配置各平台每分钟配额
                      </Text>
                    ) : null}
                  </>
                );
              }}
            </Form.Item>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer">
            <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
              保存配置
            </Button>
          </ProCard>
        </Form>
      </div>
    </PageContainer>
  );
}
