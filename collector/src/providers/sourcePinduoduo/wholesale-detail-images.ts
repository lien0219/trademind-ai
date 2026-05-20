import {
  imageDedupeKey,
  isJunkImageUrl,
  resolveImageUrl,
  type ImageFilters,
} from '../sourceCustom/image-utils.js';
import type { ImageSource } from './wholesale-detail-shared.js';

export type ClassifiedImage = {
  url: string;
  source: ImageSource;
  order: number;
  width: number;
  height: number;
};

export type RegionImageBuckets = {
  main: ClassifiedImage[];
  detail: ClassifiedImage[];
};

export type ImageFilterSummary = {
  scanned: number;
  keptMain: number;
  keptDetail: number;
  skuImagesBound: number;
  filtered: number;
  filteredBySource: Partial<Record<ImageSource, number>>;
  filteredByReason: Record<string, number>;
};

export type ImageSummary = {
  mainImagesCount: number;
  descriptionImagesCount: number;
  skuImagesCount: number;
  filteredImagesCount: number;
};

/** Debug counts only — no HTML/Cookie. */
export type ImageDebugSummary = {
  galleryCandidates: number;
  thumbnailCandidates: number;
  detailCandidates: number;
  skuImageCandidates: number;
  unknownCandidates: number;
  filteredCandidates: number;
  mainImagesAfterFilter: number;
  descriptionImagesAfterFilter: number;
  fallbackUsed: boolean;
  fallbackReason: string;
};

const PDD_PRODUCT_HOST_RE = /pddpic\.com|img-pddpic|commimg\.pddpic|yangkeduo\.com\/goods/i;

/** Strict junk — always drop. */
const STRICT_JUNK_KEYWORDS = [
  'qrcode',
  'qr_code',
  '/qr/',
  'favicon',
  'avatar',
  'kefu',
  'sprite',
  'placeholder',
  'loading.gif',
  '1x1',
  'blank.gif',
  'icon.',
  '/icon/',
  'logo.',
  '/logo/',
  'share.',
  '/share/',
  'cart.',
  '/cart/',
];

/** Soft junk — drop unless clearly a PDD product CDN image. */
const SOFT_JUNK_KEYWORDS = [
  'shop',
  'store',
  'mall',
  'merchant',
  'seller',
  'banner',
  'guarantee',
  'promise',
  'customer',
  'coupon',
  'badge',
  'service',
  'arrow',
  'play',
];

export function isPddProductImageUrl(url: string): boolean {
  return PDD_PRODUCT_HOST_RE.test(url.toLowerCase());
}

export function upgradePddImageSize(url: string): string {
  return url
    .replace(/([?&])imageView2\/[^&]+/gi, '')
    .replace(/_\d+x\d+\.(jpg|jpeg|png|webp)/gi, '.$1')
    .replace(/\/w\/\d+\/h\/\d+/gi, '');
}

function urlQualityScore(url: string, width: number, height: number): number {
  let score = url.length;
  if (isPddProductImageUrl(url)) score += 80;
  if (!/_\d+x\d+/i.test(url) && !/imageView2/i.test(url)) score += 40;
  const wm = url.match(/(?:w[=_/]|width[=_])(\d+)/i);
  if (wm) score += Math.min(Number(wm[1]), 2000);
  score += Math.max(width, height);
  return score;
}

function preferHigherResUrl(a: string, b: string, wa = 0, wb = 0, ha = 0, hb = 0): string {
  return urlQualityScore(a, wa, ha) >= urlQualityScore(b, wb, hb) ? a : b;
}

function recordFilter(summary: ImageFilterSummary, source: ImageSource, reason: string): void {
  summary.filtered += 1;
  summary.filteredBySource[source] = (summary.filteredBySource[source] ?? 0) + 1;
  summary.filteredByReason[reason] = (summary.filteredByReason[reason] ?? 0) + 1;
}

function hasKnownDimensions(width: number, height: number): boolean {
  return width > 0 && height > 0;
}

