import { tr } from '@/i18n';
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
    get title() { return tr("我只想去背景"); },
    get description() { return tr("适合商品白底图、抠图。"); },
    recommendedProviders: ['removebg'],
  },
  {
    id: 'generate_scene',
    get title() { return tr("我想生成商品场景图"); },
    get description() { return tr("适合把商品放到厨房、户外、办公室等场景。"); },
    recommendedProviders: ['dashscope_image', 'openai_image', 'volcengine_image', 'siliconflow_image'],
  },
  {
    id: 'replace_bg',
    get title() { return tr("我想替换商品背景"); },
    get description() { return tr("适合保留商品主体，替换背景。"); },
    recommendedProviders: ['openai_image', 'comfyui', 'dashscope_image'],
  },
  {
    id: 'comfyui_custom',
    get title() { return tr("我有自己的 ComfyUI 工作流"); },
    get description() { return tr("适合会部署本地 AI 绘图工作流的用户。"); },
    recommendedProviders: ['comfyui'],
  },
];

const DIFFICULTY_LABEL: Record<string, string> = {
  get easy() { return tr("简单"); },
  get medium() { return tr("中等"); },
  get advanced() { return tr("高级"); },
};

const REGION_LABEL: Record<string, string> = {
  get global() { return tr("国际"); },
  get china() { return tr("国内友好"); },
  get both() { return tr("通用"); },
};

const STATUS_LABEL: Record<string, string> = {
  get available() { return tr("可用"); },
  get beta() { return tr("测试"); },
  get planned() { return tr("后续支持"); },
  get disabled() { return tr("停用"); },
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
    return tr("AI 视觉模型");
  }
  const hit = caps.find((c) => c.provider === p);
  return hit?.displayName ?? provider;
}

/** AI 图片任务背景预设：界面中文，提交英文 prompt 片段 */
export type ImagePromptPreset = { label: string; value: string };

export const DEFAULT_AI_IMAGE_BACKGROUND = 'white studio background';
export const DEFAULT_AI_IMAGE_STYLE = 'clean ecommerce';

export const AI_IMAGE_BACKGROUND_PRESETS: ImagePromptPreset[] = [
  { get label() { return tr("白色摄影棚背景"); }, value: 'white studio background' },
  { get label() { return tr("纯白背景"); }, value: 'pure white background' },
  { get label() { return tr("浅灰渐变背景"); }, value: 'light gray gradient background' },
  { get label() { return tr("木质桌面"); }, value: 'wooden tabletop' },
  { get label() { return tr("厨房台面"); }, value: 'kitchen counter' },
  { get label() { return tr("户外花园"); }, value: 'outdoor garden' },
  { get label() { return tr("办公桌面"); }, value: 'office desk' },
  { get label() { return tr("居家生活场景"); }, value: 'cozy home lifestyle setting' },
];

export const AI_IMAGE_STYLE_PRESETS: ImagePromptPreset[] = [
  { get label() { return tr("干净电商风"); }, value: 'clean ecommerce' },
  { get label() { return tr("极简产品摄影"); }, value: 'minimalist product photo' },
  { get label() { return tr("生活方式拍摄"); }, value: 'lifestyle product shot' },
  { get label() { return tr("高级奢华感"); }, value: 'premium luxury product styling' },
  { get label() { return tr("明亮通透"); }, value: 'bright and airy' },
  { get label() { return tr("自然光感"); }, value: 'natural soft lighting' },
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
  { get label() { return tr("简约摄影棚"); }, value: 'minimal studio' },
  { get label() { return tr("现代厨房"); }, value: 'modern kitchen' },
  { get label() { return tr("户外自然光"); }, value: 'outdoor natural setting' },
  { get label() { return tr("客厅居家"); }, value: 'cozy living room' },
  { get label() { return tr("办公桌场景"); }, value: 'office desk setup' },
  { get label() { return tr("节日礼盒氛围"); }, value: 'festive gift presentation' },
];

export const DEFAULT_AI_IMAGE_SCENE = 'minimal studio';

