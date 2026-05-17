import { InfoCircleOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const GROUP = 'inventory';

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
  const rows: SettingPutItem[] = [
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
      form.setFieldsValue({
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
              <Typography.Link href="/inventory/logs">全局库存流水</Typography.Link>。
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
