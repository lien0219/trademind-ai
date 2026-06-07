import type { AppConfigFieldDTO } from '@/services/shops';

/** 开放平台应用配置字段中文映射（按 field.name） */
export const PLATFORM_APP_FIELD_LABEL: Record<string, string> = {
  app_key: '应用 Key',
  app_secret: '应用 Secret',
  auth_base_url: '授权接口地址',
  api_base_url: 'API 接口地址',
  redirect_uri: 'OAuth 回调地址',
  api_version: 'API 版本号',
  environment: '环境',
  real_api_enabled: '启用真实接口',
  order_sync_enabled: '启用订单同步',
  order_sync_max_pages: '订单同步最大页数',
  inventory_sync_enabled: '启用库存同步',
  product_publish_enabled: '启用商品草稿创建',
  region: '区域',
  oauth_scopes: 'OAuth 授权范围',
  sandbox_enabled: '沙箱环境',
  timeout_sec: '请求超时（秒）',
  partner_id: 'Partner ID',
  partner_key: 'Partner Key',
  client_id: 'Client ID',
  client_secret: 'Client Secret',
  refresh_token: 'Refresh Token',
  lwa_auth_base_url: 'LWA 授权地址',
  lwa_token_url: 'LWA Token 地址',
  sp_api_base_url: 'SP-API 接口地址',
  marketplace_id: 'Marketplace ID',
  role_arn: 'IAM Role ARN',
  shop_domain: '店铺域名',
  scopes: '授权范围 Scopes',
  consumer_key: 'Consumer Key',
  consumer_secret: 'Consumer Secret',
  store_url: '店铺 URL',
  dev_id: 'Dev ID',
};

export const PLATFORM_APP_FIELD_HELP: Record<string, string> = {
  api_version: '用于接口路径中的版本段，如 202309',
  oauth_scopes: '多个 scope 用空格分隔；留空使用 Partner 应用默认',
  region: '站点或市场区域标识，可选',
  sandbox_enabled: '标记是否使用沙箱；实际环境以 Partner 配置为准',
  environment: '生产 / 沙箱模式；真实调用仍以后端 Provider 实现和平台官方权限为准',
  real_api_enabled: '仅保存开关；Phase 1 不发起抖店真实 API 调用',
  order_sync_enabled: '启用订单同步（抖店 Phase 8 已接入；默认关闭，开启后可在店铺页手动同步订单）',
  order_sync_max_pages: '单次任务最多拉取页数（默认 5）；每页条数由同步任务 limit 控制，总条数上限 500',
  inventory_sync_enabled: '启用库存同步（抖店 Phase 9 已接入 sku.syncStock；默认关闭，开启后可在商品详情/库存预警页手动同步）',
  product_publish_enabled: '仅保存开关；抖店商品草稿创建会在后续 Phase 接入',
  timeout_sec: '外部 HTTP 请求超时，建议 5–600 秒',
  redirect_uri: '须与开放平台应用登记的重定向 URI 完全一致',
  refresh_token: '通常保存在店铺授权中；此处为应用级占位',
  role_arn: '配置后使用 STS AssumeRole 再签名 SP-API 请求',
  shop_domain: '例如 your-store.myshopify.com',
  store_url: '须为 HTTPS 的生产或测试店铺地址',
};

export const PLATFORM_APP_FIELD_PLACEHOLDER: Record<string, string> = {
  app_key: '在开放平台控制台获取',
  app_secret: '保存后脱敏；留空则不修改',
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
  // 清理后端英文标签中的冗余括号说明
  return field.label
    .replace(/（路径段）/g, '')
    .replace(/\(seconds\)/gi, '')
    .replace(/（可选）/g, '（可选）')
    .trim();
}

export function platformAppFieldHelp(field: AppConfigFieldDTO): string | undefined {
  return PLATFORM_APP_FIELD_HELP[field.name.trim().toLowerCase()] || field.help;
}

export function platformAppFieldPlaceholder(field: AppConfigFieldDTO): string {
  if (field.placeholder) return field.placeholder;
  if (field.sensitive || field.type === 'password') {
    return '保存后脱敏；留空则不修改';
  }
  return PLATFORM_APP_FIELD_PLACEHOLDER[field.name.trim().toLowerCase()] || '';
}
