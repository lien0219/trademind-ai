import type { AppConfigFieldDTO } from '@/services/shops';
import { PLATFORM_COPY } from '@/constants/copywriting';

/** 开放平台应用配置字段中文映射（按 field.name） */
export const PLATFORM_APP_FIELD_LABEL: Record<string, string> = {
  app_key: PLATFORM_COPY.appKey,
  app_secret: PLATFORM_COPY.appSecret,
  service_id: PLATFORM_COPY.serviceId,
  auth_base_url: PLATFORM_COPY.authUrl,
  api_base_url: PLATFORM_COPY.apiUrl,
  redirect_uri: PLATFORM_COPY.callbackUrl,
  api_version: '接口版本号',
  environment: PLATFORM_COPY.environment,
  real_api_enabled: PLATFORM_COPY.realApi,
  order_sync_enabled: PLATFORM_COPY.orderSync,
  order_sync_max_pages: PLATFORM_COPY.orderSyncMaxPages,
  inventory_sync_enabled: PLATFORM_COPY.inventorySync,
  product_publish_enabled: PLATFORM_COPY.productDraftCreate,
  region: '区域',
  oauth_scopes: '授权范围',
  sandbox_enabled: '测试环境',
  timeout_sec: PLATFORM_COPY.requestTimeout,
  partner_id: 'Partner ID',
  partner_key: 'Partner Key',
  client_id: 'Client ID',
  client_secret: 'Client Secret',
  refresh_token: '授权续期凭证',
  lwa_auth_base_url: 'LWA 授权地址',
  lwa_token_url: 'LWA 凭证地址',
  sp_api_base_url: 'SP-API 接口地址',
  marketplace_id: 'Marketplace ID',
  role_arn: 'IAM Role ARN',
  shop_domain: '店铺域名',
  scopes: '授权范围',
  consumer_key: 'Consumer Key',
  consumer_secret: 'Consumer Secret',
  store_url: '店铺地址',
  dev_id: 'Dev ID',
};

export const PLATFORM_APP_FIELD_HELP: Record<string, string> = {
  app_key: '在抖店开放平台创建应用后获得。',
  app_secret: '保存后只显示星号，空着保存不会修改原有密钥。',
  service_id: '仅使用服务市场授权时需要填写。',
  auth_base_url: '通常使用系统推荐地址，只有平台要求时才需要修改。',
  api_base_url: '通常无需修改。',
  redirect_uri: '需要与抖店开放平台登记的地址完全一致。',
  api_version: '用于接口路径中的版本段，例如 202309。',
  oauth_scopes: '多个授权范围用空格分隔；留空使用平台默认。',
  region: '站点或市场区域标识，可选。',
  sandbox_enabled: '开发测试时可选择测试环境。',
  environment: '正式使用请选择「生产环境」；开发测试可选择「测试环境」。',
  real_api_enabled: '开启后，系统将实际调用平台接口。测试前请确认应用和店铺已完成授权。',
  order_sync_enabled: '开启后，可在店铺管理中手动同步抖店订单。',
  order_sync_max_pages: '限制一次同步的数据量，避免任务运行时间过长。',
  inventory_sync_enabled: '开启后，可将本地库存同步到抖店。',
  product_publish_enabled: '开启后，可将商品创建为抖店草稿，不会直接上架。',
  timeout_sec: '外部请求超时时间，建议 5–600 秒。',
  refresh_token: '通常保存在店铺授权中；此处为应用级占位。',
  role_arn: '配置后使用 STS AssumeRole 再签名 SP-API 请求。',
  shop_domain: '例如 your-store.myshopify.com',
  store_url: '须为 HTTPS 的店铺地址',
};

export const PLATFORM_APP_FIELD_PLACEHOLDER: Record<string, string> = {
  app_key: '在开放平台控制台获取',
  app_secret: '保存后只显示星号；留空则不修改',
  auth_base_url: 'https://auth.example.com',
  api_base_url: 'https://api.example.com',
  redirect_uri: 'https://your-admin.example.com/callback',
  api_version: '202309',
  environment: 'production',
  timeout_sec: '30',
};

export const PLATFORM_STATUS_META: Record<string, { label: string; color: string }> = {
  available: { label: '可用', color: 'success' },
  beta: { label: '测试中', color: 'processing' },
  planned: { label: '规划中', color: 'default' },
  disabled: { label: '停用', color: 'error' },
};

/** 开发者门户快捷链接 */
export const PLATFORM_DEV_PORTALS: { name: string; url: string }[] = [
  { name: '抖店开放平台', url: 'https://op.jinritemai.com/docs/' },
  { name: 'TikTok Shop Partner', url: 'https://partner.tiktokshop.com/' },
  { name: 'Shopee Open', url: 'https://open.shopee.com/' },
  { name: 'Lazada Open', url: 'https://open.lazada.com/' },
  { name: 'Amazon SP-API', url: 'https://developer-docs.amazon.com/sp-api/' },
  { name: 'Shopify Partners', url: 'https://partners.shopify.com/' },
];

export function platformAppFieldLabel(field: AppConfigFieldDTO): string {
  const mapped = PLATFORM_APP_FIELD_LABEL[field.name.trim().toLowerCase()];
  if (mapped) return mapped;
  return field.label
    .replace(/（路径段）/g, '')
    .replace(/\(seconds\)/gi, '')
    .replace(/OAuth /gi, '')
    .replace(/Token/gi, '凭证')
    .trim();
}

export function platformAppFieldHelp(field: AppConfigFieldDTO): string | undefined {
  return PLATFORM_APP_FIELD_HELP[field.name.trim().toLowerCase()] || field.help;
}

export function platformAppFieldPlaceholder(field: AppConfigFieldDTO): string {
  if (field.placeholder) return field.placeholder;
  if (field.sensitive || field.type === 'password') {
    return '保存后只显示星号；留空则不修改';
  }
  return PLATFORM_APP_FIELD_PLACEHOLDER[field.name.trim().toLowerCase()] || '';
}

/** 开关字段：独立行布局 */
export function isPlatformSwitchField(field: AppConfigFieldDTO): boolean {
  return field.type === 'switch';
}
