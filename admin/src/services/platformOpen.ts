import { getJSON, putJSON } from '@/services/request';

export type PlatformAppSettingsResp = {
  platform: string;
  groupKey: string;
  values: Record<string, string>;
};

export async function getPlatformAppSettings(platform: string): Promise<PlatformAppSettingsResp> {
  return getJSON(`/api/v1/platform/settings/${platform}`);
}

export async function putPlatformAppSettings(
  platform: string,
  values: Record<string, unknown>,
): Promise<PlatformAppSettingsResp> {
  return putJSON(`/api/v1/platform/settings/${platform}`, { values });
}

/** Sort providers for Tabs (settingsGroupKey filtered by caller). */
export function preferredPlatformTabOrder(platform: string): number {
  const order = [
    'tiktok',
    'shopee',
    'lazada',
    'amazon',
    'aliexpress',
    'shopify',
    'woocommerce',
    'ebay',
    'temu',
    'shein',
    'custom',
  ];
  const i = order.indexOf(platform);
  return i >= 0 ? i : 500;
}

export function externalDocUrlFor(platform: string): string | undefined {
  const m: Record<string, string> = {
    tiktok: 'https://partner.tiktokshop.com/',
    shopee: 'https://open.shopee.com/',
    lazada: 'https://open.lazada.com/',
    amazon: 'https://developer.amazonservices.com/',
    aliexpress: 'https://developers.aliexpress.com/',
    shopify: 'https://partners.shopify.com/',
    woocommerce: 'https://woocommerce.com/document/rest-api/',
    ebay: 'https://developer.ebay.com/',
  };
  return m[platform];
}
