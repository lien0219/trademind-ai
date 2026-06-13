import { Link } from '@umijs/renderer-react';
import {
  ApiOutlined,
  CloudOutlined,
  ExperimentOutlined,
  LinkOutlined,
  ReloadOutlined,
  RobotOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
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
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AI_PROVIDER_DOCS,
  AI_PROVIDER_FIELD_KEYS,
  AI_PROVIDER_METAS,
  AI_PROVIDER_PRESETS,
  ALL_AI_CONNECTION_FIELD_SPECS,
  aiProviderDocs,
  allAIConnectionFieldKeys,
  applyAIProviderPreset,
  buildAISaveFieldSpecs,
  aiProviderAPIKeyKey,
  aiProviderBaseURLKey,
  aiProviderModelKey,
  initialAIConnectionFormValues,
  type AIProviderValue,
} from '@/constants/aiProviders';
import { ACTION_COPY, PAGE_COPY } from '@/constants/copywriting';
import { fetchSettingsList, saveSettingsItems, testAIConnection } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'ai';

const GLOBAL_FIELDS: Record<string, FieldSpec> = {
  provider: {},
  temperature: {},
  max_tokens: {},
  timeout_sec: {},
  ai_batch_enabled: {},
  ai_batch_max_size: {},
  ai_batch_concurrency: {},
  ai_batch_auto_save_ai_field: {},
};

const PROVIDER_ICONS: Record<AIProviderValue, ComponentType<{ style?: CSSProperties }>> = {
  openai: RobotOutlined,
  openai_compatible: ApiOutlined,
  deepseek: ThunderboltOutlined,
  qwen: CloudOutlined,
};

