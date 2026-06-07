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

export type TaobaoTmallBatchUrlParseResult = {
  valid: string[];
  invalid: string[];
  unsupported: string[];
};

/** 解析批量链接：去空行、去重，并区分有效商品详情页 / 不支持 / 无效链接。 */
export function parseTaobaoTmallBatchUrls(raw: string): TaobaoTmallBatchUrlParseResult {
  const seen = new Set<string>();
  const valid: string[] = [];
  const invalid: string[] = [];
  const unsupported: string[] = [];
  for (const line of raw.split(/\n/)) {
    const u = line.trim();
    if (!u) continue;
    const key = u.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    const type = classifyTaobaoTmallUrl(u);
    if (type === 'product_detail') {
      valid.push(u);
    } else if (type === 'unsupported_taobao') {
      unsupported.push(u);
    } else {
      invalid.push(u);
    }
  }
  return { valid, invalid, unsupported };
}

export const TAOBAO_TMALL_BATCH_MAX_ITEMS = 20;

export const TAOBAO_TMALL_BATCH_HINT =
  '淘宝/天猫批量采集会逐条打开商品页面，建议每批不要超过 20 条。遇到登录或安全验证时，请先完成验证后重试。';

export const TAOBAO_TMALL_BATCH_LOGIN_BLOCK_MSG =
  '当前淘宝/天猫采集需要登录，请先打开淘宝/天猫采集浏览器完成登录后再开始批量采集。';

export const TAOBAO_TMALL_BATCH_VERIFY_BLOCK_MSG =
  '当前淘宝/天猫页面需要安全验证，请在采集浏览器中完成验证后重试。';