/** Size gate: unknown dimensions are kept; only reject obviously tiny icons when measured. */
function passesMainSizeGate(width: number, height: number): boolean {
  if (!hasKnownDimensions(width, height)) return true;
  const maxSide = Math.max(width, height);
  const minSide = Math.min(width, height);
  if (maxSide < 24 || minSide < 16) return false;
  if (maxSide < 80 && minSide < 40) return false;
  return true;
}

function passesDetailSizeGate(width: number, height: number): boolean {
  if (!hasKnownDimensions(width, height)) return true;
  const maxSide = Math.max(width, height);
  const minSide = Math.min(width, height);
  if (maxSide < 40 || minSide < 24) return false;
  return true;
}

function isStrictJunkUrl(url: string): boolean {
  const s = url.toLowerCase();
  for (const kw of STRICT_JUNK_KEYWORDS) {
    if (s.includes(kw)) return true;
  }
  return false;
}

function isSoftJunkUrl(url: string, fromMainRegion: boolean): boolean {
  if (isPddProductImageUrl(url) && fromMainRegion) return false;
  const s = url.toLowerCase();
  for (const kw of SOFT_JUNK_KEYWORDS) {
    if (s.includes(kw)) return true;
  }
  return false;
}

function shouldDropUrl(
  url: string,
  source: ImageSource,
  summary: ImageFilterSummary,
): boolean {
  if (isStrictJunkUrl(url)) {
    recordFilter(summary, source, 'strict_junk');
    return true;
  }
  const fromMain =
    source === 'main_gallery' || source === 'thumbnail_gallery' || source === 'sku_image';
  if (isSoftJunkUrl(url, fromMain)) {
    recordFilter(summary, source, 'soft_junk');
    return true;
  }
  if (!fromMain && isJunkImageUrl(url, SOFT_JUNK_KEYWORDS)) {
    recordFilter(summary, source, 'junk_url');
    return true;
  }
  return false;
}

function normalizeUrl(
  pageUrl: string,
  c: ClassifiedImage,
  summary: ImageFilterSummary,
): string | null {
  let url = resolveImageUrl(pageUrl, c.url);
  if (!url.startsWith('http')) {
    recordFilter(summary, c.source, 'invalid_url');
    return null;
  }
  url = upgradePddImageSize(url);
  if (shouldDropUrl(url, c.source, summary)) return null;
  return url;
}

function countBySource(items: ClassifiedImage[]): Pick<
  ImageDebugSummary,
  'galleryCandidates' | 'thumbnailCandidates' | 'detailCandidates' | 'skuImageCandidates' | 'unknownCandidates'
> {
  let galleryCandidates = 0;
  let thumbnailCandidates = 0;
  let detailCandidates = 0;
  let skuImageCandidates = 0;
  let unknownCandidates = 0;
  for (const c of items) {
    switch (c.source) {
      case 'main_gallery':
        galleryCandidates++;
        break;
      case 'thumbnail_gallery':
        thumbnailCandidates++;
        break;
      case 'detail_section':
        detailCandidates++;
        break;
      case 'sku_image':
        skuImageCandidates++;
        break;
      default:
        unknownCandidates++;
        break;
    }
  }
  return {
    galleryCandidates,
    thumbnailCandidates,
    detailCandidates,
    skuImageCandidates,
    unknownCandidates,
  };
}

function mergeMainFromBucket(
  pageUrl: string,
  items: ClassifiedImage[],
  summary: ImageFilterSummary,
): ClassifiedImage[] {
  const map = new Map<string, ClassifiedImage>();
  const sorted = [...items].sort((a, b) => {
    const rank = (s: ImageSource) => {
      if (s === 'main_gallery') return 0;
      if (s === 'thumbnail_gallery') return 1;
      return 2;
    };
    return rank(a.source) - rank(b.source) || a.order - b.order;
  });

  for (const c of sorted) {
    if (c.source !== 'main_gallery' && c.source !== 'thumbnail_gallery') continue;
    if (!passesMainSizeGate(c.width, c.height)) {
      recordFilter(summary, c.source, 'too_small');
      continue;
    }
    const url = normalizeUrl(pageUrl, c, summary);
    if (!url) continue;
    const key = imageDedupeKey(url);
    const prev = map.get(key);
    if (!prev) {
      map.set(key, { ...c, url });
    } else {
      map.set(key, {
        ...prev,
        url: preferHigherResUrl(prev.url, url, prev.width, c.width, prev.height, c.height),
      });
    }
  }
  return [...map.values()].sort((a, b) => {
    const rank = (s: ImageSource) => (s === 'main_gallery' ? 0 : 1);
    return rank(a.source) - rank(b.source) || a.order - b.order;
  });
}

