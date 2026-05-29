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
import { fetchSettingsList, saveSettingsItems, testOCRConnection } from '@/services/settings';
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
  'ocr_api_key',
  'ocr_secret',
  'ocr_aliyun_access_key_id',
  'ocr_aliyun_access_key_secret',
  'ocr_tencent_secret_id',
  'ocr_tencent_secret_key',
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

const OCR_PROVIDER_OPTIONS = [
  { label: 'AI 视觉 OCR', value: 'ai_vision' },
  { label: '本地 PaddleOCR', value: 'paddleocr' },
  { label: '阿里云 OCR', value: 'aliyun' },
  { label: '腾讯云 OCR', value: 'tencent' },
];

const TENCENT_OCR_API_OPTIONS = [
  { label: 'GeneralBasicOCR：通用印刷体识别，推荐默认', value: 'GeneralBasicOCR' },
  { label: 'GeneralFastOCR：通用印刷体识别高速版，可选', value: 'GeneralFastOCR' },
];

function hasSavedSecretValue(value?: string) {
  return Boolean((value ?? '').trim());
}

function currentOrSaved(values: Record<string, unknown>, key: string, savedKeys: Set<string>) {
  return String(values[key] ?? '').trim() || (savedKeys.has(key) ? '__saved__' : '');
}

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
  const [ocrTesting, setOcrTesting] = useState(false);
  const [caps, setCaps] = useState<ImageProviderCapability[]>([]);
  const [scenario, setScenario] = useState<ImageScenarioId | ''>('');
  const [savedEncryptedKeys, setSavedEncryptedKeys] = useState<Set<string>>(new Set());
  const provider = Form.useWatch('provider', form) as string | undefined;
  const ocrProvider = Form.useWatch('ocr_provider', form) as string | undefined;

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
      setSavedEncryptedKeys(new Set([...ENCRYPTED_KEYS].filter((key) => hasSavedSecretValue(g[key]))));
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
        ocr_provider: g.ocr_provider || 'paddleocr',
        ocr_base_url: g.ocr_base_url || '',
        ocr_api_key: '',
        ocr_secret: '',
        ocr_paddleocr_service_url: g.ocr_paddleocr_service_url || g.ocr_base_url || 'http://127.0.0.1:3101',
        ocr_aliyun_endpoint: g.ocr_aliyun_endpoint || 'ocr-api.cn-hangzhou.aliyuncs.com',
        ocr_aliyun_region: g.ocr_aliyun_region || 'cn-hangzhou',
        ocr_aliyun_api_name: g.ocr_aliyun_api_name || 'RecognizeGeneral',
        ocr_aliyun_access_key_id: '',
        ocr_aliyun_access_key_secret: '',
        ocr_tencent_endpoint: g.ocr_tencent_endpoint || 'ocr.tencentcloudapi.com',
        ocr_tencent_region: g.ocr_tencent_region || 'ap-guangzhou',
        ocr_tencent_api_name: g.ocr_tencent_api_name || 'GeneralBasicOCR',
        ocr_tencent_secret_id: '',
        ocr_tencent_secret_key: '',
        ocr_timeout_sec: g.ocr_timeout_sec ? Number(g.ocr_timeout_sec) : 30,
        ocr_min_confidence: g.ocr_min_confidence || '0.75',
        ocr_fallback_to_vision: g.ocr_fallback_to_vision || 'false',
        ocr_batch_concurrency: g.ocr_batch_concurrency ? Number(g.ocr_batch_concurrency) : 1,
        ocr_request_interval_ms: g.ocr_request_interval_ms ? Number(g.ocr_request_interval_ms) : 500,
        ocr_max_retries: g.ocr_max_retries ? Number(g.ocr_max_retries) : 1,
        erase_mode: g.erase_mode || 'auto',
        ai_inpaint_comfyui_base_url: g.ai_inpaint_comfyui_base_url || 'http://127.0.0.1:8188',
        ai_inpaint_comfyui_workflow_json: g.ai_inpaint_comfyui_workflow_json || '',
        ai_inpaint_comfyui_prompt_node_id: g.ai_inpaint_comfyui_prompt_node_id || '',
        ai_inpaint_comfyui_image_node_id: g.ai_inpaint_comfyui_image_node_id || '',
        ai_inpaint_comfyui_mask_node_id: g.ai_inpaint_comfyui_mask_node_id || '',
        ai_inpaint_comfyui_output_node_id: g.ai_inpaint_comfyui_output_node_id || '',
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

  const buildOCRTestSettingsPayload = (values: Record<string, unknown>): Record<string, string> => {
    const keys = [
      'ocr_provider',
      'ocr_base_url',
      'ocr_api_key',
      'ocr_secret',
      'ocr_paddleocr_service_url',
      'ocr_aliyun_endpoint',
      'ocr_aliyun_region',
      'ocr_aliyun_api_name',
      'ocr_aliyun_access_key_id',
      'ocr_aliyun_access_key_secret',
      'ocr_tencent_endpoint',
      'ocr_tencent_region',
      'ocr_tencent_api_name',
      'ocr_tencent_secret_id',
      'ocr_tencent_secret_key',
      'ocr_timeout_sec',
      'ocr_min_confidence',
      'ocr_fallback_to_vision',
      'ocr_batch_concurrency',
      'ocr_request_interval_ms',
      'ocr_max_retries',
    ];
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
    const isNum = key.includes('_sec') || key === 'timeout_sec' || key.includes('poll') || key.includes('interval') || key.includes('concurrency') || key.includes('retries');

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
      const saved = savedEncryptedKeys.has(key);
      return (
        <Form.Item
          key={key}
          label={spec.label}
          name={key}
          extra={saved ? `${spec.extra ?? ''} 已加密保存；为安全起见不回显明文，留空不会覆盖。` : spec.extra}
        >
          <Input.Password
            placeholder={saved ? '已保存，留空不修改；填写新 Key 才会覆盖' : '填写后保存'}
            autoComplete="new-password"
          />
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
                ocr_timeout_sec: String(values.ocr_timeout_sec ?? ''),
                ocr_fallback_to_vision: 'false',
                ai_inpaint_comfyui_workflow_json: String(values.ai_inpaint_comfyui_workflow_json ?? ''),
                ocr_batch_concurrency: String(values.ocr_batch_concurrency ?? ''),
                ocr_request_interval_ms: String(values.ocr_request_interval_ms ?? ''),
                ocr_max_retries: String(values.ocr_max_retries ?? ''),
              };
              await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
              message.success('已保存。敏感密钥已加密保存，页面不会回显明文。');
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
          
          <Typography.Title level={5} style={{ marginTop: 32, marginBottom: 16 }}>
            OCR 配置（用于图片文字翻译）
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label={ALL_IMAGE_FIELD_SPECS.ocr_provider.label}
                name="ocr_provider"
                extra="OCR 主要用于图片文字翻译、图片中文字识别和翻译后结果校验"
                rules={[{ required: true, message: '请选择 OCR 服务' }]}
              >
                <Select options={OCR_PROVIDER_OPTIONS} />
              </Form.Item>
            </Col>
          </Row>
          {ocrProvider === 'ai_vision' ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="使用当前 AI 设置中的视觉模型识别图片文字"
              description="无需填写 OCR 服务地址。请确保「设置 → AI 设置」里配置的是支持图片输入的视觉模型，例如 qwen3-vl-plus、gpt-4o-mini 等。"
            />
          ) : null}
          {ocrProvider === 'paddleocr' ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="PaddleOCR 使用本地或内网服务"
              description="请填写可由后端访问的服务地址，例如 http://127.0.0.1:xxxx。开启失败自动降级后，PaddleOCR 不可用时图片文字翻译会使用 AI 视觉 OCR 兜底。"
            />
          ) : null}
          {ocrProvider === 'aliyun' ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="阿里云 OCR 适合国内生产环境"
              description="请在阿里云控制台开通 OCR 服务并创建 AccessKeyId / AccessKeySecret。"
            />
          ) : null}
          {ocrProvider === 'tencent' ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="腾讯云 OCR 适合国内生产环境"
              description="请先在腾讯云控制台开通文字识别 OCR 服务，并创建 SecretId / SecretKey。"
            />
          ) : null}
          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.ocr_provider !== currentValues.ocr_provider}
          >
            {({ getFieldValue }) => {
              const selectedOCRProvider = getFieldValue('ocr_provider');
              if (selectedOCRProvider === 'paddleocr') {
                return (
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item
                        label={ALL_IMAGE_FIELD_SPECS.ocr_paddleocr_service_url.label}
                        name="ocr_paddleocr_service_url"
                        extra={ALL_IMAGE_FIELD_SPECS.ocr_paddleocr_service_url.extra}
                        rules={[{ required: true, message: '请填写 PaddleOCR 服务地址' }]}
                      >
                        <Input placeholder={ALL_IMAGE_FIELD_SPECS.ocr_paddleocr_service_url.placeholder} />
                      </Form.Item>
                    </Col>
                  </Row>
                );
              }
              if (selectedOCRProvider === 'aliyun') {
                return (
                  <>
                    <Row gutter={16}>
                      <Col span={12}>{renderField('ocr_aliyun_endpoint')}</Col>
                      <Col span={12}>{renderField('ocr_aliyun_region')}</Col>
                      <Col span={12}>{renderField('ocr_aliyun_api_name')}</Col>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_aliyun_access_key_id.label}
                          name="ocr_aliyun_access_key_id"
                          extra={
                            savedEncryptedKeys.has('ocr_aliyun_access_key_id')
                              ? '已加密保存；为安全起见不回显明文，留空不会覆盖。'
                              : undefined
                          }
                          rules={
                            savedEncryptedKeys.has('ocr_aliyun_access_key_id')
                              ? []
                              : [{ required: true, message: '请填写阿里云 AccessKeyId' }]
                          }
                        >
                          <Input.Password
                            placeholder={
                              savedEncryptedKeys.has('ocr_aliyun_access_key_id')
                                ? '已保存，留空不修改；填写新 AccessKeyId 才会覆盖'
                                : '填写 AccessKeyId 后保存或直接测试'
                            }
                            autoComplete="new-password"
                          />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_aliyun_access_key_secret.label}
                          name="ocr_aliyun_access_key_secret"
                          extra={
                            savedEncryptedKeys.has('ocr_aliyun_access_key_secret')
                              ? '已加密保存；为安全起见不回显明文，留空不会覆盖。'
                              : undefined
                          }
                          rules={
                            savedEncryptedKeys.has('ocr_aliyun_access_key_secret')
                              ? []
                              : [{ required: true, message: '请填写阿里云 AccessKeySecret' }]
                          }
                        >
                          <Input.Password
                            placeholder={
                              savedEncryptedKeys.has('ocr_aliyun_access_key_secret')
                                ? '已保存，留空不修改；填写新 AccessKeySecret 才会覆盖'
                                : '填写 AccessKeySecret 后保存或直接测试'
                            }
                            autoComplete="new-password"
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                  </>
                );
              }
              if (selectedOCRProvider === 'tencent') {
                return (
                  <>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_tencent_endpoint.label}
                          name="ocr_tencent_endpoint"
                          rules={[{ required: true, message: '请填写腾讯云 OCR Endpoint' }]}
                        >
                          <Input placeholder={ALL_IMAGE_FIELD_SPECS.ocr_tencent_endpoint.placeholder} />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_tencent_region.label}
                          name="ocr_tencent_region"
                          rules={[{ required: true, message: '请填写腾讯云 OCR Region' }]}
                        >
                          <Input placeholder={ALL_IMAGE_FIELD_SPECS.ocr_tencent_region.placeholder} />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_tencent_secret_id.label}
                          name="ocr_tencent_secret_id"
                          extra={
                            savedEncryptedKeys.has('ocr_tencent_secret_id')
                              ? '已加密保存；为安全起见不回显明文，留空不会覆盖。'
                              : undefined
                          }
                          rules={
                            savedEncryptedKeys.has('ocr_tencent_secret_id')
                              ? []
                              : [{ required: true, message: '请填写腾讯云 SecretId' }]
                          }
                        >
                          <Input.Password
                            placeholder={
                              savedEncryptedKeys.has('ocr_tencent_secret_id')
                                ? '已保存，留空不修改；填写新 SecretId 才会覆盖'
                                : '填写 SecretId 后保存'
                            }
                            autoComplete="new-password"
                          />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_tencent_secret_key.label}
                          name="ocr_tencent_secret_key"
                          extra={
                            savedEncryptedKeys.has('ocr_tencent_secret_key')
                              ? '已加密保存；为安全起见不回显明文，留空不会覆盖。'
                              : undefined
                          }
                          rules={
                            savedEncryptedKeys.has('ocr_tencent_secret_key')
                              ? []
                              : [{ required: true, message: '请填写腾讯云 SecretKey' }]
                          }
                        >
                          <Input.Password
                            placeholder={
                              savedEncryptedKeys.has('ocr_tencent_secret_key')
                                ? '已保存，留空不修改；填写新 SecretKey 才会覆盖'
                                : '填写 SecretKey 后保存'
                            }
                            autoComplete="new-password"
                          />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label={ALL_IMAGE_FIELD_SPECS.ocr_tencent_api_name.label}
                          name="ocr_tencent_api_name"
                          rules={[{ required: true, message: '请选择腾讯云 OCR 接口类型' }]}
                        >
                          <Select options={TENCENT_OCR_API_OPTIONS} />
                        </Form.Item>
                      </Col>
                    </Row>
                  </>
                );
              }
              return (
                null
              );
            }}
          </Form.Item>
          <Row gutter={16}>
            <Col span={8}>{renderField('ocr_timeout_sec')}</Col>
            <Col span={8}>{renderField('ocr_min_confidence')}</Col>
            <Col span={8} style={{ display: 'none' }}>
              <Form.Item
                label={ALL_IMAGE_FIELD_SPECS.ocr_fallback_to_vision.label}
                name="ocr_fallback_to_vision"
                extra="生产模式不使用 OCR 降级；保留该字段仅用于历史兼容"
              >
                <Select
                  options={[
                    { label: '开启', value: 'true' },
                    { label: '关闭', value: 'false' },
                  ]}
                />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={8}>{renderField('ocr_batch_concurrency')}</Col>
            <Col span={8}>{renderField('ocr_request_interval_ms')}</Col>
            <Col span={8}>{renderField('ocr_max_retries')}</Col>
          </Row>
          {ocrProvider && ocrProvider !== 'ai_vision' ? (
            <Form.Item style={{ marginTop: -8 }}>
              <Button
                loading={ocrTesting}
                onClick={async () => {
                  setOcrTesting(true);
                  try {
                    const values = await form.validateFields([
                      'ocr_provider',
                      'ocr_base_url',
                      'ocr_api_key',
                      'ocr_secret',
                      'ocr_paddleocr_service_url',
                      'ocr_aliyun_endpoint',
                      'ocr_aliyun_region',
                      'ocr_aliyun_api_name',
                      'ocr_aliyun_access_key_id',
                      'ocr_aliyun_access_key_secret',
                      'ocr_tencent_endpoint',
                      'ocr_tencent_region',
                      'ocr_tencent_api_name',
                      'ocr_tencent_secret_id',
                      'ocr_tencent_secret_key',
                      'ocr_timeout_sec',
                      'ocr_min_confidence',
                      'ocr_fallback_to_vision',
                      'ocr_batch_concurrency',
                      'ocr_request_interval_ms',
                      'ocr_max_retries',
                    ]);
                    const selectedProvider = String(values.ocr_provider ?? '').trim();
                    if (selectedProvider === 'aliyun') {
                      if (!currentOrSaved(values, 'ocr_aliyun_access_key_id', savedEncryptedKeys)) {
                        message.error('请先填写阿里云 AccessKeyId，或保存后再测试 OCR。');
                        return;
                      }
                      if (!currentOrSaved(values, 'ocr_aliyun_access_key_secret', savedEncryptedKeys)) {
                        message.error('请先填写阿里云 AccessKeySecret，或保存后再测试 OCR。');
                        return;
                      }
                    }
                    if (selectedProvider === 'tencent') {
                      if (!currentOrSaved(values, 'ocr_tencent_secret_id', savedEncryptedKeys)) {
                        message.error('请先填写腾讯云 SecretId，或保存后再测试 OCR。');
                        return;
                      }
                      if (!currentOrSaved(values, 'ocr_tencent_secret_key', savedEncryptedKeys)) {
                        message.error('请先填写腾讯云 SecretKey，或保存后再测试 OCR。');
                        return;
                      }
                    }
                    const res = await testOCRConnection({
                      provider: values.ocr_provider || undefined,
                      settings: buildOCRTestSettingsPayload(values),
                    });
                    const latency = res.latencyMs !== undefined ? `（${res.latencyMs} ms）` : '';
                    const blocks = res.blocks !== undefined ? `，识别到 ${res.blocks} 个文字块` : '';
                    const avg =
                      res.averageConfidence !== undefined
                        ? `，平均置信度 ${(res.averageConfidence * 100).toFixed(1)}%`
                        : '';
                    message.success(`${res.message || '当前 OCR 服务可用'}${blocks}${avg}${latency}`);
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || 'OCR 配置测试失败');
                  } finally {
                    setOcrTesting(false);
                  }
                }}
              >
                真实测试 OCR 调用
              </Button>
            </Form.Item>
          ) : null}

          <Typography.Title level={5} style={{ marginTop: 32, marginBottom: 16 }}>
            局部擦除配置（用于图片文字翻译）
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>{renderField('erase_mode')}</Col>
          </Row>
          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.erase_mode !== currentValues.erase_mode}
          >
            {({ getFieldValue }) => {
              const eraseMode = getFieldValue('erase_mode');
              if (eraseMode !== 'ai_inpaint') return null;
              return (
                <>
                  <Row gutter={16}>
                    <Col span={12}>{renderField('ai_inpaint_comfyui_base_url')}</Col>
                  </Row>
                  {renderField('ai_inpaint_comfyui_workflow_json')}
                  <Row gutter={16}>
                    <Col span={12}>{renderField('ai_inpaint_comfyui_prompt_node_id')}</Col>
                    <Col span={12}>{renderField('ai_inpaint_comfyui_image_node_id')}</Col>
                    <Col span={12}>{renderField('ai_inpaint_comfyui_mask_node_id')}</Col>
                    <Col span={12}>{renderField('ai_inpaint_comfyui_output_node_id')}</Col>
                  </Row>
                </>
              );
            }}
          </Form.Item>

          <Form.Item style={{ marginTop: 16 }}>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>
                保存全部配置
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
                      message.success(`${res.message || '图片服务配置检查通过'}${latency}`);
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
                检查图片服务配置
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
