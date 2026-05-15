const JUNK_SUBSTR = [
  'logo',
  'icon',
  'placeholder',
  'loading',
  'spacer',
  'avatar',
  'qr',
  'ewm',
  'loading.gif',
];

const TINY_SIZE_RE = /[_-](?:16|20|24|30|32|40|48|50|60|64)x(?:16|20|24|30|32|40|48|50|60|64)[x_-]/i;

export function trimStr(s: string): string {
  return s.replace(/\s+/g, ' ').trim();
}

/** 将 //host 或相对路径补全为绝对 https URL */
export function normalizeImageUrl(raw: string, baseUrl: string): string | null {
  const u = trimStr(raw);
  if (!u || u.startsWith('data:') || u.startsWith('blob:')) return null;
  try {
    if (u.startsWith('//')) return `https:${u}`;
    if (u.startsWith('http://') || u.startsWith('https://')) return u;
    return new URL(u, baseUrl).href;
  } catch {
    return null;
  }
}

export function isLikelyJunkImage(url: string): boolean {
  const lower = url.toLowerCase();
  if (!/\.(?:jpg|jpeg|png|webp|gif)/i.test(lower)) return true;
  for (const j of JUNK_SUBSTR) {
    if (lower.includes(j)) return true;
  }
  if (TINY_SIZE_RE.test(lower)) return true;
  if (/_sum\.(jpg|png|webp)/i.test(lower)) return true;
  return false;
}

export function dedupeStrings(urls: string[], max: number): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const u of urls) {
    const k = u.split('?')[0] ?? u;
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(u);
    if (out.length >= max) break;
  }
  return out;
}

const ATTR_VALUE_MAX = 280;

/** 表格 / 列表参数：过滤空键值与过长内容（避免整段详情进 attributes） */
export function sanitizeAttributeMap(input: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k0, v0] of Object.entries(input)) {
    const k = trimStr(k0);
    let v = trimStr(v0);
    if (!k || !v) continue;
    if (v.length > ATTR_VALUE_MAX) continue;
    if (v.includes('<img') || v.includes('</')) continue;
    out[k] = v;
  }
  return out;
}

/** 尽力把价格、库存转成 number */
export function coerceNumber(v: unknown): number | undefined {
  if (v === null || v === undefined) return undefined;
  if (typeof v === 'number' && Number.isFinite(v)) return v;
  if (typeof v === 'string') {
    const s = v.replace(/[,，]/g, '').replace(/[^\d.]/g, '');
    const n = Number(s);
    return Number.isFinite(n) ? n : undefined;
  }
  return undefined;
}

export function coerceInt(v: unknown): number | undefined {
  const n = coerceNumber(v);
  if (n === undefined) return undefined;
  return Math.round(n);
}

/** 安全截断字符串，用于 raw 片段 */
export function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return `${s.slice(0, max)}…`;
}

/** 从 script 文本中尝试截取并 parse 一个顶级 JSON 对象（以第一个 { 匹配嵌套） */
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
