/**
 * 1688 浏览器端 DOM / script 抽取（仅通过 Playwright page.evaluate 注入）。
 * 禁止 toString/eval；所有 helper 必须定义在本函数内部。
 */
import type { Page } from 'playwright';
import {
  ATTRIBUTE_ROW_SELECTORS,
  DETAIL_SELECTORS,
  MAIN_GALLERY_SELECTORS,
  SKU_SECTION_SELECTORS,
  SKU_TABLE_ROW_SELECTORS,
  TITLE_SELECTORS,
} from './selectors.js';
import type { BrowserExtractPayload } from './types.js';

const SCRIPT_SNIPPET_MAX = 120_000;
const MAX_SCRIPT_FRAGMENTS = 14;

export type Extract1688DomArg = {
  titleSelectors: string[];
  mainSel: string[];
  detailSel: string[];
  attrSel: string[];
  skuSectionSel: string[];
  skuTableSel: string[];
  snippetMax: number;
  maxFragments: number;
};

export type Extract1688DomResult = BrowserExtractPayload & { __blocked__?: number };

/** Playwright 会序列化此函数到浏览器；勿引用 Node 模块/外部 helper。 */
export function extract1688DomInPage(arg: Extract1688DomArg): Extract1688DomResult {
  const {
    titleSelectors,
    mainSel,
    detailSel,
    attrSel,
    skuSectionSel,
    skuTableSel,
    snippetMax,
    maxFragments,
  } = arg;

  const baseHref = window.location.href;

  const pickImgUrl = (el: Element): string | null => {
    const img = el as HTMLImageElement;
    const order = ['data-lazy-src', 'data-src', 'data-original', 'data-img', 'data-zoom', 'src'];
    for (const attr of order) {
      const v =
        img.getAttribute(attr) ||
        (attr === 'src' && img.currentSrc ? img.currentSrc : null) ||
        (attr === 'src' ? img.src : null);
      if (!v || !String(v).trim()) continue;
      if (String(v).startsWith('data:')) continue;
      return String(v).trim();
    }
    return null;
  };

  const skipImgAncestor = (el: Element): boolean => {
    let p: Element | null = el.parentElement;
    while (p) {
      const cls = p.className;
      const blob = `${typeof cls === 'string' ? cls : String(cls ?? '')} ${p.id ?? ''}`.toLowerCase();
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

  const collectBackgroundImages = (): string[] => {
    const urls: string[] = [];
    document.querySelectorAll('[style*="background"]').forEach((el) => {
      const style = (el as HTMLElement).style?.backgroundImage ?? '';
      const m = /url\(["']?(.*?)["']?\)/i.exec(style);
      if (m && m[1]) urls.push(m[1]);
    });
    return urls;
  };

  const collectSrcSetImages = (): string[] => {
    const urls: string[] = [];
    document.querySelectorAll('source[srcset], img[srcset]').forEach((el) => {
      const srcset = el.getAttribute('srcset') ?? '';
      for (const part of srcset.split(',')) {
        const u = part.trim().split(/\s+/)[0];
        if (u) urls.push(u);
      }
    });
    return urls;
  };

  const collectBroadGalleryImages = (): string[] => {
    const urls: string[] = [];
    document.querySelectorAll('img[src], img[data-src], img[data-lazy-src], img[data-original]').forEach((node) => {
      if (skipImgAncestor(node)) return;
      const raw = pickImgUrl(node);
      if (raw) urls.push(raw);
    });
    return urls;
  };

  const domSkuDimensions: Array<{ name: string; values: string[] }> = [];
  const domSkuTableRows: Array<{ label: string; priceText?: string; stockText?: string }> = [];
  const dimLabelRe = /^(颜色|尺码|尺寸|规格|型号|款式|容量|套餐|版本|内长|厚度)/;

  const isJunkSkuValue = (v: string, dimName: string): boolean => {
    if (!v || v.length < 2 || v.length > 100) return true;
    if (v === dimName) return true;
    if (/^(颜色|尺寸|尺码|规格|库存|价格|数量|厚度)$/.test(v)) return true;
    if (/¥|￥/.test(v)) return true;
    if (/库存\s*\d+/.test(v)) return true;
    if (/\d+(?:\.\d+)?\s*mm\s*¥|\d+mm.*¥.*库存/i.test(v)) return true;
    return false;
  };

  const pushDim = (name: string, values: string[]) => {
    const n = name.replace(/[:：\s]+$/u, '').trim();
    const vs = [...new Set(values.map((v) => v.trim()).filter((v) => !isJunkSkuValue(v, n)))];
    if (!n || vs.length === 0) return;
    const existing = domSkuDimensions.find((d) => d.name === n);
    if (existing) {
      for (const v of vs) if (!existing.values.includes(v)) existing.values.push(v);
    } else {
      domSkuDimensions.push({ name: n, values: vs });
    }
  };

  const extractOptionText = (el: Element): string => {
    const img = el.querySelector('img');
    const imgAlt = img?.getAttribute('alt')?.trim();
    if (imgAlt && imgAlt.length >= 2 && imgAlt.length <= 80) return imgAlt;
    const title = el.getAttribute('title')?.trim();
    if (title && title.length >= 2 && title.length <= 80) return title;
    const textEl = el.querySelector('[class*="text"], [class*="name"], span');
    const text = (textEl?.textContent ?? el.textContent ?? '').replace(/\s+/g, ' ').trim();
    return text.slice(0, 100);
  };

  for (const sel of skuSectionSel) {
    document.querySelectorAll(sel).forEach((wrap) => {
      let label = '';
      const labelNode = wrap.querySelector(
        '[class*="label"], [class*="title"], dt, .name, [class*="prop-name"], [class*="sku-item-label"]',
      );
      if (labelNode) label = (labelNode.textContent ?? '').replace(/[:：\s]+$/u, '').trim();
      if (!label || (!dimLabelRe.test(label) && label.length > 16)) return;

      const isSizeSection = /尺寸|尺码|厚度|内长|规格/i.test(label);
      const values: string[] = [];
      const optionSelectors = [
        '[class*="sku-item"]:not([class*="sku-item-wrapper"]):not([class*="sku-item-label"])',
        '[class*="select-item"]',
        '[class*="prop-item"]',
        '[class*="value-item"]',
        '[class*="sku-filter-button"]',
        'button[class*="sku"]',
        '[role="button"][class*="sku"]',
      ];
      for (const optSel of optionSelectors) {
        wrap.querySelectorAll(optSel).forEach((el) => {
          const t = extractOptionText(el);
          if (!t || isJunkSkuValue(t, label)) return;
          if (isSizeSection && /mm\s*¥|库存/.test(t)) return;
          values.push(t);
        });
      }
      if (values.length === 0 && !isSizeSection) {
        wrap.querySelectorAll('[class*="item"], li, a').forEach((el) => {
          if (el.querySelector('[class*="item"], li, button')) return;
          const t = extractOptionText(el);
          if (!t || isJunkSkuValue(t, label)) return;
          values.push(t);
        });
      }
      if (label && values.length) pushDim(label, values);
    });
  }

  const seenTableLabels = new Set<string>();
  const pushTableRow = (text: string) => {
    const blob = text.replace(/\s+/g, ' ').trim();
    if (blob.length < 4 || blob.length > 400) return;
    const hasPrice = /¥\s*[\d.]+|￥\s*[\d.]+/.test(blob);
    const hasStock = /库存\s*\d+/.test(blob);
    if (!hasPrice && !hasStock) return;

    const sizePatterns = [
      /(内长\d+[^¥￥\n]*?(?:【[^】]+】)?)/,
      /([\d.]+\s*mm)/i,
      /(厚度[\d.]+\s*mm)/i,
    ];
    let label = '';
    for (const re of sizePatterns) {
      const m = re.exec(blob);
      if (m && m[1]) {
        label = m[1].trim();
        break;
      }
    }
    if (!label) {
      const beforePrice = blob.split(/¥|￥/)[0] ?? '';
      label = beforePrice.replace(/^(尺寸|尺码|规格|厚度)\s*/i, '').trim().slice(0, 80);
    }
    if (!label || seenTableLabels.has(label)) return;
    seenTableLabels.add(label);
    const priceM = /(?:¥|￥)\s*([\d.]+)/.exec(blob);
    const stockM = /库存\s*(\d+)/.exec(blob);
    domSkuTableRows.push({
      label,
      priceText: priceM && priceM[1] ? priceM[1] : undefined,
      stockText: stockM && stockM[1] ? stockM[1] : undefined,
    });
  };

  for (const sel of skuTableSel) {
    document.querySelectorAll(sel).forEach((row) => pushTableRow(row.textContent ?? ''));
  }
  document.querySelectorAll('[class*="sku"], [class*="count"], tr, li, div, span').forEach((row) => {
    const text = (row.textContent ?? '').replace(/\s+/g, ' ').trim();
    if (text.length > 200 || text.length < 5) return;
    const compact = /([\d.]+\s*mm)\s*(?:¥|￥)\s*([\d.]+).*?库存\s*(\d+)/i.exec(text);
    if (compact) {
      pushTableRow(text);
      return;
    }
    if (/内长\d+/.test(text) && /(?:¥|￥)/.test(text) && /库存/.test(text)) {
      pushTableRow(text);
    }
  });

  let headingText = '';
  for (const sel of titleSelectors) {
    const txt = document.querySelector(sel)?.textContent?.trim();
    if (txt && txt.length > 2 && txt.length < 400) {
      headingText = txt;
      break;
    }
  }

  const meta: {
    description?: string;
    ogTitle?: string;
    ogImage?: string;
    twitterImage?: string;
    keywords?: string;
  } = {};
  document.querySelectorAll('meta').forEach((m) => {
    const prop = m.getAttribute('property') ?? m.getAttribute('name');
    const content = m.getAttribute('content')?.trim();
    if (!content) return;
    if (prop === 'og:title' || prop === 'ogTitle') meta.ogTitle = content;
    if (prop === 'og:image') meta.ogImage = content;
    if (prop === 'twitter:image') meta.twitterImage = content;
    if (prop === 'keywords' || m.getAttribute('name') === 'keywords') meta.keywords = content;
    if (prop === 'description' || m.getAttribute('name') === 'description') meta.description = content;
  });

  const domPriceTexts: string[] = [];
  document
    .querySelectorAll(
      '[class*="price"], [class*="obj-price"], [class*="price-range"], [class*="price-text"], [class*="wholesale"]',
    )
    .forEach((el) => {
      const t = (el.textContent ?? '').replace(/\s+/g, ' ').trim();
      if (t && /¥|￥|批发|起批|价格/.test(t) && t.length < 200) domPriceTexts.push(t);
    });

  const paramPairs: Array<{ key: string; value: string }> = [];
  for (const sel of attrSel) {
    document.querySelectorAll(sel).forEach((node) => {
      let textBlob = '';
      node.querySelectorAll('span, dd, td').forEach((c) => {
        textBlob += ` ${c.textContent ?? ''} `;
      });
      const kv = /\b([\u4e00-\u9fa5a-zA-Z0-9·（）()]{2,30})\s*[:：]\s*([^\n\r:：]{1,120})/.exec(textBlob.trim());
      if (kv && kv[1] && kv[2]) paramPairs.push({ key: kv[1].trim(), value: kv[2].trim() });
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
    if (
      !/skuMap|skuProps|saleProp|saleProps|sku_props|offerImageList|offerImage|imageList|subject|gallery|skuInfoMap|skuModel|tradeModel|amountOnSale|canBookCount/i.test(
        t,
      )
    ) {
      continue;
    }
    pushSnippet(t);
    if (snippets.length >= maxFragments) break;
  }

  const win = window as unknown as Record<string, unknown>;
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
      /* ignore */
    }
  }

  document.querySelectorAll('script[type="application/ld+json"]').forEach((s) => {
    const txt = s.textContent?.trim();
    if (txt && txt.length < 60000 && txt.length > 20) snippets.push(txt);
  });

  const bodyPeek = document.body?.innerText?.slice(0, 3500) ?? '';
  const htmlPeek = document.documentElement?.innerHTML?.slice(0, 4000) ?? '';
  const blocked =
    /安全验证|请完成验证|访问过于频繁|captcha|滑块验证|人机验证|nc-container|punish-page/i.test(bodyPeek) ||
    /punish|x5secdata|captcha/i.test(htmlPeek) ||
    (/验证/.test(bodyPeek) && headingText.length < 2) ||
    (/请登录|账号登录/.test(bodyPeek) && headingText.length < 2);

  const dedupeLocal = (list: string[]): string[] => {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const u of list) {
      const k = u.split('?')[0] ?? u;
      if (seen.has(k)) continue;
      seen.add(k);
      out.push(u);
    }
    return out;
  };

  const docTitle = typeof document.title === 'string' ? document.title.trim() : '';
  const galleryUrls = dedupeLocal([
    ...collectFrom(mainSel),
    ...collectBroadGalleryImages(),
    ...collectSrcSetImages(),
    ...collectBackgroundImages(),
  ]);
  const detailUrls = dedupeLocal([...collectFrom(detailSel), ...collectBackgroundImages()]);

  return {
    finalUrl: baseHref,
    docTitle,
    meta,
    headingText,
    galleryUrls,
    detailUrls,
    domPriceTexts,
    paramPairs,
    domSkuDimensions,
    domSkuTableRows,
    scriptSnippets: snippets,
    __blocked__: blocked ? 1 : 0,
  };
}

export function build1688ExtractDomArg(): Extract1688DomArg {
  return {
    titleSelectors: TITLE_SELECTORS,
    mainSel: MAIN_GALLERY_SELECTORS,
    detailSel: DETAIL_SELECTORS,
    attrSel: ATTRIBUTE_ROW_SELECTORS,
    skuSectionSel: SKU_SECTION_SELECTORS,
    skuTableSel: SKU_TABLE_ROW_SELECTORS,
    snippetMax: SCRIPT_SNIPPET_MAX,
    maxFragments: MAX_SCRIPT_FRAGMENTS,
  };
}

export async function extract1688BrowserPayload(
  page: Page,
): Promise<BrowserExtractPayload & { __blocked__?: number }> {
  return page.evaluate(extract1688DomInPage, build1688ExtractDomArg());
}
