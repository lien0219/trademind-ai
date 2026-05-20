import { Link } from '@umijs/renderer-react';
import {
  ApiOutlined,
  CloudOutlined,
  ExperimentOutlined,
  ReloadOutlined,
  RobotOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Radio,
  Row,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ComponentType, CSSProperties } from 'react';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems, testAIConnection } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'ai';

const FIELDS: Record<string, FieldSpec> = {
  provider: {},
  base_url: {},
  model: {},
  api_key: { encrypted: true },
  temperature: {},
  max_tokens: {},
  timeout_sec: {},
  ai_batch_enabled: {},
  ai_batch_max_size: {},
  ai_batch_concurrency: {},
  ai_batch_auto_save_ai_field: {},
};

type ProviderValue = 'openai' | 'openai_compatible' | 'deepseek' | 'qwen';

type ProviderMeta = {
  value: ProviderValue;
  title: string;
  desc: string;
  tag?: string;
  Icon: ComponentType<{ style?: CSSProperties }>;
};

const PROVIDER_METAS: ProviderMeta[] = [
  {
    value: 'openai',
    title: 'OpenAI',
    desc: '官方 GPT 系列；适合英文与多语言商品文案',
    tag: '官方',
    Icon: RobotOutlined,
  },
  {
    value: 'openai_compatible',
    title: 'OpenAI Compatible',
    desc: 'Ollama、自建网关等兼容接口',
    tag: '通用',
    Icon: ApiOutlined,
  },
  {
    value: 'deepseek',
    title: 'DeepSeek',
    desc: '高性价比中文理解；标题 / 描述 / 客服建议',
    tag: '推荐',
    Icon: ThunderboltOutlined,
  },
  {
    value: 'qwen',
    title: '通义千问',
    desc: 'DashScope OpenAI 兼容模式；国内部署友好',
    tag: '国内',
    Icon: CloudOutlined,
  },
];

const PROVIDER_PRESETS: Record<
  ProviderValue,
  { baseUrl: string; model: string; baseUrlHelp: string }
> = {
  openai: {
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
    baseUrlHelp: 'OpenAI 官方 API 根路径，不含 /chat/completions',
  },
  openai_compatible: {
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
    baseUrlHelp: 'OpenAI 兼容接口根路径，不含 /chat/completions',
  },
  deepseek: {
    baseUrl: 'https://api.deepseek.com/v1',
    model: 'deepseek-chat',
    baseUrlHelp: 'DeepSeek OpenAI 兼容根路径；生产环境请以官方文档为准',
  },
  qwen: {
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    model: 'qwen-plus',
    baseUrlHelp: '通义千问 DashScope OpenAI 兼容根路径；生产环境请以控制台为准',
  },
};

function applyProviderPreset(
  provider: ProviderValue,
  current: { base_url?: string; model?: string },
  forceFill: boolean,
) {
  const preset = PROVIDER_PRESETS[provider];
  const next: { base_url?: string; model?: string } = {};
  if (forceFill || !String(current.base_url || '').trim()) {
    next.base_url = preset.baseUrl;
  }
  if (forceFill || !String(current.model || '').trim()) {
    next.model = preset.model;
  }
  return next;
}

