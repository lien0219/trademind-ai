import { getJSON, postJSON, putJSON } from '@/services/request';

export type SettingsPayload = Record<string, unknown>;

/** 获取系统设置 — GET /api/v1/settings */
export async function fetchSettings() {
  return getJSON<SettingsPayload>('/api/v1/settings');
}

/** 保存系统设置 — PUT /api/v1/settings */
export async function saveSettings(body: SettingsPayload) {
  return putJSON<SettingsPayload, SettingsPayload>('/api/v1/settings', body);
}

/** AI 连接测试 */
export async function testAI() {
  return postJSON<void>('/api/v1/settings/test-ai');
}

/** 存储连接测试 */
export async function testStorage() {
  return postJSON<void>('/api/v1/settings/test-storage');
}
