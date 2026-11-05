import { describe, expect, it } from 'vitest';
import { isPublicAdminPath } from '../publicRoutes';

describe('isPublicAdminPath', () => {
  it.each(['/', '/user/login', '/user/login/', '/user/register', '/user/register/'])(
    'keeps %s available without authentication',
    (pathname) => {
      expect(isPublicAdminPath(pathname)).toBe(true);
    },
  );

  it.each(['/dashboard', '/user', '/user/register/invite', '/tenant/members'])('protects %s', (pathname) => {
    expect(isPublicAdminPath(pathname)).toBe(false);
  });
});
