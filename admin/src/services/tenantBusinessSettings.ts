import { getJSON, putJSON, postJSON } from '@/services/request';
import { captureTenantContext, isTenantContextCurrent, type TenantContextSnapshot } from '@/utils/tenantContext';

export type BusinessModule = 'general' | 'ai' | 'collect' | 'inventory' | 'notifications' | 'orders' | 'branding';
export type BusinessSnapshot = { module: BusinessModule; version: number; values: Record<string, any>; inherited: boolean };
export type BusinessRevision = { version: number; module: BusinessModule; action: string; actorId: string; createdAt: string; values: Record<string,unknown> };
const BRANDING_CHANGED_EVENT = 'trademind:tenant-branding-changed';

export function subscribeBrandingChanges(listener: () => void): () => void {
  if (typeof window === 'undefined') return () => {};
  window.addEventListener(BRANDING_CHANGED_EVENT, listener);
  return () => window.removeEventListener(BRANDING_CHANGED_EVENT, listener);
}

function notifyBrandingChange(module: BusinessModule, context: TenantContextSnapshot) {
  if (module === 'branding' && typeof window !== 'undefined' && isTenantContextCurrent(context)) {
    window.dispatchEvent(new Event(BRANDING_CHANGED_EVENT));
  }
}

export const getBusinessSettings = (module: BusinessModule) =>
  getJSON<BusinessSnapshot>(`/api/v1/tenant/settings/${module}`);
export const putBusinessSettings = async (module: BusinessModule, body: {expectedVersion:number; values:Record<string, any>}) => {
  const context = captureTenantContext();
  const result = await putJSON<BusinessSnapshot, typeof body>(`/api/v1/tenant/settings/${module}`,body);
  notifyBrandingChange(module, context);
  return result;
};
export const listBusinessSettingsHistory = (module: BusinessModule) =>
  getJSON<{items:BusinessRevision[]}>(`/api/v1/tenant/settings/${module}/history`);
export const restoreBusinessSettings = async (module: BusinessModule, body: {targetVersion:number;expectedVersion:number}) => {
  const context = captureTenantContext();
  const result = await postJSON<BusinessSnapshot, typeof body>(`/api/v1/tenant/settings/${module}/restore`,body);
  notifyBrandingChange(module, context);
  return result;
};
