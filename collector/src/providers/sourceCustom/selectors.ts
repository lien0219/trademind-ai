import type { Page } from 'playwright';

import { evaluateInPage } from '../../browser/evaluate-in-page.js';

export async function extractSelectorStrings(
  page: Page,
  selectors: string[],
  attrRaw: string | undefined,
  multiple: boolean,
): Promise<string[]> {
  const selectorsSafe = selectors.filter((s) => typeof s === 'string' && s.length <= 512).slice(0, 40);
  const attr = (attrRaw && attrRaw.trim()) || 'text';
  return evaluateInPage(
    page,
    ({ selectors: sels, attr: a, multiple: mul }) => {
      const lazyImgAttrs = [
        'src',
        'data-src',
        'data-original',
        'data-origin',
        'data-lazy-img',
        'data-lazysrc',
        'data-lazyload',
        'data-url',
        'data-img',
        'init-src',
        'data-lazy-src',
      ];

      const pickSrcsetFirst = (el: Element): string => {
        const ss = el.getAttribute('srcset') ?? el.getAttribute('data-srcset') ?? '';
        if (!ss.trim()) return '';
        const first = ss.split(',')[0]?.trim().split(/\s+/)[0]?.trim() ?? '';
        return first;
      };

      const pickImgUrl = (el: Element): string => {
        const tag = el.tagName.toLowerCase();
        if (tag !== 'img' && tag !== 'source') {
          return '';
        }
        for (const a of lazyImgAttrs) {
          const v = el.getAttribute(a);
          if (v && !v.startsWith('data:image')) return v.trim();
        }
        return pickSrcsetFirst(el);
      };

      const pickAttr = (el: Element, name: string): string => {
        const tag = el.tagName.toLowerCase();
        if (name === 'text') {
          if (tag === 'meta') {
            const c = el.getAttribute('content');
            return typeof c === 'string' ? c.trim() : '';
          }
          return (el.textContent ?? '').trim();
        }
        if (name === 'html') return (el as HTMLElement).innerHTML?.trim() ?? '';
        if (name === 'src') {
          const direct = pickImgUrl(el);
          if (direct) return direct;
          const v = el.getAttribute('src');
          return typeof v === 'string' ? v.trim() : '';
        }
        if (name === 'href' || name === 'content' || name === 'data-src' || name === 'data-original') {
          const v = el.getAttribute(name);
          return typeof v === 'string' ? v.trim() : '';
        }
        if (tag === 'meta') {
          const c = el.getAttribute('content');
          return typeof c === 'string' ? c.trim() : '';
        }
        const v = (el as HTMLElement).getAttribute?.(name);
        return typeof v === 'string' ? v.trim() : '';
      };

      const out: string[] = [];
      const seen = new Set<string>();

      for (const sel of sels) {
        try {
          const nodes = Array.from(document.querySelectorAll(sel));
          const targets = mul ? nodes : nodes.slice(0, 1);
          for (const el of targets) {
            const val = pickAttr(el, a);
            if (!val) continue;
            if (seen.has(val)) continue;
            seen.add(val);
            out.push(val);
            if (!mul && out.length > 0) return out;
          }
        } catch {
          /** ignore invalid selectors */
        }
        if (!mul && out.length > 0) return out;
      }
      return out;
    },
    { selectors: selectorsSafe, attr, multiple },
  );
}
