import type { Page } from 'playwright';

import type { ProductSku } from '../../types/product.js';
import type { AeBrowserPayload, AeAssembleOutput } from './types.js';
import {
  AE_ATTRIBUTE_CONTAINER_SELECTORS,
  AE_DETAIL_AREA_SELECTORS,
  AE_MAIN_GALLERY_SELECTORS,
  AE_TITLE_SELECTORS,
} from './selectors.js';
import {
  coerceInt,
  coerceNumber,
  dedupeStrings,
  extractImageUrlsFromHtmlOrText,
  isLikelyJunkImage,
  normalizeImageUrl,
  parseJsonFragmentsFromScripts,
  sanitizeAliExpressTitle,
  sanitizeAttributeMap,
  truncate,
  trimStr,
  collectScriptJsonCandidates,
} from './utils.js';

const SCRIPT_SNIPPET_MAX = 120_000;
const MAX_SCRIPT_FRAGMENTS = 16;

function normalizeAndCollectImg(raw: string, baseUrl: string, out: string[]): void {
  const abs = normalizeImageUrl(raw, baseUrl);
  if (!abs || isLikelyJunkImage(abs)) return;
  out.push(abs);
}

function mergeUrlBuckets(primary: string[], secondary: string[], baseUrl: string, max: number): string[] {
  const merged: string[] = [];
  for (const bucket of [primary, secondary]) {
    for (const raw of bucket) normalizeAndCollectImg(raw, baseUrl, merged);
  }
  return dedupeStrings(merged, max);
}

/** 递归从 JSON 中抽标题片段 */
function walkExtractTitleCandidates(x: unknown, depth: number, acc: string[]): void {
  if (depth > 22 || acc.length >= 24) return;
  if (!x || typeof x !== 'object') return;
  if (Array.isArray(x)) {
    for (const el of x) walkExtractTitleCandidates(el, depth + 1, acc);
    return;
  }
  const o = x as Record<string, unknown>;
  const prefs = ['productTitle', 'subject', 'goodsTitle', 'pageTitle'];
  for (const k of prefs) {
    const v = o[k];
    if (typeof v === 'string') {
      const t = sanitizeAliExpressTitle(v);
      if (t.length >= 6 && t.length <= 380) acc.push(t);
    }
  }
  for (const vv of Object.values(o)) walkExtractTitleCandidates(vv, depth + 1, acc);
}

function pickBestTitle(cands: string[]): string {
  const dedup = dedupeStrings([...new Set(cands)], 80);
  return dedup.sort((a, b) => b.length - a.length)[0] ?? '';
}

function walkCollectCurrency(x: unknown, depth: number, acc: Set<string>): void {
  if (depth > 22) return;
  if (!x || typeof x !== 'object') return;
  const o = x as Record<string, unknown>;
  for (const [k, v] of Object.entries(o)) {
    const kl = k.toLowerCase();
    if (
      kl === 'currency' ||
      kl === 'currencycode' ||
      kl === 'cur_currency' ||
      kl === 'origincurrency'
    ) {
      if (typeof v === 'string' && /^[A-Z]{2,5}$/.test(v.trim())) acc.add(v.trim());
    }
    if (typeof v === 'string' && kl.includes('currency') && /^[A-Z]{2,5}$/.test(trimStr(v))) {
      acc.add(trimStr(v));
    }
    walkCollectCurrency(v, depth + 1, acc);
  }
}

/** 递归搜价格（AE 多级嵌套） */
function divePrice(bucket: Record<string, unknown>, depth = 0): number | undefined {
  if (depth > 14) return undefined;
  const tryKeys = ['value', 'number', 'amount', 'salePrice', 'minPrice', 'maxPrice'];
  for (const k of tryKeys) {
    const pv = coerceNumber(bucket[k]);
    if (pv !== undefined && pv >= 0.01 && pv < 999_999) return pv;
  }
  /** `salePrice: {amount:{value:string}}` */
  for (const v of Object.values(bucket)) {
    if (v && typeof v === 'object' && !Array.isArray(v)) {
      const p = divePrice(v as Record<string, unknown>, depth + 1);
      if (p !== undefined) return p;
    }
  }
  return undefined;
}

