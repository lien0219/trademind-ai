import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer, TmPageHeaderExtra } from '@/components/ui';
import {
  Alert,
  Button,
  Col,
  Divider,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { Link } from '@umijs/renderer-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  PUBLISH_FIELD_SECTION_META,
  groupPublishFields,
  isFullWidthPublishField,
  platformPublishFieldHelp,
  platformPublishFieldLabel,
  platformPublishFieldPlaceholder,
  publishSwitchFields,
} from '@/constants/platformPublishConfig';
import { PLATFORM_STATUS_META } from '@/constants/platformAppConfig';
import type { AppConfigFieldDTO, PlatformProviderMeta } from '@/services/shops';
import {
  externalDocUrlFor,
  preferredPlatformTabOrder,
} from '@/services/platformOpen';
import {
  getPlatformPublishSettings,
  putPlatformPublishSettings,
} from '@/services/platformPublish';
import { queryPlatformProviders } from '@/services/shops';

const { Paragraph, Text } = Typography;

function valuesToFormFields(fields: AppConfigFieldDTO[], values: Record<string, string>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of fields) {
    const raw = values[f.name] ?? '';
    switch (f.type) {
      case 'switch':
        out[f.name] = raw === 'true';
        break;
      case 'number':
        out[f.name] = raw === '' ? undefined : Number(raw);
        break;
      default:
        out[f.name] = raw;
    }
  }
  return out;
}

function buildPutValues(fields: AppConfigFieldDTO[], formVals: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of fields) {
    const v = formVals[f.name];
    if (v === undefined) continue;
    if (f.type === 'switch') {
      out[f.name] = !!v;
      continue;
    }
    if (f.type === 'number') {
      if (typeof v === 'number' && Number.isFinite(v)) {
        out[f.name] = v;
      }
      continue;
    }
    if (typeof v === 'string') {
      out[f.name] = v;
      continue;
    }
    out[f.name] = v;
  }
  return out;
}

function renderFieldControl(f: AppConfigFieldDTO) {
  const ph = tr(platformPublishFieldPlaceholder(f) ?? '');
  switch (f.type) {
    case 'password':
      return <Input.Password placeholder={ph} autoComplete={f.sensitive ? 'new-password' : 'off'} />;
    case 'textarea':
      return <Input.TextArea rows={4} placeholder={ph} />;
    case 'number':
      return <InputNumber min={f.name === 'timeout_sec' ? 5 : undefined} style={{ width: '100%' }} placeholder={ph || undefined} />;
    case 'switch':
      return <Switch />;
    case 'select':
      return (
        <Select
          allowClear={!f.required}
          options={(f.options ?? []).map((o) => ({ label: tr(o.label), value: o.value }))}
          placeholder={ph || undefined}
        />
      );
    case 'text':
    default:
      return <Input placeholder={ph} autoComplete="off" />;
  }
}

function PublishToggleCard({ field }: { field: AppConfigFieldDTO }) {
  useLanguage();
  const label = tr(platformPublishFieldLabel(field) ?? '');
  const extra = tr(platformPublishFieldHelp(field) ?? '');
  return (
    <div className="tm-system-settings__toggle-card">
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Text className="tm-system-settings__toggle-label">{label}</Text>
          {extra ? (
            <Text type="secondary" className="tm-system-settings__toggle-extra">
              {extra}
            </Text>
          ) : null}
        </div>
        <Form.Item name={field.name} valuePropName="checked" style={{ marginBottom: 0, flexShrink: 0 }}>
          <Switch />
        </Form.Item>
      </div>
    </div>
  );
}

function PublishFieldItem({ field }: { field: AppConfigFieldDTO }) {
  useLanguage();
  const label = tr(platformPublishFieldLabel(field) ?? '');
  return (
    <Form.Item
      name={field.name}
      label={label}
      valuePropName={field.type === 'switch' ? 'checked' : 'value'}
      rules={[{ required: field.required, message: trFormat("请填写{value0}", { value0: label }) }]}
      extra={tr(platformPublishFieldHelp(field) ?? '')}
    >
      {renderFieldControl(field)}
    </Form.Item>
  );
}

function hasPublishSchema(p: PlatformProviderMeta): boolean {
  const gk = (p.publishSettingsGroupKey ?? p.publishConfigSchema?.groupKey ?? '').trim();
  return gk.length > 0;
}

