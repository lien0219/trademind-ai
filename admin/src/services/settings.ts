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

/** POST /api/v1/settings/test-ai */
export async function testAIConnection() {
  return postJSON<{ ok: boolean }>('/api/v1/settings/test-ai');
}

/** POST /api/v1/settings/test-storage */
export async function testStorageConnection() {
  return postJSON<{ ok: boolean }>('/api/v1/settings/test-storage');
}