/** 判断是否像 AE SKU 路由键 `14:100014064#7643024` */
function looksLikeAeSkuRouteKey(key: string): boolean {
  if (key.length < 5 || key.length > 560) return false;
  return /#\d+/.test(key) && /\d:\d/.test(key);
}

function walkSkuKeyedMaps(root: unknown, hits: Record<string, Record<string, unknown>>[], depth = 0): void {
  if (depth > 24 || hits.length >= 8) return;
  if (!root || typeof root !== 'object') return;
  if (Array.isArray(root)) {
    for (const el of root) walkSkuKeyedMaps(el, hits, depth + 1);
    return;
  }
  const o = root as Record<string, unknown>;
  const entries = Object.entries(o);
  if (entries.length >= 2) {
    const skuLike = entries.filter(([k]) => looksLikeAeSkuRouteKey(k));
    if (skuLike.length >= entries.length * 0.66 && skuLike.length >= 2) {
      const vals = skuLike.slice(0, Math.min(skuLike.length, 420));
      const rec: Record<string, Record<string, unknown>> = {};
      for (const [k, v] of vals) {
        if (v && typeof v === 'object' && !Array.isArray(v))
          rec[k] = v as Record<string, unknown>;
      }
      if (Object.keys(rec).length >= 2) hits.push(rec);
    }
  }
  for (const v of Object.values(o)) walkSkuKeyedMaps(v, hits, depth + 1);
}

interface DimHint {
  name: string;
  valuesByLongId: Map<string, { label: string; image?: string }>;
}

/** 扫描 skuPropertyList 近似结构 */
function walkExtractDimensionHints(root: unknown, dims: DimHint[], depth = 0): void {
  if (depth > 26) return;
  if (!root || typeof root !== 'object') return;
  if (Array.isArray(root)) {
    if (
      root.length > 0 &&
      root.every(
        (row) =>
          row &&
          typeof row === 'object' &&
          (typeof (row as Record<string, unknown>).skuPropertyName === 'string' ||
            typeof (row as Record<string, unknown>).skuProp === 'object'),
      )
    ) {
      /** 单层 skuPropertyList */
      for (const row of root) extractOneDim(row, dims);
    }
    for (const el of root) walkExtractDimensionHints(el, dims, depth + 1);
    return;
  }
  const o = root as Record<string, unknown>;
  if (
    typeof o.skuPropertyName === 'string' &&
    Array.isArray(o.skuPropertyValues) &&
    o.skuPropertyValues.length > 0
  ) {
    extractOneDim(o, dims);
    return;
  }
  /** 常见于 skuModule.properties / sku_properties */
  for (const v of Object.values(o)) walkExtractDimensionHints(v, dims, depth + 1);
}

