import { tr } from '@/i18n';
/** AI 图片结构化 warning / error 码（与 backend aiproductimage/warning_codes.go 对齐） */
export const AI_IMAGE_WARNING_CODES = [
  'provider_config_missing',
  'dashscope_key_missing',
  'background_remove_unsupported',
  'white_background_provider_missing',
  'logo_remove_unsupported',
  'image_download_failed',
  'image_mime_invalid',
  'image_too_large',
  'image_decode_failed',
  'provider_timeout',
  'provider_rate_limited',
  'provider_return_invalid_url',
  'storage_public_url_missing',
  'unsupported_operation',
] as const;

export type AIImageWarningCode = (typeof AI_IMAGE_WARNING_CODES)[number];

export const AI_IMAGE_WARNING_LABEL: Record<string, string> = {
  get provider_config_missing() { return tr("图片 AI 服务未配置"); },
  get dashscope_key_missing() { return tr("通义万相 API Key 未配置"); },
  get background_remove_unsupported() { return tr("当前图片 AI 服务不支持去背景"); },
  get white_background_provider_missing() { return tr("白底图能力不可用"); },
  get logo_remove_unsupported() { return tr("去 Logo 能力暂不支持"); },
  get image_download_failed() { return tr("图片下载失败"); },
  get image_mime_invalid() { return tr("图片格式无效"); },
  get image_too_large() { return tr("图片文件过大"); },
  get image_decode_failed() { return tr("图片解码失败"); },
  get provider_timeout() { return tr("图片 AI 服务超时"); },
  get provider_rate_limited() { return tr("图片 AI 服务限流"); },
  get provider_return_invalid_url() { return tr("处理结果无效"); },
  get storage_public_url_missing() { return tr("存储公网地址未配置"); },
  get unsupported_operation() { return tr("不支持的处理类型"); },
};

export const AI_IMAGE_WARNING_MESSAGE: Record<string, string> = {
  get provider_config_missing() { return tr("当前未选择可用的图片 AI 服务，无法执行去背景、白底图等处理。"); },
  get dashscope_key_missing() { return tr("白底图 / 背景优化等能力依赖通义万相，请补充 dashscope_image_api_key 后重新处理。"); },
  get background_remove_unsupported() { return tr("所选图片 AI 服务不支持去背景能力，可更换接入服务或改用白底图 / 背景优化。"); },
  get white_background_provider_missing() { return tr("当前图片 AI 配置不支持白底图生成，请配置通义万相或 remove.bg 等支持白底图的接入服务。"); },
  get logo_remove_unsupported() { return tr("当前图片 AI 服务不支持去 Logo，该能力可能处于预留或降级状态。"); },
  get image_download_failed() { return tr("无法从源图链接下载图片，请确认图片 URL 可访问或重新上传图片。"); },
  get image_mime_invalid() { return tr("源图格式不被支持，请上传 JPG / PNG / WebP 等常见格式。"); },
  get image_too_large() { return tr("源图超过处理上限，请压缩后再试。"); },
  get image_decode_failed() { return tr("无法解析源图内容，请确认文件未损坏。"); },
  get provider_timeout() { return tr("图片处理超时，可稍后重试或检查接入服务网络与超时设置。"); },
  get provider_rate_limited() { return tr("图片 AI 服务返回限流，请稍后重试或降低并发。"); },
  get provider_return_invalid_url() { return tr("图片 AI 返回的结果链接无效，请重试或检查接入服务配置。"); },
  get storage_public_url_missing() { return tr("图片结果需要公网可访问地址，请在存储设置配置 public_base 并测试公网访问。"); },
  get unsupported_operation() { return tr("当前配置不支持该图片处理类型。"); },
};

export const AI_IMAGE_WARNING_SETTINGS_URL: Record<string, string> = {
  provider_config_missing: '/settings/image',
  dashscope_key_missing: '/settings/image',
  background_remove_unsupported: '/settings/image',
  white_background_provider_missing: '/settings/image',
  logo_remove_unsupported: '/settings/image',
  storage_public_url_missing: '/settings/storage',
  unsupported_operation: '/settings/image',
};

export function aiImageWarningLabel(code?: string | null): string {
  const k = (code || '').trim().toLowerCase();
  if (!k) return '—';
  return AI_IMAGE_WARNING_LABEL[k] || k;
}

export function aiImageWarningMessage(code?: string | null): string {
  const k = (code || '').trim().toLowerCase();
  if (!k) return '';
  return AI_IMAGE_WARNING_MESSAGE[k] || '';
}

export function aiImageWarningSettingsUrl(code?: string | null): string | undefined {
  const k = (code || '').trim().toLowerCase();
  if (!k) return undefined;
  return AI_IMAGE_WARNING_SETTINGS_URL[k];
}

/** 批次概览降级说明（H1.3 统一文案） */
export const AI_IMAGE_DEGRADED_SUMMARY =
  '当前图片处理已完成，但部分能力因接入服务配置缺失降级。你可以补充对应密钥后重新处理。';
