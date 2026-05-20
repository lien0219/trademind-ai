import type { Page } from 'playwright';
import { evaluateInPageVoid } from '../../browser/evaluate-in-page.js';
import type { ProductSku } from '../../types/product.js';
import { normalizeImageList, type ImageFilters } from '../sourceCustom/image-utils.js';
import { normalizePriceText } from '../sourceCustom/price-normalize.js';

const PDD_IMAGE_FILTERS: ImageFilters = {
  dedupeByImageKey: true,
  excludeKeywords: [
    'icon',
    'logo',
    'avatar',
    'app',
    'share',
    'sprite',
    'loading',
    'placeholder',
    'kefu',
    'service',
    'arrow',
    'play',
  ],
};

const PLACEHOLDER_TITLE_RE =
  /^拼多多$|^请打开|^打开app|^下载拼多多|^商品详情$|^安全验证|^登录|^验证码/i;

export type PinduoduoParseResult = {
  title: string;
  price?: number;
  currency: string;
  priceText?: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string | number | boolean>;
  skus: ProductSku[];
  warnings: string[];
  blocked: boolean;
  raw: Record<string, unknown>;
};

function sanitizeTitle(raw: string): string {
  const t = raw.replace(/\s+/g, ' ').trim();
  if (!t || PLACEHOLDER_TITLE_RE.test(t)) return '';
  if (t.length < 4) return '';
  return t.slice(0, 500);
}

function coercePrice(v: unknown): number | undefined {
  if (typeof v === 'number' && Number.isFinite(v) && v > 0) {
    if (v > 100_000) return v / 100;
    return v;
  }
  if (typeof v === 'string') {
    const norm = normalizePriceText(v);
    return norm.price;
  }
  return undefined;
}

function walkCollectStrings(x: unknown, keys: string[], depth: number, acc: string[]): void {
  if (depth > 24 || acc.length >= 40) return;
  if (!x || typeof x !== 'object') return;
  if (Array.isArray(x)) {
    for (const el of x) walkCollectStrings(el, keys, depth + 1, acc);
    return;
  }
  const o = x as Record<string, unknown>;
  for (const k of keys) {
    const v = o[k];
    if (typeof v === 'string' && v.trim()) acc.push(v.trim());
  }
  for (const v of Object.values(o)) walkCollectStrings(v, keys, depth + 1, acc);
}

function walkCollectPrice(x: unknown, depth: number): number | undefined {
  if (depth > 24) return undefined;
  if (!x || typeof x !== 'object') return undefined;
  const o = x as Record<string, unknown>;
  const priceKeys = [
    'minGroupPrice',
    'maxGroupPrice',
    'groupPrice',
    'price',
    'salePrice',
    'activityPrice',
    'linePrice',
    'marketPrice',
    'minOnSaleGroupPrice',
    'minNormalPrice',
  ];
  for (const k of priceKeys) {
    const p = coercePrice(o[k]);
    if (p !== undefined && p > 0 && p < 999_999) return p;
  }
  for (const v of Object.values(o)) {
    const p = walkCollectPrice(v, depth + 1);
    if (p !== undefined) return p;
  }
  return undefined;
}

function walkCollectImages(x: unknown, depth: number, acc: string[]): void {
  if (depth > 24 || acc.length >= 120) return;
  if (!x || typeof x !== 'object') return;
  if (Array.isArray(x)) {
    for (const el of x) {
      if (typeof el === 'string' && el.startsWith('http')) acc.push(el);
      else walkCollectImages(el, depth + 1, acc);
    }
    return;
  }
  const o = x as Record<string, unknown>;
  for (const k of ['url', 'src', 'imageUrl', 'thumbUrl', 'picUrl', 'hdUrl', 'originUrl']) {
    const v = o[k];
    if (typeof v === 'string' && v.startsWith('http')) acc.push(v);
  }
  for (const v of Object.values(o)) walkCollectImages(v, depth + 1, acc);
}

