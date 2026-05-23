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
  return STORAGE_KIND_LABEL[k] || kind || '存储';
}

export function storageConnectionSectionTitle(kind?: string): string {
  const k = (kind || 'local').trim().toLowerCase();
  if (k === 'local') return '本地磁盘配置';
  return `${storageKindLabel(k)} 连接参数`;
}
