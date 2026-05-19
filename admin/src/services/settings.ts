import { getJSON, postJSON, putJSON } from '@/services/request';

export type SettingRow = {
  id?: number;
  tenantId?: number;
  groupKey: string;
  itemKey: string;
  itemValue: string;
  valueType?: string;
  isEncrypted: boolean;
  remark?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type SettingsListData = { items: SettingRow[] };

export type SettingPutItem = {
  tenantId?: number;
  groupKey: string;
  itemKey: string;
  itemValue: string;
  valueType?: string;
  isEncrypted: boolean;
  remark?: string;
};

/** GET /api/v1/settings */
export async function fetchSettingsList() {
  return getJSON<SettingsListData>('/api/v1/settings');
}

/** PUT /api/v1/settings */
export async function saveSettingsItems(items: SettingPutItem[]) {
  return putJSON<SettingsListData, { items: SettingPutItem[] }>('/api/v1/settings', { items });
}

/** POST /api/v1/settings/test-platform-tiktok */
export async function testPlatformTikTokConfig() {
  return postJSON<{ ok: boolean }>('/api/v1/settings/test-platform-tiktok', {});
}

export type TestAIConnectionResult = {
  ok: boolean;
  message?: string;
  provider?: string;
  model?: string;
  latencyMs?: number;
};

/** POST /api/v1/settings/test-ai */
export async function testAIConnection() {
  return postJSON<TestAIConnectionResult>('/api/v1/settings/test-ai');
}

/** POST /api/v1/settings/test-storage */
export async function testStorageConnection() {
  return postJSON<{ ok: boolean }>('/api/v1/settings/test-storage');
}

/** POST /api/v1/settings/test-email */
export async function testEmailConnection(to: string) {
  return postJSON<{ ok: boolean }>('/api/v1/settings/test-email', { to });
}

export type IntegrationOverviewData = {
  ai: { configured: boolean; provider?: string; model?: string };
  image: { providerCurrent?: string; removebg: boolean; openaiImage: boolean; comfyui: boolean };
  storage: { kind?: string; configured: boolean };
  mail: { configured: boolean };
  platforms: {
    platform: string;
    name: string;
    status: string;
    groupKey?: string;
    appConfigured: boolean;
  }[];
  collectRulesCount: number;
  disclaimerShort?: string;
};

/** GET /api/v1/settings/integrations/overview */
export async function fetchIntegrationsOverview() {
  return getJSON<IntegrationOverviewData>('/api/v1/settings/integrations/overview');
}
