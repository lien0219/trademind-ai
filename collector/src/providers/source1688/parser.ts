import type { Page } from 'playwright';

import { evaluateInPage } from '../../browser/evaluate-in-page.js';

import type { ProductSku } from '../../types/product.js';
import type { BrowserExtractPayload, Parse1688Result } from './types.js';
import {
  ATTRIBUTE_ROW_SELECTORS,
  DETAIL_SELECTORS,
  MAIN_GALLERY_SELECTORS,
  TITLE_SELECTORS,
} from './selectors.js';
import {
  coerceInt,
  coerceNumber,
  collectScriptJsonCandidates,
  dedupeStrings,
  isLikelyJunkImage,
  normalizeImageUrl,
  sanitizeAttributeMap,
  truncate,
  trimStr,
} from './utils.js';

const SCRIPT_SNIPPET_MAX = 120_000;
const MAX_SCRIPT_FRAGMENTS = 14;
const IMAGE_URL_RE =
  /\b(https?:\/\/[^"')\s]+\.(?:jpg|jpeg|png|webp|gif)|\/\/[^"')\s]+\.(?:jpg|jpeg|png|webp|gif))(?:\?[^"')\s]*)?/gi;

function normalizeAndFilterImg(raw: string, baseUrl: string, out: string[]): void {
  const abs = normalizeImageUrl(raw, baseUrl);
  if (!abs || isLikelyJunkImage(abs)) return;
  out.push(abs);
}

function mergeUrlLists(primary: string[], secondary: string[], baseUrl: string, max: number): string[] {
  const merged: string[] = [];
  for (const bucket of [primary, secondary]) {
    for (const raw of bucket) normalizeAndFilterImg(raw, baseUrl, merged);
  }
  return dedupeStrings(merged, max);
}

function extractTitleFromRoots(roots: unknown[]): string | undefined {
  const preferredKeys = ['subject', 'offerTitle', 'productTitle'];
  let best = '';
  function walk(x: unknown, depth: number): void {
    if (depth > 22 || x === null || typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const el of x) walk(el, depth + 1);
      return;
    }
    const o = x as Record<string, unknown>;
    for (const pk of preferredKeys) {
      const candidate = o[pk];
      if (typeof candidate !== 'string') continue;
      const t = trimStr(candidate);
      if (t.length >= 6 && t.length <= 300 && !best) best = t;
    }
    /** 泛化 title 字段：仅当像商品名时采纳 */
    const title = o.title;
    if (typeof title === 'string' && !best) {
      const t = trimStr(title);
      if (t.length >= 6 && t.length <= 300 && !/^\d+$/.test(t)) best = t;
    }
    for (const v of Object.values(o)) walk(v, depth + 1);
  }
  for (const r of roots) walk(r, 0);
  return best || undefined;
}

/** 递归收集可能是图片链接的字段 */
function walkCollectImages(root: unknown, acc: Set<string>): void {
  function walk(x: unknown, depth: number, keyHint: string): void {
    if (depth > 22 || x === null || x === undefined) return;
    const hint = keyHint.toLowerCase();
    if (typeof x === 'string') {
      const s = x.trim();
      if (!/^https?:\/\//i.test(s) && !/^\/\//.test(s)) return;
      if (!/\.(jpg|jpeg|png|webp|gif)(\?|$)/i.test(s)) return;
      if (hint.includes('video') || hint.includes('coverurl')) return;
      acc.add(s.startsWith('//') ? `https:${s}` : s);
      return;
    }
    if (typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const i of x) walk(i, depth + 1, keyHint);
      return;
    }
    for (const [k, v] of Object.entries(x as Record<string, unknown>)) {
      walk(v, depth + 1, `${keyHint}.${k}`);
    }
  }
  walk(root, 0, '');
}

interface DimRow {
  name: string;
  values: string[];
}

