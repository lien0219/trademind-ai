import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Button, Form, Input, InputNumber, Select, Switch, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const GROUP_SYSTEM = 'system';
const GROUP_TC = 'taskcenter';

function truthyStored(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function buildTCItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const gk = GROUP_TC;
  const boolStr = (b: unknown) => (b ? 'true' : 'false');
  return [
    { tenantId, groupKey: gk, itemKey: 'enable_task_alerts', itemValue: boolStr(values.enable_task_alerts), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_min_severity', itemValue: String(values.alert_min_severity ?? 'high'), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_platform_permission', itemValue: boolStr(values.alert_on_platform_permission), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_platform_config', itemValue: boolStr(values.alert_on_platform_config), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_inventory_mapping_missing', itemValue: boolStr(values.alert_on_inventory_mapping_missing), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_worker_lease_expired', itemValue: boolStr(values.alert_on_worker_lease_expired), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_repeated_failures', itemValue: boolStr(values.alert_on_repeated_failures), valueType: 'string', isEncrypted: false, remark: '' },
    {
      tenantId,
      groupKey: gk,
      itemKey: 'repeated_failure_threshold',
      itemValue: String(values.repeated_failure_threshold ?? '3'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: gk,
      itemKey: 'repeated_failure_window_minutes',
      itemValue: String(values.repeated_failure_window_minutes ?? '60'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
  ];
}

export default function SystemSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      form.setFieldsValue({
        ...pickGroup(items, GROUP_SYSTEM),
      });
      const tc = pickGroup(items, GROUP_TC);
      form.setFieldsValue({
        enable_task_alerts:
          tc.enable_task_alerts === '' || tc.enable_task_alerts === undefined ? true : truthyStored(tc.enable_task_alerts),
        alert_min_severity: tc.alert_min_severity || 'high',
        alert_on_platform_permission:
          tc.alert_on_platform_permission === '' ? true : truthyStored(tc.alert_on_platform_permission),
        alert_on_platform_config: tc.alert_on_platform_config === '' ? true : truthyStored(tc.alert_on_platform_config),
        alert_on_inventory_mapping_missing:
          tc.alert_on_inventory_mapping_missing === ''
            ? true
            : truthyStored(tc.alert_on_inventory_mapping_missing),
        alert_on_worker_lease_expired:
          tc.alert_on_worker_lease_expired === '' ? true : truthyStored(tc.alert_on_worker_lease_expired),
        alert_on_repeated_failures:
          tc.alert_on_repeated_failures === '' ? true : truthyStored(tc.alert_on_repeated_failures),
        repeated_failure_threshold: parseInt(String(tc.repeated_failure_threshold ?? '3'), 10) || 3,
        repeated_failure_window_minutes: parseInt(String(tc.repeated_failure_window_minutes ?? '60'), 10) || 60,
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
    <PageContainer title="系统设置">
      <ProCard
        title="站点"
        bordered
        extra={
          <Button type="link" onClick={load} disabled={loading}>
            重新加载
          </Button>
        }
        style={{ marginBottom: 16 }}
      >
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 560 }}
          onFinish={async (values) => {
            try {
              const sysPut: SettingPutItem[] = [
                {
                  tenantId: 0,
                  groupKey: GROUP_SYSTEM,
                  itemKey: 'site_name',
                  itemValue: String(values.site_name ?? ''),
                  valueType: 'string',
                  isEncrypted: false,
                  remark: '',
                },
                {
                  tenantId: 0,
                  groupKey: GROUP_SYSTEM,
                  itemKey: 'timezone',
                  itemValue: String(values.timezone ?? ''),
                  valueType: 'string',
                  isEncrypted: false,
                  remark: '',
                },
              ];
              await saveSettingsItems(sysPut.concat(buildTCItems(values)));
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
          <ProCard title="任务中心 · 站内告警策略" bordered type="inner" style={{ marginTop: 8 }}>
            <Form.Item label="自动生成站内告警" name="enable_task_alerts" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="最低告警等级" name="alert_min_severity">
              <Select
                options={[
                  { value: 'low', label: 'low' },
                  { value: 'medium', label: 'medium' },
                  { value: 'high', label: 'high' },
                  { value: 'critical', label: 'critical' },
                ]}
              />
            </Form.Item>
            <Form.Item label="平台权限失败默认告警" name="alert_on_platform_permission" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="平台配置不完整告警" name="alert_on_platform_config" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="库存映射缺失告警" name="alert_on_inventory_mapping_missing" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="租约过期类告警" name="alert_on_worker_lease_expired" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="开启重复失败统计" name="alert_on_repeated_failures" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="重复失败阈值（retry_count ≥）" name="repeated_failure_threshold">
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item label="统计时间窗（分钟）" name="repeated_failure_window_minutes">
              <InputNumber min={5} style={{ width: '100%' }} />
            </Form.Item>
          </ProCard>
          <Form.Item style={{ marginTop: 16 }}>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
