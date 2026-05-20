import {
  dedupeStrings,
  isLikelyJunkImage,
  isLikelyProductImage,
  normalizeImageUrl,
  trimStr,
} from './utils.js';

export type ImageExtractBuckets = {
  mainImages: string[];
  detailImages: string[];
  skuImages: string[];
  imageCandidateCount: number;
  usedFallback: boolean;
  warnings: string[];
  extractorHints: string[];
};

const IMAGE_JSON_KEY_RE =
  /mainImages?|images?|imageList|album|offerImages?|skuImages?|detailImages?|gallery|offerImg|picUrl|fullPathImage|summImage|imageUrl/i;

const CDN_HOST_RE = /(?:1688|alicdn|tbcdn|img\.alicdn|cbu\d+\.alicdn)/i;

function pushImage(raw: string, baseUrl: string, bucket: string[], seen: Set<string>): void {
  const abs = normalizeImageUrl(trimStr(raw), baseUrl);
  if (!abs || !/^https?:\/\//i.test(abs)) return;
  if (isLikelyJunkImage(abs)) return;
  const key = abs.split('?')[0] ?? abs;
  if (seen.has(key)) return;
  seen.add(key);
  bucket.push(abs);
}

function classifyBucket(keyHint: string): 'main' | 'detail' | 'sku' | 'any' {
  const h = keyHint.toLowerCase();
  if (/detail|desc|content|wireless/i.test(h)) return 'detail';
  if (/sku|spec|prop|variant/i.test(h)) return 'sku';
  if (/main|gallery|album|offerimg|offerimage|hero|thumb/i.test(h)) return 'main';
  return 'any';
}

function walkJsonImages(
  root: unknown,
  baseUrl: string,
  main: string[],
  detail: string[],
  sku: string[],
  seen: Set<string>,
  depth = 0,
  keyHint = '',
): void {
  if (depth > 24 || root == null) return;

  if (typeof root === 'string') {
    const s = root.trim();
    if (!/^https?:\/\//i.test(s) && !/^\/\//.test(s)) return;
    if (!/\.(jpg|jpeg|png|webp|gif)(\?|$)/i.test(s) && !CDN_HOST_RE.test(s)) return;
    const kind = classifyBucket(keyHint);
    if (kind === 'detail') pushImage(s, baseUrl, detail, seen);
    else if (kind === 'sku') pushImage(s, baseUrl, sku, seen);
    else pushImage(s, baseUrl, main, seen);
    return;
  }

  if (Array.isArray(root)) {
    for (const item of root) walkJsonImages(item, baseUrl, main, detail, sku, seen, depth + 1, keyHint);
    return;
  }

  if (typeof root !== 'object') return;
  for (const [k, v] of Object.entries(root as Record<string, unknown>)) {
    const hint = `${keyHint}.${k}`;
    if (IMAGE_JSON_KEY_RE.test(k)) {
      walkJsonImages(v, baseUrl, main, detail, sku, seen, depth + 1, hint);
    } else if (typeof v === 'string' && IMAGE_JSON_KEY_RE.test(k)) {
      walkJsonImages(v, baseUrl, main, detail, sku, seen, depth + 1, hint);
    } else if (typeof v === 'object') {
      walkJsonImages(v, baseUrl, main, detail, sku, seen, depth + 1, hint);
    }
  }
}

/** 从 script JSON 多字段提取主图/详情图/SKU 图 */
export function extractImagesFromJsonRoots(roots: unknown[], baseUrl: string): ImageExtractBuckets {
  const main: string[] = [];
  const detail: string[] = [];
  const sku: string[] = [];
  const seen = new Set<string>();
  for (const r of roots) walkJsonImages(r, baseUrl, main, detail, sku, seen);

  const productMain = dedupeStrings(
    main.filter((u) => isLikelyProductImage(u) || CDN_HOST_RE.test(u)),
    20,
  );
  const productDetail = dedupeStrings(
    detail.filter((u) => isLikelyProductImage(u) || /\/img\/ibank\//i.test(u)),
    40,
  );
  const productSku = dedupeStrings(sku.filter((u) => !isLikelyJunkImage(u)), 30);

  return {
    mainImages: productMain,
    detailImages: productDetail,
    skuImages: productSku,
    imageCandidateCount: seen.size,
    usedFallback: false,
    warnings: [],
    extractorHints: productMain.length ? ['json-main'] : [],
  };
}

export function mergeDomMetaImages(input: {
  domGallery: string[];
  domDetail: string[];
  ogImage?: string;
  twitterImage?: string;
  baseUrl: string;
}): { main: string[]; detail: string[] } {
  const main: string[] = [];
  const detail: string[] = [];
  const seen = new Set<string>();
  for (const raw of input.domGallery) pushImage(raw, input.baseUrl, main, seen);
  for (const raw of input.domDetail) pushImage(raw, input.baseUrl, detail, seen);
  if (input.ogImage) pushImage(input.ogImage, input.baseUrl, main, seen);
  if (input.twitterImage) pushImage(input.twitterImage, input.baseUrl, main, seen);
  return { main: dedupeStrings(main, 16), detail: dedupeStrings(detail, 40) };
}

/** 主图缺失时用 detail/sku 图兜底 */
export function applyMainImageFallbacks(buckets: ImageExtractBuckets): ImageExtractBuckets {
  if (buckets.mainImages.length > 0) return buckets;
  const fallback = dedupeStrings(
    [...buckets.skuImages.slice(0, 4), ...buckets.detailImages.slice(0, 6)],
    10,
  ).filter((u) => !isLikelyJunkImage(u));
  if (fallback.length === 0) return buckets;
  return {
    ...buckets,
    mainImages: fallback,
    usedFallback: true,
    warnings: [...buckets.warnings, 'main_images_fallback_used'],
    extractorHints: [...buckets.extractorHints, 'fallback-detail-or-sku'],
  };
}

export function mergeImageBuckets(
  primary: ImageExtractBuckets,
  secondary: ImageExtractBuckets,
): ImageExtractBuckets {
  return {
    mainImages: dedupeStrings([...primary.mainImages, ...secondary.mainImages], 12),
    detailImages: dedupeStrings([...primary.detailImages, ...secondary.detailImages], 35),
    skuImages: dedupeStrings([...primary.skuImages, ...secondary.skuImages], 30),
    imageCandidateCount: primary.imageCandidateCount + secondary.imageCandidateCount,
    usedFallback: primary.usedFallback || secondary.usedFallback,
    warnings: [...primary.warnings, ...secondary.warnings],
    extractorHints: [...primary.extractorHints, ...secondary.extractorHints],
  };
}
