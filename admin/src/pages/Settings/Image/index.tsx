import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ALL_IMAGE_FIELD_SPECS,
  IMAGE_SCENARIOS,
  type ImageProviderCapability,
  type ImageScenarioId,
  PROVIDER_FIELD_KEYS,
  isProviderSelectable,
  providerDifficultyLabel,
  providerRegionLabel,
  providerStatusLabel,
} from '@/constants/imageProviders';
import { fetchImageProviders, testImageProvider } from '@/services/imageProviders';
import { taskTypeLabel } from '@/services/imageTasks';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'image';

const ENCRYPTED_KEYS = new Set([
  'removebg_api_key',
  'openai_image_api_key',
  'comfyui_api_key',
  'dashscope_image_api_key',
  'volcengine_image_api_key',
  'siliconflow_image_api_key',
  'hunyuan_image_api_key',
]);

function buildFieldsSpec(): Record<string, FieldSpec> {
  const out: Record<string, FieldSpec> = { provider: {} };
  for (const key of Object.keys(ALL_IMAGE_FIELD_SPECS)) {
    if (key === 'provider') continue;
    out[key] = {
      encrypted: ENCRYPTED_KEYS.has(key),
      valueType: ALL_IMAGE_FIELD_SPECS[key].valueType === 'json' ? 'json' : undefined,
    };
  }
  return out;
}

const FIELDS = buildFieldsSpec();

function ProviderMetaTags({ cap }: { cap: ImageProviderCapability }) {
  return (
    <Space size={4} wrap style={{ marginTop: 4 }}>
      <Tag>{providerDifficultyLabel(cap.difficulty)}</Tag>
      <Tag color="blue">{providerRegionLabel(cap.regionFriendly)}</Tag>
      {cap.requiresApiKey ? <Tag>需 API Key</Tag> : null}
      {cap.requiresSelfHosted ? <Tag color="orange">需自部署</Tag> : null}
      {cap.status !== 'available' ? <Tag color="default">{providerStatusLabel(cap.status)}</Tag> : null}
      {cap.supportedTasks.map((t) => (
        <Tag key={t} color="geekblue">
          {taskTypeLabel(t)}
        </Tag>
      ))}
    </Space>
  );
}

