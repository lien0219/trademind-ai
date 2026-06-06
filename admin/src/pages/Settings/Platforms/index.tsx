import { ApiOutlined, LinkOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
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
  PLATFORM_DEV_PORTALS,
  PLATFORM_STATUS_META,
  platformAppFieldHelp,
  platformAppFieldLabel,
  platformAppFieldPlaceholder,
} from '@/constants/platformAppConfig';
import type { AppConfigFieldDTO, PlatformProviderMeta } from '@/services/shops';
import {
  externalDocUrlFor,
  getPlatformAppSettings,
  preferredPlatformTabOrder,
  putPlatformAppSettings,
  startDouyinOAuth,
  testPlatformAppSettings,
} from '@/services/platformOpen';
import { queryPlatformProviders } from '@/services/shops';

const { Paragraph, Text, Title } = Typography;

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
    if (v === undefined) {
      continue;
    }
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

function isFullWidthField(f: AppConfigFieldDTO): boolean {
  return f.type === 'textarea' || f.name === 'oauth_scopes' || f.name === 'scopes';
}

function renderFieldControl(f: AppConfigFieldDTO) {
  const ph = platformAppFieldPlaceholder(f);
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
          options={(f.options ?? []).map((o) => ({ label: o.label, value: o.value }))}
          placeholder={ph || undefined}
        />
      );
    case 'text':
    default:
      return <Input placeholder={ph} autoComplete="off" />;
  }
}