export default function AISettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const provider = Form.useWatch('provider', form) as AIProviderValue | undefined;
  const batchEnabled = Form.useWatch('ai_batch_enabled', form) as boolean | undefined;
  const preset = provider ? AI_PROVIDER_PRESETS[provider] : AI_PROVIDER_PRESETS.openai_compatible;
  const providerDocs = aiProviderDocs(provider);

  const visibleFieldKeys = useMemo(
    () => (provider ? AI_PROVIDER_FIELD_KEYS[provider] ?? [] : []),
    [provider],
  );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        provider: (g.provider as AIProviderValue) || 'openai_compatible',
        ...initialAIConnectionFormValues(g),
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
    if (!provider) {
      message.error('请选择 AI 服务商');
      return;
    }
    setTesting(true);
    try {
      const baseKey = aiProviderBaseURLKey(provider);
      const modelKey = aiProviderModelKey(provider);
      const apiKeyKey = aiProviderAPIKeyKey(provider);
      const values = await form.validateFields([baseKey, modelKey, apiKeyKey, 'timeout_sec']);
      const apiKey = String(values[apiKeyKey] ?? '').trim();
      const res = await testAIConnection({
        provider,
        base_url: values[baseKey],
        model: values[modelKey],
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

  const renderConnectionField = (key: string) => {
    const spec = ALL_AI_CONNECTION_FIELD_SPECS[key];
    if (!spec) return null;
    const isPassword = !!spec.encrypted;
    return (
      <Form.Item
        key={key}
        label={spec.label}
        name={key}
        rules={
          provider && AI_PROVIDER_FIELD_KEYS[provider]?.includes(key) && key.endsWith('_api_key')
            ? [{ required: true, message: '请输入 API 密钥' }]
            : key.endsWith('_base_url') || key.endsWith('_model')
              ? [{ required: true, message: `请输入${spec.label}` }]
              : undefined
        }
        extra={
          key.endsWith('_api_key')
            ? '敏感字段；保存后显示为 ****。测试连接时若留空占位则不覆盖已存密钥。各服务商密钥独立保存。'
            : spec.extra
        }
      >
        {isPassword ? (
          <Input.Password placeholder={spec.placeholder} autoComplete="new-password" />
        ) : (
          <Input placeholder={spec.placeholder} />
        )}
      </Form.Item>
    );
  };

  const renderProviderDocLinks = (compact?: boolean) => {
    if (!providerDocs) return null;
    return (
      <Space wrap size={compact ? [8, 4] : [12, 8]}>
        <Typography.Link href={providerDocs.docsUrl} target="_blank" rel="noreferrer">
          <LinkOutlined /> {providerDocs.docsLabel}
        </Typography.Link>
        {providerDocs.consoleUrl ? (
          <Typography.Link href={providerDocs.consoleUrl} target="_blank" rel="noreferrer">
            <LinkOutlined /> {providerDocs.consoleLabel ?? '申请 API Key'}
          </Typography.Link>
        ) : null}
      </Space>
    );
  };

  return (
    <TmPageContainer
      title="AI 设置"
      subTitle={PAGE_COPY.aiSettings.description}
    >
      <div className="tm-ai-settings">
        <ProCard variant="outlined" className="tm-ai-settings__hero">
          <div className="tm-ai-settings__hero-inner">
            <div className="tm-ai-settings__hero-icon">
              <ExperimentOutlined />
            </div>
            <div className="tm-ai-settings__hero-body">
              <Typography.Title level={5} className="tm-ai-settings__hero-title">
                自备大模型 API
              </Typography.Title>
              <Typography.Paragraph type="secondary" className="tm-ai-settings__hero-desc">
                在 OpenAI、DeepSeek、通义千问、Ollama 等渠道申请 API 密钥。各服务商可分别保存配置；下方选择「当前默认使用」的服务商，保存时仅更新该服务商连接信息，不会影响其他已保存密钥。
              </Typography.Paragraph>
              <Space wrap size={[8, 8]}>
                <Tag color="blue">统一后端调用</Tag>
                <Tag>标题优化</Tag>
                <Tag>描述生成</Tag>
                <Tag>批量 AI</Tag>
                <Tag>AI 生成采集规则</Tag>
                <Link to="/settings/integrations">第三方集成总览 →</Link>
              </Space>
            </div>
          </div>
        </ProCard>

        <Form
          form={form}
          layout="vertical"
          preserve
          onFinish={async (values) => {
            try {
              const activeProvider = (values.provider as AIProviderValue) || 'openai_compatible';
              const payload: Record<string, unknown> = {
                provider: activeProvider,
                timeout_sec: String(values.timeout_sec ?? ''),
                temperature: String(values.temperature ?? ''),
                max_tokens: String(values.max_tokens ?? ''),
                ai_batch_enabled: values.ai_batch_enabled ? 'true' : 'false',
                ai_batch_max_size: String(values.ai_batch_max_size ?? '100'),
                ai_batch_concurrency: String(values.ai_batch_concurrency ?? '2'),
                ai_batch_auto_save_ai_field: values.ai_batch_auto_save_ai_field ? 'true' : 'false',
              };
              for (const key of AI_PROVIDER_FIELD_KEYS[activeProvider]) {
                payload[key] = values[key] ?? '';
              }
              const saveSpecs: Record<string, FieldSpec> = {
                ...GLOBAL_FIELDS,
                ...buildAISaveFieldSpecs(activeProvider),
              };
              await saveSettingsItems(toPutItems(GROUP, saveSpecs, payload));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <ProCard
            variant="outlined"
            title="AI 服务商"
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
                  const value = e.target.value as AIProviderValue;
                  const baseKey = aiProviderBaseURLKey(value);
                  const modelKey = aiProviderModelKey(value);
                  const apiKeyKey = aiProviderAPIKeyKey(value);
                  const current = form.getFieldsValue([baseKey, modelKey, apiKeyKey]);
                  form.setFieldsValue(
                    applyAIProviderPreset(
                      value,
                      { baseUrl: current[baseKey], model: current[modelKey] },
                      false,
                    ),
                  );
                }}
              >
                <Row gutter={[12, 12]}>
                  {AI_PROVIDER_METAS.map(({ value, title, desc, tag }) => {
                    const Icon = PROVIDER_ICONS[value];
                    const docs = AI_PROVIDER_DOCS[value];
                    return (
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
                              {docs ? (
                                <div
                                  className="tm-ai-provider-desc"
                                  onClick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                  }}
                                >
                                  <Typography.Link
                                    href={docs.docsUrl}
                                    target="_blank"
                                    rel="noreferrer"
                                    style={{ fontSize: 12 }}
                                  >
                                    官方文档
                                  </Typography.Link>
                                </div>
                              ) : null}
                            </div>
                          </div>
                        </Radio>
                      </Col>
                    );
                  })}
                </Row>
              </Radio.Group>
            </Form.Item>
          </ProCard>

          <Row gutter={[16, 16]} className="tm-ai-settings__row">
            <Col xs={24} lg={14}>
              <ProCard variant="outlined" title="连接配置" className="tm-ai-settings__panel tm-ai-settings__panel--fill">
                {providerDocs ? (
                  <Alert
                    type="info"
                    showIcon
                    className="tm-ai-settings__doc-alert"
                    style={{ marginBottom: 16 }}
                    message="配置指引"
                    description={
                      <>
                        <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
                          请参考官方文档确认接口地址、模型名称，并在控制台申请 API Key 后填入下方表单。
                        </Typography.Paragraph>
                        {renderProviderDocLinks()}
                      </>
                    }
                  />
                ) : null}
                {visibleFieldKeys.map((k) => renderConnectionField(k))}
                {allAIConnectionFieldKeys()
                  .filter((k) => !visibleFieldKeys.includes(k))
                  .map((k) => (
                    <Form.Item key={`hidden-${k}`} name={k} hidden>
                      <Input />
                    </Form.Item>
                  ))}
              </ProCard>
            </Col>
            <Col xs={24} lg={10}>
              <ProCard variant="outlined" title="生成参数" className="tm-ai-settings__panel tm-ai-settings__panel--fill">
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Item label="随机度（Temperature）" name="temperature" extra="默认 0.7，越低越稳定">
                      <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="最大输出长度（tokens）" name="max_tokens" extra="默认 512">
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
                  {preset ? (
                    <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
                      当前服务商默认接口：{preset.baseUrl} · 模型：{preset.model}
                    </Typography.Paragraph>
                  ) : null}
                </div>
              </ProCard>
            </Col>
          </Row>

          <ProCard variant="outlined" title="批量 AI（商品运营）" className="tm-ai-settings__panel">
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

          <ProCard variant="outlined" className="tm-ai-settings__footer">
            <Space wrap className="tm-action-space">
              <Button type="primary" htmlType="submit" loading={loading} size="large">
                {ACTION_COPY.saveSettings}
              </Button>
              <Button size="large" loading={testing} onClick={() => void runTest()}>
                测试连接
              </Button>
            </Space>
          </ProCard>
        </Form>
      </div>
    </TmPageContainer>
  );
}
