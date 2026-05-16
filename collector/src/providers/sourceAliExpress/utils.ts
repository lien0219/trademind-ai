const JUNK_SUBSTR = ['logo', 'icon', 'placeholder', 'loading', 'spacer', 'avatar', '1x1', 'svg'];

const AE_SIZE_TAIL_RE = /_(\d+)x(\d+)(\.[a-z0-9]{2,5})$/i;

export function trimStr(s: string): string {
  return s.replace(/\s+/g, ' ').trim();
}

/** 尝试去掉常见 Ali CDN 末尾尺寸后缀，失败则回原 URL */
export function stripAliexpressSizeSuffix(url: string): string {
  try {
    const u = url;
    const m = AE_SIZE_TAIL_RE.exec(u);
    if (m && m[1].length <= 5 && m[2].length <= 5) {
      const rest = u.slice(0, m.index) + (m[3] ?? '').toLowerCase();
      return rest;
    }
    return url;
  } catch {
    return url;
  }
}

export function normalizeImageUrl(raw: string, baseUrl: string): string | null {
  const u = trimStr(raw);
  if (!u || u.startsWith('data:') || u.startsWith('blob:')) return null;
  try {
    let href: string;
    if (u.startsWith('//')) href = `https:${u}`;
    else if (u.startsWith('http://') || u.startsWith('https://')) href = u;
    else href = new URL(u, baseUrl).href;
    return stripAliexpressSizeSuffix(href);
  } catch {
    return null;
  }
}

export function isLikelyJunkImage(url: string): boolean {
  const lower = url.toLowerCase();
  if (!/\.(?:jpg|jpeg|png|webp|gif)(\?|$)/i.test(lower) && !/\.avif(?=$|\?)/i.test(lower)) {
    /** 少量 AE 外链无扩展名跳过 */
    if (!/\//.test(lower) || lower.includes('.svg')) return true;
    if (/\.(gif|webp|jpeg|jpg|png)(?=\?|$)/i.test(lower)) return false;
    return true;
  }
  if (/blank\.(gif|png)/i.test(lower)) return true;
  for (const j of JUNK_SUBSTR) {
    if (lower.includes(j)) return true;
  }
  return false;
}

export function dedupeStrings(urls: string[], max: number): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const u of urls) {
    const k = u.split('?')[0]?.toLowerCase() ?? u.toLowerCase();
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(u);
    if (out.length >= max) break;
  }
  return out;
}

const ATTR_VALUE_MAX = 280;

export function sanitizeAttributeMap(input: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k0, v0] of Object.entries(input)) {
    const k = trimStr(k0);
    let v = trimStr(v0);
    if (!k || !v) continue;
    if (v.length > ATTR_VALUE_MAX) continue;
    if (/<img|<\/|href=/i.test(v)) continue;
    out[k] = v;
  }
  return out;
}

export function coerceNumber(v: unknown): number | undefined {
  if (v === null || v === undefined) return undefined;
  if (typeof v === 'number' && Number.isFinite(v)) return v;
  if (typeof v === 'string') {
    const s = v.replace(/[,，]/g, '').replace(/[^\d.-]/g, '');
    const n = Number(s);
    return Number.isFinite(n) ? n : undefined;
  }
  if (typeof v === 'object' && v) {
    const o = v as Record<string, unknown>;
    const n = coerceNumber(o.value ?? o.amount ?? o.amountValue ?? o.minAmount ?? o.amountWithSymbol);
    if (n !== undefined) return n;
  }
  return undefined;
}

export function coerceInt(v: unknown): number | undefined {
  const n = coerceNumber(v);
  if (n === undefined) return undefined;
  return Math.round(n);
}

export function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return `${s.slice(0, max)}…`;
}