function walkCollectAttributes(x: unknown, depth: number, acc: Record<string, string>): void {
  if (depth > 24 || Object.keys(acc).length >= 40) return;
  if (!x || typeof x !== 'object') return;
  if (Array.isArray(x)) {
    for (const el of x) {
      if (el && typeof el === 'object' && !Array.isArray(el)) {
        const item = el as Record<string, unknown>;
        const key =
          (typeof item.key === 'string' && item.key) ||
          (typeof item.name === 'string' && item.name) ||
          (typeof item.propertyName === 'string' && item.propertyName) ||
          '';
        const val =
          (typeof item.value === 'string' && item.value) ||
          (typeof item.values === 'string' && item.values) ||
          (Array.isArray(item.values) && item.values.every((v) => typeof v === 'string')
            ? (item.values as string[]).join(' / ')
            : '') ||
          '';
        if (key.trim() && val.trim()) acc[key.trim()] = val.trim();
      }
      walkCollectAttributes(el, depth + 1, acc);
    }
    return;
  }
  const o = x as Record<string, unknown>;
  if (Array.isArray(o.goodsProperty)) {
    walkCollectAttributes(o.goodsProperty, depth + 1, acc);
  }
  if (Array.isArray(o.property)) {
    walkCollectAttributes(o.property, depth + 1, acc);
  }
  for (const v of Object.values(o)) walkCollectAttributes(v, depth + 1, acc);
}

function walkCollectSkus(x: unknown, depth: number, acc: ProductSku[]): void {
  if (depth > 24 || acc.length >= 80) return;
  if (!x || typeof x !== 'object') return;
  if (Array.isArray(x)) {
    for (const el of x) walkCollectSkus(el, depth + 1, acc);
    return;
  }
  const o = x as Record<string, unknown>;
  const specs = o.specs ?? o.skuSpecs ?? o.spec;
  if (Array.isArray(specs) && specs.length > 0) {
    const props: Record<string, string> = {};
    for (const s of specs) {
      if (!s || typeof s !== 'object') continue;
      const spec = s as Record<string, unknown>;
      const k =
        (typeof spec.spec_key === 'string' && spec.spec_key) ||
        (typeof spec.specKey === 'string' && spec.specKey) ||
        (typeof spec.key === 'string' && spec.key) ||
        '';
      const v =
        (typeof spec.spec_value === 'string' && spec.spec_value) ||
        (typeof spec.specValue === 'string' && spec.specValue) ||
        (typeof spec.value === 'string' && spec.value) ||
        '';
      if (k && v) props[k] = v;
    }
    if (Object.keys(props).length > 0) {
      const price = coercePrice(o.groupPrice ?? o.price ?? o.skuPrice);
      const stock = typeof o.quantity === 'number' ? o.quantity : undefined;
      acc.push({
        properties: props,
        price,
        stock,
        skuCode: typeof o.skuId === 'string' ? o.skuId : undefined,
        image: typeof o.thumbUrl === 'string' ? o.thumbUrl : undefined,
        raw: { snippet: o },
      });
    }
  }
  if (Array.isArray(o.skus)) walkCollectSkus(o.skus, depth + 1, acc);
  for (const v of Object.values(o)) walkCollectSkus(v, depth + 1, acc);
}