function extractOneDim(row: unknown, dims: DimHint[]): void {
  if (!row || typeof row !== 'object') return;
  const o = row as Record<string, unknown>;
  const name = trimStr(
    String(
      (o.skuPropertyName ?? o.propName ?? o.name ?? '') as string,
    ),
  );
  const rawVals = Array.isArray(o.skuPropertyValues)
    ? o.skuPropertyValues
    : Array.isArray(o.sku_property_values_list)
      ? (o.sku_property_values_list as unknown[])
      : [];
  const map = new Map<string, { label: string; image?: string }>();
  if (!name || rawVals.length === 0) return;

  for (const cell of rawVals) {
    if (!cell || typeof cell !== 'object') continue;
    const c = cell as Record<string, unknown>;
    const label = trimStr(
      String(c.propertyValueDisplayName ?? c.name ?? c.value ?? ''),
    );
    let idRaw = '';
    const longId =
      c.propertyValueIdLong ??
      c.propertyValueLongId ??
      c.propertyValueDefinitionNameId ??
      c.skuPropertyValueId ??
      c.skuValueId ??
      '';
    idRaw =
      typeof longId === 'string' ? longId : typeof longId === 'number' ? String(longId) : '';

    /** 备选：哈希 id */
    const alt = c.propertyValueDefinitionName ?? c.sku_property_value_id ?? '';
    if (!idRaw && typeof alt === 'string' && /\d/.test(alt)) idRaw = trimStr(String(alt));
    /** 兜底 key */
    const k = idRaw || truncate(label.replace(/\s+/g, '_'), 40);

    let image: string | undefined;
    const imgCand =
      typeof c.skuPropertyImagePath === 'string'
        ? c.skuPropertyImagePath
        : typeof c.imagePath === 'string'
          ? c.imagePath
          : typeof c.skuPropertyTips === 'string'
            ? c.skuPropertyTips
            : undefined;
    if (typeof imgCand === 'string') {
      const abs = normalizeImageUrl(imgCand, 'https://www.aliexpress.com/');
      if (abs && !isLikelyJunkImage(abs)) image = abs;
    }
    if (!label || !k) continue;
    if (!map.has(k)) map.set(k, { label, image });
  }

  /** 同名维度折叠 */
  const existing = dims.find((d) => d.name.toLowerCase() === name.toLowerCase());
  if (existing) {
    for (const [k2, vv] of map) {
      if (!existing.valuesByLongId.has(k2)) existing.valuesByLongId.set(k2, vv);
    }
    return;
  }
  dims.push({ name, valuesByLongId: map });
}

/**
 * 将 AE sku 路由键（常含多维 id，`;` 分隔）映射回 `skuPropertyValues` 中的展示名。
 * 仅当有高置信命中时才填入 properties，否则返回空以避免伪造。
 */
