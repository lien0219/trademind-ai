import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import { evaluateInPage } from '../../browser/evaluate-in-page.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import {
  evaluateGenericPageAccess,
  resolveAccessStatusFromSignals,
} from './access-detect.js';
import type { AnalyzePageOptions, PageStructureDigest } from './analyze-page-types.js';

const DEFAULT_MAX_CANDIDATES = 20;
const SAMPLE_MAX_LEN = 120;

function clampSample(s: string, max = SAMPLE_MAX_LEN): string {
  const t = s.replace(/\s+/g, ' ').trim();
  if (t.length <= max) return t;
  return `${t.slice(0, max)}…`;
}

async function navigatePage(page: Page, urlStr: string): Promise<{ httpStatus?: number; navError?: string }> {
  const gotoTimeout = getDefaultNavigationTimeoutMs();
  let httpStatus: number | undefined;
  try {
    const resp = await page.goto(urlStr, {
      waitUntil: 'domcontentloaded',
      timeout: gotoTimeout,
    });
    httpStatus = resp?.status();
  } catch (e) {
    const err = e instanceof Error ? e.message : String(e);
    if (/timeout/i.test(err)) {
      return { navError: `TIMEOUT:${err}` };
    }
    return { navError: `NAVIGATION_FAILED:${err}` };
  }
  await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);
  return { httpStatus };
}

type RawDigestPayload = Omit<PageStructureDigest, 'url' | 'finalUrl' | 'accessStatus'> & {
  documentTitle: string;
};

