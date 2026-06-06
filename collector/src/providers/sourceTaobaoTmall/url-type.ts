const SUPPORTED_PRODUCT_HOSTS = new Set([
  'item.taobao.com',
  'detail.tmall.com',
  'detail.tmall.hk',
  'world.taobao.com',
  'chaoshi.tmall.com',
  'ju.taobao.com',
]);

const TAOBAO_ECOSYSTEM_HOST_SUFFIXES = ['.taobao.com', '.tmall.com', '.tmall.hk'];

function normalizeHost(hostname: string): string {
  return hostname.trim().toLowerCase();
}

export function isTaobaoEcosystemHost(hostname: string): boolean {
  const host = normalizeHost(hostname);
  if (!host) return false;
  if (host === 'taobao.com' || host === 'tmall.com' || host === 'tmall.hk') return true;
  return TAOBAO_ECOSYSTEM_HOST_SUFFIXES.some((s) => host.endsWith(s));
}

export type TaobaoTmallUrlType = 'product_detail' | 'unsupported_taobao' | 'invalid';

export function classifyTaobaoTmallUrl(urlStr: string): TaobaoTmallUrlType {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return 'invalid';
    const host = normalizeHost(u.hostname);
    if (SUPPORTED_PRODUCT_HOSTS.has(host)) return 'product_detail';
    if (isTaobaoEcosystemHost(host)) return 'unsupported_taobao';
    return 'invalid';
  } catch {
    return 'invalid';
  }
}

export function validateTaobaoTmallProductUrl(urlStr: string): boolean {
  return classifyTaobaoTmallUrl(urlStr) === 'product_detail';
}

export const UNSUPPORTED_TAOBAO_URL_MESSAGE =
  '当前链接不是标准淘宝/天猫商品详情页，请复制商品详情页链接后重试。';

export function taobaoTmallUrlHint(): string {
  return '请输入淘宝/天猫商品详情页链接（item.taobao.com、detail.tmall.com、detail.tmall.hk、world.taobao.com、chaoshi.tmall.com、ju.taobao.com）';
}

export function classifyTaobaoTmallHost(urlStr: string): string {
  try {
    return normalizeHost(new URL(urlStr.trim()).hostname);
  } catch {
    return '';
  }
}
