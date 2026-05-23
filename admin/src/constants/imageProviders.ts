/** Image provider capability from GET /api/v1/image/providers */
export type ImageProviderCapability = {
  provider: string;
  displayName: string;
  status: 'available' | 'beta' | 'planned' | 'disabled';
  difficulty: 'easy' | 'medium' | 'advanced';
  regionFriendly: 'global' | 'china' | 'both';
  requiresApiKey: boolean;
  requiresSelfHosted: boolean;
  supportedTasks: string[];
  recommendedFor: string[];
  docsUrl: string;
  description?: string;
};

export type ImageScenarioId =
  | 'remove_bg'
  | 'generate_scene'
  | 'replace_bg'
  | 'comfyui_custom';

export const IMAGE_SCENARIOS: {
  id: ImageScenarioId;
  title: string;
  description: string;
  recommendedProviders: string[];
}[] = [
  {
    id: 'remove_bg',
    title: '我只想去背景',
    description: '适合商品白底图、抠图。',
    recommendedProviders: ['removebg'],
  },
  {
    id: 'generate_scene',
    title: '我想生成商品场景图',
    description: '适合把商品放到厨房、户外、办公室等场景。',
    recommendedProviders: ['dashscope_image', 'openai_image', 'volcengine_image', 'siliconflow_image'],
  },
  {
    id: 'replace_bg',
    title: '我想替换商品背景',
    description: '适合保留商品主体，替换背景。',
    recommendedProviders: ['openai_image', 'comfyui', 'dashscope_image'],
  },
  {
    id: 'comfyui_custom',
    title: '我有自己的 ComfyUI 工作流',
    description: '适合会部署本地 AI 绘图工作流的用户。',
    recommendedProviders: ['comfyui'],
  },
];

const DIFFICULTY_LABEL: Record<string, string> = {
  easy: '简单',
  medium: '中等',
  advanced: '高级',
};

const REGION_LABEL: Record<string, string> = {
  global: '国际',
  china: '国内友好',
  both: '通用',
};

const STATUS_LABEL: Record<string, string> = {
  available: '可用',
  beta: '测试',
  planned: '后续支持',
  disabled: '停用',
};

export function providerDifficultyLabel(d: string) {
  return DIFFICULTY_LABEL[d] ?? d;
}

export function providerRegionLabel(r: string) {
  return REGION_LABEL[r] ?? r;
}

export function providerStatusLabel(s: string) {
  return STATUS_LABEL[s] ?? s;
}

export function isProviderSelectable(cap: ImageProviderCapability) {
  return cap.status === 'available' || cap.status === 'beta';
}

export function providersForTask(
  caps: ImageProviderCapability[],
  taskType: string,
): ImageProviderCapability[] {
  const tt = taskType.trim().toLowerCase();
  return caps.filter(
    (c) => isProviderSelectable(c) && c.supportedTasks.some((t) => t.toLowerCase() === tt),
  );
}

export function displayNameForProvider(caps: ImageProviderCapability[], provider: string) {
  const p = provider.trim().toLowerCase();
  if (p === 'ai_vision') {
    return 'AI 视觉模型';
  }
  const hit = caps.find((c) => c.provider === p);
  return hit?.displayName ?? provider;
}

/** AI 图片任务背景预设：界面中文，提交英文 prompt 片段 */
export type ImagePromptPreset = { label: string; value: string };

export const DEFAULT_AI_IMAGE_BACKGROUND = 'white studio background';
export const DEFAULT_AI_IMAGE_STYLE = 'clean ecommerce';

export const AI_IMAGE_BACKGROUND_PRESETS: ImagePromptPreset[] = [
  { label: '白色摄影棚背景', value: 'white studio background' },
  { label: '纯白背景', value: 'pure white background' },
  { label: '浅灰渐变背景', value: 'light gray gradient background' },
  { label: '木质桌面', value: 'wooden tabletop' },
  { label: '厨房台面', value: 'kitchen counter' },
  { label: '户外花园', value: 'outdoor garden' },
  { label: '办公桌面', value: 'office desk' },
  { label: '居家生活场景', value: 'cozy home lifestyle setting' },
];

export const AI_IMAGE_STYLE_PRESETS: ImagePromptPreset[] = [
  { label: '干净电商风', value: 'clean ecommerce' },
  { label: '极简产品摄影', value: 'minimalist product photo' },
  { label: '生活方式拍摄', value: 'lifestyle product shot' },
  { label: '高级奢华感', value: 'premium luxury product styling' },
  { label: '明亮通透', value: 'bright and airy' },
  { label: '自然光感', value: 'natural soft lighting' },
];

