import type { ImageProviderCapability } from '@/constants/imageProviders';
import { getJSON, postJSON } from '@/services/request';

/** GET /api/v1/image/providers */
export async function fetchImageProviders() {
  return getJSON<ImageProviderCapability[]>('/api/v1/image/providers');
}

export type TestImageProviderResult = {
  provider: string;
  ok: boolean;
  message: string;
  latencyMs?: number;
  supportedTasks?: string[];
  configStatus?: string;
  testMode?: string;
};

export type TestImageProviderPayload = {
  provider?: string;
  testMode?: 'config_only' | 'live';
  /** Optional form overrides; masked secrets are ignored so stored values are used. */
  settings?: Record<string, string>;
};

/** POST /api/v1/settings/test-image — optional settings tests current form without saving */
export async function testImageProvider(payload?: TestImageProviderPayload) {
  return postJSON<TestImageProviderResult, TestImageProviderPayload | Record<string, never>>(
    '/api/v1/settings/test-image',
    payload ?? {},
  );
}
