import { tr, trFormat } from '@/i18n';
/** 存储方式显示名 */
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
  return STORAGE_KIND_LABEL[k] ? tr(STORAGE_KIND_LABEL[k]) : kind || tr('存储');
}

export function storageConnectionSectionTitle(kind?: string): string {
  const k = (kind || 'local').trim().toLowerCase();
  if (k === 'local') return tr('本地磁盘配置');
  return trFormat('{kind} 连接参数', { kind: storageKindLabel(k) });
}