function PublishPanel({ meta }: { meta: PlatformProviderMeta }) {
  const language = useLanguage();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const schema = meta.publishConfigSchema;
  const fields = schema?.fields ?? [];
  const st = PLATFORM_STATUS_META[meta.status] ?? { label: meta.status, color: 'default' };

  const fieldSections = useMemo(() => groupPublishFields(fields), [fields, language]);
  const switchFields = useMemo(() => publishSwitchFields(fields), [fields, language]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const row = await getPlatformPublishSettings(meta.platform);
      const flds = row.schema?.fields?.length ? row.schema.fields : meta.publishConfigSchema?.fields ?? [];
      form.resetFields();
      form.setFieldsValue(valuesToFormFields(flds, row.values ?? {}));
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载失败"));
    } finally {
      setLoading(false);
    }
  }, [form, meta]);

  useEffect(() => {
    void load();
  }, [load]);

  const docUrl = externalDocUrlFor(meta.platform);

  return (
    <Spin spinning={loading}>
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {meta.status === 'planned' && (
          <Alert
            showIcon
            type="warning"
            message={tr("该平台刊登能力规划中")}
            description={tr("可先保存刊登侧默认参数；真实对接启用前不会调用平台上架 API。")}
          />
        )}
        {schema?.description ? (
          <Alert showIcon type="info" message={trFormat("{value0} 刊登说明", { value0: meta.name })} description={schema.description} />
        ) : null}
        <Space wrap size={[8, 4]}>
          <Text type="secondary">{tr("接入状态")}</Text>
          <Tag color={st.color}>{st.label}</Tag>
        </Space>

        <Form
          layout="vertical"
          form={form}
          onFinish={async (vals: Record<string, unknown>) => {
            try {
              const payload = buildPutValues(fields, vals);
              await putPlatformPublishSettings(meta.platform, payload);
              message.success(tr("已保存"));
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || tr("保存失败"));
            }
          }}
        >
          {fieldSections.map(({ section, fields: sectionFields }) => {
            const metaSec = PUBLISH_FIELD_SECTION_META[section];
            return (
              <div key={section} className="tm-platform-publish__section">
                <Divider orientation="left" orientationMargin={0} style={{ margin: '0 0 16px' }}>
                  <Text strong>{tr(metaSec.title ?? '')}</Text>
                </Divider>
                {tr(metaSec.description ?? '') ? (
                  <Paragraph type="secondary" style={{ marginTop: -8, marginBottom: 16, fontSize: 13 }}>
                    {tr(metaSec.description ?? '')}
                  </Paragraph>
                ) : null}
                <Row gutter={[24, 0]}>
                  {sectionFields.map((f) => (
                    <Col xs={24} md={isFullWidthPublishField(f) ? 24 : 12} key={f.name}>
                      <PublishFieldItem field={f} />
                    </Col>
                  ))}
                </Row>
              </div>
            );
          })}

          {switchFields.length > 0 ? (
            <div className="tm-platform-publish__section">
              <Divider orientation="left" orientationMargin={0} style={{ margin: '0 0 16px' }}>
                <Text strong>{tr(PUBLISH_FIELD_SECTION_META.publish.title ?? '')}</Text>
              </Divider>
              {tr(PUBLISH_FIELD_SECTION_META.publish.description ?? '') ? (
                <Paragraph type="secondary" style={{ marginTop: -8, marginBottom: 16, fontSize: 13 }}>
                  {tr(PUBLISH_FIELD_SECTION_META.publish.description ?? '')}
                </Paragraph>
              ) : null}
              <Row gutter={[16, 16]}>
                {switchFields.map((f) => (
                  <Col xs={24} md={switchFields.length === 1 ? 24 : 12} key={f.name}>
                    <PublishToggleCard field={f} />
                  </Col>
                ))}
              </Row>
            </div>
          ) : null}

          <div className="tm-system-settings__footer" style={{ marginTop: 8, position: 'static', boxShadow: 'none', padding: '12px 0 0' }}>
            <Space wrap>
              <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>{tr("保存刊登预设")}</Button>
              <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>{tr("重新加载")}</Button>
              {docUrl ? (
                <Typography.Link href={docUrl} target="_blank" rel="noreferrer">{tr("开放平台文档")}</Typography.Link>
              ) : null}
            </Space>
          </div>
        </Form>
      </Space>
    </Spin>
  );
}

export default function PlatformPublishSettingsPage() {
  const language = useLanguage();
  const [loadingProviders, setLoadingProviders] = useState(true);
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);
  const [tab, setTab] = useState<string>();

  const withSchema = useMemo(() => {
    return [...providers]
      .filter(hasPublishSchema)
      .sort((a, b) => preferredPlatformTabOrder(a.platform) - preferredPlatformTabOrder(b.platform));
  }, [providers, language]);

  const loadProviders = useCallback(async () => {
    setLoadingProviders(true);
    try {
      const { list } = await queryPlatformProviders();
      setProviders(Array.isArray(list) ? list : []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载平台列表失败"));
    } finally {
      setLoadingProviders(false);
    }
  }, []);

  useEffect(() => {
    void loadProviders();
  }, [loadProviders]);

  useEffect(() => {
    if (!tab && withSchema.length > 0) {
      const tiktok = withSchema.find((p) => p.platform === 'tiktok');
      setTab((tiktok ?? withSchema[0]).platform);
    }
  }, [tab, withSchema]);

  const items = withSchema.map((p) => {
    const st = PLATFORM_STATUS_META[p.status];
    return {
      key: p.platform,
      label: (
        <Space size={6}>
          <span>{p.name}</span>
          {st && p.status !== 'available' ? (
            <Tag color={st.color} style={{ margin: 0 }}>
              {st.label}
            </Tag>
          ) : null}
        </Space>
      ),
      children: <PublishPanel meta={p} />,
    };
  });

  return (
    <TmPageContainer
      title={tr("刊登设置")}
      extra={
        <TmPageHeaderExtra>
          <Link to="/settings/platforms">{tr("平台接入设置")}</Link>
          <Link to="/product/publish-tasks">{tr("商品 · 刊登任务")}</Link>
        </TmPageHeaderExtra>
      }
    >
      <div className="tm-system-settings tm-platform-publish">
        <ProCard variant="outlined" title={tr("选择平台")} className="tm-system-settings__panel">
          <Spin spinning={loadingProviders}>
            {items.length === 0 ? (
              <Paragraph type="secondary">{tr("暂无可配置刊登参数的平台，请稍后刷新或联系管理员。")}</Paragraph>
            ) : (
              <Tabs activeKey={tab} onChange={setTab} items={items} />
            )}
          </Spin>
        </ProCard>
      </div>
    </TmPageContainer>
  );
}
