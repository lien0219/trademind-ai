import { tr } from '@/i18n';
import {
  PLATFORM_DISPLAY_LABEL,
  SUPPORTED_COMMERCE_PLATFORM_KEYS,
} from '@/constants/platformLabels';

/** AI 技能模板 · 使用场景中文映射（与后端 ai_prompts.scene 一致） */
export const AI_PROMPT_SCENE_LABEL: Record<string, string> = {
  product: '商品优化',
  customer_service: '智能客服',
  collect: '商品采集',
};

export function aiPromptSceneLabel(scene?: string): string {
  const k = (scene || '').trim();
  if (!k) return '—';
  return AI_PROMPT_SCENE_LABEL[k] || k;
}

export const AI_PROMPT_SCENE_OPTIONS = Object.entries(AI_PROMPT_SCENE_LABEL).map(([value, label]) => ({
  value,
  label,
}));

/** AI 文本服务商中文映射（与「设置 → AI」provider 一致） */
export const AI_TEXT_PROVIDER_LABEL: Record<string, string> = {
  openai: 'OpenAI',
  openai_compatible: 'OpenAI Compatible',
  deepseek: 'DeepSeek',
  qwen: '通义千问',
  doubao: '豆包',
  gemini: 'Gemini',
  claude: 'Claude',
  ollama: 'Ollama',
};

export function aiTextProviderLabel(provider?: string): string {
  const k = (provider || '').trim().toLowerCase().replace(/-/g, '_');
  if (!k) return '';
  return AI_TEXT_PROVIDER_LABEL[k] || provider || '';
}

export const AI_TEXT_PROVIDER_OPTIONS = Object.entries(AI_TEXT_PROVIDER_LABEL).map(([value, label]) => ({
  value,
  label,
}));

export const AI_PROMPT_USE_SYSTEM_DEFAULT = '跟随系统默认';

/** AI 生成语言：中文展示，value 保持为 Prompt 使用的标准语言代码。 */
export const AI_LANGUAGE_OPTIONS = [
  { value: 'zh', label: '中文' },
  { value: 'en', label: '英语' },
  { value: 'th', label: '泰语' },
  { value: 'vi', label: '越南语' },
  { value: 'id', label: '印度尼西亚语' },
  { value: 'ms', label: '马来语' },
  { value: 'ja', label: '日语' },
  { value: 'ko', label: '韩语' },
  { value: 'de', label: '德语' },
  { value: 'fr', label: '法语' },
  { value: 'es', label: '西班牙语' },
  { value: 'pt', label: '葡萄牙语' },
];

/** AI 文案语气：中文展示，value 保持与现有 Prompt 变量兼容。 */
export const AI_TONE_OPTIONS = [
  { value: 'professional', label: '专业稳健' },
  { value: 'friendly', label: '亲切自然' },
  { value: 'concise', label: '简洁直接' },
  { value: 'persuasive', label: '卖点导向' },
  { value: 'enthusiastic', label: '热情活力' },
  { value: 'formal', label: '正式严谨' },
];

/** AI 商品文案目标平台；显示名同时作为 Prompt 值，兼容既有请求 payload。 */
export const AI_TARGET_PLATFORM_OPTIONS = [
  { value: '跨境通用', label: '跨境通用' },
  ...SUPPORTED_COMMERCE_PLATFORM_KEYS.map((platform) => ({
    value: PLATFORM_DISPLAY_LABEL[platform],
    label: PLATFORM_DISPLAY_LABEL[platform],
  })),
];

/** AI 会话上下文平台使用站内平台编码，并保留手动会话。 */
export const AI_CONTEXT_PLATFORM_OPTIONS = [
  ...SUPPORTED_COMMERCE_PLATFORM_KEYS,
  'manual',
].map((platform) => ({
  value: platform,
  label: PLATFORM_DISPLAY_LABEL[platform],
}));

/** AI 任务记录 · 任务类型中文映射（与 backend ai_tasks.task_type 一致） */
export const AI_TASK_TYPE_LABEL: Record<string, string> = {
  title_optimize: '标题优化',
  product_description_generate: '商品描述生成',
  customer_reply_generate: '客服回复建议',
  collect_rule_generate: '采集规则生成',
};

/** AI 任务记录 · 技能模板编号中文映射（与 backend ai_prompts.code 一致） */
export const AI_PROMPT_CODE_LABEL: Record<string, string> = {
  product_title_optimize: '商品标题优化',
  product_description_generate: '商品描述生成',
  customer_reply_generate: 'AI 客服回复建议',
  collect_rule_generate: 'AI 生成自定义采集规则',
};

export function aiTaskTypeLabel(taskType?: string): string {
  const k = (taskType || '').trim();
  if (!k) return '—';
  return AI_TASK_TYPE_LABEL[k] ? tr(AI_TASK_TYPE_LABEL[k]) : k;
}

export function aiPromptCodeLabel(code?: string): string {
  const k = (code || '').trim();
  if (!k) return '—';
  return AI_PROMPT_CODE_LABEL[k] ? tr(AI_PROMPT_CODE_LABEL[k]) : k;
}

export const AI_TASK_TYPE_OPTIONS = Object.entries(AI_TASK_TYPE_LABEL).map(([value, label]) => ({
  value,
  label,
}));

export const AI_PROMPT_CODE_OPTIONS = Object.entries(AI_PROMPT_CODE_LABEL).map(([value, label]) => ({
  value,
  label,
}));
