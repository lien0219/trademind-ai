import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Form,
  Input,
  InputNumber,
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
import type { AppConfigFieldDTO, PlatformProviderMeta } from '@/services/shops';
import {
  externalDocUrlFor,
  getPlatformAppSettings,
  preferredPlatformTabOrder,
  putPlatformAppSettings,
} from '@/services/platformOpen';
import { queryPlatformProviders } from '@/services/shops';

const STATUS_META: Record<string, { label: string; color: string }> = {
  available: { label: '可用', color: 'success' },
  beta: { label: '测试中', color: 'processing' },
  planned: { label: '规划中', color: 'default' },
  disabled: { label: '停用', color: 'error' },
};

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

/** Build PATCH map: omit undefined so backend keeps previous value; booleans always sent. */
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

function renderFieldControl(f: AppConfigFieldDTO) {
  const ph = f.placeholder ?? '';
  switch (f.type) {
    case 'password':
      return <Input.Password placeholder={ph} autoComplete={f.sensitive ? 'new-password' : 'off'} />;
    case 'textarea':
      return <Input.TextArea rows={4} placeholder={ph} />;
    case 'number':
      return <InputNumber min={f.name === 'timeout_sec' ? 5 : undefined} style={{ width: '100%' }} />;
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

  const schema = meta.appConfigSchema;
  const fields = schema?.fields ?? [];
  const st = STATUS_META[meta.status] ?? { label: meta.status, color: 'default' };

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

  return (
    <Spin spinning={loading}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        {meta.status === 'planned' && (
          <Alert
            showIcon
            type="warning"
            message="该平台能力暂未接入，可先保存开放平台配置。"
            description="OAuth、TestConnection、订单同步等仍可能返回「未实现」；应用级参数可先保存在贸灵供后续对接使用。"
          />
        )}
        {schema.description && <Typography.Paragraph type="secondary">{schema.description}</Typography.Paragraph>}
        <Typography.Paragraph type="secondary">
          运行时：<Tag color={st.color}>{st.label}</Tag>；Settings 分组 <Typography.Text code>{meta.settingsGroupKey}</Typography.Text>
        </Typography.Paragraph>
        <Form
          layout="vertical"
          form={form}
          style={{ maxWidth: 720 }}
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
          {fields.map((f) => (
            <Form.Item
              key={f.name}
              name={f.name}
              label={f.label}
              valuePropName={f.type === 'switch' ? 'checked' : 'value'}
              rules={[{ required: f.required, message: `${f.label} 为必填` }]}
              extra={f.help}
            >
              {renderFieldControl(f)}
            </Form.Item>
          ))}
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>
                保存配置
              </Button>
              <Button onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
              {docUrl && (
                <Typography.Link href={docUrl} target="_blank" rel="noreferrer">
                  开放平台门户
                </Typography.Link>
              )}
            </Space>
          </Form.Item>
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
    if (!tab && withSchema.length > 0) {
      const tiktok = withSchema.find((p) => p.platform === 'tiktok');
      setTab((tiktok ?? withSchema[0]).platform);
    }
  }, [tab, withSchema]);

  const items = withSchema.map((p) => ({
    key: p.platform,
    label: `${p.name}${p.platform === 'tiktok' ? '（Beta）' : ''}`,
    children: <PlatformPanel meta={p} />,
  }));

  return (
    <PageContainer title="平台开放配置">
      <ProCard bordered>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="由您在各平台开放平台自建应用并把参数填回贸灵"
          description={
            <>
              贸灵不写死任何三方 App Key / Secret。敏感项服务端 AES-GCM 加密存入 <Typography.Text code>settings</Typography.Text>{' '}
              并按 schema 分组（如 <Typography.Text code>platform_tiktok</Typography.Text>）。
              OAuth 后的 <Typography.Text strong>access_token / refresh_token</Typography.Text> 仅保存在店铺的{' '}
              <Typography.Text code>shop_auth_tokens</Typography.Text>，
              <Typography.Text strong>不要</Typography.Text>
              写进通用 settings。
              <br />
              授权店铺请前往{' '}
              <Link to="/shops">店铺管理</Link>
              ，若未完成应用级配置会先提示你到本页补齐。
              <br />
              参考门户：{' '}
              <Typography.Link href="https://partner.tiktokshop.com/" target="_blank" rel="noreferrer">
                TikTok Shop Partner
              </Typography.Link>
              、{' '}
              <Typography.Link href="https://open.shopee.com/" target="_blank" rel="noreferrer">
                Shopee Open
              </Typography.Link>
              、{' '}
              <Typography.Link href="https://open.lazada.com/" target="_blank" rel="noreferrer">
                Lazada Open
              </Typography.Link>
              、Amazon SP-API、AliExpress、Shopify Partners、Woo REST、eBay Developers 等——按实际对接平台选择。
            </>
          }
        />
        <Spin spinning={loadingProviders}>
          {items.length === 0 ? (
            <Typography.Paragraph type="secondary">暂无带应用配置 Schema 的平台（请刷新或检查后端注册）。</Typography.Paragraph>
          ) : (
            <Tabs activeKey={tab} onChange={setTab} items={items} />
          )}
        </Spin>
      </ProCard>
    </PageContainer>
  );
}