function routeKeyToProps(skuKey: string, dims: DimHint[]): Record<string, string> {
  const props: Record<string, string> = {};
  if (!dims.length) return props;

  const key = trimStr(skuKey);
  /** 第一轮：任一 longId 字面出现在 key 中的维度取值 */
  for (const dim of dims) {
    for (const [longId, cell] of dim.valuesByLongId) {
      if (longId.length >= 5 && key.includes(longId)) {
        props[dim.name] = cell.label;
        break;
      }
    }
  }

  const segs = key.split(';').map((x) => trimStr(x)).filter(Boolean);
  /** 第二轮：段落与维度次序对齐——段内再找 long id */
  for (let i = 0; i < dims.length && i < segs.length; i++) {
    const dim = dims[i];
    if (props[dim.name]) continue;
    const seg = segs[i];
    let hit: string | undefined;
    for (const longId of dim.valuesByLongId.keys()) {
      if (!longId) continue;
      if (seg.includes(longId)) {
        hit = dim.valuesByLongId.get(longId)?.label ?? hit;
      }
    }
    if (!hit && seg.includes('#')) {
      const hashes = [...seg.matchAll(/#(\d{4,})/g)].map((m) => m[1] ?? '').filter(Boolean);
      for (const h of hashes) {
        const cell =
          [...dim.valuesByLongId.entries()].find(([id]) => id === h || id.endsWith(h) || h.endsWith(id))
            ?.[1]?.label ??
          [...dim.valuesByLongId.entries()].find(([id]) => id.includes(h) || h.includes(id))?.[1]?.label;
        if (cell) {
          hit = cell;
          break;
        }
      }
    }
    if (hit) props[dim.name] = hit;
  }

  /** 第三轮：仅用数字 token 回填尚未命中的维度（需 token 与该维某 id 完全一致）*/
  const tokens = [...key.matchAll(/\d{6,}/g)].map((m) => m[0]);
  const usedLabels = new Set(Object.values(props));
  for (const dim of dims) {
    if (props[dim.name]) continue;
    for (const t of tokens) {
      const cell =
        [...dim.valuesByLongId.entries()].find(([longId]) => longId === t)?.[1] ??
        [...dim.valuesByLongId.entries()].find(([longId]) => longId.endsWith(t) || t.endsWith(longId))
          ?.[1];
      if (cell && !usedLabels.has(cell.label)) {
        props[dim.name] = cell.label;
        usedLabels.add(cell.label);
        break;
      }
    }
  }

  return props;
}

function mineAeSkus(
  jsonRoots: unknown[],
  baseUrl: string,
  rawSkuSlices: Record<string, unknown>[],
): ProductSku[] {
  const dims: DimHint[] = [];
  for (const r of jsonRoots) walkExtractDimensionHints(r, dims);

  for (const r of jsonRoots.slice(0, 8)) {
    if (!r || typeof r !== 'object') continue;
    const rec = r as Record<string, unknown>;
    const sm = rec.skuModule ?? rec.sku_props ?? rec.skuProperties;
    if (sm) walkExtractDimensionHints(sm, dims);
  }

  const maps: Record<string, Record<string, unknown>>[] = [];
  for (const r of jsonRoots) walkSkuKeyedMaps(r, maps);
  /** 扁平合并最大的一份 map（通常第一份最大）*/
  maps.sort((a, b) => Object.keys(b).length - Object.keys(a).length);
  const best = maps[0];
  const out: ProductSku[] = [];
  if (!best || Object.keys(best).length < 1) return out;

  for (const [skuKeyRaw, bucketRaw] of Object.entries(best).slice(0, 72)) {
    const bucket = bucketRaw;
    const price =
      divePrice(bucket) ??
      coerceNumber((bucket.skuAmount as Record<string, unknown> | undefined)?.value) ??
      coerceNumber((bucket as Record<string, unknown>).offerPrice ?? (bucket as Record<string, unknown>).minPriceDiscount);
    const stock =
      coerceInt(bucket.availQuantity) ??
      coerceInt(bucket.skuAvailQuantity) ??
      coerceInt(bucket.skuBulkOrder) ??
      coerceInt(bucket.totalAvailQuantity);

    let image: string | undefined;
    for (const k of ['skuImageUri', 'imagePath', 'imageUrl']) {
      const v = bucket[k];
      if (typeof v === 'string') {
        const u = normalizeImageUrl(v, baseUrl);
        if (u && !isLikelyJunkImage(u)) {
          image = u;
          break;
        }
      }
    }

    const propsParsed = routeKeyToProps(trimStr(skuKeyRaw), dims);

    rawSkuSlices.push({ keySnippet: truncate(skuKeyRaw, 280), bucketKeysSample: truncate(JSON.stringify(Object.keys(bucket).slice(0, 48)), 2000), priceGuess: price });
    /** 若没有 props 且不强行合成：仍输出一条 SKU 仅价格（properties 为空但 Go BuildImportSKU 需要 properties 或非空 SKUName fallback）—— 用户：properties 必选；可无组合则返回空SKU不要伪造 */
    if (Object.keys(propsParsed).length === 0) continue;

    out.push({
      skuCode: trimStr(String(bucket.skuAttr ?? bucket.skuPropIds ?? skuKeyRaw).slice(0, 260)),
      properties: propsParsed,
      price: price ?? undefined,
      stock: stock ?? undefined,
      image,
      raw: {
        skuRouteKeySnippet: truncate(skuKeyRaw, 200),
      },
    });
  }

  return out.slice(0, 48);
}

function walkCollectImages(root: unknown, acc: Set<string>, depth = 0): void {
  if (depth > 26) return;
  if (root === null || root === undefined) return;
  if (typeof root === 'string') {
    const s = root.trim();
    if ((!/^https?:\/\//i.test(s) && !/^\/\//.test(s)) || !/\.(jpg|jpeg|png|webp|gif)/i.test(s)) return;
    acc.add(s.startsWith('//') ? `https:${s}` : s);
    return;
  }
  if (typeof root !== 'object') return;

  const NOISE_KEY = /^(__|experiment|analytics|tracking|reviews|seller|coupon|couponList|traffic|marketing)\b/i;

  if (Array.isArray(root)) {
    const cap = depth > 4 ? Math.min(root.length, 48) : root.length;
    for (let i = 0; i < cap; i++) walkCollectImages(root[i], acc, depth + 1);
    return;
  }
  for (const [k, v] of Object.entries(root as Record<string, unknown>)) {
    if (NOISE_KEY.test(k)) continue;
    const lk = k.toLowerCase();
    const looksImgKey =
      /image|imgurl|gallery|photos|webp|poster|thumbnail|skuPropertyImagePath|skuimage|slides|carousel|pics|medias/i.test(
        lk,
      );
    const looksVideo = lk.includes('video') && !lk.includes('preview') && !lk.includes('poster');
    if (looksVideo) continue;
    if (looksImgKey || depth < 14) walkCollectImages(v, acc, depth + 1);
  }
}

/** 属性：productProperties[]、structured specs */
function walkCollectAttributes(root: unknown, acc: Record<string, string>, depth = 0): void {
  if (depth > 22) return;
  if (!root || typeof root !== 'object') return;
  if (Array.isArray(root)) {
    for (const el of root) walkCollectAttributes(el, acc, depth + 1);
    return;
  }
  const o = root as Record<string, unknown>;
  const propArr = (o.productProperties ?? o.attrs ?? o.props) as unknown[];
  if (Array.isArray(propArr)) {
    for (const row of propArr) {
      if (!row || typeof row !== 'object') continue;
      const r = row as Record<string, unknown>;
      const k = trimStr(String(r.attrName ?? r.attrNameCn ?? r.name ?? r.key ?? '').replace(/:$/, ''));
      const v = trimStr(String(r.attrValue ?? r.value ?? '').replace(/^[:：]/, '').trim());
      if (!k || !v || acc[k]) continue;
      acc[k] = v;
    }
  }
  for (const v of Object.values(o)) walkCollectAttributes(v, acc, depth + 1);
}

/** 正文 HTML → 候选详情图 URL */
function extractDescriptionHtmlSlices(roots: unknown[]): string[] {
  const blobs: string[] = [];
  function walk(x: unknown, d: number): void {
    if (d > 20 || blobs.length >= 12) return;
    if (!x || typeof x !== 'object') return;
    const o = x as Record<string, unknown>;
    for (const k of ['mobileDetail', 'content', 'html', 'detailHtml', 'description', 'detailDesc']) {
      const v = o[k];
      if (typeof v === 'string' && v.includes('<')) blobs.push(v.slice(0, 280_000));
    }
    for (const v of Object.values(o)) walk(v, d + 1);
  }
  for (const r of roots) walk(r, 0);
  return blobs;
}

export async function extractBrowserPayload(page: Page): Promise<AeBrowserPayload & { __blocked__?: number }> {
  const titleSelectors = AE_TITLE_SELECTORS;
  const mainSel = AE_MAIN_GALLERY_SELECTORS;
  const detailSel = AE_DETAIL_AREA_SELECTORS;
  const attrSel = AE_ATTRIBUTE_CONTAINER_SELECTORS;

  return page.evaluate(
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

      function pickImgUrl(el: Element): string | null {
        const img = el as HTMLImageElement;
        const order = ['data-lazy-src', 'data-src', 'data-original', 'srcset', 'src'];
        for (const attr of order) {
          let v = img.getAttribute(attr);
          if (attr === 'srcset' && v) {
            const first = v.split(',')[0]?.trim().split(/\s+/)[0];
            if (first) v = first;
          }
          if (!v?.trim()) continue;
          if (v.startsWith('data:')) continue;
          return v.trim();
        }
        const cur = img.currentSrc;
        return cur?.trim() ? cur.trim() : null;
      }

      function collectFrom(selectors: string[]): string[] {
        const urls: string[] = [];
        for (const sel of selectors) {
          try {
            document.querySelectorAll(sel).forEach((node) => {
              const raw = pickImgUrl(node);
              if (raw) urls.push(raw);
            });
          } catch {
            /** malformed selector skipped */
          }
        }
        return urls;
      }

      let headingText = '';
      for (const sel of titleSelectors) {
        try {
          const txt = document.querySelector(sel)?.textContent?.trim();
          if (txt && txt.length > 2 && txt.length < 400) {
            headingText = txt;
            break;
          }
        } catch {
          /** skip */
        }
      }

      const meta: AeBrowserPayload['meta'] = {};
      document.querySelectorAll('meta').forEach((m) => {
        const prop = m.getAttribute('property') ?? m.getAttribute('itemprop') ?? m.getAttribute('name');
        const content = m.getAttribute('content')?.trim();
        if (!content) return;
        if (prop === 'og:title') meta.ogTitle = content;
        if (prop === 'og:image') meta.ogImage = content;
        if (prop === 'og:description') meta.ogDescription = content;
        if (prop === 'product:price:currency') meta.priceCurrency = content.trim();
        if (prop === 'twitter:title') meta.twitterTitle = content;
        if (
          !meta.priceCurrency &&
          (prop === 'og:price:currency' || /^priceCurrency$/i.test(prop ?? '') || /\bcurrency\b/i.test(prop ?? ''))
        ) {
          if (/^[A-Z]{2,5}$/.test(content)) meta.priceCurrency = content.trim();
        }
      });

      const paramPairs: Array<{ key: string; value: string }> = [];
      for (const sel of attrSel) {
        try {
          document.querySelectorAll(sel).forEach((node) => {
            const kv = /\b([\u0000-\uFFFFa-zA-Z0-9·（）()%°\-.]{2,48})\s*[:：]\s*([^\n\r:]{1,260})/u.exec(node.textContent?.trim() ?? '');
            if (kv?.[1] && kv?.[2]) paramPairs.push({ key: kv[1].trim(), value: kv[2].trim() });

            Array.from(node.querySelectorAll('[class*="specification--line"]')).forEach((row) => {
              const ks = row.querySelector('[class*="title"]');
              const vs = row.querySelector('[class*="desc"]');
              const k = ks?.textContent?.replace(/:$/, '').trim();
              const v = vs?.textContent?.trim();
              if (k && v && v.length < 320) paramPairs.push({ key: k, value: v });
            });
          });
        } catch {
          /** skip */
        }
      }

      const snippets: string[] = [];
      for (const scriptEl of Array.from(document.scripts)) {
        const t = scriptEl.text ?? '';
        if (t.length < 120) continue;
        const suspicious = /skuModule|skuProperty|productTitle|salePrice|imagePath|spec(?:s)?Module/i.test(t);
        const broad =
          /sku(?:Module|Def|Paths|PathsList|Maps|Combinations|Pricing|Property)|image(?:Path|List)s?|gallery|inventory|availQuantity|productTitle|skuProperty|spec(?:s)?Module|descriptionModule|detailDesc|shippingFrom|warehouse|warehouseName/i.test(t);
        if (!suspicious && !broad) continue;
        snippets.push(t.length > snippetMax ? t.slice(0, snippetMax) : t);
        if (snippets.length >= maxFragments) break;
      }

      document.querySelectorAll('script[type="application/ld+json"]').forEach((s) => {
        const txt = s.textContent?.trim();
        if (txt && txt.length < 240_000 && txt.length > 20) snippets.unshift(txt.slice(0, snippetMax));
      });

      const bodyPeek = document.body?.innerText?.slice(0, 3400) ?? '';
      let blocked =
        /captcha/i.test(bodyPeek) ||
        /verify\s+you'?re\s+human/i.test(bodyPeek) ||
        /security\s+(check|verification)/i.test(bodyPeek) ||
        /请先登录|访问被拒绝|Forbidden/i.test(bodyPeek) ||
        (/\/item\//i.test(window.location.pathname) &&
          (/complete\s+the\s+verification/i.test(bodyPeek) || /sorry,\s*i\s+couldn'?t\s+find\s+what\s+you/i.test(bodyPeek)));

      /** PDP 占位：极短正文 + 人机提示 */
      if (headingText.length < 8 && (/verify|validation/i.test(bodyPeek) || /登录/i.test(bodyPeek))) {
        blocked = true;
      }

      const docTitle = typeof document.title === 'string' ? document.title.trim() : '';

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
      } as AeBrowserPayload & { __blocked__?: number };
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

/** 诊断摘要：不写整 HTML */
function buildStateDigest(
  scriptSnippets: string[],
  roots: unknown[],
  extra: Record<string, unknown>,
): Record<string, unknown> {
  const joined = truncate(scriptSnippets.slice(0, 3).map((x) => x.slice(0, 640)).join('\n'), 2200).toLowerCase();
  return {
    skuModuleProbe: /\bskumodule\b/i.test(joined),
    skuPropertyProbe: /\bskuproperty\b/i.test(joined),
    imagePathProbe: /\b(imagepath|imgurl|gallery)\b/i.test(joined),
    specsModuleProbe: /\bspecsModule\b|\bspecification\b|\bproductproperties\b/i.test(joined),
    rootCount: roots.length,
    ...extra,
  };
}

export function assembleAeProduct(inputUrl: string, payloadUnclean: AeBrowserPayload & { __blocked__?: number }): AeAssembleOutput {
  const blocked = payloadUnclean.__blocked__ === 1;
  const { __blocked__, ...payloadClean } = payloadUnclean;

  const baseUrl = payloadClean.finalUrl || inputUrl;

  const jsonRoots = parseJsonFragmentsFromScripts(payloadClean.scriptSnippets);

  const titleCandJson: string[] = [];
  for (const r of jsonRoots) walkExtractTitleCandidates(r, 0, titleCandJson);

  let titleRaw =
    pickBestTitle(titleCandJson) ||
    sanitizeAliExpressTitle(trimStr(payloadClean.headingText || '')) ||
    sanitizeAliExpressTitle(trimStr(payloadClean.meta.ogTitle || '')) ||
    sanitizeAliExpressTitle(trimStr(payloadClean.docTitle || '')) ||
    '';

  /** JSON-LD 标题 */
  for (const s of payloadClean.scriptSnippets.slice(-4)) {
    if (!s.includes('@type')) continue;
    for (const o of collectScriptJsonCandidates(s.slice(0, 80_000), 4)) {
      if (!o || typeof o !== 'object') continue;
      const rec = (Array.isArray(o) ? (o as unknown[])[0] : o) as Record<string, unknown>;
      const n = typeof rec?.name === 'string' ? sanitizeAliExpressTitle(rec.name as string) : '';
      if (n.length >= 6) titleRaw = titleRaw || n;
    }
  }

  /** 币种 */
  const curSet = new Set<string>();
  if (payloadClean.meta.priceCurrency) curSet.add(payloadClean.meta.priceCurrency);
  for (const r of jsonRoots) walkCollectCurrency(r, 0, curSet);
  let currency = [...curSet].find((x) => /^[A-Z]{3}$/.test(x)) ?? [...curSet][0];
  if (!currency) currency = 'USD';

  /** 图 */
  const fromJsonImg = new Set<string>();
  for (const r of jsonRoots) walkCollectImages(r, fromJsonImg);

  /** 结构化 imagePathList — 常见于模块 */
  for (const r of jsonRoots) {
    const walkFlat = (o: unknown, d: number) => {
      if (d > 18 || !o || typeof o !== 'object') return;
      const rec = o as Record<string, unknown>;
      for (const k of ['imagePathList', 'imagePath', 'photos', 'imageList']) {
        const v = rec[k];
        if (typeof v === 'string') walkCollectImages(v, fromJsonImg, 0);
        if (Array.isArray(v))
          v.forEach((x) =>
            typeof x === 'string' ? walkCollectImages(x, fromJsonImg, 0) : walkFlat(x, d + 1),
          );
      }
      for (const v of Object.values(rec)) walkFlat(v, d + 1);
    };
    walkFlat(r, 0);
  }

  const textImgUrls = new Set<string>();
  for (const s of payloadClean.scriptSnippets.slice(0, 8))
    extractImageUrlsFromHtmlOrText(s, baseUrl).forEach((u) => textImgUrls.add(u));

  let mainMerged = mergeUrlBuckets(
    payloadClean.galleryUrls,
    [...fromJsonImg, ...textImgUrls],
    baseUrl,
    40,
  );
  if (payloadClean.meta.ogImage)
    normalizeAndCollectImg(payloadClean.meta.ogImage, baseUrl, mainMerged);
  mainMerged = dedupeStrings(mainMerged, 30);

  const detailDom = mergeUrlBuckets(payloadClean.detailUrls, [], baseUrl, 80);
  const htmlBlobs = extractDescriptionHtmlSlices(jsonRoots.slice(0, 10));
  const fromDescHtml = new Set<string>();
  for (const h of htmlBlobs) extractImageUrlsFromHtmlOrText(h, baseUrl).forEach((x) => fromDescHtml.add(x));

  /** 详情图中去除与首屏主图完全重复 path */
  const mainKey = new Set(mainMerged.map((m) => m.split('?')[0]?.toLowerCase() ?? ''));

  const descriptionImages = dedupeStrings(
    [...detailDom, ...fromDescHtml].filter((u) => !mainKey.has(u.split('?')[0]?.toLowerCase() ?? '')),
    40,
  ).slice(0, 34);

  const attributes: Record<string, string> = {};
  for (const root of jsonRoots.slice(0, 18)) walkCollectAttributes(root, attributes);
  /* DOM param pairs first */
  for (const pp of payloadClean.paramPairs) {
    const k = trimStr(pp.key);
    const v = trimStr(pp.value);
    if (!attributes[k]) attributes[k] = v;
  }
  const attrsSafe = sanitizeAttributeMap(attributes);

  const rawSkuSlices: Record<string, unknown>[] = [];

  /** SKU */
  let skus: ProductSku[] = [];
  try {
    skus = mineAeSkus(jsonRoots, baseUrl, rawSkuSlices);
  } catch {
    skus = [];
  }

  const stateDigest = buildStateDigest(payloadClean.scriptSnippets, jsonRoots, {
    hasTitleCandidate: Boolean(titleRaw),
    skuCount: skus.length,
    mainImageCountGuess: mainMerged.length,
    detailCandidateCountGuess: detailDom.length + fromDescHtml.size,
    blockedGuess: blocked,
  });

  const rawShell = {
    title: truncate(titleCandJson.sort((a, b) => b.length - a.length)[0] ?? payloadClean.meta.ogTitle ?? titleRaw, 400),
    url: baseUrl,
    mainImageCandidates: dedupeStrings(mainMerged, 18),
    detailImageCandidates: dedupeStrings([...detailDom, ...fromDescHtml], 45),
    attributeCandidates: attrsSafe,
    skuCandidates: rawSkuSlices.slice(0, 40),
    pageMeta: {
      blockedHint: blocked,
      ogTitle: truncate(payloadClean.meta.ogTitle ?? '', 200),
      ogDescription: truncate(payloadClean.meta.ogDescription ?? '', 240),
      docTitle: truncate(payloadClean.docTitle, 200),
    },
    extractedAt: new Date().toISOString(),
  };

  return {
    title: titleRaw.trim(),
    currency,
    mainImages: mainMerged.slice(0, 10),
    descriptionImages: descriptionImages.slice(0, 30),
    attributes: attrsSafe,
    skus,
    rawShell,
    blocked,
    stateDigest,
  };
}