export default function AISettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const provider = Form.useWatch('provider', form) as ProviderValue | undefined;
  const batchEnabled = Form.useWatch('ai_batch_enabled', form) as boolean | undefined;
  const preset = provider ? PROVIDER_PRESETS[provider] : PROVIDER_PRESETS.openai_compatible;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        provider: (g.provider as ProviderValue) || 'openai_compatible',
        base_url: g.base_url || '',
        model: g.model || '',
        api_key: g.api_key || '',
        temperature: g.temperature !== undefined && g.temperature !== '' ? Number(g.temperature) : 0.7,
        max_tokens: g.max_tokens !== undefined && g.max_tokens !== '' ? Number(g.max_tokens) : 512,
        timeout_sec: g.timeout_sec ? Number(g.timeout_sec) : 120,
        ai_batch_enabled: g.ai_batch_enabled !== 'false',
        ai_batch_max_size: g.ai_batch_max_size ? Number(g.ai_batch_max_size) : 100,
        ai_batch_concurrency: g.ai_batch_concurrency ? Number(g.ai_batch_concurrency) : 2,
        ai_batch_auto_save_ai_field: g.ai_batch_auto_save_ai_field !== 'false',
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

  const runTest = async () => {
    setTesting(true);
    try {
      const values = await form.validateFields(['provider', 'base_url', 'model', 'api_key', 'timeout_sec']);
      const apiKey = String(values.api_key ?? '').trim();
      const res = await testAIConnection({
        provider: values.provider,
        base_url: values.base_url,
        model: values.model,
        timeout_sec: String(values.timeout_sec ?? ''),
        ...(apiKey && !apiKey.includes('****') ? { api_key: apiKey } : {}),
      });
      const latency = res.latencyMs !== undefined ? `（${res.latencyMs} ms）` : '';
      const detail =
        res.provider || res.model
          ? ` [${[res.provider, res.model].filter(Boolean).join(' / ')}]`
          : '';
      message.success(`${res.message || '连接成功'}${detail}${latency}`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '连接失败');
    } finally {
      setTesting(false);
    }
  };

  return (
    <PageContainer
      title="AI 设置"
      subTitle="配置大模型 Provider，供标题优化、描述生成、客服建议、AI 采集规则等文本能力使用"
    >
      <div className="tm-ai-settings">
        <ProCard bordered className="tm-ai-settings__hero">
          <div className="tm-ai-settings__hero-inner">
            <div className="tm-ai-settings__hero-icon">
              <ExperimentOutlined />
            </div>
            <div className="tm-ai-settings__hero-body">
              <Typography.Title level={5} className="tm-ai-settings__hero-title">
                自备大模型 API
              </Typography.Title>
              <Typography.Paragraph type="secondary" className="tm-ai-settings__hero-desc">
                在 OpenAI / DeepSeek / 通义千问 / Ollama（OpenAI 兼容）等渠道申请 Key。管理端不直连模型，统一经后端
                AI Gateway 调用；支持 Chat Completions 文本能力（标题、描述、客服建议、批量 AI、采集规则生成等）。
              </Typography.Paragraph>
              <Space wrap size={[8, 8]}>
                <Tag color="blue">AI Gateway</Tag>
                <Tag>标题优化</Tag>
                <Tag>描述生成</Tag>
                <Tag>批量 AI</Tag>
                <Tag>采集规则 AI</Tag>
                <Link to="/settings/integrations">第三方集成总览 →</Link>
              </Space>
            </div>
          </div>
        </ProCard>

        <Form
          form={form}
          layout="vertical"
          onFinish={async (values) => {
            try {
              const payload = {
                ...values,
                timeout_sec: String(values.timeout_sec ?? ''),
                temperature: String(values.temperature ?? ''),
                max_tokens: String(values.max_tokens ?? ''),
                ai_batch_enabled: values.ai_batch_enabled ? 'true' : 'false',
                ai_batch_max_size: String(values.ai_batch_max_size ?? '100'),
                ai_batch_concurrency: String(values.ai_batch_concurrency ?? '2'),
                ai_batch_auto_save_ai_field: values.ai_batch_auto_save_ai_field ? 'true' : 'false',
              };
              await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <ProCard
            bordered
            title="Provider 类型"
            className="tm-ai-settings__panel"
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Form.Item name="provider" rules={[{ required: true }]} style={{ marginBottom: 0 }}>
              <Radio.Group
                className="tm-ai-provider-group"
                onChange={(e) => {
                  const value = e.target.value as ProviderValue;
                  const current = form.getFieldsValue(['base_url', 'model']);
                  form.setFieldsValue(applyProviderPreset(value, current, true));
                }}
              >
                <Row gutter={[12, 12]}>
                  {PROVIDER_METAS.map(({ value, title, desc, tag, Icon }) => (
                    <Col xs={24} sm={12} key={value}>
                      <Radio value={value} className="tm-ai-provider-radio">
                        <div className="tm-ai-provider-card-main">
                          <span className="tm-ai-provider-icon">
                            <Icon />
                          </span>
                          <div className="tm-ai-provider-text">
                            <div className="tm-ai-provider-title-row">
                              <span className="tm-ai-provider-title">{title}</span>
                              {tag ? <Tag className="tm-ai-provider-tag">{tag}</Tag> : null}
                            </div>
                            <div className="tm-ai-provider-desc">{desc}</div>
                          </div>
                        </div>
                      </Radio>
                    </Col>
                  ))}
                </Row>
              </Radio.Group>
            </Form.Item>
          </ProCard>

          <Row gutter={[16, 16]} className="tm-ai-settings__row">
            <Col xs={24} lg={14}>
              <ProCard bordered title="连接配置" className="tm-ai-settings__panel tm-ai-settings__panel--fill">
                <Form.Item
                  label="Base URL"
                  name="base_url"
                  rules={[{ required: true, message: '请输入 Base URL' }]}
                  extra={preset.baseUrlHelp}
                >
                  <Input placeholder={preset.baseUrl} />
                </Form.Item>
                <Form.Item label="模型" name="model" rules={[{ required: true, message: '请输入模型名' }]}>
                  <Input placeholder={preset.model} />
                </Form.Item>
                <Form.Item
                  label="API Key"
                  name="api_key"
                  rules={[{ required: true, message: '请输入 API Key' }]}
                  extra="敏感字段；保存后列表显示为 ****。测试连接时若留空占位则不覆盖已存密钥。"
                >
                  <Input.Password placeholder="sk-..." autoComplete="new-password" />
                </Form.Item>
              </ProCard>
            </Col>
            <Col xs={24} lg={10}>
              <ProCard bordered title="生成参数" className="tm-ai-settings__panel tm-ai-settings__panel--fill">
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Item label="Temperature" name="temperature" extra="默认 0.7">
                      <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="Max tokens" name="max_tokens" extra="默认 512">
                      <InputNumber min={1} max={32000} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item
                  label="超时（秒）"
                  name="timeout_sec"
                  rules={[{ required: true }]}
                  extra="测试可用 60；标题/描述建议 ≥120"
                >
                  <InputNumber min={5} max={600} style={{ width: '100%' }} />
                </Form.Item>
                <div className="tm-ai-settings__tips">
                  <Typography.Text type="secondary" className="tm-ai-settings__tips-title">
                    参数建议
                  </Typography.Text>
                  <ul className="tm-ai-settings__tips-list">
                    <li>标题优化：Temperature 0.4–0.7，Max tokens ≥1024</li>
                    <li>描述生成：Timeout ≥120，Max tokens 2000+</li>
                    <li>DeepSeek 等大模型：Timeout 建议 180</li>
                  </ul>
                </div>
              </ProCard>
            </Col>
          </Row>

          <ProCard bordered title="批量 AI（商品运营）" className="tm-ai-settings__panel">
            <Alert
              type="warning"
              showIcon
              className="tm-ai-settings__batch-alert"
              message="批量调用可能产生模型费用，请控制单次数量与并发。"
            />
            <div className="tm-ai-settings__batch-switch">
              <div>
                <Typography.Text strong>启用批量 AI 接口</Typography.Text>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
                  开启后管理端可创建批量任务；关闭后接口拒绝执行
                </Typography.Paragraph>
              </div>
              <Form.Item name="ai_batch_enabled" valuePropName="checked" noStyle>
                <Switch />
              </Form.Item>
            </div>
            <Row gutter={[16, 0]} className={batchEnabled ? undefined : 'tm-ai-settings__batch-fields--dim'}>
              <Col xs={24} sm={12} md={8}>
                <Form.Item
                  label="单次批量上限"
                  name="ai_batch_max_size"
                  rules={[{ required: true }]}
                  extra="商品数，过大可能超时或产生高额费用"
                >
                  <InputNumber min={1} max={5000} style={{ width: '100%' }} disabled={!batchEnabled} />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8}>
                <Form.Item
                  label="并行度（文本）"
                  name="ai_batch_concurrency"
                  rules={[{ required: true }]}
                  extra="同时处理的商品数"
                >
                  <InputNumber min={1} max={16} style={{ width: '100%' }} disabled={!batchEnabled} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item
                  label="默认写入 AI 草稿字段"
                  name="ai_batch_auto_save_ai_field"
                  valuePropName="checked"
                  extra="未传 applyMode 时写入 ai_title / ai_description"
                >
                  <Switch disabled={!batchEnabled} />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard bordered className="tm-ai-settings__footer">
            <Space wrap className="tm-action-space">
              <Button type="primary" htmlType="submit" loading={loading} size="large">
                保存配置
              </Button>
              <Button size="large" loading={testing} onClick={() => void runTest()}>
                测试连接
              </Button>
            </Space>
          </ProCard>
        </Form>
      </div>
    </PageContainer>
  );
}
