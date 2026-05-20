import { getJSON, postJSON } from './request';

export type Provider1688AuthStatusValue =
  | 'ok'
  | 'not_logged_in'
  | 'verification_required'
  | 'unknown';

export type Provider1688AuthStatus = {
  provider: string;
  status: Provider1688AuthStatusValue;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  profilePath?: string;
};

export type Provider1688OpenLoginResult = {
  profilePath: string;
  message: string;
  alreadyOpen: boolean;
};

export async function fetch1688AuthStatus() {
  return getJSON<Provider1688AuthStatus>('/api/collector/providers/1688/auth-status');
}

export async function open1688LoginBrowser() {
  return postJSON<Provider1688OpenLoginResult>('/api/collector/providers/1688/open-login-browser', {});
}