async function extractPageDigestInBrowser(page: Page, maxCandidates: number): Promise<RawDigestPayload> {
  return evaluateInPage(
    page,
    ({ maxCandidates: maxN, sampleMax }) => {
      const clamp = (s: string, maxLen = sampleMax) => {
        const t = s.replace(/\s+/g, ' ').trim();
        if (t.length <= maxLen) return t;
        return `${t.slice(0, maxLen)}…`;
      };

      type Cand = { selector: string; sample?: string; attr?: string; count: number; confidence: number };

      const buildSelector = (el: Element): string => {
        if (el.id && /^[a-zA-Z][\w-]*$/.test(el.id)) {
          return `#${el.id}`;
        }
        const tag = el.tagName.toLowerCase();
        const cls = Array.from(el.classList)
          .filter((c) => c && !/^[\d]/.test(c) && c.length <= 48)
          .slice(0, 2);
        if (cls.length) {
          return `${tag}.${cls.join('.')}`;
        }
        const parent = el.parentElement;
        if (parent) {
          const siblings = Array.from(parent.children).filter((c) => c.tagName === el.tagName);
          const idx = siblings.indexOf(el as Element);
          if (idx >= 0 && siblings.length > 1) {
            return `${tag}:nth-of-type(${idx + 1})`;
          }
        }
        return tag;
      };

      const countMatches = (sel: string): number => {
        try {
          return document.querySelectorAll(sel).length;
        } catch {
          return 0;
        }
      };

      const pushUnique = (list: Cand[], item: Cand) => {
        if (!item.selector || item.count <= 0) return;
        if (list.some((x) => x.selector === item.selector)) return;
        list.push(item);
      };

      const metaTitle =
        document.querySelector('meta[property="og:title"]')?.getAttribute('content')?.trim() ??
        document.querySelector('title')?.textContent?.trim() ??
        '';
      const metaDesc =
        document.querySelector('meta[name="description"]')?.getAttribute('content')?.trim() ?? '';
      const ogTitle = document.querySelector('meta[property="og:title"]')?.getAttribute('content')?.trim() ?? '';
      const ogImage = document.querySelector('meta[property="og:image"]')?.getAttribute('content')?.trim() ?? '';

      const titleCandidates: Cand[] = [];
      const titleSelectors = [
        '.sku-name',
        '.itemInfo-wrap .sku-name',
        '.p-name',
        '#productTitle',
        '.product-title',
        '.product-name',
        '.item-title',
        '[itemprop="name"]',
        '[property="og:title"]',
        'meta[name="twitter:title"]',
        'h1',
      ];
      for (const sel of titleSelectors) {
        const nodes = document.querySelectorAll(sel);
        if (!nodes.length) continue;
        const el = nodes[0] as Element;
        const sample =
          el.tagName.toLowerCase() === 'meta'
            ? el.getAttribute('content') ?? ''
            : (el.textContent ?? '').trim();
        if (!sample.trim()) continue;
        let confidence = 0.72;
        if (sel.includes('sku-name') || sel.includes('p-name') || sel === '#productTitle') confidence = 0.92;
        else if (sel.includes('og:title')) confidence = 0.85;
        else if (sel === 'h1') confidence = 0.35;
        pushUnique(titleCandidates, {
          selector: sel,
          sample: clamp(sample),
          attr: el.tagName.toLowerCase() === 'meta' ? 'content' : 'text',
          count: nodes.length,
          confidence,
        });
      }

      const priceCandidates: Cand[] = [];
      const priceSelectors = [
        '.summary-price .price',
        '.p-price',
        '.price',
        '[itemprop="price"]',
        '[data-price]',
        '.product-price',
        '.sale-price',
        '#price',
        '.price-current',
        '.sku-price',
      ];
      for (const sel of priceSelectors) {
        const nodes = document.querySelectorAll(sel);
        if (!nodes.length) continue;
        const el = nodes[0] as Element;
        const sample =
          el.getAttribute('content') ??
          el.getAttribute('data-price') ??
          (el.textContent ?? '').trim();
        if (!sample.trim()) continue;
        pushUnique(priceCandidates, {
          selector: sel,
          sample: clamp(sample),
          attr: el.hasAttribute('content') ? 'content' : 'text',
          count: nodes.length,
          confidence: sel.includes('itemprop') ? 0.85 : 0.65,
        });
      }

      const mainImageCandidates: Cand[] = [];
      const mainImageSelectors = [
        '#spec-list img',
        '.spec-list img',
        '.jqzoom img',
        'meta[property="og:image"]',
        '#spec-img',
        'img#spec-img',
        '.product-gallery img',
        '.gallery img',
        '#main-img',
        '.main-image img',
        '[itemprop="image"]',
      ];
      for (const sel of mainImageSelectors) {
        const nodes = document.querySelectorAll(sel);
        if (!nodes.length) continue;
        const el = nodes[0] as Element;
        const sample =
          el.getAttribute('content') ??
          el.getAttribute('src') ??
          el.getAttribute('data-src') ??
          el.getAttribute('data-origin') ??
          '';
        if (!sample.trim()) continue;
        pushUnique(mainImageCandidates, {
          selector: sel,
          sample: clamp(sample),
          attr: el.tagName.toLowerCase() === 'meta' ? 'content' : 'src',
          count: nodes.length,
          confidence: sel.includes('og:image') ? 0.88 : 0.72,
        });
      }

      const detailImageCandidates: Cand[] = [];
      const detailSelectors = [
        '#J-detail-content img',
        '.detail-content img',
        '.detail img',
        '.product-description img',
        '.description img',
        '#detail img',
        '.desc-content img',
        '.detail-content img',
      ];
      for (const sel of detailSelectors) {
        const nodes = document.querySelectorAll(sel);
        if (!nodes.length) continue;
        const el = nodes[0] as HTMLImageElement;
        const sample = el.getAttribute('src') ?? el.getAttribute('data-src') ?? '';
        if (!sample.trim()) continue;
        pushUnique(detailImageCandidates, {
          selector: sel,
          sample: clamp(sample),
          attr: 'src',
          count: nodes.length,
          confidence: 0.68,
        });
      }

      const attributeCandidates: Cand[] = [];
      const attrPatterns: Array<{ row: string; key?: string; value?: string; confidence: number }> = [
        { row: '.Ptable-item dl', key: 'dt', value: 'dd', confidence: 0.88 },
        { row: '.parameter2 li', key: '.name', value: '.value', confidence: 0.86 },
        { row: '.spec-row', key: '.spec-key', value: '.spec-value', confidence: 0.8 },
        { row: 'dl dt', confidence: 0.75 },
        { row: 'table tr', confidence: 0.55 },
        { row: '.attributes li', confidence: 0.6 },
        { row: '[class*="param"] tr', confidence: 0.58 },
      ];
      for (const pat of attrPatterns) {
        const rows = document.querySelectorAll(pat.row);
        if (!rows.length) continue;
        const sel = pat.key && pat.value ? `${pat.row} (${pat.key}/${pat.value})` : pat.row;
        pushUnique(attributeCandidates, {
          selector: sel,
          sample: clamp((rows[0]?.textContent ?? '').trim()),
          count: rows.length,
          confidence: pat.confidence,
        });
      }

      const skuCandidates: Cand[] = [];
      const skuSelectors = [
        '.sku-item',
        '.sku-list li',
        '[class*="sku"] option',
        '.product-sku .item',
        '#sku-list li',
      ];
      for (const sel of skuSelectors) {
        const nodes = document.querySelectorAll(sel);
        if (!nodes.length) continue;
        pushUnique(skuCandidates, {
          selector: sel,
          sample: clamp((nodes[0]?.textContent ?? '').trim()),
          count: nodes.length,
          confidence: 0.5,
        });
      }

      const sortTrim = (list: Cand[]) =>
        list.sort((a, b) => b.confidence - a.confidence).slice(0, maxN);

      const imageSamples: Array<{
        selector: string;
        srcSample: string;
        naturalWidth?: number;
        naturalHeight?: number;
        count: number;
      }> = [];
      const imgSeen = new Set<string>();
      const imgs = Array.from(document.querySelectorAll('img')).slice(0, 120);
      for (const img of imgs) {
        const src =
          img.getAttribute('src') ??
          img.getAttribute('data-src') ??
          img.getAttribute('data-origin') ??
          img.getAttribute('data-lazy-img') ??
          '';
        if (!src.trim() || src.startsWith('data:')) continue;
        const low = src.toLowerCase();
        if (low.includes('logo') || low.includes('icon') || low.includes('1x1')) continue;
        const sel = buildSelector(img);
        if (imgSeen.has(sel)) continue;
        imgSeen.add(sel);
        const nw = img.naturalWidth || img.width || 0;
        const nh = img.naturalHeight || img.height || 0;
        imageSamples.push({
          selector: sel,
          srcSample: clamp(src, 160),
          naturalWidth: nw || undefined,
          naturalHeight: nh || undefined,
          count: countMatches(sel),
        });
        if (imageSamples.length >= maxN) break;
      }
      imageSamples.sort((a, b) => {
        const areaA = (a.naturalWidth ?? 0) * (a.naturalHeight ?? 0);
        const areaB = (b.naturalWidth ?? 0) * (b.naturalHeight ?? 0);
        return areaB - areaA;
      });

      const domHints: string[] = [];
      if (document.querySelector('script[type="application/ld+json"]')) {
        domHints.push('has_json_ld');
      }
      if (document.querySelector('meta[property^="og:"]')) {
        domHints.push('has_open_graph');
      }
      if (document.querySelector('.product-gallery, .gallery, #spec-img')) {
        domHints.push('has_gallery_region');
      }
      if (document.querySelector('.detail, .product-description, #detail')) {
        domHints.push('has_detail_region');
      }

      const textSamples: string[] = [];
      for (const sel of ['h1', 'h2', '.price', '.product-title']) {
        const el = document.querySelector(sel);
        const t = (el?.textContent ?? '').replace(/\s+/g, ' ').trim();
        if (t) textSamples.push(`${sel}: ${clamp(t, 80)}`);
        if (textSamples.length >= 8) break;
      }

      return {
        documentTitle: document.title?.trim() ?? '',
        title: metaTitle || document.title?.trim() || '',
        meta: { title: metaTitle, description: metaDesc, ogTitle, ogImage },
        candidates: {
          title: sortTrim(titleCandidates),
          price: sortTrim(priceCandidates),
          mainImages: sortTrim(mainImageCandidates),
          descriptionImages: sortTrim(detailImageCandidates),
          attributes: sortTrim(attributeCandidates),
          sku: sortTrim(skuCandidates),
        },
        domHints,
        textSamples: textSamples.slice(0, 10),
        imageSamples: imageSamples.slice(0, maxN),
      };
    },
    { maxCandidates, sampleMax: SAMPLE_MAX_LEN },
  );
}

