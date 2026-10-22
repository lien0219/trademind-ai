import { postJSON } from '@/services/request';

export type LoginUser = {
  id: string;
  username: string; // login identity (email or phone)
  email?: string;
  phone?: string;
  displayName: string;
};

export type LoginResult = {
  token: string;
  expiresAt: number;
  user: LoginUser;
  sessionMode?: string;
  refreshToken?: string;
  deprecatedSessionMode?: boolean;
};

/** POST /api/v1/auth/refresh */
export async function refreshSession() {
  return postJSON<{ token: string; expiresAt: number; refreshToken?: string }>(
    '/api/v1/auth/refresh',
    {},
  );
}

/** POST /api/v1/auth/login */
export async function login(account: string, password: string) {
  return postJSON<LoginResult>('/api/v1/auth/login', {
    account,
    password,
  });
}

/** POST /api/v1/auth/send-email-code */
export async function sendEmailCode(email: string, scene: 'register' = 'register') {
  return postJSON<{ ok: boolean }>('/api/v1/auth/send-email-code', {
    email,
    scene,
  });
}

/** POST /api/v1/auth/register */
export async function register(params: { email: string; code: string; password: string; confirmPassword: string }) {
  return postJSON<LoginResult>('/api/v1/auth/register', params);
}

export type ProfileUser = LoginUser & {
  createdAt?: string;
  updatedAt?: string;
};

export async function changeRequiredPassword(body: {
  account: string;
  currentPassword: string;
  newPassword: string;
}): Promise<{ ok: boolean }> {
  return postJSON('/api/v1/auth/change-required-password', body);
}
