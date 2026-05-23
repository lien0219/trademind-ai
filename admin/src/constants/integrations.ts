import { aiTextProviderLabel } from '@/constants/aiPrompts';

/** 存储方式中文映射（与「设置 → 存储」kind 一致） */
export const STORAGE_KIND_LABEL: Record<string, string> = {
  local: '本地磁盘',
  s3: 'Amazon S3',
  cos: '腾讯云 COS',
  oss: '阿里云 OSS',
  r2: 'Cloudflare R2',
  minio: 'MinIO',
};

export function storageKindLabel(kind?: string): string {
  const k = (kind || 'local').trim().toLowerCase();
  return STORAGE_KIND_LABEL[k] || kind || '—';
}

/** 图片 AI 子服务就绪状态（集成总览摘要） */
export const IMAGE_SUB_SERVICE_LABEL: Record<string, string> = {
  removebg: 'remove.bg',
  openaiImage: 'OpenAI Image',
  comfyui: 'ComfyUI',
};

/** 图片 provider 显示名（与 settings.image.provider 一致） */
export const IMAGE_PROVIDER_DISPLAY: Record<string, string> = {
  noop: '不启用',
  removebg: 'remove.bg',
  openai_image: 'OpenAI Image',
  comfyui: 'ComfyUI',
  dashscope_image: '通义万相',
  volcengine_image: '火山方舟',
  siliconflow_image: '硅基流动',
  hunyuan_image: '腾讯混元',
};

export function imageProviderLabel(provider?: string): string {
  const k = (provider || '').trim().toLowerCase();
  if (!k) return '';
  return IMAGE_PROVIDER_DISPLAY[k] || provider || '';
}

export function integrationConfiguredTag(configured: boolean): { text: string; color: 'success' | 'default' | 'warning' } {
  return configured
    ? { text: '已配置', color: 'success' }
    : { text: '未配置', color: 'warning' };
}

export function imageSubServiceStatusLabel(filled: boolean, kind: 'key' | 'url'): string {
  if (filled) {
    return kind === 'url' ? '地址已填' : '密钥已填';
  }
  return kind === 'url' ? '未填地址' : '未填密钥';
}

export { aiTextProviderLabel };
