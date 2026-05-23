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
  return AI_TASK_TYPE_LABEL[k] || k;
}

export function aiPromptCodeLabel(code?: string): string {
  const k = (code || '').trim();
  if (!k) return '—';
  return AI_PROMPT_CODE_LABEL[k] || k;
}

export const AI_TASK_TYPE_OPTIONS = Object.entries(AI_TASK_TYPE_LABEL).map(([value, label]) => ({
  value,
  label,
}));

export const AI_PROMPT_CODE_OPTIONS = Object.entries(AI_PROMPT_CODE_LABEL).map(([value, label]) => ({
  value,
  label,
}));
