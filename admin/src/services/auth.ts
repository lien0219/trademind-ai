import { postJSON } from '@/services/request';

export type LoginUser = {
  id: string;
  username: string;
  displayName: string;
};

export type LoginResult = {
  token: string;
  expiresAt: number;
  user: LoginUser;
};

/** POST /api/v1/auth/login */
export async function login(username: string, password: string) {
  return postJSON<LoginResult, { username: string; password: string }>('/api/v1/auth/login', {
    username,
    password,
  });
}

export type ProfileUser = LoginUser & {
  createdAt?: string;
  updatedAt?: string;
};