export default function ImageSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [caps, setCaps] = useState<ImageProviderCapability[]>([]);
  const [scenario, setScenario] = useState<ImageScenarioId | ''>('');
  const provider = Form.useWatch('provider', form) as string | undefined;

  const loadCaps = useCallback(async () => {
    try {
      const list = await fetchImageProviders();
      setCaps(list);
    } catch {
      setCaps([]);
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        provider: g.provider || 'noop',
        provider_preset: g.provider_preset || '',
        image_task_default_size: g.image_task_default_size || '1024x1024',
        image_task_default_quality: g.image_task_default_quality || 'standard',
        removebg_api_key: g.removebg_api_key || '',
        removebg_base_url: g.removebg_base_url || 'https://api.remove.bg/v1.0',
        openai_image_base_url: g.openai_image_base_url || '',
        openai_image_api_key: g.openai_image_api_key || '',
        openai_image_model: g.openai_image_model || 'gpt-image-1',
        openai_image_size: g.openai_image_size || '1024x1024',
        openai_image_quality: g.openai_image_quality || 'standard',
        openai_image_background: g.openai_image_background || '',
        comfyui_base_url: g.comfyui_base_url || 'http://127.0.0.1:8188',
        comfyui_api_key: g.comfyui_api_key || '',
        comfyui_workflow_json: g.comfyui_workflow_json || '',
        comfyui_prompt_node_id: g.comfyui_prompt_node_id || '',
        comfyui_image_node_id: g.comfyui_image_node_id || '',
        comfyui_output_node_id: g.comfyui_output_node_id || '',
        comfyui_timeout_sec: g.comfyui_timeout_sec ? Number(g.comfyui_timeout_sec) : 180,
        comfyui_poll_interval_seconds: g.comfyui_poll_interval_seconds
          ? Number(g.comfyui_poll_interval_seconds)
          : 2,
        comfyui_max_poll_seconds: g.comfyui_max_poll_seconds ? Number(g.comfyui_max_poll_seconds) : 180,
        dashscope_image_api_key: g.dashscope_image_api_key || '',
        dashscope_image_base_url: g.dashscope_image_base_url || '',
        dashscope_image_model: g.dashscope_image_model || 'wan2.7-image-pro',
        dashscope_image_size: g.dashscope_image_size || '2K',
        dashscope_image_quality: g.dashscope_image_quality || '',
        volcengine_image_api_key: g.volcengine_image_api_key || '',
        volcengine_image_base_url: g.volcengine_image_base_url || '',
        volcengine_image_model: g.volcengine_image_model || '',
        volcengine_image_size: g.volcengine_image_size || '1024x1024',
        siliconflow_image_api_key: g.siliconflow_image_api_key || '',
        siliconflow_image_base_url: g.siliconflow_image_base_url || '',
        siliconflow_image_model: g.siliconflow_image_model || '',
        siliconflow_image_size: g.siliconflow_image_size || '1024x1024',
        hunyuan_image_api_key: g.hunyuan_image_api_key || '',
        hunyuan_image_base_url: g.hunyuan_image_base_url || '',
        hunyuan_image_model: g.hunyuan_image_model || '',
        timeout_sec: g.timeout_sec ? Number(g.timeout_sec) : 60,
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void loadCaps();
    void load();
  }, [load, loadCaps]);

  const currentCap = useMemo(
    () => caps.find((c) => c.provider === (provider || '').trim()),
    [caps, provider],
  );

  const providerOptions = useMemo(() => {
    const rec = scenario ? IMAGE_SCENARIOS.find((s) => s.id === scenario)?.recommendedProviders : null;
    return caps.map((c) => {
      const recommended = rec?.includes(c.provider);
      const disabled = !isProviderSelectable(c);
      const suffix =
        c.status === 'planned' ? '（后续支持）' : recommended ? '（推荐）' : '';
      return {
        value: c.provider,
        disabled,
        label: `${c.displayName}${suffix}`,
      };
    });
  }, [caps, scenario]);

  const visibleFieldKeys = PROVIDER_FIELD_KEYS[provider || 'noop'] ?? ['timeout_sec'];

  const buildTestSettingsPayload = (values: Record<string, unknown>): Record<string, string> => {
    const keys = new Set<string>(['provider', 'provider_preset', ...visibleFieldKeys]);
    const out: Record<string, string> = {};
    for (const key of keys) {
      const raw = values[key];
      if (raw == null) continue;
      const val = String(raw).trim();
      if (val === '') continue;
      if (ENCRYPTED_KEYS.has(key) && val.includes('****')) continue;
      out[key] = String(raw);
    }
    return out;
  };

  const onScenarioPick = (id: ImageScenarioId) => {
    setScenario(id);
    const sc = IMAGE_SCENARIOS.find((s) => s.id === id);
    const first = sc?.recommendedProviders.find((p) => {
      const c = caps.find((x) => x.provider === p);
      return c && isProviderSelectable(c);
    });
    if (first) {
      form.setFieldsValue({ provider: first, provider_preset: id });
    }
  };

  const renderField = (key: string) => {
    const spec = ALL_IMAGE_FIELD_SPECS[key];
    if (!spec) return null;
    const isJson = spec.valueType === 'json';
    const isEnc = ENCRYPTED_KEYS.has(key);
    const isNum = key.includes('_sec') || key === 'timeout_sec' || key.includes('poll') || key.includes('interval');

    if (isNum) {
      return (
        <Form.Item key={key} label={spec.label} name={key} extra={spec.extra}>
          <InputNumber min={key === 'timeout_sec' ? 5 : 1} max={key.includes('max_poll') ? 7200 : 3600} style={{ width: '100%' }} />
        </Form.Item>
      );
    }
    if (isJson) {
      return (
        <Form.Item key={key} label={spec.label} name={key} extra={spec.extra}>
          <Input.TextArea rows={12} placeholder="{}" style={{ fontFamily: 'monospace', fontSize: 12 }} />
        </Form.Item>
      );
    }
    if (isEnc) {
      return (
        <Form.Item key={key} label={spec.label} name={key} extra={spec.extra}>
          <Input.Password placeholder="留空不修改；填写新 Key 保存" autoComplete="new-password" />
        </Form.Item>
      );
    }
    return (
      <Form.Item key={key} label={spec.label} name={key} extra={spec.extra}>
        <Input placeholder={spec.placeholder} />
      </Form.Item>
    );
  };

  return (
    <PageContainer title="图片 AI 设置">
      <ProCard bordered style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="图片 AI 用于商品图去背景、生成场景图、替换背景"
          description="你可以选择云端服务，也可以选择本地 ComfyUI。所有请求由系统后端发起；API 密钥需自行到对应控制台申请。测试与生成可能产生费用。"
        />
      </ProCard>

      <ProCard title="1. 选择使用场景" bordered style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          {IMAGE_SCENARIOS.map((sc) => (
            <Col xs={24} sm={12} key={sc.id}>
              <Card
                hoverable
                size="small"
                type={scenario === sc.id ? 'inner' : undefined}
                style={{
                  borderColor: scenario === sc.id ? 'var(--ant-color-primary)' : undefined,
                  cursor: 'pointer',
                }}
                onClick={() => onScenarioPick(sc.id)}
              >
                <Typography.Text strong>{sc.title}</Typography.Text>
                <div style={{ marginTop: 8, color: 'var(--ant-color-text-secondary)', fontSize: 13 }}>
                  {sc.description}
                </div>
                <div style={{ marginTop: 8 }}>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    推荐：
                  </Typography.Text>
                  {sc.recommendedProviders.map((p) => {
                    const c = caps.find((x) => x.provider === p);
                    return (
                      <Tag key={p} style={{ marginTop: 4 }}>
                        {c?.displayName ?? p}
                      </Tag>
                    );
                  })}
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      </ProCard>

      <ProCard
        title="2. 选择图片处理服务"
        bordered
        style={{ marginBottom: 16 }}
        extra={
          <Button type="link" onClick={load} disabled={loading}>
            重新加载
          </Button>
        }
      >
        <Form form={form} layout="vertical" style={{ maxWidth: 800 }}>
          <Form.Item
            label="默认图片服务"
            name="provider"
            rules={[
              { required: true },
              {
                validator: async (_, v) => {
                  const c = caps.find((x) => x.provider === v);
                  if (c?.status === 'planned') {
                    throw new Error('该图片服务尚未开放，不能设为默认');
                  }
                },
              },
            ]}
            extra="请到对应服务商控制台申请 API 密钥；留空占位 **** 不会覆盖已保存的密钥"
          >
            <Select options={providerOptions} />
          </Form.Item>
          <Form.Item name="provider_preset" hidden>
            <Input />
          </Form.Item>
          {currentCap ? (
            <Alert
              type={currentCap.status === 'planned' ? 'warning' : 'info'}
              showIcon
              style={{ marginBottom: 16 }}
              message={currentCap.displayName}
              description={
                <>
                  <div>{currentCap.description}</div>
                  {currentCap.recommendedFor?.length ? (
                    <div style={{ marginTop: 4 }}>适合：{currentCap.recommendedFor.join('、')}</div>
                  ) : null}
                  <ProviderMetaTags cap={currentCap} />
                </>
              }
            />
          ) : null}
        </Form>
      </ProCard>

      <ProCard title="3. 填写当前图片服务配置" bordered>
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 720 }}
          onFinish={async (values) => {
            try {
              const payload = {
                ...values,
                timeout_sec: String(values.timeout_sec ?? ''),
                comfyui_timeout_sec: String(values.comfyui_timeout_sec ?? ''),
                comfyui_poll_interval_seconds: String(values.comfyui_poll_interval_seconds ?? ''),
                comfyui_max_poll_seconds: String(values.comfyui_max_poll_seconds ?? ''),
                comfyui_workflow_json: String(values.comfyui_workflow_json ?? ''),
              };
              await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          {currentCap?.status === 'planned' ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message="该图片服务为预留项"
              description="可保存配置项，但无法创建真实图片任务，请等待后续版本。"
            />
          ) : null}
          {visibleFieldKeys.map((k) => renderField(k))}
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>
                保存
              </Button>
              <Button
                loading={testing}
                disabled={!provider || currentCap?.status === 'planned'}
                onClick={async () => {
                  setTesting(true);
                  try {
                    const values = await form.validateFields();
                    const res = await testImageProvider({
                      provider: provider || undefined,
                      testMode: 'config_only',
                      settings: buildTestSettingsPayload(values),
                    });
                    const latency = res.latencyMs !== undefined ? `（${res.latencyMs} ms）` : '';
                    if (res.ok) {
                      message.success(`${res.message || '配置检查通过'}${latency}`);
                    } else {
                      message.warning(`${res.message || '配置不完整'}${latency}`);
                    }
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '测试失败');
                  } finally {
                    setTesting(false);
                  }
                }}
              >
                测试配置
              </Button>
            </Space>
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
            真实调用测试与图片生成可能产生费用；ComfyUI 需自行部署可访问实例。
          </Typography.Paragraph>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