function parseScriptJsonCandidates(html: string): unknown[] {
  const out: unknown[] = [];
  const patterns = [
    /window\.rawData\s*=\s*(\{[\s\S]*?\});/i,
    /window\.store\s*=\s*(\{[\s\S]*?\});/i,
    /"store"\s*:\s*(\{[\s\S]*?\})\s*,\s*"router"/i,
  ];
  for (const re of patterns) {
    const m = html.match(re);
    if (!m?.[1]) continue;
    try {
      out.push(JSON.parse(m[1]));
    } catch {
      /* ignore */
    }
  }
  const scriptRe = /<script[^>]*>([\s\S]*?)<\/script>/gi;
  let sm: RegExpExecArray | null;
  while ((sm = scriptRe.exec(html)) && out.length < 12) {
    const body = sm[1];
    if (!body.includes('goods') && !body.includes('goodsName')) continue;
    const jsonRe = /(\{[\s\S]{80,80000}\})/g;
    let jm: RegExpExecArray | null;
    while ((jm = jsonRe.exec(body)) && out.length < 12) {
      try {
        out.push(JSON.parse(jm[1]));
      } catch {
        /* ignore */
      }
    }
  }
  return out;
}

async function scrollForDetailImages(page: Page): Promise<void> {
  for (let i = 0; i < 4; i++) {
    await page.evaluate(() => window.scrollBy(0, Math.max(window.innerHeight, 600))).catch(() => undefined);
    await page.waitForTimeout(400);
  }
}

export async function extractPinduoduoPayload(page: Page): Promise<Record<string, unknown>> {
  return evaluateInPageVoid(page, () => {
    const titleCandidates: string[] = [];
    const pushTitle = (s: string) => {
      const t = s.replace(/\s+/g, ' ').trim();
      if (t.length >= 4) titleCandidates.push(t);
    };

    pushTitle(document.title ?? '');
    const ogTitle = document.querySelector('meta[property="og:title"]')?.getAttribute('content');
    if (ogTitle) pushTitle(ogTitle);

    const titleSelectors = [
      '.goods-title',
      '.enable-select',
      '[class*="goodsName"]',
      '[class*="GoodsName"]',
      'h1',
      '[data-testid="goods-title"]',
    ];
    for (const sel of titleSelectors) {
      const el = document.querySelector(sel);
      const text = el?.textContent?.trim();
      if (text) pushTitle(text);
    }

    const priceTexts: string[] = [];
    const priceSelectors = ['[class*="price"]', '[class*="Price"]', '.goods-price', '[data-testid="price"]'];
    for (const sel of priceSelectors) {
      document.querySelectorAll(sel).forEach((el) => {
        const t = el.textContent?.trim();
        if (t && /[¥￥元]|\d/.test(t)) priceTexts.push(t);
      });
    }

    const imageUrls: string[] = [];
    const collectImg = (raw: string | null | undefined) => {
      if (!raw?.trim()) return;
      imageUrls.push(raw.trim());
    };

    document.querySelectorAll('img').forEach((img) => {
      collectImg(img.getAttribute('src'));
      collectImg(img.getAttribute('data-src'));
      collectImg(img.getAttribute('data-lazy-img'));
      collectImg(img.getAttribute('data-original'));
    });

    document.querySelectorAll('[style*="background"]').forEach((el) => {
      const style = el.getAttribute('style') ?? '';
      const m = style.match(/url\(['"]?([^'")]+)['"]?\)/i);
      if (m?.[1]) collectImg(m[1]);
    });

    const ogImage = document.querySelector('meta[property="og:image"]')?.getAttribute('content');
    if (ogImage) collectImg(ogImage);

    const specButtons: string[] = [];
    document
      .querySelectorAll('[class*="sku"] button, [class*="spec"] button, [class*="Sku"] [role="button"]')
      .forEach((el) => {
        const t = el.textContent?.trim();
        if (t && t.length <= 40) specButtons.push(t);
      });

    const detailImages: string[] = [];
    document
      .querySelectorAll(
        '[class*="detail"] img, [class*="Detail"] img, [id*="detail"] img, .goods-detail img',
      )
      .forEach((img) => {
        collectImg(img.getAttribute('src'));
        collectImg(img.getAttribute('data-src'));
        collectImg(img.getAttribute('data-lazy-img'));
        detailImages.push(
          img.getAttribute('src') ||
            img.getAttribute('data-src') ||
            img.getAttribute('data-lazy-img') ||
            '',
        );
      });

    return {
      pageUrl: location.href,
      docTitle: document.title ?? '',
      titleCandidates,
      priceTexts,
      imageUrls,
      detailImageUrls: detailImages.filter(Boolean),
      specButtons: [...new Set(specButtons)],
      htmlSnippet: document.documentElement?.outerHTML?.slice(0, 200_000) ?? '',
    };
  });
}