/** 反向提示词常用项：界面中文，提交英文 negative prompt 片段 */
export const AI_IMAGE_NEGATIVE_PROMPT_PRESETS: ImagePromptPreset[] = [
  { get label() { return tr("水印与文字"); }, value: 'watermark, logo, text overlay, caption' },
  { get label() { return tr("模糊失焦"); }, value: 'blurry, out of focus, motion blur' },
  { get label() { return tr("畸形失真"); }, value: 'deformed, distorted, bad anatomy, extra limbs' },
  { get label() { return tr("低画质"); }, value: 'low quality, jpeg artifacts, pixelated, noisy' },
  { get label() { return tr("杂乱背景"); }, value: 'cluttered background, messy scene, distracting elements' },
];

export function aiImageNegativePromptLabel(value: string): string {
  const v = value.trim();
  const hit = AI_IMAGE_NEGATIVE_PROMPT_PRESETS.find((p) => p.value === v);
  return hit?.label ?? v;
}

/** AI 图片任务表单字段文案 */
export const AI_IMAGE_FIELD = {
  prompt: {
    get label() { return tr("画面描述"); },
    get placeholder() { return tr("例如：商品居中摆放，柔和侧光，突出毛绒材质与细节"); },
    get extra() { return tr("补充摆放、光线、构图等要求；系统将结合商品标题与背景/风格生成提示词"); },
  },
  negativePrompt: {
    get label() { return tr("排除内容"); },
    get subLabel() { return tr("反向提示词"); },
    get placeholder() { return tr("例如：不要水印、文字、模糊、畸形、低画质（可点击下方常用项快速填入）"); },
    get extra() { return tr("描述不希望出现在画面中的元素，提交时将转为英文反向提示词"); },
    get presetsLabel() { return tr("常用排除项"); },
  },
  background: {
    get label() { return tr("背景"); },
    get placeholder() { return tr("选择背景或目标背景"); },
  },
  style: {
    get label() { return tr("风格"); },
    get placeholder() { return tr("选择画面风格"); },
  },
  scene: {
    get label() { return tr("场景"); },
    get placeholder() { return tr("选择商品所处场景"); },
    get extra() { return tr("场景描述将写入 AI 提示词"); },
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
  provider: { get label() { return tr("默认图片服务"); } },
  provider_preset: { get label() { return tr("场景预设（内部）"); } },
  image_task_default_size: { get label() { return tr("任务默认尺寸"); } },
  image_task_default_quality: { get label() { return tr("任务默认质量"); } },
  removebg_base_url: {
    get label() { return tr("remove.bg 接口地址"); },
    get extra() { return tr("默认 https://api.remove.bg/v1.0"); },
    placeholder: 'https://api.remove.bg/v1.0',
  },
  removebg_api_key: { get label() { return tr("remove.bg 接口密钥"); }, encrypted: true, get extra() { return tr("请到 remove.bg 控制台申请"); } },
  openai_image_base_url: {
    get label() { return tr("OpenAI 图片接口地址"); },
    placeholder: 'https://api.openai.com/v1',
    get extra() { return tr("留空则使用 OpenAI 官方地址；可用兼容代理"); },
  },
  openai_image_api_key: { get label() { return tr("OpenAI Image 接口密钥"); }, encrypted: true, get extra() { return tr("请到 OpenAI 或代理服务商控制台申请"); } },
  openai_image_model: { get label() { return tr("OpenAI Image 模型"); }, placeholder: 'gpt-image-1' },
  openai_image_size: { get label() { return tr("OpenAI Image 尺寸"); }, placeholder: '1024x1024' },
  openai_image_quality: { get label() { return tr("OpenAI Image 质量"); }, placeholder: 'standard / medium / high' },
  openai_image_background: { get label() { return tr("OpenAI Image 背景（可选）"); }, placeholder: 'transparent | opaque | auto' },
  comfyui_base_url: { get label() { return tr("ComfyUI 地址"); }, placeholder: 'http://127.0.0.1:8188', get extra() { return tr("高级：需自行部署"); } },
  comfyui_api_key: { get label() { return tr("ComfyUI 接口密钥（可选）"); }, encrypted: true },
  comfyui_workflow_json: {
    get label() { return tr("ComfyUI 工作流 JSON"); },
    valueType: 'json',
    get extra() { return tr("支持占位符 {{prompt}}、{{sourceImageUrl}} 等"); },
  },
  comfyui_prompt_node_id: { get label() { return tr("提示词节点 ID"); }, get placeholder() { return tr("例如 6"); } },
  comfyui_image_node_id: { get label() { return tr("载入图片节点 ID"); }, get placeholder() { return tr("例如 10"); } },
  comfyui_output_node_id: { get label() { return tr("输出节点 ID"); }, get placeholder() { return tr("例如 9"); } },
  comfyui_timeout_sec: { get label() { return tr("ComfyUI HTTP 超时（秒）"); } },
  comfyui_poll_interval_seconds: { get label() { return tr("ComfyUI 轮询间隔（秒）"); } },
  comfyui_max_poll_seconds: { get label() { return tr("ComfyUI 最长等待（秒）"); } },
  dashscope_image_base_url: {
    get label() { return tr("通义万相接口地址"); },
    get placeholder() { return tr("留空使用官方"); },
    get extra() { return tr("阿里云百炼 / DashScope"); },
  },
  dashscope_image_api_key: { get label() { return tr("通义万相 接口密钥"); }, encrypted: true, get extra() { return tr("请到阿里云百炼控制台申请"); } },
  dashscope_image_model: { get label() { return tr("通义万相模型"); }, placeholder: 'wan2.7-image-pro' },
  dashscope_image_size: { get label() { return tr("通义万相尺寸"); }, placeholder: '2K', get extra() { return tr("推荐 1K / 2K / 4K；也兼容旧像素格式如 1024*1024"); } },
  dashscope_image_quality: {
    get label() { return tr("通义万相质量（可选）"); },
    get placeholder() { return tr("留空即可"); },
    get extra() { return tr("一般无需填写；输出清晰度主要由「通义万相尺寸」控制（如 2K、4K 或 1024*1024）。若服务商文档要求质量参数，可填 standard 或 high"); },
  },
  volcengine_image_base_url: {
    get label() { return tr("火山方舟接口地址"); },
    placeholder: 'https://ark.cn-beijing.volces.com/api/v3',
  },
  volcengine_image_api_key: { get label() { return tr("火山方舟 接口密钥"); }, encrypted: true, get extra() { return tr("请到火山引擎控制台申请"); } },
  volcengine_image_model: { get label() { return tr("火山方舟模型接入点 ID"); }, placeholder: 'doubao-seedream-3-0-t2i' },
  volcengine_image_size: { get label() { return tr("火山方舟尺寸"); }, placeholder: '1024x1024' },
  siliconflow_image_base_url: {
    get label() { return tr("硅基流动接口地址"); },
    placeholder: 'https://api.siliconflow.cn/v1',
  },
  siliconflow_image_api_key: { get label() { return tr("硅基流动 接口密钥"); }, encrypted: true, get extra() { return tr("请到硅基流动控制台申请"); } },
  siliconflow_image_model: { get label() { return tr("硅基流动模型"); }, placeholder: 'black-forest-labs/FLUX.1-schnell' },
  siliconflow_image_size: { get label() { return tr("硅基流动尺寸"); }, placeholder: '1024x1024' },
  hunyuan_image_base_url: { get label() { return tr("腾讯混元接口地址（预留）"); }, get extra() { return tr("后续版本支持"); } },
  hunyuan_image_api_key: { get label() { return tr("腾讯混元 接口密钥（预留）"); }, encrypted: true },
  hunyuan_image_model: { get label() { return tr("腾讯混元模型（预留）"); } },
  timeout_sec: { get label() { return tr("通用图片任务超时（秒）"); } },
  ocr_provider: { get label() { return tr("OCR 服务"); }, placeholder: 'paddleocr', get extra() { return tr("可用: ai_vision / paddleocr / aliyun / tencent"); } },
  ocr_base_url: { get label() { return tr("OCR 服务地址"); }, placeholder: 'http://127.0.0.1:xxxx', get extra() { return tr("本地 PaddleOCR 必填，需填写后端可访问的服务地址"); } },
  ocr_api_key: { get label() { return tr("OCR 接口密钥"); }, encrypted: true, get extra() { return tr("云端 OCR 服务需要填写"); } },
  ocr_secret: { get label() { return tr("OCR 密钥"); }, encrypted: true, get extra() { return tr("云端 OCR 服务需要填写"); } },
  ocr_paddleocr_service_url: { get label() { return tr("PaddleOCR 服务地址"); }, placeholder: 'http://127.0.0.1:3101', get extra() { return tr("本地 PaddleOCR 必填，需填写后端可访问的服务地址"); } },
  ocr_aliyun_endpoint: { get label() { return tr("阿里云 OCR 接口地址"); }, placeholder: 'ocr-api.cn-hangzhou.aliyuncs.com' },
  ocr_aliyun_region: { get label() { return tr("阿里云 OCR 区域"); }, placeholder: 'cn-hangzhou' },
  ocr_aliyun_api_name: { get label() { return tr("阿里云 OCR 接口类型"); }, placeholder: 'RecognizeGeneral' },
  ocr_aliyun_access_key_id: { get label() { return tr("阿里云访问密钥 ID"); }, encrypted: true },
  ocr_aliyun_access_key_secret: { get label() { return tr("阿里云访问密钥"); }, encrypted: true },
  ocr_tencent_endpoint: { get label() { return tr("腾讯云 OCR 接口地址"); }, placeholder: 'ocr.tencentcloudapi.com' },
  ocr_tencent_region: { get label() { return tr("腾讯云 OCR 区域"); }, placeholder: 'ap-guangzhou' },
  ocr_tencent_api_name: { get label() { return tr("腾讯云 OCR 接口类型"); }, placeholder: 'GeneralBasicOCR' },
  ocr_tencent_secret_id: { get label() { return tr("腾讯云密钥 ID"); }, encrypted: true },
  ocr_tencent_secret_key: { get label() { return tr("腾讯云密钥"); }, encrypted: true },
  ocr_timeout_sec: { get label() { return tr("OCR 超时（秒）"); }, placeholder: '30' },
  ocr_min_confidence: { get label() { return tr("OCR 最低置信度"); }, placeholder: '0.75', get extra() { return tr("低于此置信度的文字将被忽略"); } },
  ocr_fallback_to_vision: { get label() { return tr("失败自动降级到 AI 视觉 OCR（高级实验，不用于生产）"); }, placeholder: 'false' },
  ocr_batch_concurrency: { get label() { return tr("单次批量并发"); }, placeholder: '1' },
  ocr_request_interval_ms: { get label() { return tr("OCR 请求间隔（毫秒）"); }, placeholder: '500' },
  ocr_max_retries: { get label() { return tr("OCR 最大重试次数"); }, placeholder: '1' },
  erase_mode: { get label() { return tr("默认擦除方式"); }, placeholder: 'auto', get extra() { return tr("可用: auto / precise_mask / background_sample / blur_fill / opencv_inpaint / ai_inpaint"); } },
  ai_inpaint_comfyui_base_url: { get label() { return tr("局部擦除 ComfyUI 地址"); }, placeholder: 'http://127.0.0.1:8188' },
  ai_inpaint_comfyui_workflow_json: { get label() { return tr("局部擦除 ComfyUI 工作流 JSON"); }, valueType: 'json', get extra() { return tr("支持占位符 {{sourceImageUrl}}、{{maskImageUrl}} 等"); } },
  ai_inpaint_comfyui_prompt_node_id: { get label() { return tr("局部擦除提示词节点 ID"); } },
  ai_inpaint_comfyui_image_node_id: { get label() { return tr("局部擦除原图节点 ID"); } },
  ai_inpaint_comfyui_mask_node_id: { get label() { return tr("局部擦除遮罩节点 ID"); } },
  ai_inpaint_comfyui_output_node_id: { get label() { return tr("局部擦除输出节点 ID"); } },
};
