import { getJSON, getWithParams, patchJSON, postJSON, putJSON } from '@/services/request';

export type StorePermissionRow = {
  id?: string;
  storeId: string;
  storeName?: string;
  platform?: string;
  permissionScope: 'view' | 'operate' | 'manage';
};

export type AdminUserRow = {
  id: string;
  username: string;
  email?: string;
  phone?: string;
  displayName: string;
  role: string;
  status: string;
  storePermissions?: StorePermissionRow[];
  lastOperationAt?: string;
  createdAt: string;
  updatedAt: string;
};

export async function fetchAdminUsers(params: Record<string, string | number | undefined>) {
  return getWithParams<{ list: AdminUserRow[]; pagination: { total: number } }>(
    '/api/v1/admin/users',
    params,
  );
}

export async function fetchAdminUser(id: string) {
  return getJSON<AdminUserRow>(`/api/v1/admin/users/${id}`);
}

export async function createAdminUser(body: {
  email?: string;
  phone?: string;
  password: string;
  displayName?: string;
  role: string;
}) {
  return postJSON<AdminUserRow>('/api/v1/admin/users', body);
}

type UpdateAdminUserBody = { displayName?: string; role?: string; status?: string };

type SetStorePermissionsBody = {
  items: { storeId: string; platform?: string; permissionScope: string }[];
};

export async function updateAdminUser(id: string, body: UpdateAdminUserBody) {
  return patchJSON<AdminUserRow, UpdateAdminUserBody>(`/api/v1/admin/users/${id}`, body);
}

export async function setAdminUserStorePermissions(id: string, items: SetStorePermissionsBody['items']) {
  const body: SetStorePermissionsBody = { items };
  return putJSON<AdminUserRow, SetStorePermissionsBody>(
    `/api/v1/admin/users/${id}/store-permissions`,
    body,
  );
}

export type AdminRoleRow = {
  code: string;
  name: string;
  description: string;
  permissions: string[];
  enabled: boolean;
  system: boolean;
  memberCount: number;
};

export async function fetchAdminRoles(): Promise<{ list: AdminRoleRow[] }> {
  return getJSON('/api/v1/admin/roles');
}

export async function fetchRolePermissionCatalog(): Promise<{ permissions: string[] }> {
  return getJSON('/api/v1/admin/permissions');
}

export type RoleEditorValues = {
  name?: string;
  description?: string;
  permissions?: string[];
  enabled?: boolean;
};

export async function createAdminRole(values: RoleEditorValues): Promise<AdminRoleRow> {
  return postJSON('/api/v1/admin/roles', values);
}

export async function updateAdminRole(code: string, values: RoleEditorValues): Promise<AdminRoleRow> {
  return patchJSON(`/api/v1/admin/roles/${encodeURIComponent(code)}`, values);
}

export async function resetAdminUserPassword(id: string): Promise<{ temporaryPassword: string }> {
  return postJSON(`/api/v1/admin/users/${encodeURIComponent(id)}/reset-password`, {});
}
