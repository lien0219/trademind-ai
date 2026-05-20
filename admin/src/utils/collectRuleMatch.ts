/** Mirrors backend collectrule.NormalizeRuleDomain + domainMatches for UI hints. */
export function normalizeRuleDomain(raw: string): string {
  let s = raw.trim().toLowerCase();
  if (!s) return '';
  try {
    if (s.includes('://')) {
      const u = new URL(s);
      if (u.hostname) return u.hostname.toLowerCase();
    }
  } catch {
    /* fall through */
  }
  const slash = s.indexOf('/');
  if (slash >= 0) s = s.slice(0, slash);
  const colon = s.indexOf(':');
  if (colon >= 0) s = s.slice(0, colon);
  return s.replace(/^\.+|\.+$/g, '');
}

export function domainMatches(host: string, domain: string): boolean {
  const h = host.trim().toLowerCase();
  const d = normalizeRuleDomain(domain);
  if (!h || !d) return false;
  return h === d || h.endsWith(`.${d}`);
}

/** e.g. item.jd.com → jd.com, detail.1688.com → 1688.com */
export function suggestRuleDomainForHost(host: string): string {
  const parts = host.trim().toLowerCase().split('.').filter(Boolean);
  if (parts.length >= 2) return parts.slice(-2).join('.');
  return host.trim().toLowerCase();
}

export function formatRuleDomainMismatchMessage(
  rawUrl: string,
  ruleDomain: string,
): string {
  let host = '';
  try {
    host = new URL(rawUrl.trim()).hostname.toLowerCase();
  } catch {
    return `链接与所选规则域名不匹配（当前规则：${normalizeRuleDomain(ruleDomain)}）`;
  }
  const suggested = suggestRuleDomainForHost(host);
  const normalized = normalizeRuleDomain(ruleDomain);
  return `链接主机名为 ${host}，与规则域名「${normalized}」不匹配。请将规则域名改为 ${suggested}（可匹配 ${host} 等子域，勿只填 www.${suggested}）`;
}

export function ruleMatchesURL(
  rule: { domain: string; matchPattern?: string },
  rawUrl: string,
): boolean {
  let host = '';
  try {
    host = new URL(rawUrl.trim()).hostname.toLowerCase();
  } catch {
    return false;
  }
  if (!domainMatches(host, rule.domain)) return false;
  const pat = rule.matchPattern?.trim();
  if (!pat) return true;
  try {
    return new RegExp(pat).test(rawUrl.trim());
  } catch {
    return false;
  }
}