export function aiImageBackgroundLabel(value: string): string {
  const v = value.trim();
  const hit = AI_IMAGE_BACKGROUND_PRESETS.find((p) => p.value === v);
  return hit?.label ?? v;
}

export function aiImageStyleLabel(value: string): string {
  const v = value.trim();
  const hit = AI_IMAGE_STYLE_PRESETS.find((p) => p.value === v);
  return hit?.label ?? v;
}

/** 场景预设：界面中文，提交英文 prompt 片段 */
export const AI_IMAGE_SCENE_PRESETS: ImagePromptPreset[] = [
  { label: '简约摄影棚', value: 'minimal studio' },
  { label: '现代厨房', value: 'modern kitchen' },
  { label: '户外自然光', value: 'outdoor natural setting' },
  { label: '客厅居家', value: 'cozy living room' },
  { label: '办公桌场景', value: 'office desk setup' },
  { label: '节日礼盒氛围', value: 'festive gift presentation' },
];

export const DEFAULT_AI_IMAGE_SCENE = 'minimal studio';

/** 反向提示词常用项：界面中文，提交英文 negative prompt 片段 */
export const AI_IMAGE_NEGATIVE_PROMPT_PRESETS: ImagePromptPreset[] = [
  { label: '水印与文字', value: 'watermark, logo, text overlay, caption' },
  { label: '模糊失焦', value: 'blurry, out of focus, motion blur' },
  { label: '畸形失真', value: 'deformed, distorted, bad anatomy, extra limbs' },
  { label: '低画质', value: 'low quality, jpeg artifacts, pixelated, noisy' },
  { label: '杂乱背景', value: 'cluttered background, messy scene, distracting elements' },
];

export function aiImageNegativePromptLabel(value: string): string {
  const v = value.trim();
  const hit = AI_IMAGE_NEGATIVE_PROMPT_PRESETS.find((p) => p.value === v);
  return hit?.label ?? v;
}

/** AI 图片任务表单字段文案 */
export const AI_IMAGE_FIELD = {
  prompt: {
    label: '画面描述',
    placeholder: '例如：商品居中摆放，柔和侧光，突出毛绒材质与细节',
    extra: '补充摆放、光线、构图等要求；系统将结合商品标题与背景/风格生成提示词',
  },
  negativePrompt: {
    label: '排除内容',
    subLabel: '反向提示词',
    placeholder: '例如：不要水印、文字、模糊、畸形、低画质（可点击下方常用项快速填入）',
    extra: '描述不希望出现在画面中的元素，提交时将转为英文反向提示词',
    presetsLabel: '常用排除项',
  },
  background: {
    label: '背景',
    placeholder: '选择背景或目标背景',
  },
  style: {
    label: '风格',
    placeholder: '选择画面风格',
  },
  scene: {
    label: '场景',
    placeholder: '选择商品所处场景',
    extra: '场景描述将写入 AI 提示词',
  },
} as const;

/** Fields shown per provider on settings page */
export const PROVIDER_FIELD_KEYS: Record<string, string[]> = {
  noop: ['timeout_sec'],
  removebg: ['removebg_base_url', 'removebg_api_key', 'timeout_sec'],
  openai_image: [
    'openai_image_base_url',
    'openai_image_api_key',
    'openai_image_model',
    'openai_image_size',
    'openai_image_quality',
    'openai_image_background',
    'timeout_sec',
  ],
  comfyui: [
    'comfyui_base_url',
    'comfyui_api_key',
    'comfyui_workflow_json',
    'comfyui_prompt_node_id',
    'comfyui_image_node_id',
    'comfyui_output_node_id',
    'comfyui_timeout_sec',
    'comfyui_poll_interval_seconds',
    'comfyui_max_poll_seconds',
    'timeout_sec',
  ],
  dashscope_image: [
    'dashscope_image_base_url',
    'dashscope_image_api_key',
    'dashscope_image_model',
    'dashscope_image_size',
    'dashscope_image_quality',
    'timeout_sec',
  ],
  volcengine_image: [
    'volcengine_image_base_url',
    'volcengine_image_api_key',
    'volcengine_image_model',
    'volcengine_image_size',
    'timeout_sec',
  ],
  siliconflow_image: [
    'siliconflow_image_base_url',
    'siliconflow_image_api_key',
    'siliconflow_image_model',
    'siliconflow_image_size',
    'timeout_sec',
  ],
  hunyuan_image: ['hunyuan_image_base_url', 'hunyuan_image_api_key', 'hunyuan_image_model'],
};