function mergeDetailFromBucket(
  pageUrl: string,
  items: ClassifiedImage[],
  mainKeys: Set<string>,
  summary: ImageFilterSummary,
): ClassifiedImage[] {
  const map = new Map<string, ClassifiedImage>();
  for (const c of [...items].sort((a, b) => a.order - b.order)) {
    if (c.source !== 'detail_section') continue;
    if (!passesDetailSizeGate(c.width, c.height)) {
      recordFilter(summary, c.source, 'too_small');
      continue;
    }
    const url = normalizeUrl(pageUrl, c, summary);
    if (!url) continue;
    const key = imageDedupeKey(url);
    if (mainKeys.has(key)) {
      recordFilter(summary, c.source, 'duplicate_of_main');
      continue;
    }
    const prev = map.get(key);
    if (!prev) map.set(key, { ...c, url });
    else {
      map.set(key, {
        ...prev,
        url: preferHigherResUrl(prev.url, url, prev.width, c.width, prev.height, c.height),
      });
    }
  }
  return [...map.values()].sort((a, b) => a.order - b.order);
}

function dedupeUrlList(pageUrl: string, urls: string[], limit: number): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of urls) {
    let u = upgradePddImageSize(resolveImageUrl(pageUrl, raw));
    if (!u.startsWith('http') || isStrictJunkUrl(u)) continue;
    const key = imageDedupeKey(u);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(u);
    if (out.length >= limit) break;
  }
  return out;
}

function pickFromCandidates(
  pageUrl: string,
  items: ClassifiedImage[],
  limit: number,
  preferPdd: boolean,
): string[] {
  const scored = items
    .map((c) => {
      const url = upgradePddImageSize(resolveImageUrl(pageUrl, c.url));
      if (!url.startsWith('http') || isStrictJunkUrl(url)) return null;
      if (preferPdd && !isPddProductImageUrl(url) && isSoftJunkUrl(url, false)) return null;
      return {
        url,
        score: urlQualityScore(url, c.width, c.height) + (isPddProductImageUrl(url) ? 50 : 0),
      };
    })
    .filter((x): x is { url: string; score: number } => x !== null)
    .sort((a, b) => b.score - a.score);
  const seen = new Set<string>();
  const out: string[] = [];
  for (const s of scored) {
    const key = imageDedupeKey(s.url);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(s.url);
    if (out.length >= limit) break;
  }
  return out;
}

export type ClassifyRegionOptions = {
  mainLimit?: number;
  detailLimit?: number;
  ogImageFallback?: string;
  skuImageUrls?: string[];
  unknownCandidates?: ClassifiedImage[];
};

export type ClassifyRegionResult = {
  mainImages: string[];
  descriptionImages: string[];
  summary: ImageFilterSummary;
  imageDebug: ImageDebugSummary;
  fallbackUsed: boolean;
  fallbackReason: string;
};

