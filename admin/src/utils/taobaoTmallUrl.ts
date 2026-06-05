const ALLOWED_HOSTS = new Set([
  'item.taobao.com',
  'detail.tmall.com',
  'detail.tmall.hk',
  'world.taobao.com',
]);

export function validateTaobaoTmallUrl(urlStr: string): boolean {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
    return ALLOWED_HOSTS.has(u.hostname.toLowerCase());
  } catch {
    return false;
  }
}

export function taobaoTmallUrlHint(urlStr: string): string | null {
  const u = urlStr.trim();
  if (!u) return null;
  if (validateTaobaoTmallUrl(u)) {
    const host = new URL(u).hostname.toLowerCase();
    if (host === 'item.taobao.com') return '已识别为淘宝商品详情页';
    if (host.startsWith('detail.tmall')) return '已识别为天猫商品详情页';
    if (host === 'world.taobao.com') return '已识别为淘宝全球购商品页';
    return '已识别为淘宝/天猫商品详情页';
  }
  return '请输入淘宝/天猫商品详情页链接（item.taobao.com、detail.tmall.com 等）';
}

export function hasTaobaoTmallLoginContext(url?: string): boolean {
  return !!url?.trim() && validateTaobaoTmallUrl(url);
}

export function resolveTaobaoTmallLoginTargetUrl(contextUrl?: string): string | undefined {
  const u = contextUrl?.trim();
  if (u && validateTaobaoTmallUrl(u)) return u;
  return undefined;
}
