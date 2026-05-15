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
};

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