export const ALL_IMAGE_FIELD_SPECS: Record<
  string,
  { encrypted?: boolean; valueType?: 'json'; label: string; extra?: string; placeholder?: string }
> = {
  provider: { label: '默认图片服务' },
  provider_preset: { label: '场景预设（内部）' },
  image_task_default_size: { label: '任务默认尺寸' },
  image_task_default_quality: { label: '任务默认质量' },
  removebg_base_url: {
    label: 'remove.bg 接口地址',
    extra: '默认 https://api.remove.bg/v1.0',
    placeholder: 'https://api.remove.bg/v1.0',
  },
  removebg_api_key: { label: 'remove.bg API Key', encrypted: true, extra: '请到 remove.bg 控制台申请' },
  openai_image_base_url: {
    label: 'OpenAI 图片接口地址',
    placeholder: 'https://api.openai.com/v1',
    extra: '留空则使用 OpenAI 官方地址；可用兼容代理',
  },
  openai_image_api_key: { label: 'OpenAI Image API Key', encrypted: true, extra: '请到 OpenAI 或代理服务商控制台申请' },
  openai_image_model: { label: 'OpenAI Image 模型', placeholder: 'gpt-image-1' },
  openai_image_size: { label: 'OpenAI Image 尺寸', placeholder: '1024x1024' },
  openai_image_quality: { label: 'OpenAI Image 质量', placeholder: 'standard / medium / high' },
  openai_image_background: { label: 'OpenAI Image 背景（可选）', placeholder: 'transparent | opaque | auto' },
  comfyui_base_url: { label: 'ComfyUI 地址', placeholder: 'http://127.0.0.1:8188', extra: '高级：需自行部署' },
  comfyui_api_key: { label: 'ComfyUI API Key（可选）', encrypted: true },
  comfyui_workflow_json: {
    label: 'ComfyUI 工作流 JSON',
    valueType: 'json',
    extra: '支持占位符 {{prompt}}、{{sourceImageUrl}} 等',
  },
  comfyui_prompt_node_id: { label: '提示词节点 ID', placeholder: '例如 6' },
  comfyui_image_node_id: { label: '载入图片节点 ID', placeholder: '例如 10' },
  comfyui_output_node_id: { label: '输出节点 ID', placeholder: '例如 9' },
  comfyui_timeout_sec: { label: 'ComfyUI HTTP 超时（秒）' },
  comfyui_poll_interval_seconds: { label: 'ComfyUI 轮询间隔（秒）' },
  comfyui_max_poll_seconds: { label: 'ComfyUI 最长等待（秒）' },
  dashscope_image_base_url: {
    label: '通义万相接口地址',
    placeholder: '留空使用官方',
    extra: '阿里云百炼 / DashScope',
  },
  dashscope_image_api_key: { label: '通义万相 API Key', encrypted: true, extra: '请到阿里云百炼控制台申请' },
  dashscope_image_model: { label: '通义万相模型', placeholder: 'wan2.7-image-pro' },
  dashscope_image_size: { label: '通义万相尺寸', placeholder: '2K', extra: '推荐 1K / 2K / 4K；也兼容旧像素格式如 1024*1024' },
  dashscope_image_quality: { label: '通义万相质量（可选）' },
  volcengine_image_base_url: {
    label: '火山方舟接口地址',
    placeholder: 'https://ark.cn-beijing.volces.com/api/v3',
  },
  volcengine_image_api_key: { label: '火山方舟 API Key', encrypted: true, extra: '请到火山引擎控制台申请' },
  volcengine_image_model: { label: '火山方舟模型 Endpoint ID', placeholder: 'doubao-seedream-3-0-t2i' },
  volcengine_image_size: { label: '火山方舟尺寸', placeholder: '1024x1024' },
  siliconflow_image_base_url: {
    label: '硅基流动接口地址',
    placeholder: 'https://api.siliconflow.cn/v1',
  },
  siliconflow_image_api_key: { label: '硅基流动 API Key', encrypted: true, extra: '请到硅基流动控制台申请' },
  siliconflow_image_model: { label: '硅基流动模型', placeholder: 'black-forest-labs/FLUX.1-schnell' },
  siliconflow_image_size: { label: '硅基流动尺寸', placeholder: '1024x1024' },
  hunyuan_image_base_url: { label: '腾讯混元接口地址（预留）', extra: '后续版本支持' },
  hunyuan_image_api_key: { label: '腾讯混元 API Key（预留）', encrypted: true },
  hunyuan_image_model: { label: '腾讯混元模型（预留）' },
  timeout_sec: { label: '通用图片任务超时（秒）' },
};