export function assemblePinduoduoProduct(
  sourceUrl: string,
  payload: Awaited<ReturnType<typeof extractPinduoduoPayload>>,
): PinduoduoParseResult {
  const warnings: string[] = [];
  const pageUrl = String(payload.pageUrl || sourceUrl);

  const jsonRoots = parseScriptJsonCandidates(String(payload.htmlSnippet ?? ''));
  const titleFromJson: string[] = [];
  const imageFromJson: string[] = [];
  const attrs: Record<string, string | number | boolean> = {};
  const attrStrings: Record<string, string> = {};
  const skuAcc: ProductSku[] = [];
  let priceFromJson: number | undefined;

  for (const root of jsonRoots) {
    walkCollectStrings(root, ['goodsName', 'goods_name', 'shareTitle', 'shareDesc', 'title'], 0, titleFromJson);
    walkCollectImages(root, 0, imageFromJson);
    walkCollectAttributes(root, 0, attrStrings);
    walkCollectSkus(root, 0, skuAcc);
    const p = walkCollectPrice(root, 0);
    if (p !== undefined) priceFromJson = priceFromJson ?? p;
  }
  for (const [k, v] of Object.entries(attrStrings)) {
    attrs[k] = v;
  }

  const titleCandidates = [
    ...(payload.titleCandidates as string[]).map(sanitizeTitle),
    ...titleFromJson.map(sanitizeTitle),
  ].filter(Boolean);
  const title = titleCandidates.sort((a, b) => b.length - a.length)[0] ?? '';

  let priceText: string | undefined;
  let price: number | undefined = priceFromJson;
  for (const pt of (payload.priceTexts as string[]) ?? []) {
    const norm = normalizePriceText(pt);
    if (norm.price && norm.price > 0) {
      price = price ?? norm.price;
      priceText = priceText ?? norm.priceText;
      break;
    }
  }

  const mainImages = normalizeImageList(
    pageUrl,
    [...((payload.imageUrls as string[]) ?? []), ...imageFromJson],
    10,
    PDD_IMAGE_FILTERS,
  );

  const descriptionImages = normalizeImageList(
    pageUrl,
    [...((payload.detailImageUrls as string[]) ?? []), ...imageFromJson],
    30,
    PDD_IMAGE_FILTERS,
  ).filter((u) => !mainImages.includes(u));

  if (descriptionImages.length === 0 && mainImages.length > 0) {
    warnings.push('详情图可能需要滚动加载或专用接口，当前未完整识别。');
  }

  const skus = skuAcc.slice(0, 80);
  if (skus.length === 0) {
    warnings.push('商品规格可能由动态接口加载，当前版本未完整识别。');
  }

  return {
    title,
    price,
    currency: 'CNY',
    priceText,
    mainImages,
    descriptionImages,
    attributes: attrs,
    skus,
    warnings,
    blocked: false,
    raw: {
      extractProvider: 'pinduoduo',
      pageMeta: { docTitle: payload.docTitle, pageUrl },
      titleCandidates: payload.titleCandidates,
      priceTexts: payload.priceTexts,
      specButtons: payload.specButtons,
      jsonRootCount: jsonRoots.length,
      extractedAt: new Date().toISOString(),
    },
  };
}

export async function extractAndAssemblePinduoduo(page: Page, sourceUrl: string): Promise<PinduoduoParseResult> {
  await scrollForDetailImages(page);
  const payload = await extractPinduoduoPayload(page);
  return assemblePinduoduoProduct(sourceUrl, payload);
}
