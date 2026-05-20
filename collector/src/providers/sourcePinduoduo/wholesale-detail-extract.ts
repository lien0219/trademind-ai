/**
 * pifa.pinduoduo.com/goods/detail DOM extraction (serialized to browser via page.evaluate).
 */
export type PifaImageCandidate = {
  url: string;
  source:
    | 'main_gallery'
    | 'thumbnail_gallery'
    | 'detail_section'
    | 'sku_image'
    | 'shop_info'
    | 'unknown';
  order: number;
  width: number;
  height: number;
};

export type PifaWholesaleDomPayload = {
  pageUrl: string;
  docTitle: string;
  titleCandidates: string[];
  priceRangeText?: string;
  priceTexts: string[];
  mainImageCandidates: PifaImageCandidate[];
  detailImageCandidates: PifaImageCandidate[];
  unknownImageCandidates: PifaImageCandidate[];
  ogImageUrl?: string;
  introTexts: string[];
  skuRows: Array<{ name: string; priceText?: string; stock?: number; imageUrl?: string }>;
  attributes: Record<string, string>;
  specButtonCount: number;
  introFound: boolean;
};

export function extractPifaWholesaleDetailInPage(): PifaWholesaleDomPayload {
  const pageUrl = location.href;
  const docTitle = document.title?.trim() ?? '';
  const vw = window.innerWidth || 1280;
  const vh = window.innerHeight || 900;
  let orderSeq = 0;

  const PLATFORM_TITLE_RE =
    /拼多多批发|拼多多官方采购批发平台|多多批发|^登录$|^首页$|购物车|搜索/i;
  const TITLE_NOISE =
    /分享商品|收藏|加入购物车|立即购买|联系客服|店铺信息|已售|批发价|原价|去拼单|客服|进店/i;

  const isBadTitle = (t: string) => {
    const s = t.replace(/\s+/g, ' ').trim();
    return !s || s.length < 4 || PLATFORM_TITLE_RE.test(s) || s.length > 400;
  };

  const pushTitle = (raw: string, list: string[]) => {
    const t = raw.replace(/\s+/g, ' ').trim();
    if (!isBadTitle(t)) list.push(t);
  };

  const titleFromLeafText = (el: Element): string => {
    const own = [...el.childNodes]
      .filter((n) => n.nodeType === Node.TEXT_NODE)
      .map((n) => n.textContent ?? '')
      .join('')
      .replace(/\s+/g, ' ')
      .trim();
    if (own.length >= 4) return own;
    if (el.children.length === 0) return el.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    const parts: string[] = [];
    for (const child of el.children) {
      if (child.tagName === 'BUTTON' || child.getAttribute('role') === 'button') continue;
      const tag = child.tagName.toLowerCase();
      if (tag === 'svg' || tag === 'img' || tag === 'i') continue;
      const t = child.textContent?.replace(/\s+/g, ' ').trim() ?? '';
      if (t && !TITLE_NOISE.test(t) && t.length <= 200) parts.push(t);
    }
    return parts.join(' ').trim();
  };

  const titleCandidates: string[] = [];
  const titleSelectors = [
    '[class*="goods-title"]',
    '[class*="goodsTitle"]',
    '[class*="GoodsTitle"]',
    '[class*="product-title"]',
    '[class*="item-title"]',
    '[class*="detail-title"]',
    '[class*="goods-name"]',
    '[class*="goodsName"]',
    '[data-testid*="title"]',
    'h1',
    'h2',
  ];

  for (const sel of titleSelectors) {
    document.querySelectorAll(sel).forEach((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.left < vw * 0.28) return;
      const text = titleFromLeafText(el);
      if (text) pushTitle(text, titleCandidates);
    });
  }

  const pickImgUrl = (el: Element): string => {
    const img = el as HTMLImageElement;
    const fromSrcset = (): string => {
      const srcset = img.getAttribute('srcset') ?? '';
      if (!srcset.trim()) return '';
      const parts = srcset
        .split(',')
        .map((p) => p.trim().split(/\s+/)[0])
        .filter(Boolean);
      return parts[parts.length - 1] ?? '';
    };
    const order = [
      'data-original',
      'data-src',
      'data-lazy-img',
      'data-lazy-src',
      'data-img',
      'data-url',
      'data-zoom',
      'src',
    ];
    for (const attr of order) {
      const v =
        img.getAttribute(attr) ||
        (attr === 'src' && img.currentSrc ? img.currentSrc : null) ||
        (attr === 'src' ? img.src : null);
      if (v && String(v).trim() && !String(v).startsWith('data:')) return String(v).trim();
    }
    return fromSrcset();
  };

  const collectBgUrl = (el: Element): string => {
    const style = (el as HTMLElement).style?.backgroundImage ?? el.getAttribute('style') ?? '';
    const m = /url\(['"]?([^'")]+)['"]?\)/i.exec(style);
    return m?.[1]?.trim() ?? '';
  };

  const ancestorHint = (el: Element): string => {
    let p: Element | null = el.parentElement;
    let depth = 0;
    let blob = '';
    while (p && depth < 10) {
      const cls = typeof p.className === 'string' ? p.className : '';
      blob += ` ${cls} ${p.id ?? ''} ${p.tagName}`;
      p = p.parentElement;
      depth++;
    }
    return blob.toLowerCase();
  };

  const isIrrelevantHint = (hint: string): boolean =>
    /shop|store|mall|merchant|seller|kefu|service|footer|header|nav|toolbar|cart|share|qrcode|qr|logo|icon|avatar|banner|coupon|guarantee|promise|customer|float|sidebar|toolbar/.test(
      hint,
    );

  const mainImageCandidates: PifaImageCandidate[] = [];
  const detailImageCandidates: PifaImageCandidate[] = [];
  const unknownImageCandidates: PifaImageCandidate[] = [];
  const mainUrlKeys = new Set<string>();

  const pushMainImage = (url: string, el: Element, rect: DOMRect, source: 'main_gallery' | 'thumbnail_gallery') => {
    if (!url) return;
    const item: PifaImageCandidate = {
      url,
      source,
      order: orderSeq++,
      width: Math.round(rect.width),
      height: Math.round(rect.height),
    };
    mainImageCandidates.push(item);
    mainUrlKeys.add(url.split('?')[0] ?? url);
  };

  const pushUnknownImage = (url: string, el: Element, rect: DOMRect, source: PifaImageCandidate['source'] = 'unknown') => {
    if (!url) return;
    const base = url.split('?')[0] ?? url;
    if (mainUrlKeys.has(base)) return;
    unknownImageCandidates.push({
      url,
      source,
      order: orderSeq++,
      width: Math.round(rect.width),
      height: Math.round(rect.height),
    });
  };

  const pushDetailImage = (url: string, el: Element, rect: DOMRect) => {
    if (!url) return;
    detailImageCandidates.push({
      url,
      source: 'detail_section',
      order: orderSeq++,
      width: Math.round(rect.width),
      height: Math.round(rect.height),
    });
  };

  const collectMediaInRoot = (
    root: Element,
    sink: 'main' | 'detail',
    opts?: { leftMaxRatio?: number },
  ) => {
    const leftMax = vw * (opts?.leftMaxRatio ?? (sink === 'main' ? 0.44 : 0));
    root.querySelectorAll('img').forEach((img) => {
      const rect = img.getBoundingClientRect();
      if (rect.width < 12 || rect.height < 12) return;
      const hint = ancestorHint(img);
      if (isIrrelevantHint(hint)) return;
      if (sink === 'main' && rect.left > leftMax) return;
      if (sink === 'detail' && rect.left < vw * 0.38 && rect.width < 200) return;
      const url = pickImgUrl(img);
      if (!url) return;
      if (sink === 'main') {
        const source: 'main_gallery' | 'thumbnail_gallery' =
          rect.width >= 120 || rect.height >= 120 ? 'main_gallery' : 'thumbnail_gallery';
        pushMainImage(url, img, rect, source);
      } else {
        pushDetailImage(url, img, rect);
      }
    });
    root.querySelectorAll('[style*="background"]').forEach((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width < 48 || rect.height < 48) return;
      const hint = ancestorHint(el);
      if (isIrrelevantHint(hint)) return;
      const url = collectBgUrl(el);
      if (!url) return;
      if (sink === 'main') {
        if (rect.left > leftMax) return;
        pushMainImage(url, el, rect, 'main_gallery');
      } else {
        pushDetailImage(url, el, rect);
      }
    });
  };

  const findMainGalleryRoot = (): Element | null => {
    const selectors = [
      '[class*="gallery"]',
      '[class*="swiper"]',
      '[class*="carousel"]',
      '[class*="image-view"]',
      '[class*="pic-view"]',
      '[class*="goods-img"]',
      '[class*="goodsImg"]',
      '[class*="preview"]',
      '[class*="thumb"]',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (!el) continue;
      const rect = el.getBoundingClientRect();
      if (rect.left < vw * 0.45 && rect.width >= 80) return el;
    }

    const leftMax = vw * 0.42;
    const buckets = new Map<Element, number>();
    document.querySelectorAll('img').forEach((img) => {
      const rect = img.getBoundingClientRect();
      if (rect.left > leftMax || rect.top > vh * 0.82) return;
      if (rect.width < 28 || rect.height < 28) return;
      if (isIrrelevantHint(ancestorHint(img))) return;
      let p: Element | null = img.parentElement;
      for (let d = 0; p && d < 6; d++) {
        const pr = p.getBoundingClientRect();
        if (pr.left > leftMax) break;
        buckets.set(p, (buckets.get(p) ?? 0) + 1);
        p = p.parentElement;
      }
    });
    let best: Element | null = null;
    let bestCount = 0;
    for (const [el, count] of buckets) {
      if (count > bestCount) {
        bestCount = count;
        best = el;
      }
    }
    return bestCount >= 1 ? best : null;
  };

  const galleryRoot = findMainGalleryRoot();
  if (galleryRoot) {
    collectMediaInRoot(galleryRoot, 'main', { leftMaxRatio: 0.5 });
  } else {
    document.querySelectorAll('img').forEach((img) => {
      const rect = img.getBoundingClientRect();
      if (rect.left > vw * 0.48 || rect.top > vh * 0.85) return;
      if (rect.width < 12 || rect.height < 12) return;
      if (isIrrelevantHint(ancestorHint(img))) return;
      const url = pickImgUrl(img);
      if (!url) return;
      const source: 'main_gallery' | 'thumbnail_gallery' =
        rect.width >= 100 || rect.height >= 100 ? 'main_gallery' : 'thumbnail_gallery';
      pushMainImage(url, img, rect, source);
    });
  }

  document.querySelectorAll('img').forEach((img) => {
    const rect = img.getBoundingClientRect();
    if (rect.width < 16 || rect.height < 16) return;
    if (isIrrelevantHint(ancestorHint(img))) return;
    const hint = ancestorHint(img);
    if (/shop|store|mall|merchant|kefu|qrcode|qr|logo|icon|avatar|cart|share|service|banner/.test(hint)) {
      return;
    }
    const url = pickImgUrl(img);
    if (!url) return;
    if (rect.left < vw * 0.5 && rect.top < vh * 0.82) {
      if (rect.width >= 48 || rect.height >= 48 || rect.width === 0) {
        pushUnknownImage(url, img, rect, rect.left < vw * 0.44 ? 'main_gallery' : 'unknown');
      }
    }
  });

  const ogImageUrl =
    document.querySelector('meta[property="og:image"]')?.getAttribute('content')?.trim() || undefined;

  const priceTexts: string[] = [];
  let priceRangeText: string | undefined;
  const priceRangeRe = /[¥￥]\s*[\d,.]+(?:\s*[-–—~至]\s*[\d,.]+)?/;

  const scanPriceText = (text: string) => {
    const t = text.replace(/\s+/g, ' ').trim();
    if (!/[¥￥]/.test(t)) return;
    const m = t.match(priceRangeRe);
    if (m) {
      priceTexts.push(m[0]);
      if (!priceRangeText || m[0].length > priceRangeText.length) priceRangeText = m[0];
    }
  };

  document.querySelectorAll('[class*="price"], [class*="Price"]').forEach((el) => {
    const t = el.textContent?.trim();
    if (t) scanPriceText(t);
  });

  const findPriceAnchor = (): Element | null => {
    for (const el of document.querySelectorAll('[class*="price"], [class*="Price"], span, div')) {
      const t = el.textContent?.trim() ?? '';
      if (priceRangeRe.test(t) && t.length < 80) return el;
    }
    return null;
  };

  const priceAnchor = findPriceAnchor();
  if (priceAnchor) {
    let node: Element | null = priceAnchor.parentElement;
    for (let depth = 0; node && depth < 6; depth++) {
      const prev = node.previousElementSibling;
      if (prev) {
        const text = titleFromLeafText(prev);
        if (text.length >= 8 && text.length <= 300 && !/[¥￥]/.test(text)) {
          pushTitle(text, titleCandidates);
        }
      }
      node = node.parentElement;
    }
  }

  const scoreTitle = (t: string): number => {
    if (isBadTitle(t)) return -1e6;
    let score = t.length;
    if (TITLE_NOISE.test(t)) score -= 200;
    if (/分享商品/.test(t)) score -= 500;
    if (/【|】/.test(t)) score += 30;
    return score;
  };

  const rankedTitles = [...new Set(titleCandidates)].sort((a, b) => scoreTitle(b) - scoreTitle(a));
  titleCandidates.length = 0;
  titleCandidates.push(...rankedTitles);

  const ogTitle = document.querySelector('meta[property="og:title"]')?.getAttribute('content') ?? '';
  if (ogTitle) pushTitle(ogTitle, titleCandidates);
  if (!titleCandidates.length && docTitle) pushTitle(docTitle, titleCandidates);

  const parseSkuNameLine = (line: string): string => {
    return line
      .replace(/仅剩\s*\d+\s*件/gi, '')
      .replace(/库存\s*\d+\s*件?/gi, '')
      .trim();
  };

  const parseRowText = (text: string) => {
    const lines = text
      .split('\n')
      .map((l) => l.replace(/\s+/g, ' ').trim())
      .filter(Boolean);
    if (lines.length === 0) return null;

    let name = '';
    let priceText: string | undefined;
    let stock: number | undefined;

    for (const line of lines) {
      if (/^[¥￥]\s*[\d,.]+/.test(line)) {
        priceText = line.match(/[¥￥]\s*[\d,.]+/)?.[0];
        continue;
      }
      const stockM = line.match(/仅剩\s*(\d+)\s*件/);
      if (stockM) {
        stock = Number.parseInt(stockM[1], 10);
        const namePart = parseSkuNameLine(line);
        if (namePart.length > name.length) name = namePart;
        continue;
      }
      if (/^[\d+\-]+$/.test(line) || /^[+-]$/.test(line)) continue;
      if (/^起批|^库存|^已选|^数量|^分享/.test(line)) continue;
      const cleaned = parseSkuNameLine(line);
      if (cleaned.length > name.length && cleaned.length <= 120) name = cleaned;
    }

    if (!name || name.length < 2) return null;
    if (!priceText && !/【|】|版|色|线|模型/.test(name)) return null;
    return { name, priceText, stock };
  };

  const skuRows: Array<{ name: string; priceText?: string; stock?: number; imageUrl?: string }> = [];
  const seenSku = new Set<string>();

  const tryAddSkuRow = (el: Element) => {
    const text = (el as HTMLElement).innerText?.trim() ?? '';
    if (!text || text.length > 350 || text.length < 4) return;
    if (!/[¥￥][\d,.]+/.test(text)) return;
    const parsed = parseRowText(text);
    if (!parsed) return;
    const key = `${parsed.name}|${parsed.priceText ?? ''}`;
    if (seenSku.has(key)) return;
    seenSku.add(key);

    let imageUrl = '';
    const img = el.querySelector('img');
    if (img) {
      const rect = img.getBoundingClientRect();
      imageUrl = pickImgUrl(img);
      if (imageUrl) {
        pushUnknownImage(imageUrl, img, rect, 'sku_image');
      }
    }
    skuRows.push({ ...parsed, imageUrl: imageUrl || undefined });
  };

  const skuContainerSelectors = [
    '[class*="sku-list"]',
    '[class*="skuList"]',
    '[class*="spec-list"]',
    '[class*="specList"]',
    '[class*="goods-spec"]',
    '[class*="goodsSpec"]',
    '[class*="sku-item"]',
    '[class*="Sku"]',
  ];

  for (const sel of skuContainerSelectors) {
    document.querySelectorAll(sel).forEach((container) => {
      container.querySelectorAll('li, [class*="item"], [class*="row"], div').forEach((row) => {
        tryAddSkuRow(row);
      });
    });
  }

  if (skuRows.length < 2) {
    document.querySelectorAll('li, div').forEach((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.left < vw * 0.35) return;
      if (rect.width < 200 || rect.height < 28 || rect.height > 220) return;
      tryAddSkuRow(el);
    });
  }

  let specButtonCount = 0;
  document
    .querySelectorAll('[class*="sku"] button, [class*="spec"] button, [class*="Sku"] [role="button"]')
    .forEach((el) => {
      const t = el.textContent?.trim();
      if (t && t.length >= 2 && t.length <= 60) specButtonCount++;
    });

  const attributes: Record<string, string> = {};
  const introMarkers = ['商品介绍', '商品参数', '产品参数', '参数'];
  let introFound = false;
  let introEl: Element | null = null;

  for (const marker of introMarkers) {
    const found = [...document.querySelectorAll('div, section, h2, h3, span, a')].find((el) => {
      const t = el.textContent?.trim() ?? '';
      return t === marker || (t.length <= 12 && t.startsWith(marker));
    });
    if (found) {
      introFound = true;
      introEl = found;
      break;
    }
  }

  if (introEl) {
    introEl.scrollIntoView({ block: 'start' });
  }

  const introRoot =
    introEl?.closest('section') ??
    introEl?.parentElement?.parentElement ??
    document.querySelector('[class*="detail"], [class*="intro"], [id*="detail"]') ??
    null;

  const parseAttrPairs = (root: Element) => {
    root.querySelectorAll('dl').forEach((dl) => {
      dl.querySelectorAll('dt').forEach((dt) => {
        const key = dt.textContent?.trim().replace(/[:：]\s*$/, '') ?? '';
        const dd = dt.nextElementSibling;
        const val = dd?.textContent?.trim() ?? '';
        if (key && val && key.length <= 40 && val.length <= 200 && !TITLE_NOISE.test(key)) {
          attributes[key] = val;
        }
      });
    });
    root.querySelectorAll('tr').forEach((tr) => {
      const cells = [...tr.querySelectorAll('th,td')].map((c) => c.textContent?.trim() ?? '');
      if (cells.length >= 2 && cells[0] && cells[1]) {
        attributes[cells[0].replace(/[:：]\s*$/, '')] = cells[1];
      }
    });
    root.querySelectorAll('[class*="param"], [class*="attr"], [class*="property"], li, div').forEach(
      (row) => {
        const text = row.textContent?.replace(/\s+/g, ' ').trim() ?? '';
        const m = text.match(/^([^:：]{2,24})[:：]\s*(.{1,120})$/);
        if (m && !TITLE_NOISE.test(m[1])) {
          attributes[m[1].trim()] = m[2].trim();
        }
      },
    );
  };

  const introTexts: string[] = [];
  const INTRO_NOISE =
    /分享|购物车|立即购买|联系客服|店铺|已售|批发价|原价|拼多多批发|下载|App|扫码/i;

  if (introRoot) {
    parseAttrPairs(introRoot);
    introRoot.querySelectorAll('p, div, section, article, span').forEach((el) => {
      if (el.children.length > 10) return;
      const t = el.textContent?.replace(/\s+/g, ' ').trim() ?? '';
      if (t.length < 24 || t.length > 1500) return;
      if (INTRO_NOISE.test(t)) return;
      if (/^[¥￥]/.test(t)) return;
      if (/^商品介绍$|^商品参数$|^产品参数$/.test(t)) return;
      introTexts.push(t);
    });

    collectMediaInRoot(introRoot, 'detail');
  }

  return {
    pageUrl,
    docTitle,
    titleCandidates,
    priceRangeText,
    priceTexts,
    mainImageCandidates,
    detailImageCandidates,
    unknownImageCandidates,
    ogImageUrl,
    introTexts: [...new Set(introTexts)].slice(0, 8),
    skuRows,
    attributes,
    specButtonCount,
    introFound,
  };
}
