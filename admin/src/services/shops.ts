import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';

export type AppConfigFieldDTO = {
  name: string;
  label: string;
  type: string;
  required: boolean;
  sensitive: boolean;
  placeholder?: string;
  help?: string;
  defaultValue?: unknown;
  options?: { label: string; value: string }[];
};

export type AppConfigSchemaDTO = {
  groupKey: string;
  title: string;
  description?: string;
  fields: AppConfigFieldDTO[];
};

export type PlatformProviderMeta = {
  platform: string;
  name: string;
  status: string;
  authType: string;
  capabilities: string[];
  authSchema: {
    name: string;
    label: string;
    type: string;
    required: boolean;
    sensitive: boolean;
    hint?: string;
  }[];
  appConfigSchema: AppConfigSchemaDTO;
  settingsGroupKey: string;
};

export type ShopAuthPublic = {
  authType: string;
  appKey?: string;
  appSecret?: string;
  accessToken?: string;
  refreshToken?: string;
  sellerId?: string;
  merchantId?: string;
  marketplaceId?: string;
  expiresAt?: string;
  refreshExpiresAt?: string;
  scopes?: unknown;
  authConfig?: Record<string, unknown>;
};

export type ShopDetail = {
  id: string;
  tenantId: number;
  platform: string;
  shopName: string;
  shopCode?: string;
  externalShopId?: string;
  status: string;
  authStatus: string;
  region?: string;
  currency?: string;
  timezone?: string;
  defaultLanguage?: string;
  capabilities?: unknown;
  platformConfig?: unknown;
  remark?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  auth?: ShopAuthPublic | null;
};

export type ShopListRow = {
  id: string;
  platform: string;
  shopName: string;
  shopCode?: string;
  status: string;
  authStatus: string;
  region?: string;
  currency?: string;
  capabilities?: unknown;
  updatedAt: string;
};

export async function queryPlatformProviders(): Promise<{ list: PlatformProviderMeta[] }> {
  return getJSON('/api/v1/platform/providers');
}

export async function queryShops(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  status?: string;
  authStatus?: string;
  shopName?: string;
}): Promise<{
  list: ShopListRow[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}> {
  return getWithParams('/api/v1/shops', params);
}

export async function createShop(payload: Record<string, unknown>): Promise<ShopDetail> {
  return postJSON('/api/v1/shops', payload);
}

export async function getShop(id: string): Promise<ShopDetail> {
  return getJSON(`/api/v1/shops/${id}`);
}

export async function updateShop(id: string, payload: Record<string, unknown>): Promise<ShopDetail> {
  return putJSON(`/api/v1/shops/${id}`, payload);
}

export async function deleteShop(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/shops/${id}`);
}

export async function updateShopAuth(id: string, payload: Record<string, unknown>): Promise<{ auth: ShopAuthPublic }> {
  return putJSON(`/api/v1/shops/${id}/auth`, payload);
}

export async function testShopConnection(
  id: string,
): Promise<{
  ok: boolean;
  message?: string;
  shopName?: string;
  externalShopId?: string;
  region?: string;
  currency?: string;
  sellerMerchantId?: string;
}> {
  return postJSON(`/api/v1/shops/${id}/test-connection`, {});
}

export async function getTikTokOAuthAuthorizeUrl(
  shopId: string,
  redirectUri?: string,
): Promise<{ authorizeUrl: string; state: string }> {
  return getWithParams(`/api/v1/shops/${shopId}/oauth/tiktok/authorize-url`, {
    redirectUri: redirectUri || undefined,
  });
}

export async function postTikTokOAuthCallback(
  shopId: string,
  payload: { code: string; state: string },
): Promise<ShopDetail> {
  return postJSON(`/api/v1/shops/${shopId}/oauth/tiktok/callback`, payload);
}

export async function getShopeeOAuthAuthorizeUrl(
  shopId: string,
  redirectUri?: string,
): Promise<{ authorizeUrl: string; state: string }> {
  return getWithParams(`/api/v1/shops/${shopId}/oauth/shopee/authorize-url`, {
    redirectUri: redirectUri || undefined,
  });
}

export async function postShopeeOAuthCallback(
  shopId: string,
  payload: { code: string; state: string; shopId: string; mainAccountId?: string },
): Promise<ShopDetail> {
  return postJSON(`/api/v1/shops/${shopId}/oauth/shopee/callback`, payload);
}

export async function getLazadaOAuthAuthorizeUrl(
  shopId: string,
  redirectUri?: string,
): Promise<{ authorizeUrl: string; state: string }> {
  return getWithParams(`/api/v1/shops/${shopId}/oauth/lazada/authorize-url`, {
    redirectUri: redirectUri || undefined,
  });
}

export async function postLazadaOAuthCallback(
  shopId: string,
  payload: { code: string; state: string },
): Promise<ShopDetail> {
  return postJSON(`/api/v1/shops/${shopId}/oauth/lazada/callback`, payload);
}

export async function getAmazonOAuthAuthorizeUrl(
  shopId: string,
  redirectUri?: string,
): Promise<{ authorizeUrl: string; state: string }> {
  return getWithParams(`/api/v1/shops/${shopId}/oauth/amazon/authorize-url`, {
    redirectUri: redirectUri || undefined,
  });
}

export async function postAmazonOAuthCallback(
  shopId: string,
  payload: { code: string; state: string; sellingPartnerId: string; marketplaceId?: string },
): Promise<ShopDetail> {
  return postJSON(`/api/v1/shops/${shopId}/oauth/amazon/callback`, payload);
}
