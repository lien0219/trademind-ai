export type ImageFilters = {
  minWidth?: number;
  minHeight?: number;
  excludeKeywords?: string[];
  dedupeByImageKey?: boolean;
};

const DEFAULT_EXCLUDE_KEYWORDS = [
  'favicon',
  'logo',
  'icon',
  'placeholder',
  '1x1',
  'pixel',
  'tracking',
  'spacer',
  'blank.gif',
  'play',
  'arrow',
  'service',
  'kefu',
  'customer',
  'avatar',
  'sprite',
];

/** Resolve protocol-relative and relative URLs. */
export function resolveImageUrl(pageUrl: string, raw: string): string {
  const s = raw.trim();
  if (!s) return '';
  if (s.startsWith('//')) {
    try {
      const base = new URL(pageUrl);
      return `${base.protocol}${s}`;
    } catch {
      return `https:${s}`;
    }
  }
  try {
    return new URL(s, pageUrl).href;
  } catch {
    return s;
  }
}

/** Upgrade JD thumbnail paths to larger size when possible. */
export function upgradeJdImageSize(url: string): string {
  return url
    .replace(/\/s\d+x\d+_/g, '/s800x800_')
    .replace(/\/n\d+\//g, '/n1/')
    .replace(/!q\d+\.jpg$/i, '!q90.jpg');
}

/** Dedupe key: strip query and normalize JD size suffix. */
export function imageDedupeKey(url: string): string {
  try {
    const u = new URL(url);
    let path = u.pathname;
    path = path.replace(/\/s\d+x\d+_/g, '/sSIZE_');
    return `${u.hostname}${path}`;
  } catch {
    return url.split('?')[0] ?? url;
  }
}

export function isJunkImageUrl(u: string, extraExclude: string[] = []): boolean {
  const s = u.toLowerCase();
  if (!s.startsWith('http://') && !s.startsWith('https://') && !s.startsWith('data:')) return true;
  if (s.startsWith('data:')) return true;

  const allExclude = [...DEFAULT_EXCLUDE_KEYWORDS, ...extraExclude.map((k) => k.toLowerCase())];
  for (const kw of allExclude) {
    if (!kw) continue;
    if (s.includes(kw)) return true;
  }
  return false;
}

export function normalizeImageList(
  pageUrl: string,
  urls: string[],
  limit: number,
  filters?: ImageFilters,
): string[] {
  const minW = filters?.minWidth ?? 0;
  const minH = filters?.minHeight ?? 0;
  const dedupe = filters?.dedupeByImageKey !== false;
  const excludeKw = filters?.excludeKeywords ?? [];

  const seen = new Set<string>();
  const out: string[] = [];

  for (const raw of urls) {
    let abs = resolveImageUrl(pageUrl, raw);
    if (!abs || !abs.startsWith('http')) continue;
    if (abs.includes('360buyimg') || abs.includes('jfs')) {
      abs = upgradeJdImageSize(abs);
    }
    if (isJunkImageUrl(abs, excludeKw)) continue;

    const key = dedupe ? imageDedupeKey(abs) : abs;
    if (seen.has(key)) continue;
    seen.add(key);

    if (minW > 0 || minH > 0) {
      const sizeM = abs.match(/\/s(\d+)x(\d+)_/);
      if (sizeM) {
        const w = Number.parseInt(sizeM[1], 10);
        const h = Number.parseInt(sizeM[2], 10);
        if (w > 0 && w < minW) continue;
        if (h > 0 && h < minH) continue;
      }
    }

    out.push(abs);
    if (out.length >= limit) break;
  }
  return out;
}
