export type AIProviderValue = 'openai' | 'openai_compatible' | 'deepseek' | 'qwen';

export type AIProviderDocs = {
  /** API / 接入说明文档 */
  docsUrl: string;
  docsLabel: string;
  /** 控制台或申请密钥入口（可选） */
  consoleUrl?: string;
  consoleLabel?: string;
};

export type AIProviderMeta = {
  value: AIProviderValue;
  title: string;
  desc: string;
  tag?: string;
};

export const AI_PROVIDER_METAS: AIProviderMeta[] = [
  {
    value: 'openai',
    title: 'OpenAI',
    desc: '官方 GPT 系列；适合英文与多语言商品文案',
    tag: '官方',
  },
  {
    value: 'openai_compatible',
    title: 'OpenAI Compatible',
    desc: 'Ollama、自建网关等兼容接口',
    tag: '通用',
  },
  {
    value: 'deepseek',
    title: 'DeepSeek',
    desc: '高性价比中文理解；标题 / 描述 / 客服建议',
    tag: '推荐',
  },
  {
    value: 'qwen',
    title: '通义千问',
    desc: 'DashScope OpenAI 兼容模式；国内部署友好',
    tag: '国内',
  },
];

export const AI_PROVIDER_VALUES = AI_PROVIDER_METAS.map((m) => m.value);

/** 各服务商官方文档与控制台链接（管理端 AI 设置页跳转用） */
export const AI_PROVIDER_DOCS: Record<AIProviderValue, AIProviderDocs> = {
  openai: {
    docsUrl:
      'https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create/',
    docsLabel: 'OpenAI Chat Completions 文档',
    consoleUrl: 'https://platform.openai.com/api-keys',
    consoleLabel: 'OpenAI 控制台 · 申请 API Key',
  },
  openai_compatible: {
    docsUrl:
      'https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create/',
    docsLabel: 'OpenAI 兼容 Chat Completions 规范',
    consoleUrl: 'https://github.com/ollama/ollama/blob/main/docs/openai.md',
    consoleLabel: 'Ollama OpenAI 兼容说明',
  },
  deepseek: {
    docsUrl: 'https://api-docs.deepseek.com/',
    docsLabel: 'DeepSeek API 文档',
    consoleUrl: 'https://platform.deepseek.com/api_keys',
    consoleLabel: 'DeepSeek 控制台 · 申请 API Key',
  },
  qwen: {
    docsUrl:
      'https://help.aliyun.com/zh/model-studio/developer-reference/compatibility-of-openai-with-dashscope',
    docsLabel: '通义千问 OpenAI 兼容文档',
    consoleUrl: 'https://bailian.console.aliyun.com/?apiKey=1#/api-key',
    consoleLabel: '阿里云百炼 · 申请 API Key',
  },
};

export function aiProviderDocs(provider: AIProviderValue | undefined): AIProviderDocs | null {
  if (!provider) return null;
  return AI_PROVIDER_DOCS[provider] ?? null;
}

export const AI_PROVIDER_PRESETS: Record<
  AIProviderValue,
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

export function aiProviderBaseURLKey(provider: AIProviderValue): string {
  return `${provider}_base_url`;
}

export function aiProviderModelKey(provider: AIProviderValue): string {
  return `${provider}_model`;
}

export function aiProviderAPIKeyKey(provider: AIProviderValue): string {
  return `${provider}_api_key`;
}

/** Connection fields shown per provider on the AI settings page. */
export const AI_PROVIDER_FIELD_KEYS: Record<AIProviderValue, string[]> = {
  openai: ['openai_base_url', 'openai_model', 'openai_api_key'],
  openai_compatible: ['openai_compatible_base_url', 'openai_compatible_model', 'openai_compatible_api_key'],
  deepseek: ['deepseek_base_url', 'deepseek_model', 'deepseek_api_key'],
  qwen: ['qwen_base_url', 'qwen_model', 'qwen_api_key'],
};

export const ALL_AI_CONNECTION_FIELD_SPECS: Record<
  string,
  { encrypted?: boolean; label: string; extra?: string; placeholder?: string }
