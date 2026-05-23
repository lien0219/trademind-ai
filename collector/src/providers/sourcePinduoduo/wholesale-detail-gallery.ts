import type { Page } from 'playwright';
import type { PifaImageCandidate } from './wholesale-detail-extract.js';

export type InteractiveGalleryResult = {
  thumbnailCandidates: PifaImageCandidate[];
  clickedMainImages: PifaImageCandidate[];
  scriptGalleryUrls: string[];
};

const MAX_ARROW_CLICKS = 5;
const MAX_THUMB_CLICKS = 12;

const GALLERY_HELPERS = `
(function () {
  if (window.__pddGallery) return;
  function pickImgUrl(el) {
    const img = el.tagName === 'IMG' ? el : el.querySelector('img');
    if (!img) {
      const style = el.style?.backgroundImage || el.getAttribute('style') || '';
      const m = /url\\(['"]?([^'")]+)['"]?\\)/i.exec(style);
      return m?.[1]?.trim() || '';
    }
    const fromSrcset = () => {
      const srcset = img.getAttribute('srcset') || '';
      if (!srcset.trim()) return '';
      const parts = srcset.split(',').map((p) => p.trim().split(/\\s+/)[0]).filter(Boolean);
      return parts[parts.length - 1] || '';
    };
    const attrs = ['data-original','data-src','data-lazy-img','data-lazy-src','data-img','data-url','data-zoom','src'];
    for (const attr of attrs) {
      const v = img.getAttribute(attr) || (attr === 'src' && img.currentSrc ? img.currentSrc : null) || (attr === 'src' ? img.src : null);
      if (v && String(v).trim() && !String(v).startsWith('data:')) return String(v).trim();
    }
    return fromSrcset();
  }
  function ancestorHint(el) {
    let p = el.parentElement;
    let depth = 0;
    let blob = '';
    while (p && depth < 10) {
      const cls = typeof p.className === 'string' ? p.className : '';
      blob += ' ' + cls + ' ' + (p.id || '') + ' ' + p.tagName;
      p = p.parentElement;
      depth++;
    }
    return blob.toLowerCase();
  }
  function isIrrelevantHint(hint) {
    return /shop|store|mall|merchant|seller|kefu|service|footer|header|nav|toolbar|cart|share|qrcode|qr|logo|icon|avatar|banner|coupon|guarantee|promise|customer|float|sidebar|loading|placeholder/.test(hint);
  }
  function findGalleryRoot() {
    const vw = window.innerWidth || 1280;
    const selectors = [
      '[class*="gallery"]','[class*="swiper"]','[class*="carousel"]',
      '[class*="image-view"]','[class*="pic-view"]','[class*="goods-img"]',
      '[class*="goodsImg"]','[class*="preview"]','[class*="thumb"]',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (!el) continue;
      const rect = el.getBoundingClientRect();
      if (rect.left < vw * 0.5 && rect.width >= 60) return el;
    }
    return null;
  }
  function findMainLargeImage(galleryRoot) {
    const vw = window.innerWidth || 1280;
    const root = galleryRoot || findGalleryRoot() || document.body;
    let best = null;
    let bestArea = 0;
    root.querySelectorAll('img').forEach((img) => {
      const rect = img.getBoundingClientRect();
      if (rect.left > vw * 0.5) return;
      if (rect.width < 80 || rect.height < 80) return;
      const hint = ancestorHint(img);
      if (isIrrelevantHint(hint)) return;
      const area = rect.width * rect.height;
      if (area > bestArea) {
        bestArea = area;
        best = { url: pickImgUrl(img), width: Math.round(rect.width), height: Math.round(rect.height) };
      }
    });
    root.querySelectorAll('[style*="background"]').forEach((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.left > vw * 0.5 || rect.width < 80 || rect.height < 80) return;
      const hint = ancestorHint(el);
      if (isIrrelevantHint(hint)) return;
      const url = pickImgUrl(el);
      if (!url) return;
      const area = rect.width * rect.height;
      if (area > bestArea) {
        bestArea = area;
        best = { url, width: Math.round(rect.width), height: Math.round(rect.height) };
      }
    });
    return best;
  }
  function collectVisibleThumbnails(galleryRoot) {
    const vw = window.innerWidth || 1280;
    const root = galleryRoot || findGalleryRoot();
    if (!root) return [];
    const out = [];
    const seen = new Set();
    const push = (url, el, rect) => {
      if (!url) return;
      const base = url.split('?')[0];
      if (seen.has(base)) return;
      seen.add(base);
      out.push({
        url,
        source: 'thumbnail_gallery',
        order: out.length,
        width: Math.round(rect.width),
        height: Math.round(rect.height),
      });
    };
    root.querySelectorAll('img').forEach((img) => {
      const rect = img.getBoundingClientRect();
      if (rect.width < 12 || rect.height < 12) return;
      if (rect.left > vw * 0.5) return;
      const hint = ancestorHint(img);
      if (isIrrelevantHint(hint)) return;
      push(pickImgUrl(img), img, rect);
    });
    root.querySelectorAll('[class*="thumb"], li, [class*="slide"]').forEach((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width < 12 || rect.height < 12 || rect.left > vw * 0.5) return;
      const hint = ancestorHint(el);
      if (isIrrelevantHint(hint)) return;
      const url = pickImgUrl(el);
      if (url) push(url, el, rect);
    });
    return out;
  }
  function findNextArrow(galleryRoot) {
    const root = galleryRoot || findGalleryRoot();
    const scope = root || document;
    const candidates = [...scope.querySelectorAll('button, [role="button"], span, div, i, svg')];
    for (const el of candidates) {
      const rect = el.getBoundingClientRect();
      if (rect.width < 8 || rect.height < 8 || rect.width > 120) continue;
      const hint = (
        (typeof el.className === 'string' ? el.className : '') +
        ' ' +
        (el.getAttribute('aria-label') || '') +
        ' ' +
        (el.textContent || '')
      ).toLowerCase();
      if (/next|right|arrow|下一|›|»|chevron-right|icon-right/.test(hint)) return el;
    }
    return null;
  }
  function findClickableThumbs(galleryRoot) {
    const vw = window.innerWidth || 1280;
    const root = galleryRoot || findGalleryRoot();
    if (!root) return [];
    const items = [];
    const selectors = [
      '[class*="thumb"] img','[class*="Thumb"] img','[class*="slide"] img',
      'li img','[class*="swiper"] img','[class*="carousel"] img',
    ];
    for (const sel of selectors) {
      root.querySelectorAll(sel).forEach((img) => {
        const rect = img.getBoundingClientRect();
        if (rect.width < 12 || rect.height < 12 || rect.left > vw * 0.5) return;
        const hint = ancestorHint(img);
        if (isIrrelevantHint(hint)) return;
        items.push(img);
      });
    }
    if (items.length === 0) {
      root.querySelectorAll('img').forEach((img) => {
        const rect = img.getBoundingClientRect();
        if (rect.width >= 12 && rect.width < 220 && rect.height < 220 && rect.left < vw * 0.5) {
          const hint = ancestorHint(img);
          if (!isIrrelevantHint(hint)) items.push(img);
        }
      });
    }
    return items;
  }
  function extractScriptGalleryUrls() {
    const urls = [];
    const seen = new Set();
    const push = (v) => {
      if (typeof v !== 'string' || !v.trim()) return;
      let s = v.trim();
      if (s.startsWith('//')) s = 'https:' + s;
      if (!/^https?:\\/\\//i.test(s)) return;
      const base = s.split('?')[0];
      if (seen.has(base)) return;
      seen.add(base);
      urls.push(s);
    };
    const walk = (x, depth) => {
      if (depth > 20 || urls.length >= 30) return;
      if (!x || typeof x !== 'object') return;
      if (Array.isArray(x)) {
        for (const el of x) {
          if (typeof el === 'string') push(el);
          else walk(el, depth + 1);
        }
        return;
      }
      const o = x;
      for (const k of ['url','src','imageUrl','thumbUrl','picUrl','hdUrl','originUrl','gallery','images','viewImageData','detailGallery']) {
        const v = o[k];
        if (typeof v === 'string') push(v);
        else if (Array.isArray(v)) walk(v, depth + 1);
      }
      for (const v of Object.values(o)) walk(v, depth + 1);
    };
    const roots = [];
    try { if (window.__INITIAL_STATE__) roots.push(window.__INITIAL_STATE__); } catch (_) {}
    try { if (window.rawData) roots.push(window.rawData); } catch (_) {}
    try { if (window.store) roots.push(window.store); } catch (_) {}
    for (const r of roots) walk(r, 0);
    return urls;
  }
  function collectDetailSectionImages() {
    const g = window.__pddGallery;
    const vw = window.innerWidth || 1280;
    const vh = window.innerHeight || 900;
    const out = [];
    const introMarkers = ['商品介绍', '商品参数', '产品参数', '图文详情'];
    let introEl = null;
    for (const marker of introMarkers) {
      const found = [...document.querySelectorAll('div, section, h2, h3, span, a')].find((el) => {
        const t = el.textContent?.trim() ?? '';
        return t === marker || (t.length <= 12 && t.startsWith(marker));
      });
      if (found) { introEl = found; break; }
    }
    const introRoot =
      introEl?.closest('section') ??
      introEl?.parentElement?.parentElement ??
      document.querySelector('[class*="detail"], [class*="intro"], [id*="detail"]');
    const roots = introRoot ? [introRoot] : [];
    if (!introRoot) {
      document.querySelectorAll('[class*="detail"], [class*="intro"]').forEach((el) => roots.push(el));
    }
    const push = (url, rect) => {
      if (!url) return;
      out.push({ url, width: Math.round(rect.width), height: Math.round(rect.height) });
    };
    for (const root of roots) {
      root.querySelectorAll('img').forEach((img) => {
        const rect = img.getBoundingClientRect();
        if (rect.width < 40 || rect.height < 24) return;
        if (rect.top < vh * 0.15 && rect.left < vw * 0.42 && rect.width < 300) return;
        const hint = g.ancestorHint(img);
        if (g.isIrrelevantHint(hint)) return;
        push(g.pickImgUrl(img), rect);
      });
      root.querySelectorAll('[style*="background"]').forEach((el) => {
        const rect = el.getBoundingClientRect();
        if (rect.width < 60 || rect.height < 40) return;
        const hint = g.ancestorHint(el);
        if (g.isIrrelevantHint(hint)) return;
        push(g.pickImgUrl(el), rect);
      });
    }
    return out;
  }
  window.__pddGallery = {
    pickImgUrl, ancestorHint, isIrrelevantHint, findGalleryRoot, findMainLargeImage,
    collectVisibleThumbnails, findNextArrow, findClickableThumbs, extractScriptGalleryUrls,
    collectDetailSectionImages,
  };
})();
`;

