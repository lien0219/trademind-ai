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

export type ProviderPinduoduoAuthStatusValue =
  | 'ok'
  | 'not_logged_in'
  | 'wechat_auth_required'
  | 'verification_required'
  | 'app_redirect'
  | 'homepage_only'
  | 'unknown';

export type PinduoduoAuthEvidence = {
  hasProductTitle: boolean;
  hasPrice: boolean;
  hasMainImage: boolean;
  hasLoginText: boolean;
  hasWechatAuth: boolean;
  hasAppRedirect: boolean;
};

export type ProviderPinduoduoAuthStatus = {
  provider: string;
  profileKey?: string;
  status: ProviderPinduoduoAuthStatusValue;
  loginStatus?: ProviderPinduoduoAuthStatusValue;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  checkedUrl?: string;
  finalUrl?: string;
  accessStatus?: string;
  urlType?: string;
  checkMode?: string;
  evidence?: PinduoduoAuthEvidence;
};

export type ProviderPinduoduoOpenLoginResult = {
  message: string;
  alreadyOpen: boolean;
};

export type PinduoduoCheckLoginParams = {
  /** 优先：失败任务 / 采集弹窗中的商品详情链接 */
  url?: string;
  /** 设置页「用于检测的商品链接」（可传未保存的表单值） */
  testUrl?: string;
};

export async function checkPinduoduoLogin(params?: PinduoduoCheckLoginParams) {
  const body: Record<string, string> = {};
  const u = params?.url?.trim();
  const t = params?.testUrl?.trim();
  if (u) body.url = u;
  if (t) body.testUrl = t;
  return postJSON<ProviderPinduoduoAuthStatus>(
    '/api/collector/providers/pinduoduo/check-login',
    body,
  );
}

/** @deprecated 使用 checkPinduoduoLogin */
export async function fetchPinduoduoAuthStatus(contextUrl?: string, testUrl?: string) {
  return checkPinduoduoLogin({
    url: contextUrl?.trim() || undefined,
    testUrl: testUrl?.trim() || undefined,
  });
}

export async function openPinduoduoLoginBrowser(url?: string) {
  const body = url?.trim() ? { url: url.trim() } : {};
  return postJSON<ProviderPinduoduoOpenLoginResult>(
    '/api/collector/providers/pinduoduo/open-login-browser',
    body,
  );
}

export type ProviderTaobaoTmallAuthStatusValue =
  | 'logged_in'
  | 'login_required'
  | 'verify_required'
  | 'unknown';

export type ProviderTaobaoTmallAuthStatus = {
  provider: string;
  profileKey?: string;
  status: ProviderTaobaoTmallAuthStatusValue;
  loginStatus?: ProviderTaobaoTmallAuthStatusValue;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  checkedUrl?: string;
  finalUrl?: string;
};

export type ProviderTaobaoTmallOpenLoginResult = {
  message: string;
  alreadyOpen: boolean;
};

export type TaobaoTmallCheckLoginParams = {
  url?: string;
  testUrl?: string;
};

export async function checkTaobaoTmallLogin(params?: TaobaoTmallCheckLoginParams) {
  const body: Record<string, string> = {};
  const u = params?.url?.trim();
  const t = params?.testUrl?.trim();
  if (u) body.url = u;
  if (t) body.testUrl = t;
  return postJSON<ProviderTaobaoTmallAuthStatus>(
    '/api/collector/providers/taobao_tmall/check-login',
    body,
  );
}

export async function openTaobaoTmallLoginBrowser(url?: string) {
  const body = url?.trim() ? { url: url.trim() } : {};
  return postJSON<ProviderTaobaoTmallOpenLoginResult>(
    '/api/collector/providers/taobao_tmall/open-login-browser',
    body,
  );
}
