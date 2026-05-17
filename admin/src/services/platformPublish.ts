import { getJSON, putJSON } from '@/services/request';
import type { AppConfigSchemaDTO } from '@/services/shops';

export type PlatformPublishSettingsResp = {
  platform: string;
  groupKey: string;
  schema: AppConfigSchemaDTO;
  values: Record<string, string>;
};

export async function getPlatformPublishSettings(platform: string): Promise<PlatformPublishSettingsResp> {
  return getJSON(`/api/v1/platform/publish-settings/${platform}`);
}

export async function putPlatformPublishSettings(
  platform: string,
  values: Record<string, unknown>,
): Promise<PlatformPublishSettingsResp> {
  return putJSON(`/api/v1/platform/publish-settings/${platform}`, { values });
}