function flattenSkuPropLike(value: unknown): DimRow[] {
  const dims: DimRow[] = [];
  if (!Array.isArray(value)) return dims;
  for (const row of value) {
    if (!row || typeof row !== 'object') continue;
    const o = row as Record<string, unknown>;
    let name =
      trimStr(String(o.prop ?? o.name ?? o.fname ?? o.label ?? (o.dimension as string | undefined) ?? ''));
    if (!name && typeof o.pid !== 'undefined') name = String(o.pid);

    let values: unknown = o.value ?? o.values ?? o.vlist ?? o.skus ?? o.enum;
    let parts: Array<{ vn?: unknown; vid?: unknown; name?: unknown; value?: unknown; text?: unknown; vname?: unknown; specId?: unknown; id?: unknown }>;
    if (Array.isArray(values)) {
      parts = values as typeof parts;
    } else if (values && typeof values === 'object' && Array.isArray((values as { list?: unknown[] }).list)) {
      parts = (values as { list: typeof parts }).list ?? [];
    } else {
      continue;
    }
    const vnSet: string[] = [];
    for (const p of parts) {
      if (!p || typeof p !== 'object') continue;
      const label = trimStr(
        String(p.name ?? p.value ?? (p.text as string | undefined) ?? p.vname ?? ''),
      );
      const vidRaw = trimStr(String(p.vid ?? p.specId ?? p.id ?? ''));
      const vn = label || vidRaw;
      if (vn) vnSet.push(vn);
    }
    if (name && vnSet.length > 0) dims.push({ name, values: dedupeStrings(vnSet, 80) });
  }
  return dims;
}

