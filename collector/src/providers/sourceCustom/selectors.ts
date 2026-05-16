import type { Page } from 'playwright';

export async function extractSelectorStrings(
  page: Page,
  selectors: string[],
  attrRaw: string | undefined,
  multiple: boolean,
): Promise<string[]> {
  const selectorsSafe = selectors.filter((s) => typeof s === 'string' && s.length <= 512).slice(0, 40);
  const attr = (attrRaw && attrRaw.trim()) || 'text';
  return page.evaluate(
    ({ selectors: sels, attr: a, multiple: mul }) => {
      const pickAttr = (el: Element, name: string): string => {
        const tag = el.tagName.toLowerCase();
        if (name === 'text') {
          const tag = el.tagName.toLowerCase();
          if (tag === 'meta') {
            const c = el.getAttribute('content');
            return typeof c === 'string' ? c.trim() : '';
          }
          return (el.textContent ?? '').trim();
        }
        if (name === 'html') return (el as HTMLElement).innerHTML?.trim() ?? '';
        if (name === 'src' || name === 'href' || name === 'content' || name === 'data-src' || name === 'data-original') {
          const v = el.getAttribute(name);
          return typeof v === 'string' ? v.trim() : '';
        }
        /** meta[property] uses attr content via rule selecting meta tags */
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
