import type { Page } from 'playwright';

import { evaluateInPageVoid } from '../../browser/evaluate-in-page.js';

/** Common meta / link tags that carry product image URLs. */
export async function extractMetaImageHints(page: Page): Promise<string[]> {
  return evaluateInPageVoid(page, () => {
    const out: string[] = [];
    const seen = new Set<string>();
    const push = (u: string | undefined) => {
      const s = (u ?? '').trim();
      if (!s || seen.has(s)) return;
      seen.add(s);
      out.push(s);
    };

    const metas = [
      'meta[property="og:image"]',
      'meta[property="og:image:secure_url"]',
      'meta[name="twitter:image"]',
      'meta[itemprop="image"]',
      'meta[name="og:image"]',
    ];
    for (const sel of metas) {
      const el = document.querySelector(sel);
      push(el?.getAttribute('content') ?? undefined);
    }

    const link = document.querySelector('link[rel="preload"][as="image"]');
    push(link?.getAttribute('href') ?? undefined);

    return out;
  });
}