async function injectGalleryHelpers(page: Page): Promise<void> {
  await page.evaluate(GALLERY_HELPERS);
}

export async function waitForMainGalleryReady(page: Page): Promise<void> {
  await injectGalleryHelpers(page);
  await page.evaluate(() => {
    const root =
      document.querySelector('[class*="gallery"], [class*="swiper"], [class*="carousel"]') ??
      document.querySelector('[class*="preview"], [class*="goods-img"]');
    root?.scrollIntoView({ block: 'center', behavior: 'instant' as ScrollBehavior });
  });
  await page.waitForTimeout(600);
  await page
    .waitForFunction(
      () => {
        const imgs = document.querySelectorAll(
          '[class*="gallery"] img, [class*="swiper"] img, [class*="carousel"] img, [class*="preview"] img',
        );
        for (const img of imgs) {
          const el = img as HTMLImageElement;
          if (el.naturalWidth > 0 || el.complete) return true;
        }
        return document.querySelectorAll('img').length > 0;
      },
      { timeout: 8000 },
    )
    .catch(() => undefined);
  await page.waitForTimeout(400);
}

export async function collectInteractiveGalleryImages(page: Page): Promise<InteractiveGalleryResult> {
  await injectGalleryHelpers(page);

  const thumbnailCandidates: PifaImageCandidate[] = [];
  const clickedMainImages: PifaImageCandidate[] = [];
  const seenThumbKeys = new Set<string>();
  const seenMainKeys = new Set<string>();
  let orderSeq = 0;

  const mergeThumbs = (batch: PifaImageCandidate[]) => {
    for (const item of batch) {
      const key = item.url.split('?')[0] ?? item.url;
      if (seenThumbKeys.has(key)) continue;
      seenThumbKeys.add(key);
      thumbnailCandidates.push({ ...item, order: orderSeq++ });
    }
  };

  const mergeMain = (url: string, width: number, height: number) => {
    if (!url) return;
    const key = url.split('?')[0] ?? url;
    if (seenMainKeys.has(key)) return;
    seenMainKeys.add(key);
    clickedMainImages.push({
      url,
      source: 'main_gallery',
      order: orderSeq++,
      width,
      height,
    });
  };

  const initial = await page.evaluate(() => {
    const g = window.__pddGallery!;
    const thumbs = g.collectVisibleThumbnails();
    const main = g.findMainLargeImage();
    return { thumbs, main };
  });
  mergeThumbs(initial.thumbs as PifaImageCandidate[]);
  if (initial.main && typeof initial.main === 'object') {
    const m = initial.main as { url: string; width: number; height: number };
    mergeMain(m.url, m.width, m.height);
  }

  for (let i = 0; i < MAX_ARROW_CLICKS; i++) {
    const clicked = await page.evaluate(() => {
      const g = window.__pddGallery!;
      const arrow = g.findNextArrow();
      if (!arrow) return false;
      (arrow as HTMLElement).click();
      return true;
    });
    if (!clicked) break;
    await page.waitForTimeout(650);
    const batch = await page.evaluate(() => window.__pddGallery!.collectVisibleThumbnails());
    const before = thumbnailCandidates.length;
    mergeThumbs(batch as PifaImageCandidate[]);
    if (thumbnailCandidates.length === before) break;
  }

  const thumbCount = await page.evaluate(() => window.__pddGallery!.findClickableThumbs().length);
  const clicks = Math.min(thumbCount, MAX_THUMB_CLICKS);

  for (let idx = 0; idx < clicks; idx++) {
    await page
      .evaluate((i) => {
        const thumbs = window.__pddGallery!.findClickableThumbs();
        (thumbs[i] as HTMLElement | undefined)?.click();
      }, idx)
      .catch(() => undefined);
    await page.waitForTimeout(450);
    const main = await page.evaluate(() => window.__pddGallery!.findMainLargeImage());
    if (main && typeof main === 'object') {
      const m = main as { url: string; width: number; height: number };
      mergeMain(m.url, m.width, m.height);
    }
  }

  const scriptGalleryUrls = await page.evaluate(() => window.__pddGallery!.extractScriptGalleryUrls());

  return { thumbnailCandidates, clickedMainImages, scriptGalleryUrls };
}