export function classifyRegionImages(
  pageUrl: string,
  buckets: RegionImageBuckets,
  opts?: ClassifyRegionOptions,
): ClassifyRegionResult {
  const mainLimit = opts?.mainLimit ?? 10;
  const detailLimit = opts?.detailLimit ?? 40;
  const allForCount = [
    ...buckets.main,
    ...buckets.detail,
    ...(opts?.unknownCandidates ?? []),
  ];
  const countSeed = countBySource(allForCount);
  const skuUrlCount = (opts?.skuImageUrls ?? []).filter((u) => u.trim()).length;

  const summary: ImageFilterSummary = {
    scanned: buckets.main.length + buckets.detail.length + (opts?.unknownCandidates?.length ?? 0),
    keptMain: 0,
    keptDetail: 0,
    skuImagesBound: skuUrlCount,
    filtered: 0,
    filteredBySource: {},
    filteredByReason: {},
  };

  const mergedMain = mergeMainFromBucket(pageUrl, buckets.main, summary);
  let mainImages = mergedMain.slice(0, mainLimit).map((x) => x.url);
  let fallbackUsed = false;
  let fallbackReason = '';

  const tryFallback = (urls: string[], reason: string) => {
    if (mainImages.length > 0) return;
    const next = dedupeUrlList(pageUrl, urls, mainLimit);
    if (next.length > 0) {
      mainImages = next;
      fallbackUsed = true;
      fallbackReason = reason;
    }
  };

  if (mainImages.length === 0 && opts?.ogImageFallback?.trim()) {
    tryFallback([opts.ogImageFallback.trim()], 'og_image');
  }

  if (mainImages.length === 0 && (opts?.skuImageUrls?.length ?? 0) > 0) {
    tryFallback(opts!.skuImageUrls!, 'sku_images');
  }

  let descriptionImages: string[] = [];
  const mainKeys = new Set(mainImages.map((u) => imageDedupeKey(u)));
  const mergedDetail = mergeDetailFromBucket(pageUrl, buckets.detail, mainKeys, summary);
  descriptionImages = mergedDetail.slice(0, detailLimit).map((x) => x.url);

  if (mainImages.length === 0 && descriptionImages.length > 0) {
    tryFallback([descriptionImages[0]], 'detail_first_image');
  }

  if (mainImages.length === 0 && (opts?.unknownCandidates?.length ?? 0) > 0) {
    const fromUnknown = pickFromCandidates(pageUrl, opts!.unknownCandidates!, mainLimit, true);
    if (fromUnknown.length > 0) {
      mainImages = fromUnknown;
      fallbackUsed = true;
      fallbackReason = 'unknown_pool_pdd';
    }
  }

  if (mainImages.length === 0) {
    const pool = pickFromCandidates(
      pageUrl,
      [...buckets.main, ...buckets.detail, ...(opts?.unknownCandidates ?? [])],
      mainLimit,
      true,
    );
    if (pool.length > 0) {
      mainImages = pool;
      fallbackUsed = true;
      fallbackReason = 'page_product_pool';
    }
  }

  summary.keptMain = mainImages.length;
  summary.keptDetail = descriptionImages.length;

  const imageDebug: ImageDebugSummary = {
    galleryCandidates: countSeed.galleryCandidates,
    thumbnailCandidates: countSeed.thumbnailCandidates,
    detailCandidates: countSeed.detailCandidates,
    skuImageCandidates: countSeed.skuImageCandidates + skuUrlCount,
    unknownCandidates: countSeed.unknownCandidates,
    filteredCandidates: summary.filtered,
    mainImagesAfterFilter: mainImages.length,
    descriptionImagesAfterFilter: descriptionImages.length,
    fallbackUsed,
    fallbackReason,
  };

  return {
    mainImages,
    descriptionImages,
    summary,
    imageDebug,
    fallbackUsed,
    fallbackReason,
  };
}

export function toImageSummary(
  summary: ImageFilterSummary,
  skuImagesCount: number,
): ImageSummary {
  return {
    mainImagesCount: summary.keptMain,
    descriptionImagesCount: summary.keptDetail,
    skuImagesCount,
    filteredImagesCount: summary.filtered,
  };
}

export function normalizePddImageList(
  pageUrl: string,
  urls: string[],
  limit: number,
  filters?: ImageFilters,
): string[] {
  const extra = [...SOFT_JUNK_KEYWORDS, ...(filters?.excludeKeywords ?? [])];
  const byKey = new Map<string, string>();
  for (const raw of urls) {
    let abs = resolveImageUrl(pageUrl, raw);
    if (!abs.startsWith('http')) continue;
    abs = upgradePddImageSize(abs);
    if (isStrictJunkUrl(abs)) continue;
    if (!isPddProductImageUrl(abs) && isJunkImageUrl(abs, extra)) continue;
    const key = imageDedupeKey(abs);
    const prev = byKey.get(key);
    byKey.set(key, prev ? preferHigherResUrl(prev, abs) : abs);
  }
  return [...byKey.values()].slice(0, limit);
}
