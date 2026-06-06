const SUPPORTED_HOSTS = new Set([
  'item.taobao.com',
  'detail.tmall.com',
  'detail.tmall.hk',
  'world.taobao.com',
  'chaoshi.tmall.com',
  'ju.taobao.com',
]);

const TAOBAO_ECOSYSTEM_SUFFIXES = ['.taobao.com', '.tmall.com', '.tmall.hk'];

function isTaobaoEcosystemHost(host: string): boolean {
  const h = host.toLowerCase();
  if (SUPPORTED_HOSTS.has(h)) return true;
  if (h === 'taobao.com' || h === 'tmall.com' || h === 'tmall.hk') return true;
  return TAOBAO_ECOSYSTEM_SUFFIXES.some((s) => h.endsWith(s));
}

export type TaobaoTmallUrlType = 'product_detail' | 'unsupported_taobao' | 'invalid';

export function classifyTaobaoTmallUrl(urlStr: string): TaobaoTmallUrlType {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return 'invalid';
    const host = u.hostname.toLowerCase();
    if (SUPPORTED_HOSTS.has(host)) return 'product_detail';
    if (isTaobaoEcosystemHost(host)) return 'unsupported_taobao';
    return 'invalid';
  } catch {
    return 'invalid';
  }
}

export function validateTaobaoTmallUrl(urlStr: string): boolean {
  return classifyTaobaoTmallUrl(urlStr) === 'product_detail';
}

export function taobaoTmallUrlHint(urlStr: string): string | null {
  const u = urlStr.trim();
  if (!u) return null;
  const type = classifyTaobaoTmallUrl(u);
  if (type === 'product_detail') {
    const host = new URL(u).hostname.toLowerCase();
    if (host === 'item.taobao.com') return '已识别为淘宝商品详情页';
    if (host.startsWith('detail.tmall')) return '已识别为天猫商品详情页';
    if (host === 'chaoshi.tmall.com') return '已识别为天猫超市商品页';
    if (host === 'ju.taobao.com') return '已识别为聚划算商品页';
    if (host === 'world.taobao.com') return '已识别为淘宝全球购商品页';
    return '已识别为淘宝/天猫商品详情页';
  }
  if (type === 'unsupported_taobao') {
    return '当前链接不是标准淘宝/天猫商品详情页，请复制商品详情页链接后重试。';
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
