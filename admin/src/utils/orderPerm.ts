/** 当前用户是否可写订单域（绑定 SKU、重试同步等）。只读账号返回 false。 */
export function canWriteOrders(role?: string | null): boolean {
  const r = (role ?? 'admin').trim().toLowerCase();
  return r !== 'readonly';
}
