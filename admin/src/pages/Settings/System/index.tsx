import { ReloadOutlined, SaveOutlined, SettingOutlined } from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import { Alert, Button, Col, Divider, Form, Input, InputNumber, Row, Select, Switch, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  ALERT_SEVERITY_OPTIONS,
  SYSTEM_TIMEZONE_OPTIONS,
  TASK_ALERT_CATEGORY_TOGGLES,
} from '@/constants/systemSettings';
import { fetchSettingsList, saveSettingsItems, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const { Text, Paragraph } = Typography;

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
    { tenantId, groupKey: gk, itemKey: 'alert_min_severity', itemValue: String(values.alert_min_severity ?? ''), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_platform_permission', itemValue: boolStr(values.alert_on_platform_permission), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_platform_config', itemValue: boolStr(values.alert_on_platform_config), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_inventory_mapping_missing', itemValue: boolStr(values.alert_on_inventory_mapping_missing), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_worker_lease_expired', itemValue: boolStr(values.alert_on_worker_lease_expired), valueType: 'string', isEncrypted: false, remark: '' },
    { tenantId, groupKey: gk, itemKey: 'alert_on_repeated_failures', itemValue: boolStr(values.alert_on_repeated_failures), valueType: 'string', isEncrypted: false, remark: '' },
    {
      tenantId,
      groupKey: gk,
      itemKey: 'repeated_failure_threshold',
      itemValue: values.repeated_failure_threshold === undefined || values.repeated_failure_threshold === null || values.repeated_failure_threshold === ''
        ? ''
        : String(values.repeated_failure_threshold),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    {
      tenantId,
      groupKey: gk,
      itemKey: 'repeated_failure_window_minutes',
      itemValue:
        values.repeated_failure_window_minutes === undefined ||
        values.repeated_failure_window_minutes === null ||
        values.repeated_failure_window_minutes === ''
          ? ''
          : String(values.repeated_failure_window_minutes),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
    { tenantId, groupKey: gk, itemKey: 'enable_alert_scan_worker', itemValue: boolStr(values.enable_alert_scan_worker), valueType: 'string', isEncrypted: false, remark: '' },
    {
      tenantId,
      groupKey: gk,
      itemKey: 'alert_scan_interval_seconds',
      itemValue:
        values.alert_scan_interval_seconds === undefined ||
        values.alert_scan_interval_seconds === null ||
        values.alert_scan_interval_seconds === ''
          ? ''
          : String(values.alert_scan_interval_seconds),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
  ];
}

function AlertToggleItem({
  name,
  label,
  extra,
}: {
  name: (typeof TASK_ALERT_CATEGORY_TOGGLES)[number]['name'];
  label: string;
  extra: string;
}) {
  return (
    <div className="tm-system-settings__toggle-card">
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Text className="tm-system-settings__toggle-label">{label}</Text>
          <Text type="secondary" className="tm-system-settings__toggle-extra">
            {extra}
          </Text>
        </div>
        <Form.Item name={name} valuePropName="checked" style={{ marginBottom: 0, flexShrink: 0 }}>
          <Switch />
        </Form.Item>
      </div>
    </div>
  );
}

export default function SystemSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const timezoneValue = Form.useWatch('timezone', form);

  const timezoneOptions = useMemo(() => {
    if (!timezoneValue || SYSTEM_TIMEZONE_OPTIONS.some((o) => o.value === timezoneValue)) {
      return SYSTEM_TIMEZONE_OPTIONS;
    }
    return [{ label: `${timezoneValue}（当前）`, value: timezoneValue }, ...SYSTEM_TIMEZONE_OPTIONS];
  }, [timezoneValue]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      form.setFieldsValue({
        ...pickGroup(items, GROUP_SYSTEM),
      });
      const tc = pickGroup(items, GROUP_TC);
      form.setFieldsValue({
        enable_task_alerts: truthyStored(tc.enable_task_alerts),
        alert_min_severity: tc.alert_min_severity || undefined,
        alert_on_platform_permission: truthyStored(tc.alert_on_platform_permission),
        alert_on_platform_config: truthyStored(tc.alert_on_platform_config),
        alert_on_inventory_mapping_missing: truthyStored(tc.alert_on_inventory_mapping_missing),
        alert_on_worker_lease_expired: truthyStored(tc.alert_on_worker_lease_expired),
        alert_on_repeated_failures: truthyStored(tc.alert_on_repeated_failures),
        repeated_failure_threshold:
          tc.repeated_failure_threshold === '' || tc.repeated_failure_threshold === undefined
            ? undefined
            : parseInt(String(tc.repeated_failure_threshold), 10) || undefined,
        repeated_failure_window_minutes:
          tc.repeated_failure_window_minutes === '' || tc.repeated_failure_window_minutes === undefined
            ? undefined
            : parseInt(String(tc.repeated_failure_window_minutes), 10) || undefined,
        enable_alert_scan_worker: truthyStored(tc.enable_alert_scan_worker),
        alert_scan_interval_seconds:
          tc.alert_scan_interval_seconds === '' || tc.alert_scan_interval_seconds === undefined
            ? undefined
            : parseInt(String(tc.alert_scan_interval_seconds), 10) || undefined,
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

  const onFinish = async (values: Record<string, unknown>) => {
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
  };

  return (
    <TmPageContainer title={PAGE_COPY.systemSettings.title} subTitle={PAGE_COPY.systemSettings.description}>
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <SettingOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">
                全局运行参数
              </Typography.Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                站点名称与时区用于后台展示与时间计算；任务中心告警策略决定哪些失败会自动写入站内告警列表。外部邮件 /
                Webhook 通知请在「告警通知配置」中单独设置。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <Form form={form} layout="vertical" onFinish={onFinish}>
          <ProCard
            variant="outlined"
            title="站点信息"
            className="tm-system-settings__panel"
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={10}>
                <Form.Item label="站点名称" name="site_name" rules={[{ required: true, message: '请输入站点名称' }]}>
                  <Input placeholder="贸灵 TradeMind" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={10}>
                <Form.Item
                  label="时区"
                  name="timezone"
                  rules={[{ required: true, message: '请选择时区' }]}
                  extra="用于任务时间、日志与告警展示；存库为 IANA 时区标识"
                >
                  <Select
                    showSearch
                    optionFilterProp="label"
                    placeholder="请选择时区"
                    options={timezoneOptions}
                  />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="任务中心 · 站内告警策略" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 20 }}
              message="告警生成规则"
              description="开启「自动生成站内告警」后，系统会根据下方分类开关与重复失败规则写入任务中心告警列表；未达最低等级的事件将被忽略。"
            />

            <Row gutter={[24, 16]}>
              <Col xs={24} md={12} lg={8}>
                <Form.Item label="自动生成站内告警" name="enable_task_alerts" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={8}>
                <Form.Item
                  label="最低告警等级"
                  name="alert_min_severity"
                  tooltip="留空表示不按等级过滤，仅依据下方分类开关与重复失败规则生成告警"
                >
                  <Select allowClear placeholder="未设置（不过滤等级）" options={ALERT_SEVERITY_OPTIONS} />
                </Form.Item>
              </Col>
            </Row>

            <Divider orientation="left" plain>
              分类告警开关
            </Divider>
            <Row gutter={[12, 12]}>
              {TASK_ALERT_CATEGORY_TOGGLES.map((item) => (
                <Col xs={24} sm={12} lg={6} key={item.name}>
                  <AlertToggleItem name={item.name} label={item.label} extra={item.extra} />
                </Col>
              ))}
            </Row>

            <Divider orientation="left" plain>
              重复失败统计
            </Divider>
            <Form.Item
              noStyle
              shouldUpdate={(prev, next) => prev.alert_on_repeated_failures !== next.alert_on_repeated_failures}
            >
              {({ getFieldValue }) => {
                const repeatedFailuresEnabled = !!getFieldValue('alert_on_repeated_failures');
                return (
                  <>
                    <Row gutter={[24, 0]} align="middle">
                      <Col xs={24} md={8} lg={6}>
                        <Form.Item label="开启重复失败统计" name="alert_on_repeated_failures" valuePropName="checked">
                          <Switch />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={8} lg={6}>
                        <Form.Item
                          label="重复失败次数阈值"
                          name="repeated_failure_threshold"
                          tooltip="同一任务在时间窗内连续失败达到此次数时生成告警；需同时开启上方开关并填写统计时间窗"
                        >
                          <InputNumber
                            min={1}
                            style={{ width: '100%' }}
                            placeholder="例如 3"
                            disabled={!repeatedFailuresEnabled}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={8} lg={6}>
                        <Form.Item label="统计时间窗（分钟）" name="repeated_failure_window_minutes">
                          <InputNumber
                            min={5}
                            style={{ width: '100%' }}
                            placeholder="例如 60"
                            disabled={!repeatedFailuresEnabled}
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                    {!repeatedFailuresEnabled ? (
                      <Text type="secondary" style={{ display: 'block', marginTop: -8, marginBottom: 8, fontSize: 12 }}>
                        开启重复失败统计后可配置阈值与时间窗
                      </Text>
                    ) : null}
                  </>
                );
              }}
            </Form.Item>

            <Divider orientation="left" plain>
              后台定时扫描
            </Divider>
            <Form.Item
              noStyle
              shouldUpdate={(prev, next) => prev.enable_alert_scan_worker !== next.enable_alert_scan_worker}
            >
              {({ getFieldValue }) => {
                const scanWorkerEnabled = !!getFieldValue('enable_alert_scan_worker');
                return (
                  <>
                    <Row gutter={[24, 0]}>
                      <Col xs={24} md={12} lg={8}>
                        <Form.Item
                          label="启用告警定时扫描"
                          name="enable_alert_scan_worker"
                          valuePropName="checked"
                          extra="还需在部署环境中开启告警扫描总开关，进程内才会按间隔执行扫描"
                        >
                          <Switch />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12} lg={8}>
                        <Form.Item
                          label="扫描间隔（秒）"
                          name="alert_scan_interval_seconds"
                          tooltip="留空时使用部署环境默认值；建议在此处配置业务间隔（不少于 10 秒）"
                        >
                          <InputNumber
                            min={10}
                            style={{ width: '100%' }}
                            placeholder="例如 300"
                            disabled={!scanWorkerEnabled}
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                    {!scanWorkerEnabled ? (
                      <Text type="secondary" style={{ display: 'block', marginTop: -8, marginBottom: 8, fontSize: 12 }}>
                        启用告警定时扫描后可配置扫描间隔
                      </Text>
                    ) : null}
                  </>
                );
              }}
            </Form.Item>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer" style={{ marginTop: 16 }}>
            <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
              保存设置
            </Button>
          </ProCard>
        </Form>
      </div>
    </TmPageContainer>
  );
}
