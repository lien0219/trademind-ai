import type { Page } from 'playwright';

import { evaluateInPage } from '../../browser/evaluate-in-page.js';

import type { ProductSku } from '../../types/product.js';
import type { BrowserExtractPayload, Parse1688Result } from './types.js';
import {
  ATTRIBUTE_ROW_SELECTORS,
  DETAIL_SELECTORS,
  MAIN_GALLERY_SELECTORS,
  SKU_SECTION_SELECTORS,
  SKU_TABLE_ROW_SELECTORS,
  TITLE_SELECTORS,
} from './selectors.js';
import {
  extractAttributesFrom1688Data,
  extractDefaultOfferPrice,
  extractDetailImagesFrom1688Data,
  extractMainImagesFrom1688Data,
  find1688ResultData,
  mineSkusFrom1688Data,
  walkSkuPropArrays,
} from './context-parse.js';
import type { DimRow } from './sku-helpers.js';
import {
  enrichSkusFromDomTable,
  extractSkuBucketPrice,
  extractSkuBucketStock,
  inferTwoDimNames,
  normalizeSkuPropertyKeys,
  parseComboKey,
  skuNameFromProps,
} from './sku-helpers.js';
import type { DomSkuDimension, DomSkuTableRow } from './types.js';
import {
  coerceInt,
  coerceNumber,
  collectScriptJsonCandidates,
  dedupeStrings,
  isLikelyJunkImage,
  isLikelyProductImage,
  normalizeImageUrl,
  sanitizeAttributeMap,
  truncate,
  trimStr,
} from './utils.js';

const SCRIPT_SNIPPET_MAX = 120_000;
const MAX_SCRIPT_FRAGMENTS = 14;

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

const OFFER_IMAGE_KEY_RE =
  /offerimage|imagelist|gallery|mainimage|productimage|detailimage|piclist|imageurl|fullpathimage|summImage/i;

