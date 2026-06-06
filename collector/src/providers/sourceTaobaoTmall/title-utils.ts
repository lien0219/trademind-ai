const PLATFORM_TITLE_RE = /^(淘宝网|天猫|天猫超市|聚划算|淘宝全球购)\s*[-–—|｜]?\s*/i;
const SHOP_SUFFIX_RE = /\s*[-–—|｜]\s*[^-–—|｜]{1,40}?(?:旗舰店|专卖店|专营店|官方店|自营店|店)\s*$/i;

export function cleanTaobaoTitle(raw: string): string {
  let t = raw.replace(/\s+/g, ' ').trim();
  if (!t) return '';

  t = t.replace(PLATFORM_TITLE_RE, '');
  t = t.replace(/\s*[-–—|｜]\s*(淘宝网|天猫|天猫超市)\s*$/i, '');
  t = t.replace(SHOP_SUFFIX_RE, '');
  t = t.replace(/\s*[-–—|｜]\s*淘宝网\s*$/i, '');
  t = t.replace(/\s*[-–—|｜]\s*天猫\s*$/i, '');

  return t.replace(/\s+/g, ' ').trim();
}

export function extractTitleFromDocumentTitle(docTitle: string): string {
  const t = cleanTaobaoTitle(docTitle);
  if (!t || /^(淘宝网|天猫|登录|首页)$/i.test(t)) return '';
  return t;
}

export function isLikelyPlatformOnlyTitle(title: string): boolean {
  const t = title.trim();
  if (!t) return true;
  return /^(淘宝网|天猫|登录|首页|商品详情)$/i.test(t);
}
