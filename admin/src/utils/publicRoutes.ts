const PUBLIC_ADMIN_PATHS = new Set(['/', '/user/login', '/user/register', '/tenant/invitations/accept']);

export function isPublicAdminPath(pathname: string): boolean {
  const normalizedPath = pathname === '/' ? pathname : pathname.replace(/\/+$/, '');
  return PUBLIC_ADMIN_PATHS.has(normalizedPath);
}