export function tryParseLeadingJsonObject(text: string, startAt = 0): unknown | null {
  const i = text.indexOf('{', startAt);
  if (i < 0) return null;
  let depth = 0;
  let inStr = false;
  let esc = false;
  let quote: '"' | "'" | null = null;
  for (let j = i; j < text.length; j++) {
    const c = text[j];
    if (inStr) {
      if (esc) {
        esc = false;
        continue;
      }
      if (c === '\\') {
        esc = true;
        continue;
      }
      if (quote && c === quote) {
        inStr = false;
        quote = null;
        continue;
      }
      continue;
    }
    if (c === '"' || c === "'") {
      inStr = true;
      quote = c as '"' | "'";
      continue;
    }
    if (c === '{') depth++;
    if (c === '}') {
      depth--;
      if (depth === 0) {
        const slice = text.slice(i, j + 1);
        try {
          return JSON.parse(slice) as unknown;
        } catch {
          return null;
        }
      }
    }
  }
  return null;
}

export function collectScriptJsonCandidates(script: string, maxObjects: number): unknown[] {
  const out: unknown[] = [];
  let from = 0;
  while (out.length < maxObjects && from < script.length) {
    const brace = script.indexOf('{', from);
    if (brace < 0) break;
    const obj = tryParseLeadingJsonObject(script, brace);
    if (obj === null) {
      from = brace + 1;
      continue;
    }
    out.push(obj);
    from = brace + 1;
  }
  return out;
}

/** 弱化页面标题后缀 */
export function sanitizeAliExpressTitle(raw: string): string {
  let t = trimStr(raw);
  /** 拆分常见分隔后缀 */
  t = t.replace(/\s*[|｜]+\s*.+$/u, '').replace(/\s*-\s*Aliexpress\b.*$/i, '');
  t = t.replace(/\s*-?\s*AliExpress\.com\b.*$/i, '');
  t = t.replace(/\s*（[^）]{0,30}Aliexpress[^）]*）/gi, '');
  /** 尾随价格形如 $12.34 / USD 12 */
  t = t.replace(/\s*(US\$|USD|€|\$)[\s\d.,]+\s*$/i, '').trim();
  return t.slice(0, 300);
}

const IMAGE_URL_RE =
  /\b(https?:\/\/[^"')\s]+\.(?:jpg|jpeg|png|webp|gif)(?:\?[^"')\s]*)?|(?:\/\/)[^"')\s]+\.(?:jpg|jpeg|png|webp|gif)(?:\?[^"')\s]*)?)/gi;

export function extractImageUrlsFromHtmlOrText(text: string, baseUrl: string): string[] {
  const acc: string[] = [];
  IMAGE_URL_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = IMAGE_URL_RE.exec(text))) {
    let s = m[1] ?? '';
    if (s.startsWith('//')) s = `https:${s}`;
    const abs = normalizeImageUrl(s, baseUrl);
    if (!abs || isLikelyJunkImage(abs)) continue;
    acc.push(abs);
  }
  return dedupeStrings(acc, 60);
}

export function parseJsonFragmentsFromScripts(snips: string[]): unknown[] {
  const roots: unknown[] = [];
  const seen = new Set<string>();
  function pushDedup(val: unknown) {
    if (!val || typeof val !== 'object') return;
    const sig =
      typeof (val as Record<string, unknown>).productTitle === 'string'
        ? `pt:${truncate(String((val as Record<string, unknown>).productTitle), 120)}`
        : JSON.stringify(val).slice(0, 3800);
    if (seen.has(sig)) return;
    seen.add(sig);
    roots.push(val);
  }
  for (const s of snips) {
    const t = trimStr(s);
    if (t.startsWith('{') || t.startsWith('[')) {
      try {
        pushDedup(JSON.parse(t));
      } catch {
        const obj = collectScriptJsonCandidates(s, 1)[0];
        if (obj) pushDedup(obj);
      }
      continue;
    }
    collectScriptJsonCandidates(s, 3).forEach((o) => pushDedup(o));
  }
  return roots;
}