> = {
  openai_base_url: {
    label: '接口地址',
    placeholder: 'https://api.openai.com/v1',
    extra: AI_PROVIDER_PRESETS.openai.baseUrlHelp,
  },
  openai_model: { label: '模型', placeholder: 'gpt-4o-mini' },
  openai_api_key: {
    label: 'API 密钥',
    encrypted: true,
    placeholder: 'sk-...',
    extra: '请到 OpenAI 控制台申请；各服务商密钥独立保存',
  },
  openai_compatible_base_url: {
    label: '接口地址',
    placeholder: 'https://api.openai.com/v1',
    extra: AI_PROVIDER_PRESETS.openai_compatible.baseUrlHelp,
  },
  openai_compatible_model: { label: '模型', placeholder: 'gpt-4o-mini' },
  openai_compatible_api_key: {
    label: 'API 密钥',
    encrypted: true,
    placeholder: 'sk-...',
    extra: 'Ollama 等本地服务可留空或填占位；各服务商密钥独立保存',
  },
  deepseek_base_url: {
    label: '接口地址',
    placeholder: 'https://api.deepseek.com/v1',
    extra: AI_PROVIDER_PRESETS.deepseek.baseUrlHelp,
  },
  deepseek_model: { label: '模型', placeholder: 'deepseek-chat' },
  deepseek_api_key: {
    label: 'API 密钥',
    encrypted: true,
    placeholder: 'sk-...',
    extra: '请到 DeepSeek 控制台申请；各服务商密钥独立保存',
  },
  qwen_base_url: {
    label: '接口地址',
    placeholder: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    extra: AI_PROVIDER_PRESETS.qwen.baseUrlHelp,
  },
  qwen_model: { label: '模型', placeholder: 'qwen-plus' },
  qwen_api_key: {
    label: 'API 密钥',
    encrypted: true,
    placeholder: 'sk-...',
    extra: '请到阿里云百炼控制台申请；各服务商密钥独立保存',
  },
};

export function buildAIConnectionFieldsSpec(): Record<string, { encrypted?: boolean }> {
  const out: Record<string, { encrypted?: boolean }> = {};
  for (const key of Object.keys(ALL_AI_CONNECTION_FIELD_SPECS)) {
    out[key] = { encrypted: ALL_AI_CONNECTION_FIELD_SPECS[key].encrypted };
  }
  return out;
}

/** All per-provider connection field keys (base_url / model / api_key). */
export function allAIConnectionFieldKeys(): string[] {
  return Object.keys(ALL_AI_CONNECTION_FIELD_SPECS);
}

/** Fields to PUT on save: globals + active provider connection only (avoid wiping other providers). */
export function buildAISaveFieldSpecs(
  activeProvider: AIProviderValue,
): Record<string, { encrypted?: boolean }> {
  const out: Record<string, { encrypted?: boolean }> = {};
  for (const key of AI_PROVIDER_FIELD_KEYS[activeProvider]) {
    out[key] = { encrypted: !!ALL_AI_CONNECTION_FIELD_SPECS[key]?.encrypted };
  }
  return out;
}

export function applyAIProviderPreset(
  provider: AIProviderValue,
  current: { baseUrl?: string; model?: string },
  forceFill: boolean,
) {
  const preset = AI_PROVIDER_PRESETS[provider];
  const baseKey = aiProviderBaseURLKey(provider);
  const modelKey = aiProviderModelKey(provider);
  const next: Record<string, string> = {};
  if (forceFill || !String(current.baseUrl || '').trim()) {
    next[baseKey] = preset.baseUrl;
  }
  if (forceFill || !String(current.model || '').trim()) {
    next[modelKey] = preset.model;
  }
  return next;
}

export function initialAIConnectionFormValues(
  group: Record<string, string>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const provider of AI_PROVIDER_VALUES) {
    const baseKey = aiProviderBaseURLKey(provider);
    const modelKey = aiProviderModelKey(provider);
    const apiKeyKey = aiProviderAPIKeyKey(provider);
    const preset = AI_PROVIDER_PRESETS[provider];
    out[baseKey] = group[baseKey] || '';
    out[modelKey] = group[modelKey] || '';
    out[apiKeyKey] = group[apiKeyKey] || '';
    if (!out[baseKey] && provider === (group.provider as AIProviderValue)) {
      out[baseKey] = group.base_url || preset.baseUrl;
    }
    if (!out[modelKey] && provider === (group.provider as AIProviderValue)) {
      out[modelKey] = group.model || preset.model;
    }
    if (!out[apiKeyKey] && provider === (group.provider as AIProviderValue)) {
      out[apiKeyKey] = group.api_key || '';
    }
  }
  return out;
}
