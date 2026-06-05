const ICON_RE =
  /avatar|icon|logo|sprite|placeholder|blank|loading|1x1|emoji|kefu|service|comment|rate|star|shop-logo|seller/i;
const SMALL_DIM_RE = /_\d{1,2}x\d{1,2}\./i;
const PLACEHOLDER_RE = /\/s\.gif(?:\?|$)|\/spaceball\.gif|\/assets\/.*loading|1x1\.gif/i;

export function normalizeImageUrl(raw: string): string {
  let u = raw.trim();
  if (!u) return '';
  if (u.startsWith('//')) u = `https:${u}`;
  if (u.startsWith('data:')) return '';
  u = u.replace(/\.webp(\?|$)/i, '.jpg$1');
  u = u.replace(/_\d+x\d+\.(jpg|jpeg|png)/i, '.$1');
  u = u.replace(/_\d+x\d+q\d+\.(jpg|jpeg|png)/i, '.$1');
  // Tmall lazy-load often appends _.jpg after quality suffix (404 if kept).
  u = u.replace(/_\.(?:jpg|jpeg|png|webp)$/i, '');
  u = u.replace(/(\.(?:jpg|jpeg|png|webp))(?:_\d+x\d+)?_\.(?:jpg|jpeg|png|webp)$/i, '$1');
  return u.split('?')[0] ?? u;
}

export function isLikelyProductImage(url: string, width?: number, height?: number): boolean {
  const u = url.toLowerCase();
  if (!u || u.startsWith('data:')) return false;
  if (PLACEHOLDER_RE.test(u)) return false;
  if (ICON_RE.test(u)) return false;
  if (SMALL_DIM_RE.test(u)) return false;
  if (typeof width === 'number' && typeof height === 'number' && width > 0 && height > 0) {
    if (width < 80 || height < 80) return false;
    if (width === height && width < 120) return false;
  }
  return /alicdn\.com|tbcdn\.cn|taobaocdn\.com|tmall\.com|1688\.com/i.test(u) || u.startsWith('http');
}

export function dedupeUrls(urls: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of urls) {
    const u = normalizeImageUrl(raw);
    if (!u || !isLikelyProductImage(u)) continue;
    const key = u.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(u.startsWith('http') ? u : raw.trim());
  }
  return out;
}
