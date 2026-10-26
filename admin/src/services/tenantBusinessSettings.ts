import { getJSON, putJSON, postJSON } from '@/services/request';

export type BusinessModule = 'general' | 'ai' | 'collect' | 'inventory' | 'notifications' | 'orders' | 'branding';
export type BusinessSnapshot = { module: BusinessModule; version: number; values: Record<string, any>; inherited: boolean };
export type BusinessRevision = { version: number; module: BusinessModule; action: string; actorId: string; createdAt: string };
export const getBusinessSettings = (module: BusinessModule) =>
  getJSON<BusinessSnapshot>(`/api/v1/tenant/settings/${module}`);
export const putBusinessSettings = (module: BusinessModule, body: {expectedVersion:number; values:Record<string, any>}) =>
  putJSON<BusinessSnapshot, typeof body>(`/api/v1/tenant/settings/${module}`,body);
export const listBusinessSettingsHistory = (module: BusinessModule) =>
  getJSON<{items:BusinessRevision[]}>(`/api/v1/tenant/settings/${module}/history`);
export const restoreBusinessSettings = (module: BusinessModule, body: {targetVersion:number;expectedVersion:number}) =>
  postJSON<BusinessSnapshot, typeof body>(`/api/v1/tenant/settings/${module}/restore`,body);
