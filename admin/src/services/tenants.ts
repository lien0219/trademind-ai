import {
  deleteJSON,
  getJSON,
  getWithParams,
  patchJSON,
  postJSON,
} from '@/services/request';

export type TenantStatus = 'pending' | 'active' | 'suspended' | 'archived' | 'closed';

export type Tenant = {
  id: number;
  code: string;
  name: string;
  status: TenantStatus;
  description?: string;
  legalName?: string;
  ownerUserId?: string;
  contactEmail?: string;
  contactPhone?: string;
  settings?: string;
  version: number;
  createdAt: string;
  updatedAt: string;
};

export type TenantDetail = {
  tenant: Tenant;
  memberCount: number;
  shopCount: number;
};

export type OperationLog = {
  id: string;
  action: string;
  status?: string;
  createdAt: string;
  message?: string;
};

export type TenantMemberStorePermission = {
  storeId: string;
  platform?: string;
  permissionScope: string;
};

export type TenantMember = {
  id: string;
  userId: string;
  email?: string;
  displayName?: string;
  role: string;
  status: 'invited' | 'active' | 'disabled' | 'removed' | string;
  storePermissions: TenantMemberStorePermission[];
};

export type TenantInvitation = {
  id: string;
  email: string;
  role: string;
  status: 'pending' | 'accepted' | 'cancelled' | 'expired' | string;
  expiresAt: string;
  createdAt: string;
};

export type AuthTenant = {
  id: number;
  code: string;
  name: string;
  status: TenantStatus;
  role: string;
};

export type TenantMetadataBody = {
  name: string;
  description?: string;
  legalName?: string;
  contactEmail?: string;
  contactPhone?: string;
  settings?: string;
  version: number;
};

export type TenantCreateBody = {
  code: string;
  name: string;
  description?: string;
  legalName?: string;
  contactEmail?: string;
  contactPhone?: string;
  ownerEmail: string;
  ownerPassword?: string;
  ownerDisplayName?: string;
  idempotencyKey: string;
};

export function createTenantIdempotencyKey() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `tenant-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export async function queryPlatformTenants(params: {
  keyword?: string;
  status?: TenantStatus;
  page?: number;
  pageSize?: number;
}) {
  return getWithParams<{ items: Tenant[]; total: number }>('/api/v1/platform/tenants', params);
}

export async function createPlatformTenant(body: TenantCreateBody) {
  return postJSON<Tenant>('/api/v1/platform/tenants', body);
}

export async function getPlatformTenant(id: number | string) {
  return getJSON<TenantDetail>(`/api/v1/platform/tenants/${encodeURIComponent(String(id))}`);
}

export async function updatePlatformTenant(id: number | string, body: TenantMetadataBody) {
  return patchJSON<Tenant, TenantMetadataBody>(
    `/api/v1/platform/tenants/${encodeURIComponent(String(id))}`,
    body,
  );
}

export async function updatePlatformTenantStatus(
  id: number | string,
  body: { status: TenantStatus; version: number },
) {
  return patchJSON<Tenant, { status: TenantStatus; version: number }>(
    `/api/v1/platform/tenants/${encodeURIComponent(String(id))}/status`,
    body,
  );
}

export async function getPlatformTenantAudit(id: number | string) {
  return getJSON<{ items: OperationLog[] }>(
    `/api/v1/platform/tenants/${encodeURIComponent(String(id))}/audit`,
  );
}

export async function getCurrentTenant() {
  return getJSON<TenantDetail>('/api/v1/tenant/current');
}

export async function updateCurrentTenant(body: TenantMetadataBody) {
  return patchJSON<Tenant, TenantMetadataBody>('/api/v1/tenant/current', body);
}

export async function getTenantMembers() {
  return getJSON<{ items: TenantMember[] }>('/api/v1/tenant/members');
}

export async function updateTenantMember(
  id: string,
  body: { role: string; status: string; storePermissions: TenantMemberStorePermission[] },
) {
  return patchJSON<TenantMember, { role: string; status: string; storePermissions: TenantMemberStorePermission[] }>(
    `/api/v1/tenant/members/${encodeURIComponent(id)}`,
    body,
  );
}

export async function leaveTenant() {
  return postJSON<{ ok: boolean }>('/api/v1/tenant/members/leave', {});
}

export async function getTenantInvitations() {
  return getJSON<{ items: TenantInvitation[] }>('/api/v1/tenant/invitations');
}

export async function createTenantInvitation(body: {
  email: string;
  role: string;
  storePermissions: TenantMemberStorePermission[];
  expiresInHours?: number;
}) {
  return postJSON<{ invitation: TenantInvitation; token: string }>(
    '/api/v1/tenant/invitations',
    body,
  );
}

export async function cancelTenantInvitation(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/tenant/invitations/${encodeURIComponent(id)}`);
}

export async function acceptTenantInvitation(token: string) {
  return postJSON<{ tenantId: number }>(
    '/api/v1/tenant/invitations/accept',
    { token },
  );
}

export async function queryAuthTenants() {
  return getJSON<{ items: AuthTenant[]; currentTenantId?: number }>('/api/v1/auth/tenants');
}

export async function switchAuthTenant(tenantId: number) {
  return postJSON<{ token: string; expiresAt: number; user?: API.CurrentUser }>(
    '/api/v1/auth/tenants/switch',
    { tenantId },
  );
}
