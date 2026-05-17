import { InfoCircleOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, InputNumber, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

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
  ];
  return rows;
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

  return (
    <PageContainer title="库存 / 订单">
      <ProCard bordered style={{ marginBottom: 16 }}>
        <Alert
          showIcon
          type="info"
          icon={<InfoCircleOutlined />}
          message="库存策略仅影响贸灵本地 SKU（product_skus）"
          description={
            <>
              平台订单同步后再按策略尝试扣库；扣库失败或跳过<strong>不回滚平台侧数据</strong>。
              审计：<Typography.Link href="/inventory/effects">订单库存影响</Typography.Link>
              {' · '}
              <Typography.Link href="/inventory/logs">全局库存流水</Typography.Link>
              {' · '}
              <Typography.Link href="/inventory/alerts">库存预警</Typography.Link>。
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
          style={{ maxWidth: 640 }}
          onFinish={async (values) => {
            try {
              await saveSettingsItems(buildPutItems(values));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Alert
            showIcon
            type="info"
            style={{ marginBottom: 16 }}
            message="库存预警默认值说明"
            description="下列「默认预警 / 安全线」只影响新创建的 SKU，不会批量改写已有 SKU；已有行的预警线请在商品详情「库存」或「库存预警」中单独设置（后续可再做批量工具）。"
          />
          <Typography.Title level={5}>库存预警（只读查询策略）</Typography.Title>
          <Form.Item label="新建 SKU 默认预警线（warning_stock）" name="default_warning_stock">
            <InputNumber min={0} style={{ width: '100%', maxWidth: 280 }} />
          </Form.Item>
          <Form.Item label="新建 SKU 默认安全线（safety_stock）" name="default_safety_stock">
            <InputNumber min={0} style={{ width: '100%', maxWidth: 280 }} />
          </Form.Item>
          <Form.Item label="启用库存预警能力" name="enable_inventory_alerts" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="售罄类预警（out_of_stock）" name="alert_out_of_stock" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="提示平台库存与本地不一致" name="alert_platform_stock_mismatch" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            label="平台库存与本地允许差异阈值（大于此值视为不一致；0 表示必须完全一致）"
            name="platform_stock_mismatch_threshold"
          >
            <InputNumber min={0} style={{ width: '100%', maxWidth: 280 }} />
          </Form.Item>

          <Typography.Title level={5}>订单与库存联动</Typography.Title>
          <Form.Item label="平台订单入库后自动尝试 SKU 匹配" name="auto_match_order_skus" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Typography.Paragraph type="secondary">
            匹配走刊登 external_sku_id / sku_code / 本地 sku_code；失败不落单失败，详见订单「SKU 匹配」与{' '}
            <Typography.Link href="/orders/sku-matches">全局匹配记录</Typography.Link>。
          </Typography.Paragraph>

          <Form.Item
            label="平台订单：SKU 匹配成功后允许自动扣库存（仍需打开「平台同步订单到达后自动扣库存」）"
            name="auto_deduct_after_sku_match"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Typography.Paragraph type="secondary">
            默认关闭，避免未核对映射时误扣。打开后仅当平台同步策略允许扣库且行已绑定本地 SKU 时生效。
          </Typography.Paragraph>

          <Form.Item
            label="手工订单：新建时默认自动扣库存"
            name="auto_deduct_manual_orders"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Typography.Paragraph type="secondary">
            与「新建订单」弹窗内的「创建后扣库存」并联：任一为真则创建后会尝试扣减。
          </Typography.Paragraph>

          <Form.Item label="平台同步订单到达后自动扣库存" name="auto_deduct_platform_orders" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item label="订单取消 / 关闭时尝试自动回滚库存" name="auto_restore_cancelled_orders" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item
            label="扣库成功后入队平台库存同步任务（需刊登与库存同步 Worker）"
            name="inventory_sync_after_deduct"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Typography.Paragraph type="secondary">
            写入 <code>auto_sync_inventory_after_order_deduct</code> 并与旧键 <code>auto_sync_platform_inventory_after_deduct</code>{' '}
            同步，后端读取时优先新键。
          </Typography.Paragraph>

          <Form.Item
            label="已有成功扣库记录时，仍允许人工绑定未匹配行并再扣（策略校验）"
            name="allow_manual_sku_bind_after_deduct"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item label="允许 SKU 本地库存扣成负数" name="allow_negative_stock" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Button type="primary" htmlType="submit">
            保存
          </Button>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
