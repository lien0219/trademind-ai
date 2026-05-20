import { deleteJSON, getWithParams, postJSON } from './request';

export type BrowserProfileRow = {
  id: string;
  name: string;
  domain: string;
  profileKey: string;
  provider?: string;
  status: string;
  lastCheckStatus?: string;
  lastCheckUrl?: string;
  lastCheckAt?: string;
  lastErrorCode?: string;
  remark?: string;
  createdAt: string;
  updatedAt: string;
};

export type Pagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type ProfileCheckResult = {
  accessStatus: string;
  finalUrl: string;
  errorCode?: string;
  message: string;
};

export async function queryBrowserProfiles(params: {
  page?: number;
  pageSize?: number;
  domain?: string;
  provider?: string;
  status?: string;
}) {
  return getWithParams<{ list: BrowserProfileRow[]; pagination: Pagination }>(
    '/api/v1/collect/browser-profiles',
    params,
  );
}

export async function createBrowserProfile(payload: {
  name: string;
  domain: string;
  provider?: string;
  remark?: string;
}) {
  return postJSON<{
    profileId: string;
    profileKey: string;
    profile: BrowserProfileRow;
  }>('/api/v1/collect/browser-profiles', payload);
}

export async function openBrowserProfileLogin(id: string, payload: { url: string }) {
  return postJSON<{ message: string; profileKey: string }>(
    `/api/v1/collect/browser-profiles/${id}/open-login`,
    payload,
  );
}

export async function checkBrowserProfile(id: string, payload: { url: string }) {
  return postJSON<ProfileCheckResult>(`/api/v1/collect/browser-profiles/${id}/check`, payload);
}

export async function disableBrowserProfile(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/collect/browser-profiles/${id}`);
}
