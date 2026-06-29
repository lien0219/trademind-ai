/** 当前用户是否可写库存域（同步、重试、人工修正等）。只读账号返回 false。 */
export function canWriteInventory(role?: string | null): boolean {
  const r = (role ?? 'admin').trim().toLowerCase();
  return r !== 'readonly';
}
