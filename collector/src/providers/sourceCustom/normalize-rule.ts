import type { CustomFieldRule, CustomRuleDecl } from './types.js';

const ALLOWED_TYPES = new Set(['text', 'text_all', 'attr', 'attr_all', 'html', 'html_all']);

function attrFromType(type: string, attrHint?: string): { attr: string; multiple: boolean } {
  switch (type) {
    case 'text_all':
      return { attr: 'text', multiple: true };
    case 'html':
      return { attr: 'html', multiple: false };
    case 'html_all':
      return { attr: 'html', multiple: true };
    case 'attr':
      return { attr: (attrHint || 'src').trim(), multiple: false };
    case 'attr_all':
      return { attr: (attrHint || 'src').trim(), multiple: true };
    case 'text':
    default:
      return { attr: 'text', multiple: false };
  }
}

/** Map declarative { selector, type } or legacy { selectors, attr } to CustomFieldRule. */
export function normalizeFieldRule(field: unknown): CustomFieldRule | undefined {
  if (!field || typeof field !== 'object') return undefined;
  const o = field as Record<string, unknown>;

  if (Array.isArray(o.selectors) && o.selectors.length > 0) {
    return {
      selectors: o.selectors.map((s) => String(s).trim()).filter(Boolean),
      attr: typeof o.attr === 'string' ? o.attr : 'text',
      multiple: !!o.multiple,
      limit: typeof o.limit === 'number' ? o.limit : undefined,
      fallback: typeof o.fallback === 'string' ? o.fallback : undefined,
    };
  }

  const selector = String(o.selector ?? '').trim();
  if (!selector) return undefined;
  const type = String(o.type ?? 'text')
    .trim()
    .toLowerCase();
  if (type && !ALLOWED_TYPES.has(type)) {
    return { selectors: [selector], attr: 'text', multiple: false };
  }
  const { attr, multiple } = attrFromType(type, typeof o.attr === 'string' ? o.attr : undefined);
  return {
    selectors: [selector],
    attr,
    multiple,
    limit: typeof o.limit === 'number' ? o.limit : undefined,
    fallback: typeof o.fallback === 'string' ? o.fallback : undefined,
  };
}

/** Accept both legacy and v1 rule JSON from admin / Go. */
export function normalizeCustomRuleDecl(raw: unknown): CustomRuleDecl {
  if (!raw || typeof raw !== 'object') {
    return {};
  }
  const r = raw as Record<string, unknown>;
  const mainImages = r.mainImages ?? r.mainImage;
  const descriptionImages = r.descriptionImages ?? r.detailImages;
  const descHtml = r.description;

  const out: CustomRuleDecl = {
    title: normalizeFieldRule(r.title),
    currency: normalizeFieldRule(r.currency),
    mainImages: normalizeFieldRule(mainImages),
    descriptionImages: normalizeFieldRule(descriptionImages),
    attributes: r.attributes as CustomRuleDecl['attributes'],
    skus: r.skus as CustomRuleDecl['skus'],
    fallbacks: r.fallbacks as CustomRuleDecl['fallbacks'],
  };

  if (!out.descriptionImages && descHtml) {
    out.descriptionImages = normalizeFieldRule(descHtml);
  }
  if (!out.currency && r.price) {
    out.currency = normalizeFieldRule(r.price);
  }

  return out;
}