export async function scrollAndCollectDetailImages(page: Page): Promise<PifaImageCandidate[]> {
  await injectGalleryHelpers(page);

  await page.evaluate(() => {
    const markers = ['商品介绍', '图文详情', '详情'];
    for (const marker of markers) {
      const tab = [...document.querySelectorAll('div, span, a, li, button')].find((el) => {
        const t = el.textContent?.trim() ?? '';
        return t === marker || (t.length <= 8 && t.includes(marker));
      });
      if (tab) {
        (tab as HTMLElement).click();
        return;
      }
    }
  });
  await page.waitForTimeout(900);

  const all: PifaImageCandidate[] = [];
  const seen = new Set<string>();
  let orderSeq = 0;

  const collectBatch = async (): Promise<void> => {
    const batch = await page.evaluate(() => window.__pddGallery!.collectDetailSectionImages());
    for (const item of batch) {
      const key = item.url.split('?')[0] ?? item.url;
      if (seen.has(key)) continue;
      seen.add(key);
      all.push({
        url: item.url,
        source: 'detail_section',
        order: orderSeq++,
        width: item.width,
        height: item.height,
      });
    }
  };

  for (let i = 0; i < 10; i++) {
    await collectBatch();
    await page.evaluate(() => window.scrollBy(0, Math.max(window.innerHeight * 0.75, 480))).catch(() => undefined);
    await page.waitForTimeout(480);
  }
  await collectBatch();

  return all;
}

declare global {
  interface Window {
    __pddGallery?: {
      pickImgUrl: (el: Element) => string;
      ancestorHint: (el: Element) => string;
      isIrrelevantHint: (hint: string) => boolean;
      findGalleryRoot: () => Element | null;
      findMainLargeImage: (galleryRoot?: Element | null) => { url: string; width: number; height: number } | null;
      collectVisibleThumbnails: (galleryRoot?: Element | null) => PifaImageCandidate[];
      findNextArrow: (galleryRoot?: Element | null) => Element | null;
      findClickableThumbs: (galleryRoot?: Element | null) => Element[];
      extractScriptGalleryUrls: () => string[];
      collectDetailSectionImages: () => Array<{ url: string; width: number; height: number }>;
    };
  }
}