export async function analyzeCustomPage(
  browser: BrowserManager,
  urlStr: string,
  opts: AnalyzePageOptions = {},
): Promise<PageStructureDigest> {
  const maxCandidates = Math.min(Math.max(opts.maxCandidates ?? DEFAULT_MAX_CANDIDATES, 1), 20);
  const profileKey = opts.profileKey?.trim() ?? '';
  const useProfile = Boolean(opts.useBrowserProfile && profileKey);

  const run = async (page: Page): Promise<PageStructureDigest> => {
    const { httpStatus, navError } = await navigatePage(page, urlStr);
    if (navError) {
      const isTimeout = navError.startsWith('TIMEOUT:');
      return {
        url: urlStr,
        finalUrl: urlStr,
        accessStatus: isTimeout ? 'timeout' : 'navigation_failed',
        title: '',
        meta: { title: '', description: '', ogTitle: '', ogImage: '' },
        candidates: {
          title: [],
          price: [],
          mainImages: [],
          descriptionImages: [],
          attributes: [],
          sku: [],
        },
        domHints: [],
        textSamples: [],
        imageSamples: [],
      };
    }

    await page.waitForSelector('body', { timeout: 8000 }).catch(() => undefined);
    await page
      .evaluate(() => {
        window.scrollTo(0, Math.min(800, document.body.scrollHeight));
      })
      .catch(() => undefined);
    await new Promise((r) => setTimeout(r, 400));

    const signals = await evaluateGenericPageAccess(page, httpStatus);
    const accessStatus = resolveAccessStatusFromSignals(signals);
    const raw = await extractPageDigestInBrowser(page, maxCandidates);

    return {
      url: urlStr,
      finalUrl: signals.finalUrl || urlStr,
      accessStatus,
      title: raw.title || raw.documentTitle,
      meta: raw.meta,
      candidates: raw.candidates,
      domHints: raw.domHints,
      textSamples: raw.textSamples,
      imageSamples: raw.imageSamples,
    };
  };

  if (useProfile) {
    return browser.withCustomProfilePage(profileKey, run);
  }
  return browser.withPage(run);
}