/** 仅从 JSON 中带商品图语义的字段收集 URL，避免 script 内服务图标污染主图 */
function walkCollectOfferImages(root: unknown, acc: Set<string>): void {
  function walk(x: unknown, depth: number, keyHint: string): void {
    if (depth > 22 || x === null || x === undefined) return;
    const hint = keyHint.toLowerCase();
    if (typeof x === 'string') {
      if (!OFFER_IMAGE_KEY_RE.test(hint)) return;
      const s = x.trim();
      if (!/^https?:\/\//i.test(s) && !/^\/\//.test(s)) return;
      if (!/\.(jpg|jpeg|png|webp|gif)(\?|$)/i.test(s)) return;
      if (hint.includes('video') || hint.includes('coverurl')) return;
      const abs = s.startsWith('//') ? `https:${s}` : s;
      if (!isLikelyJunkImage(abs)) acc.add(abs);
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

/** DOM 主图优先；不足时用 JSON 商品图补全，不再盲扫 script 全文 URL */
function mergeMainImageBuckets(
  domGallery: string[],
  jsonOfferImages: string[],
  ogImage: string | undefined,
  baseUrl: string,
): string[] {
  const domMain = mergeUrlLists(domGallery, [], baseUrl, 16);
  if (domMain.length >= 2) {
    let out = dedupeStrings(domMain, 10);
    const supplement = jsonOfferImages
      .filter((u) => isLikelyProductImage(u))
      .filter((u) => !out.some((m) => (m.split('?')[0] ?? m) === (u.split('?')[0] ?? u)));
    if (out.length < 10 && supplement.length) {
      out = dedupeStrings([...out, ...supplement], 10);
    }
    if (ogImage) {
      const extra: string[] = [];
      normalizeAndFilterImg(ogImage, baseUrl, extra);
      if (extra.length && !out.some((m) => m.split('?')[0] === extra[0].split('?')[0])) {
        out = dedupeStrings([...out, ...extra], 10);
      }
    }
    return out;
  }
  let merged = mergeUrlLists(domGallery, jsonOfferImages.filter(isLikelyProductImage), baseUrl, 20);
  if (ogImage) normalizeAndFilterImg(ogImage, baseUrl, merged);
  return dedupeStrings(merged, 10);
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

function mineSkuStructures(roots: unknown[], defaultPrice?: number): ProductSku[] {
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
        (k === 'skuMap' || k === 'skuInfoMap' || k === 'skuInfo' || k === 'skuPriceMap') &&
        typeof o[k] === 'object' &&
        o[k] !== null &&
        !Array.isArray(o[k])
      ) {
        skuMaps.push(o[k] as Record<string, unknown>);
      }
      if (k === 'skuModel' && typeof o[k] === 'object' && o[k] !== null) {
        const sm = o[k] as Record<string, unknown>;
        if (sm.skuMap && typeof sm.skuMap === 'object') skuMaps.push(sm.skuMap as Record<string, unknown>);
        if (sm.skuInfoMap && typeof sm.skuInfoMap === 'object') skuMaps.push(sm.skuInfoMap as Record<string, unknown>);
      }
      if (
        /^sku_props$/i.test(k) ||
        k === 'saleProp' ||
        k === 'saleProps' ||
        k === 'skuProps' ||
        k === 'skuPropList' ||
        k === 'skuPropsList'
      ) {
        skuPropArrays.push(o[k]);
      }
    }
    for (const v of Object.values(o)) walkCollect(v, depth + 1);
  }
  for (const r of roots) walkCollect(r, 0);

  const skuMapMerged: Record<string, unknown> = Object.assign({}, ...skuMaps);

  const rows: DimRow[] = [];
  for (const sp of skuPropArrays) rows.push(...flattenSkuPropLike(sp));
  const dimNames = inferTwoDimNames(rows, '', '');

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
    let props = normalizeSkuPropertyKeys(parseComboKey(rawKey, rows), dimNames);
    if (Object.keys(props).length === 0 && rawKey.length > 0 && rawKey.length < 80) {
      props = { 规格: trimStr(rawKey) };
    }
    const bucket = skuMapMerged[rawKey];
    combosFromMap.push({ key: rawKey, props, bucket: (bucket ?? {}) as Record<string, unknown> });
  }

  if (combosFromMap.length === 0) {
    return singleDimFallback();
  }

  const outSkus: ProductSku[] = [];
  for (const { key: rawKey, props, bucket } of combosFromMap.slice(0, 60)) {
    let price = extractSkuBucketPrice(bucket) ?? defaultPrice;
    let stock = extractSkuBucketStock(bucket);

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
      stock: stock && stock > 0 ? stock : undefined,
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

function skusFromDomPayload(payload: BrowserExtractPayload): ProductSku[] {
  const dims = payload.domSkuDimensions ?? [];
  const rows = payload.domSkuTableRows ?? [];

  if (rows.length > 0) {
    const sizeDim = dims.find((d) => /尺寸|尺码|规格|内长/i.test(d.name));
    const colorDim = dims.find((d) => /颜色|花色|款式/i.test(d.name));
    return rows.slice(0, 60).map((row) => {
      const props: Record<string, string> = {};
      if (sizeDim) props[sizeDim.name] = row.label;
      else props['规格'] = row.label;
      if (colorDim && colorDim.values.length === 1) props[colorDim.name] = colorDim.values[0];
      return {
        skuCode: '',
        properties: props,
        price: row.priceText ? coerceNumber(row.priceText) : undefined,
        stock: row.stockText ? coerceInt(row.stockText) : undefined,
        raw: { source: 'dom-sku-table' },
      };
    });
  }

  if (dims.length > 0) {
    const dimRows: DimRow[] = dims.map((d) => ({ name: d.name, values: d.values }));
    const combos = cartesianCombinations(dimRows, 60);
    if (combos.length > 0) {
      return combos.map((props) => ({
        skuCode: '',
        properties: props,
        raw: { source: 'dom-sku-dimensions' },
      }));
    }
  }
  return [];
}

function mergeSkuLists(primary: ProductSku[], secondary: ProductSku[]): ProductSku[] {
  if (primary.length > 0 && primary.some((s) => s.price != null || s.stock != null)) return primary;
  if (secondary.length > 0) return secondary;
  return primary.length > 0 ? primary : secondary;
}

export async function extractBrowserPayload(
  page: Page,
): Promise<BrowserExtractPayload & { __blocked__?: number }> {
  const titleSelectors = TITLE_SELECTORS;
  const mainSel = MAIN_GALLERY_SELECTORS;
  const detailSel = DETAIL_SELECTORS;
  const attrSel = ATTRIBUTE_ROW_SELECTORS;
  const skuSectionSel = SKU_SECTION_SELECTORS;
  const skuTableSel = SKU_TABLE_ROW_SELECTORS;

  return evaluateInPage(
    page,
    ({
      titleSelectors,
      mainSel,
      detailSel,
      attrSel,
      skuSectionSel,
      skuTableSel,
      snippetMax,
      maxFragments,
    }: {
      titleSelectors: string[];
      mainSel: string[];
      detailSel: string[];
      attrSel: string[];
      skuSectionSel: string[];
      skuTableSel: string[];
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

      const skipImgAncestor = (el: Element): boolean => {
        let p: Element | null = el.parentElement;
        while (p) {
          const blob = `${p.className?.toString() ?? ''} ${p.id ?? ''}`.toLowerCase();
          if (/promise|guarantee|service|credit|banner|toolbar|icon|wangwang|footer|header-nav|trust|badge/.test(blob)) {
            return true;
          }
          p = p.parentElement;
        }
        return false;
      };

      const collectFrom = (selectors: string[]): string[] => {
        const urls: string[] = [];
        for (const sel of selectors) {
          document.querySelectorAll(sel).forEach((node) => {
            if (skipImgAncestor(node)) return;
            const img = node as HTMLImageElement;
            if (img.naturalWidth > 0 && img.naturalHeight > 0 && img.naturalWidth < 72 && img.naturalHeight < 72) {
              return;
            }
            const raw = pickImgUrl(node);
            if (raw) urls.push(raw);
          });
        }
        return urls;
      };

      const domSkuDimensions: DomSkuDimension[] = [];
      const domSkuTableRows: DomSkuTableRow[] = [];
      const dimLabelRe = /^(颜色|尺码|尺寸|规格|型号|款式|容量|套餐|版本|内长)/;

      const pushDim = (name: string, values: string[]) => {
        const n = name.replace(/[:：\s]+$/u, '').trim();
        const vs = [...new Set(values.map((v) => v.trim()).filter((v) => v && v.length < 120))];
        if (!n || vs.length === 0) return;
        const existing = domSkuDimensions.find((d) => d.name === n);
        if (existing) {
          for (const v of vs) if (!existing.values.includes(v)) existing.values.push(v);
        } else {
          domSkuDimensions.push({ name: n, values: vs });
        }
      };

      for (const sel of skuSectionSel) {
        document.querySelectorAll(sel).forEach((wrap) => {
          let label = '';
          const labelNode = wrap.querySelector(
            '[class*="label"], [class*="title"], dt, .name, [class*="prop-name"]',
          );
          if (labelNode) label = (labelNode.textContent ?? '').replace(/[:：\s]+$/u, '').trim();
          if (!label || (!dimLabelRe.test(label) && label.length > 16)) return;

          const values: string[] = [];
          wrap.querySelectorAll(
            '[class*="item"], [class*="value"], [class*="select-item"], button, li, a, [role="button"]',
          ).forEach((el) => {
            const t = (el.textContent ?? '').replace(/\s+/g, ' ').trim();
            if (!t || t.length > 100 || /^¥/.test(t)) return;
            if (/^(库存|价格|数量)$/.test(t)) return;
            values.push(t);
          });
          if (label && values.length) pushDim(label, values);
        });
      }

      const seenTableLabels = new Set<string>();
      const pushTableRow = (text: string) => {
        const blob = text.replace(/\s+/g, ' ').trim();
        if (blob.length < 4 || blob.length > 400) return;
        if (!/内长\d+/.test(blob)) return;
        const hasPrice = /¥\s*[\d.]+/.test(blob);
        const hasStock = /库存\s*\d+/.test(blob);
        if (!hasPrice && !hasStock) return;
        const labelM = /(内长\d+[^¥\n]*?(?:【[^】]+】)?)/.exec(blob);
        const label = (labelM?.[1] ?? blob.split(/¥|库存/)[0] ?? '').trim().slice(0, 80);
        if (!label || seenTableLabels.has(label)) return;
        seenTableLabels.add(label);
        const priceM = /¥\s*([\d.]+)/.exec(blob);
        const stockM = /库存\s*(\d+)/.exec(blob);
        domSkuTableRows.push({
          label,
          priceText: priceM?.[1],
          stockText: stockM?.[1],
        });
      };

      for (const sel of skuTableSel) {
        document.querySelectorAll(sel).forEach((row) => pushTableRow(row.textContent ?? ''));
      }
      /** 新版 1688 尺码展开列表：按行文本匹配「内长 + ¥ + 库存」 */
      document.querySelectorAll('motion.div, div, tr, li, span').forEach((row) => {
        const text = row.textContent ?? '';
        if (text.length > 200 || text.length < 6) return;
        if (!/内长\d+/.test(text) || !/¥/.test(text) || !/库存/.test(text)) return;
        pushTableRow(text);
      });

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
      const pushSnippet = (text: string) => {
        const t = text.trim();
        if (t.length < 100) return;
        snippets.push(t.length > snippetMax ? t.slice(0, snippetMax) : t);
      };

      for (const scriptEl of Array.from(document.scripts)) {
        const t = scriptEl.text ?? '';
        if (t.length < 100) continue;
        const suspicious =
          /skuMap|skuProps|saleProp|saleProps|sku_props|offerImageList|offerImage|imageList|subject|gallery|skuInfoMap|skuModel|tradeModel|amountOnSale|canBookCount/i.test(
            t,
          );
        if (!suspicious) continue;
        pushSnippet(t);
        if (snippets.length >= maxFragments) break;
      }

      const win = window as unknown as Record<string, unknown>;
      /** 优先完整序列化 window.context（含 skuMap / gallery / 库存） */
      const ctx = win.context;
      if (ctx && typeof ctx === 'object') {
        try {
          const s = JSON.stringify(ctx);
          if (s.length > 200) pushSnippet(s);
        } catch {
          /* circular */
        }
      }
      for (const gKey of [
        '__INIT_DATA',
        '__INITIAL_STATE__',
        'detailData',
        'offerDetailData',
        'iDetailConfig',
        'OFFER_DETAIL',
      ]) {
        if (snippets.length >= maxFragments) break;
        const v = win[gKey];
        if (!v || typeof v !== 'object') continue;
        try {
          const s = JSON.stringify(v);
          if (s.length < 200) continue;
          if (!/skuMap|skuProps|skuInfoMap|saleProp|subject|offerImage|amountOnSale/i.test(s)) continue;
          pushSnippet(s);
        } catch {
          /* ignore circular */
        }
      }

      /** 类型为 json 的内联 script（体积通常较小） */
      document.querySelectorAll('script[type="application/ld+json"]').forEach((s) => {
        const txt = s.textContent?.trim();
        if (txt && txt.length < 60000 && txt.length > 20) snippets.push(txt);
      });

      /** 粗略风控页（不抛出，由外层决定是否降级） */
      const bodyPeek = document.body?.innerText?.slice(0, 3500) ?? '';
      const htmlPeek = document.documentElement?.innerHTML?.slice(0, 4000) ?? '';
      const blocked =
        /安全验证|请完成验证|访问过于频繁|captcha|滑块验证|人机验证|nc-container|punish-page/i.test(bodyPeek) ||
        /punish|x5secdata|captcha/i.test(htmlPeek) ||
        (/验证/.test(bodyPeek) && headingText.length < 2) ||
        (/请登录|账号登录/.test(bodyPeek) && headingText.length < 2);

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
        domSkuDimensions,
        domSkuTableRows,
        scriptSnippets: snippets,
        __blocked__: blocked ? 1 : 0,
      } as BrowserExtractPayload & { __blocked__?: number };
    },
    {
      titleSelectors,
      mainSel,
      detailSel,
      attrSel,
      skuSectionSel,
      skuTableSel,
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

  const contextData = find1688ResultData(jsonRoots);
  const defaultPrice = contextData ? extractDefaultOfferPrice(contextData) : undefined;

  /** 结构化图片：主图 DOM + window.context.gallery；详情仅 ibank */
  const fromOfferJsonImages = new Set<string>();
  for (const r of jsonRoots) walkCollectOfferImages(r, fromOfferJsonImages);
  if (contextData) {
    for (const u of extractMainImagesFrom1688Data(contextData, baseUrl)) fromOfferJsonImages.add(u);
  }

  let mainBuckets = mergeMainImageBuckets(
    payload.galleryUrls,
    [...fromOfferJsonImages],
    payload.meta.ogImage,
    baseUrl,
  );
  if (contextData) {
    const ctxMain = extractMainImagesFrom1688Data(contextData, baseUrl);
    if (ctxMain.length >= 2) mainBuckets = dedupeStrings(ctxMain, 10);
  }

  const detailDom = mergeUrlLists(payload.detailUrls, [], baseUrl, 40).filter((u) => isLikelyProductImage(u));
  let detailFromContext: string[] = [];
  if (contextData) detailFromContext = extractDetailImagesFrom1688Data(contextData, baseUrl);
  const descriptionImages = dedupeStrings(
    [...detailDom, ...detailFromContext].filter((u) => isLikelyProductImage(u)),
    30,
  );

  const attributes: Record<string, string> = {};
  if (contextData) Object.assign(attributes, extractAttributesFrom1688Data(contextData));
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

  const dimRowsFromPayload = (payload.domSkuDimensions ?? []).map((d) => ({
    name: d.name,
    values: d.values,
  }));
  for (const sp of contextData ? walkSkuPropArrays(contextData) : []) {
    dimRowsFromPayload.push(...flattenSkuPropLike(sp));
  }

  let skus: ProductSku[] = [];
  if (contextData) {
    try {
      skus = mineSkusFrom1688Data(contextData, dimRowsFromPayload, defaultPrice);
    } catch {
      skus = [];
    }
  }
  if (skus.length === 0) {
    try {
      skus = mineSkuStructures(jsonRoots, defaultPrice);
    } catch {
      skus = [];
    }
  }
  if (skus.length === 0) {
    for (const r of jsonRoots.slice(0, 4)) {
      skus = fallbackSkusTopPrice(r);
      if (skus.length) break;
    }
  }
  const domSkus = skusFromDomPayload(payload);
  skus = mergeSkuLists(skus, domSkus);
  skus = enrichSkusFromDomTable(skus, payload.domSkuTableRows ?? []);

  const mainImages = mainBuckets;

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
    domSkuDimensionCount: payload.domSkuDimensions?.length ?? 0,
    domSkuTableRowCount: payload.domSkuTableRows?.length ?? 0,
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