function parseComboKey(raw: string): Record<string, string> {
  const props: Record<string, string> = {};
  const segs = raw.split(/[;；#]/).map((x) => x.trim()).filter(Boolean);
  for (const seg of segs) {
    const m = seg.match(/^([^:：#\s]{1,24})\s*[#:：]\s*(.+)$/);
    if (!m) continue;
    const k = trimStr(m[1]);
    const v = trimStr(m[2]);
    if (k && v) props[k] = v;
  }
  return props;
}

/** 多维笛卡尔积（规模过大时裁剪） */
function cartesianCombinations(rows: DimRow[], maxSku: number): Array<Record<string, string>> {
  if (rows.length === 0) return [];
  let combos: Record<string, string>[] = [{}];
  for (const dim of rows) {
    const next: Record<string, string>[] = [];
    for (const c of combos) {
      for (const val of dim.values) {
        next.push({ ...c, [dim.name]: val });
      }
      if (next.length >= maxSku) break;
    }
    combos = next;
    if (combos.length >= maxSku) break;
  }
  return combos.slice(0, maxSku);
}

function skuNameFromProps(props: Record<string, string>): string {
  const keys = Object.keys(props).sort();
  return keys.map((k) => `${props[k]}`).join(' / ');
}

function mineSkuStructures(roots: unknown[]): ProductSku[] {
  let skuMaps: Record<string, unknown>[] = [];
  let skuPropArrays: unknown[] = [];

  function walkCollect(x: unknown, depth: number): void {
    if (depth > 24 || !x || typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const i of x) walkCollect(i, depth + 1);
      return;
    }
    const o = x as Record<string, unknown>;
    const keys = Object.keys(o);
    for (const k of keys) {
      if (
        (k === 'skuMap' || k === 'skuInfoMap') &&
        typeof o[k] === 'object' &&
        o[k] !== null &&
        !Array.isArray(o[k])
      ) {
        skuMaps.push(o[k] as Record<string, unknown>);
      }
      if (/^sku_props$/i.test(k) || k === 'saleProp' || k === 'saleProps' || k === 'skuProps') {
        skuPropArrays.push(o[k]);
      }
    }
    for (const v of Object.values(o)) walkCollect(v, depth + 1);
  }
  for (const r of roots) walkCollect(r, 0);

  const skuMapMerged: Record<string, unknown> = Object.assign({}, ...skuMaps);

  const rows: DimRow[] = [];
  for (const sp of skuPropArrays) rows.push(...flattenSkuPropLike(sp));

  /** 单行规格（无法组合）：每个值一条 SKU */
  function singleDimFallback(): ProductSku[] {
    const out: ProductSku[] = [];
    for (const dim of rows) {
      for (const v of dim.values) {
        const props = { [dim.name]: v };
        const line: ProductSku = {
          skuCode: '',
          properties: props,
          price: undefined,
          stock: undefined,
          image: undefined,
          raw: { dims: rows.length, inferred: 'single-dimension' },
        };
        out.push(line);
      }
    }
    return out.slice(0, 40);
  }

  if (!Object.keys(skuMapMerged).length) {
    const combos = cartesianCombinations(rows, 60);
    if (combos.length > 0) {
      return combos.map((props) => ({
        skuCode: '',
        properties: props,
        raw: {
          source: rows.length > 1 ? 'cartesian-from-skuProps' : 'single-dimension-from-skuProps',
        },
      }));
    }
    return singleDimFallback();
  }

  /**
   * skuMapMerged 形如 { "尺码:XS;颜色:白色": {...}, ... }
   */
  const combosFromMap: Array<{ key: string; props: Record<string, string>; bucket: Record<string, unknown> }> = [];
  for (const rawKey of Object.keys(skuMapMerged)) {
    const props = parseComboKey(rawKey);
    if (Object.keys(props).length === 0) continue;
    const bucket = skuMapMerged[rawKey];
    combosFromMap.push({ key: rawKey, props, bucket: (bucket ?? {}) as Record<string, unknown> });
  }

  if (combosFromMap.length === 0) {
    return singleDimFallback();
  }

  const outSkus: ProductSku[] = [];
  for (const { key: rawKey, props, bucket } of combosFromMap.slice(0, 60)) {
    const priceRaw =
      bucket.price ??
      bucket.priceMoney ??
      bucket.originPrice ??
      bucket.consignPrice ??
      bucket.retailPrice ??
      (bucket.promotionPrices as Record<string, unknown> | undefined)?.finalPrice ??
      (bucket.promotionPrices as Record<string, unknown> | undefined)?.salePriceMoney;
    let price = coerceNumber(priceRaw ?? (typeof bucket.promotionPrices === 'string' ? bucket.promotionPrices : undefined));
    if (price === undefined && typeof bucket.price === 'object' && bucket.price) {
      price =
        coerceNumber((bucket.price as Record<string, unknown>).value) ??
        coerceNumber((bucket.price as Record<string, unknown>).number);
    }

    const stock =
      coerceInt(bucket.amountOnSale) ??
      coerceInt(bucket.saleCount) ??
      coerceInt(bucket.canBookCount) ??
      coerceInt(bucket.amount) ??
      coerceInt(bucket.bookCount);

    const imgRaw =
      (typeof bucket.pic === 'string' && bucket.pic) ||
      (typeof bucket.skuPicture === 'string' && bucket.skuPicture) ||
      (typeof bucket.skuPictureUrl === 'string' && bucket.skuPictureUrl) ||
      (typeof (bucket.largePic as string | undefined) === 'string' && (bucket.largePic as string));

    let imageUrl: string | undefined;
    if (imgRaw && typeof imgRaw === 'string') {
      imageUrl =
        normalizeImageUrl(imgRaw, 'https://detail.1688.com/offer/') ?? (imgRaw.startsWith('http') ? imgRaw : undefined);
      if (imageUrl && isLikelyJunkImage(imageUrl)) imageUrl = undefined;
    }

    const skuCode =
      trimStr(
        String(bucket.specId ?? bucket.skuId ?? bucket.skuVid ?? ''),
      );

    const line: ProductSku = {
      skuCode,
      properties: props,
      price,
      stock: stock ?? undefined,
      image: imageUrl,
      raw: {
        skuMapKeySample: truncate(rawKey, 120),
      },
    };
    line.raw!.skuBucketKeys = truncate(JSON.stringify(Object.keys(bucket).slice(0, 28)), 2000);


    /** 若没有属性名，则用笛卡尔补上 */
    outSkus.push(line);
  }

  /** 格式化 skuName 显示：后端用 properties 推导，附带 raw skuNameHint 便于前端 */
  return outSkus.map((ln) => {
    const pname = skuNameFromProps(ln.properties ?? {});
    return {
      ...ln,
      raw: {
        ...(ln.raw ?? {}),
        skuNameHint: pname,
      },
    };
  });
}

function fallbackSkusTopPrice(root: unknown): ProductSku[] {
  function walk(o: unknown, depth: number): ProductSku[] {
    if (depth > 18 || !o || typeof o !== 'object') return [];
    if (Array.isArray(o)) return [];
    const rec = o as Record<string, unknown>;
    if (typeof rec.priceDisplay === 'string' || coerceNumber(rec.price ?? rec.offerPrice ?? rec.promotionPrice) !== undefined) {
      const p =
        coerceNumber(rec.price ?? rec.offerPrice ?? rec.promotionPrice ?? rec.maxPrice ?? rec.referencePrice ?? rec.marketPrice)
        ??
        coerceNumber((rec.offerPriceMoney as Record<string, unknown> | undefined)?.value);
      if (p !== undefined) {
        return [
          {
            skuCode: trimStr(String(rec.offerId ?? rec.offerID ?? '')),
            properties: {},
            price: p,
            raw: { source: 'top-level-price-field' },
          },
        ];
      }
    }
    for (const v of Object.values(rec)) {
      const hit = walk(v, depth + 1);
      if (hit.length) return hit;
    }
    return [];
  }
  return walk(root, 0);
}

function parseJsonFragmentsFromScripts(snips: string[]): unknown[] {
  const roots: unknown[] = [];
  const seen = new Set<string>();
  function pushDedup(val: unknown) {
    if (!val || typeof val !== 'object') return;
    /** 仅用结构指纹去重（控制 roots 体量） */
    const sig =
      typeof (val as Record<string, unknown>).subject === 'string'
        ? `s:${trimStr(String((val as Record<string, unknown>).subject)).slice(0, 160)}`
        : JSON.stringify(val).slice(0, 4000);
    if (seen.has(sig)) return;
    seen.add(sig);
    roots.push(val);
  }
  for (const s of snips) {
    const t = trimStr(s);
    if (t.startsWith('{') || t.startsWith('[')) {
      try {
        pushDedup(JSON.parse(t));
      } catch {
        const obj = collectScriptJsonCandidates(s, 1)[0];
        if (obj) pushDedup(obj);
      }
      continue;
    }
    collectScriptJsonCandidates(s, 2).forEach((o) => pushDedup(o));
  }
  return roots;
}

function extractImageUrlsFromText(text: string, baseUrl: string): string[] {
  const acc: Set<string> = new Set();
  IMAGE_URL_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = IMAGE_URL_RE.exec(text))) {
    const abs = normalizeImageUrl(m[1], baseUrl);
    if (!abs || isLikelyJunkImage(abs)) continue;
    acc.add(abs);
  }
  return [...acc];
}

export async function extractBrowserPayload(
  page: Page,
): Promise<BrowserExtractPayload & { __blocked__?: number }> {
  const titleSelectors = TITLE_SELECTORS;
  const mainSel = MAIN_GALLERY_SELECTORS;
  const detailSel = DETAIL_SELECTORS;
  const attrSel = ATTRIBUTE_ROW_SELECTORS;

  return evaluateInPage(
    page,
    ({
      titleSelectors,
      mainSel,
      detailSel,
      attrSel,
      snippetMax,
      maxFragments,
    }: {
      titleSelectors: string[];
      mainSel: string[];
      detailSel: string[];
      attrSel: string[];
      snippetMax: number;
      maxFragments: number;
    }) => {
      const baseHref = window.location.href;

      const pickImgUrl = (el: Element): string | null => {
        const img = el as HTMLImageElement;
        const order = ['data-lazy-src', 'data-src', 'data-original', 'data-img', 'data-zoom', 'src'];
        for (const attr of order) {
          let v =
            img.getAttribute(attr) ||
            (attr === 'src' && img.currentSrc ? img.currentSrc : null) ||
            (attr === 'src' ? img.src : null);
          if (!v?.trim()) continue;
          /** 排除 data: svg */
          if (v.startsWith('data:')) continue;
          return v.trim();
        }
        return null;
      };

      const collectFrom = (selectors: string[]): string[] => {
        const urls: string[] = [];
        for (const sel of selectors) {
          document.querySelectorAll(sel).forEach((node) => {
            const raw = pickImgUrl(node);
            if (raw) urls.push(raw);
          });
        }
        return urls;
      };

      let headingText = '';
      for (const sel of titleSelectors) {
        const txt = document.querySelector(sel)?.textContent?.trim();
        if (txt && txt.length > 2 && txt.length < 400) {
          headingText = txt;
          break;
        }
      }

      const meta: BrowserExtractPayload['meta'] = {};
      document.querySelectorAll('meta').forEach((m) => {
        const prop = m.getAttribute('property') ?? m.getAttribute('name');
        const content = m.getAttribute('content')?.trim();
        if (!content) return;
        if (prop === 'og:title' || prop === 'ogTitle') meta.ogTitle = content;
        if (prop === 'og:image') meta.ogImage = content;
        if (prop === 'keywords' || m.getAttribute('name') === 'keywords') meta.keywords = content;
        if (prop === 'description' || m.getAttribute('name') === 'description') meta.description = content;
      });

      const paramPairs: Array<{ key: string; value: string }> = [];

      /** 简易属性行抽取 */
      for (const sel of attrSel) {
        document.querySelectorAll(sel).forEach((node) => {
          /** 单列「名：值」 */
          let textBlob = '';
          node.querySelectorAll('span, dd, td').forEach((c) => {
            textBlob += ` ${c.textContent ?? ''} `;
          });
          const kv = /\b([\u4e00-\u9fa5a-zA-Z0-9·（）()]{2,30})\s*[:：]\s*([^\n\r:：]{1,120})/.exec(textBlob.trim());
          if (kv && kv[1] && kv[2]) paramPairs.push({ key: kv[1].trim(), value: kv[2].trim() });

          /** 相邻 dt/dd */
          if (node.querySelector('dt') && node.querySelector('dd')) {
            Array.from(node.querySelectorAll('dt')).forEach((dt) => {
              const dd = dt.nextElementSibling;
              const k = dt.textContent?.replace(/[:：\s]+$/, '').trim();
              const v = dd?.textContent?.trim();
              if (k && v && v.length <= 260) paramPairs.push({ key: k, value: v });
            });
          }
        });
      }

      const snippets: string[] = [];
      for (const scriptEl of Array.from(document.scripts)) {
        const t = scriptEl.text ?? '';
        if (t.length < 100) continue;
        const suspicious =
          /skuMap|skuProps|saleProp|saleProps|sku_props|offerImageList|offerImage|imageList|subject|gallery|skuInfoMap|skuModel/i.test(t);
        if (!suspicious) continue;
        snippets.push(t.length > snippetMax ? t.slice(0, snippetMax) : t);
        if (snippets.length >= maxFragments) break;
      }

      /** 类型为 json 的内联 script（体积通常较小） */
      document.querySelectorAll('script[type="application/ld+json"]').forEach((s) => {
        const txt = s.textContent?.trim();
        if (txt && txt.length < 60000 && txt.length > 20) snippets.push(txt);
      });

      /** 粗略风控页（不抛出，由外层决定是否降级） */
      const bodyPeek = document.body?.innerText?.slice(0, 2500) ?? '';
      const blocked =
        /安全验证|请完成验证|访问过于频繁|captcha/i.test(bodyPeek) ||
        (/验证/.test(bodyPeek) && headingText.length < 2);

      let docTitle = typeof document.title === 'string' ? document.title.trim() : '';
      /** 若为拦截页 Title 常为「淘宝网」或无意义 — 仍可返回由外层判断 */

      return {
        finalUrl: baseHref,
        docTitle,
        meta,
        headingText,
        galleryUrls: collectFrom(mainSel),
        detailUrls: collectFrom(detailSel),
        paramPairs,
        scriptSnippets: snippets,
        __blocked__: blocked ? 1 : 0,
      } as BrowserExtractPayload & { __blocked__?: number };
    },
    {
      titleSelectors,
      mainSel,
      detailSel,
      attrSel,
      snippetMax: SCRIPT_SNIPPET_MAX,
      maxFragments: MAX_SCRIPT_FRAGMENTS,
    },
  );
}

/** 外层去掉 evaluate 加的临时字段 */
function stripEvaluatePayload(payload: BrowserExtractPayload & { __blocked__?: number }): {
  blocked: boolean;
  payload: BrowserExtractPayload;
} {
  const { __blocked__, ...rest } = payload as BrowserExtractPayload & { __blocked__?: number };
  return {
    blocked: __blocked__ === 1,
    payload: rest,
  };
}

export function assembleParsedProduct(
  inputUrl: string,
  payloadUnclean: BrowserExtractPayload & { __blocked__?: number },
): Parse1688Result & { blocked?: boolean } {
  const { blocked, payload } = stripEvaluatePayload(payloadUnclean);

  const baseUrl = payload.finalUrl || inputUrl;

  const jsonRoots = parseJsonFragmentsFromScripts(payload.scriptSnippets);

  const titleFromModels = extractTitleFromRoots(jsonRoots);

  /** 正文标题优先级：结构化 subject > DOM 标题 > og:title > document.title（过滤拦截页占位） */
  let title =
    titleFromModels ||
    trimStr(payload.headingText || '') ||
    trimStr(payload.meta.ogTitle || '') ||
    trimStr(payload.docTitle || '');
  /** 弱化「登录 / 警告」占位 */
  if (blocked && title.length < 8) title = '';

  /** 结构化图片 */
  const fromJsonImages = new Set<string>();
  for (const r of jsonRoots) walkCollectImages(r, fromJsonImages);
  const snippetTextUrls = new Set<string>();
  for (const s of payload.scriptSnippets)
    extractImageUrlsFromText(s, baseUrl).forEach((u) => snippetTextUrls.add(u));

  /** 拆分主图与详情图候选 */
  let mainBuckets: string[] = mergeUrlLists(payload.galleryUrls, [...fromJsonImages, ...snippetTextUrls], baseUrl, 24);
  if (payload.meta.ogImage) normalizeAndFilterImg(payload.meta.ogImage, baseUrl, mainBuckets);
  /** og 首张补进主图 */
  mainBuckets = dedupeStrings(mainBuckets, 20);

  const detailDom = mergeUrlLists(payload.detailUrls, [], baseUrl, 40);
  const detailJson = [...fromJsonImages].filter((u) => !mainBuckets.some((m) => m.split('?')[0] === u.split('?')[0]));
  const descriptionImages = dedupeStrings(
    [...detailDom, ...detailJson.slice(0, 15)].filter(Boolean),
    30,
  );

  const attributes: Record<string, string> = {};
  for (const p of payload.paramPairs) {
    const k = trimStr(p.key);
    let v = trimStr(p.value);
    if (!k || !v) continue;
    if (attributes[k]) continue;
    attributes[k] = v;
  }
  /** JSON 中带「参数」「attributes」块状 */
  for (const root of jsonRoots.slice(0, 12)) {
    function pickAttrContainers(x: unknown, depth: number): void {
      if (depth > 16 || x === null || typeof x !== 'object') return;
      if (!Array.isArray(x)) {
        const o = x as Record<string, unknown>;
        const cand = (o.offerAttr as unknown[]) || (o.productAttribute as unknown[]) || (o.productAttributes as unknown[]);
        if (Array.isArray(cand)) {
          for (const row of cand) {
            if (!row || typeof row !== 'object') continue;
            const r = row as Record<string, unknown>;
            const k = trimStr(
              String(r.name ?? r.attributeName ?? r.attributeID ?? ''),
            );
            let v = trimStr(String(r.value ?? r.attributeValue ?? r.text ?? ''));
            if (k && v && !attributes[k]) attributes[k] = v;
          }
        }
      }
      if (Array.isArray(x)) for (const i of x) pickAttrContainers(i, depth + 1);
      else for (const v of Object.values(x as Record<string, unknown>)) pickAttrContainers(v, depth + 1);
    }
    pickAttrContainers(root, 0);
  }

  const safeAttrs = sanitizeAttributeMap(attributes);

  let skus: ProductSku[] = [];
  try {
    skus = mineSkuStructures(jsonRoots);
  } catch {
    skus = [];
  }
  if (skus.length === 0) {
    for (const r of jsonRoots.slice(0, 4)) {
      skus = fallbackSkusTopPrice(r);
      if (skus.length) break;
    }
  }

  const mainImages = dedupeStrings(mainBuckets, 10);

  const raw: Record<string, unknown> = {
    title: title || payload.docTitle,
    url: baseUrl,
    mainImageCandidates: dedupeStrings(mainBuckets, 15),
    detailImageCandidates: dedupeStrings([...descriptionImages, ...detailDom], 35),
    attributeCandidates: safeAttrs,
    skuCandidates: skus.map((s) => ({
      properties: s.properties,
      price: s.price,
      stock: s.stock,
      skuCode: s.skuCode,
    })),
    pageMeta: {
      ...payload.meta,
      blockedHint: blocked,
      docTitle: truncate(payload.docTitle, 200),
    },
    extractedAt: new Date().toISOString(),
    jsonRootCount: jsonRoots.length,
    scriptSnippetCount: payload.scriptSnippets.length,
    scriptDigest: truncate(
      payload.scriptSnippets
        .slice(0, 2)
        .map((s) => s.slice(0, 400))
        .join('|'),
      2400,
    ),
  };

  return {
    title: title?.trim() || '（解析：未命名商品）',
    mainImages,
    descriptionImages,
    attributes: safeAttrs,
    skus,
    raw,
    blocked,
  };
}
