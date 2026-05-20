import type { Page } from 'playwright';

import { evaluateInPageVoid } from '../../browser/evaluate-in-page.js';

/**
 * Scan visible/lazy <img> on page (JD/Tmall etc. use data-lazy-img / data-origin).
 * Returns URLs sorted by approximate display area (largest first).
 */
export async function extractDomImageCandidates(page: Page, max = 24): Promise<string[]> {
  return evaluateInPageVoid(page, () => {
    const attrs = [
      'src',
      'data-src',
      'data-original',
      'data-origin',
      'data-lazy-img',
      'data-lazysrc',
      'data-url',
      'init-src',
      'data-img',
    ];
    type Cand = { url: string; area: number };
    const cands: Cand[] = [];
    const seen = new Set<string>();

    const pickUrl = (img: HTMLImageElement): string => {
      for (const a of attrs) {
        const v = img.getAttribute(a);
        if (v && !v.startsWith('data:image') && v.trim()) return v.trim();
      }
      const ss = img.getAttribute('srcset') ?? '';
      if (ss) {
        const first = ss.split(',')[0]?.trim().split(/\s+/)[0]?.trim();
        if (first) return first;
      }
      return '';
    };

    const imgs = Array.from(document.querySelectorAll('img')).slice(0, 200);
    for (const img of imgs) {
      const url = pickUrl(img);
      if (!url || seen.has(url)) continue;
      const low = url.toLowerCase();
      if (low.includes('favicon') || low.includes('logo') || low.includes('1x1')) continue;
      if (low.includes('placeholder') && !low.includes('jfs')) continue;
      seen.add(url);
      const r = img.getBoundingClientRect();
      const area = Math.max(0, r.width) * Math.max(0, r.height);
      cands.push({ url, area: area > 0 ? area : 1 });
    }

    cands.sort((a, b) => b.area - a.area);
    return cands.slice(0, max).map((c) => c.url);
  });
}