function PlatformPanel({ meta }: { meta: PlatformProviderMeta }) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [connecting, setConnecting] = useState(false);

  const schema = meta.appConfigSchema;
  const fields = schema?.fields ?? [];
  const st = PLATFORM_STATUS_META[meta.status] ?? { label: meta.status, color: 'default' };

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const row = await getPlatformAppSettings(meta.platform);
      const flds = row.schema?.fields?.length ? row.schema.fields : meta.appConfigSchema?.fields ?? [];
      form.resetFields();
      form.setFieldsValue(valuesToFormFields(flds, row.values ?? {}));
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form, meta]);

  useEffect(() => {
    void load();
  }, [load]);

  const docUrl = externalDocUrlFor(meta.platform);

  const testConnection = useCallback(async () => {
    setTesting(true);
    try {
      const res = await testPlatformAppSettings(meta.platform);
      if (res.ok) {
        message.success(res.message || '测试连接通过');
      } else {
        message.warning(res.message || '测试连接未通过');
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '测试连接失败');
    } finally {
      setTesting(false);
    }
  }, [meta.platform]);

  const connectDouyinShop = useCallback(async () => {
    setConnecting(true);
    try {
      const res = await startDouyinOAuth();
      const target = res.redirectUrl || res.authorizeUrl;
      if (!target) {
        throw new Error('missing douyin authorize url');
      }
      window.location.href = target;
    } catch (e: unknown) {
      message.error((e as Error)?.message || '发起抖店授权失败');
      setConnecting(false);
    }
  }, []);

  return (
    <Spin spinning={loading}>
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {meta.status === 'planned' && (
          <Alert
            showIcon
            type="warning"
            message="该平台能力暂未完全接入"
            description="可先保存开放平台应用参数，供后续店铺授权与 API 对接使用。"
          />
        )}
        {schema?.description ? (
          <Alert showIcon type="info" message={`${meta.name} 配置说明`} description={schema.description} />
        ) : null}
        <div>
          <Text type="secondary">接入状态 </Text>
          <Tag color={st.color}>{st.label}</Tag>
        </div>

        <Form
          layout="vertical"
          form={form}
          onFinish={async (vals: Record<string, unknown>) => {
            try {
              const payload = buildPutValues(fields, vals);
              await putPlatformAppSettings(meta.platform, payload);
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Row gutter={[24, 0]}>
            {fields.map((f) => (
              <Col xs={24} md={isFullWidthField(f) ? 24 : 12} key={f.name}>
                <Form.Item
                  name={f.name}
                  label={platformAppFieldLabel(f)}
                  valuePropName={f.type === 'switch' ? 'checked' : 'value'}
                  rules={[{ required: f.required, message: `请填写${platformAppFieldLabel(f)}` }]}
                  extra={platformAppFieldHelp(f)}
                >
                  {renderFieldControl(f)}
                </Form.Item>
              </Col>
            ))}
          </Row>

          <div className="tm-system-settings__footer" style={{ marginTop: 8, position: 'static', boxShadow: 'none', padding: '12px 0 0' }}>
            <Space wrap>
              <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
                保存配置
              </Button>
              <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
              <Button icon={<ApiOutlined />} onClick={() => void testConnection()} loading={testing} disabled={loading}>
                测试连接
              </Button>
              {meta.platform === 'douyin_shop' ? (
                <Button icon={<LinkOutlined />} onClick={() => void connectDouyinShop()} loading={connecting} disabled={loading}>
                  连接店铺
                </Button>
              ) : null}
              {docUrl ? (
                <Typography.Link href={docUrl} target="_blank" rel="noreferrer">
                  开放平台文档
                </Typography.Link>
              ) : null}
            </Space>
          </div>
        </Form>
      </Space>
    </Spin>
  );
}

export default function PlatformSettingsPage() {
  const [loadingProviders, setLoadingProviders] = useState(true);
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);
  const [tab, setTab] = useState<string>();

  const withSchema = useMemo(() => {
    return [...providers]
      .filter((p) => p.settingsGroupKey && p.settingsGroupKey.trim())
      .sort((a, b) => preferredPlatformTabOrder(a.platform) - preferredPlatformTabOrder(b.platform));
  }, [providers]);

  const loadProviders = useCallback(async () => {
    setLoadingProviders(true);
    try {
      const { list } = await queryPlatformProviders();
      setProviders(Array.isArray(list) ? list : []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载平台列表失败');
    } finally {
      setLoadingProviders(false);
    }
  }, []);

  useEffect(() => {
    void loadProviders();
  }, [loadProviders]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('platform') !== 'douyin_shop') return;
    const auth = params.get('auth');
    if (auth === 'success') {
      message.success('抖店店铺授权成功');
    } else if (auth === 'failed') {
      message.error(`抖店店铺授权失败：${params.get('reason') || 'UNKNOWN_DOUYIN_AUTH_ERROR'}`);
    }
  }, []);

  useEffect(() => {
    if (!tab && withSchema.length > 0) {
      const douyin = withSchema.find((p) => p.platform === 'douyin_shop');
      setTab((douyin ?? withSchema[0]).platform);
    }
  }, [tab, withSchema]);

  const items = withSchema.map((p) => {
    const st = PLATFORM_STATUS_META[p.status];
    return {
      key: p.platform,
      label: (
        <Space size={6}>
          <span>{p.name}</span>
          {st && p.status !== 'available' ? <Tag color={st.color} style={{ margin: 0 }}>{st.label}</Tag> : null}
        </Space>
      ),
      children: <PlatformPanel meta={p} />,
    };
  });

  return (
    <PageContainer title="平台开放配置" subTitle="各跨境平台 Partner 应用参数（App Key / Secret 等），店铺授权在「店铺管理」完成">
      <div className="tm-system-settings">
        <ProCard bordered className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <ApiOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Title level={5} className="tm-system-settings__hero-title">
                开放平台应用参数
              </Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                请在各平台开放平台创建应用，将 App Key、Secret 等填回贸灵；敏感信息加密存库。店铺授权后的 Access Token 仅保存在对应店铺，不会写入此处。完成配置后前往{' '}
                <Link to="/shops">店铺管理</Link> 授权。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <ProCard bordered title="选择平台" className="tm-system-settings__panel">
          <Paragraph type="secondary" style={{ marginBottom: 12 }}>
            开发者门户：
            {PLATFORM_DEV_PORTALS.map((p, i) => (
              <span key={p.url}>
                {i > 0 ? ' · ' : ' '}
                <Typography.Link href={p.url} target="_blank" rel="noreferrer">
                  {p.name}
                </Typography.Link>
              </span>
            ))}
          </Paragraph>
          <Spin spinning={loadingProviders}>
            {items.length === 0 ? (
              <Paragraph type="secondary">暂无可用平台配置项，请刷新或检查后端注册。</Paragraph>
            ) : (
              <Tabs activeKey={tab} onChange={setTab} items={items} />
            )}
          </Spin>
        </ProCard>
      </div>
    </PageContainer>
  );
}
