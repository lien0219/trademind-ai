const ALLOWED_HOSTS = new Set([
  'item.taobao.com',
  'detail.tmall.com',
  'detail.tmall.hk',
  'world.taobao.com',
]);

function normalizeHost(hostname: string): string {
  return hostname.trim().toLowerCase();
}

export function validateTaobaoTmallUrl(urlStr: string): boolean {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
    const host = normalizeHost(u.hostname);
    if (ALLOWED_HOSTS.has(host)) return true;
    return false;
  } catch {
    return false;
  }
}

export function taobaoTmallUrlHint(): string {
  return '请输入淘宝/天猫商品详情页链接（item.taobao.com、detail.tmall.com、detail.tmall.hk、world.taobao.com）';
}

export function classifyTaobaoTmallHost(urlStr: string): string {
  try {
    return normalizeHost(new URL(urlStr.trim()).hostname);
  } catch {
    return '';
  }
}
