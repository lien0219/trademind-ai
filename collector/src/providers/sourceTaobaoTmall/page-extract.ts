import type { Page } from 'playwright';
import type { ProductSku } from '../../types/product.js';
import { dedupeUrls, normalizeImageUrl } from './image-utils.js';

export type TaobaoPagePayload = {
  title: string;
  originalTitle: string;
  priceText: string;
  priceRange: string;
  shopName: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string>;
  skuGroups: { name: string; options: { label: string; selected: boolean; disabled: boolean }[] }[];
  skus: ProductSku[];
  debug: Record<string, unknown>;
};

function parsePriceText(text: string): number | undefined {
  const t = text.replace(/,/g, '').trim();
  const m = t.match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

export async function waitForProductCore(page: Page, timeoutMs: number): Promise<void> {
  const selectors = [
    '[class*="ItemHeader"]',
    '[class*="MainTitle"]',
    '#J_Title',
    'h1',
    '[class*="PicGallery"]',
    '#J_UlThumb',
  ];
  for (const sel of selectors) {
    try {
      await page.waitForSelector(sel, { timeout: Math.min(timeoutMs, 15_000) });
      return;
    } catch {
      /* try next */
    }
  }
}

export async function extractTaobaoPagePayload(page: Page): Promise<TaobaoPagePayload> {
  return page.evaluate(() => {
    const pickText = (el: Element | null | undefined): string =>
      (el?.textContent ?? '').replace(/\s+/g, ' ').trim();

    const titleCandidates: string[] = [];
    for (const sel of [
      '[class*="MainTitle"]',
      '[class*="ItemHeader"] h1',
      '#J_Title',
      'h1',
      'meta[property="og:title"]',
    ]) {
      const el = document.querySelector(sel);
      if (sel.includes('meta')) {
        const c = (el as HTMLMetaElement | null)?.content?.trim();
        if (c) titleCandidates.push(c);
      } else {
        const t = pickText(el);
        if (t) titleCandidates.push(t);
      }
    }
    const title = titleCandidates.find((t) => t.length > 2 && !/淘宝|天猫|登录/.test(t)) ?? titleCandidates[0] ?? '';
    const originalTitle = titleCandidates[0] ?? title;

    const priceTexts: string[] = [];
    for (const sel of [
      '[class*="Price--priceText"]',
      '[class*="priceText"]',
      '.tm-price',
      '#J_StrPrice',
      '[class*="Price"]',
    ]) {
      const t = pickText(document.querySelector(sel));
      if (t && /\d/.test(t)) priceTexts.push(t);
    }
    const priceText = priceTexts[0] ?? '';
    let priceRange = '';
    if (priceTexts.length > 1) {
      priceRange = priceTexts.slice(0, 3).join(' - ');
    }

    let shopName = '';
    for (const sel of [
      '[class*="ShopHeader"] [class*="shopName"]',
      '[class*="shopName"]',
      '.tb-shop-name a',
      'a[href*="shop"]',
    ]) {
      const t = pickText(document.querySelector(sel));
      if (t && t.length <= 80 && !/进店|客服/.test(t)) {
        shopName = t;
        break;
      }
    }

    const imgSet = new Set<string>();
    const pushImg = (raw: string | null | undefined) => {
      if (!raw) return;
      let u = raw.trim();
      if (u.startsWith('//')) u = `https:${u}`;
      if (!u.startsWith('http')) return;
      imgSet.add(u);
    };

    for (const img of document.querySelectorAll(
      '[class*="PicGallery"] img, #J_UlThumb img, [class*="thumbnail"] img, [class*="mainPic"] img',
    )) {
      pushImg((img as HTMLImageElement).src || img.getAttribute('data-src'));
    }
    for (const li of document.querySelectorAll('#J_UlThumb li, [class*="thumbnailItem"]')) {
      const bg = (li as HTMLElement).style?.backgroundImage ?? '';
      const m = bg.match(/url\(["']?(.*?)["']?\)/);
      if (m?.[1]) pushImg(m[1]);
    }
    const mainImages = [...imgSet];

    const detailSet = new Set<string>();
    const detailRoot =
      document.querySelector('#J_Description, #description, [class*="desc-root"], [class*="DetailDesc"]') ??
      document.querySelector('[id*="desc"], [class*="Detail"]');
    if (detailRoot) {
      for (const img of detailRoot.querySelectorAll('img')) {
        const src = (img as HTMLImageElement).src || img.getAttribute('data-src');
        if (src) detailSet.add(src.startsWith('//') ? `https:${src}` : src);
      }
    }
    const descriptionImages = [...detailSet];

    const attributes: Record<string, string> = {};
    for (const row of document.querySelectorAll(
      '[class*="ItemParams"] li, [class*="attributes"] li, #attributes li, .tm-tableAttr tr',
    )) {
      const t = pickText(row);
      const parts = t.split(/[:：]/);
      if (parts.length >= 2) {
        const k = parts[0]?.trim();
        const v = parts.slice(1).join(':').trim();
        if (k && v && k.length <= 40) attributes[k] = v;
      }
    }

    const skuGroups: TaobaoPagePayload['skuGroups'] = [];
    for (const group of document.querySelectorAll(
      '[class*="SkuContent"] [class*="skuItem"], #J_isku [class*="prop"], .tm-sale-prop',
    )) {
      const nameEl = group.querySelector('[class*="label"], dt, .tm-prop-title');
      const name = pickText(nameEl) || '规格';
      const options: TaobaoPagePayload['skuGroups'][0]['options'] = [];
      for (const opt of group.querySelectorAll('li, [class*="valueItem"], .tm-img-prop span')) {
        const label = pickText(opt);
        if (!label || label.length > 80) continue;
        const cls = opt.className?.toString() ?? '';
        options.push({
          label,
          selected: /selected|checked|current/i.test(cls) || opt.getAttribute('aria-checked') === 'true',
          disabled: /disabled|soldout|无货|缺货/i.test(cls + pickText(opt)),
        });
      }
      if (options.length) skuGroups.push({ name, options });
    }

    const skus: ProductSku[] = [];
    if (skuGroups.length) {
      skus.push(...cartesianFromDomGroups(skuGroups));
    }

    function cartesianFromDomGroups(groups: TaobaoPagePayload['skuGroups']): ProductSku[] {
      if (!groups.length) return [];
      let combos: Record<string, string>[] = [{}];
      for (const g of groups) {
        const opts = g.options.filter((o) => o.label && !o.disabled);
        if (!opts.length) continue;
        const next: Record<string, string>[] = [];
        for (const combo of combos) {
          for (const opt of opts) {
            next.push({ ...combo, [g.name]: opt.label });
          }
        }
        combos = next.length ? next : combos;
      }
      return combos.slice(0, 200).map((properties) => ({
        properties,
        raw: { skuGroups, fromDomGroups: true },
      }));
    }

    return {
      title,
      originalTitle,
      priceText,
      priceRange,
      shopName,
      mainImages,
      descriptionImages,
      attributes,
      skuGroups,
      skus,
      debug: {
        titleCandidates,
        priceTexts,
        mainImageCount: mainImages.length,
        detailImageCount: descriptionImages.length,
        skuGroupCount: skuGroups.length,
        pageUrl: location.href,
      },
    };
  });
}

export async function collectMainImagesInteractive(page: Page): Promise<string[]> {
  const thumbs = page.locator('#J_UlThumb li, [class*="thumbnailItem"], [class*="PicGallery"] [class*="thumb"]');
  const count = await thumbs.count().catch(() => 0);
  const urls: string[] = [];
  const limit = Math.min(count, 12);
  for (let i = 0; i < limit; i++) {
    try {
      await thumbs.nth(i).click({ timeout: 2000 });
      await page.waitForTimeout(300);
      const main = await page.evaluate(() => {
        const img =
          document.querySelector('[class*="PicGallery"] [class*="mainPic"] img') ??
          document.querySelector('#J_ImgBooth') ??
          document.querySelector('[class*="mainPic"] img');
        const src = (img as HTMLImageElement | null)?.src ?? img?.getAttribute('data-src');
        return src ?? '';
      });
      if (main) urls.push(main);
    } catch {
      /* ignore click failures */
    }
  }
  return dedupeUrls(urls.map(normalizeImageUrl));
}

export async function scrollAndCollectDetailImages(page: Page): Promise<string[]> {
  const detailSel = '#J_Description, #description, [class*="desc-root"], [class*="DetailDesc"]';
  const root = page.locator(detailSel).first();
  if ((await root.count()) === 0) return [];

  await root.scrollIntoViewIfNeeded().catch(() => undefined);
  await page.waitForTimeout(500);

  for (let i = 0; i < 6; i++) {
    await page.evaluate(() => window.scrollBy(0, Math.max(window.innerHeight * 0.8, 400)));
    await page.waitForTimeout(450);
  }

  const imgs = await page.evaluate((sel) => {
    const rootEl = document.querySelector(sel);
    if (!rootEl) return [] as string[];
    const out: string[] = [];
    for (const img of rootEl.querySelectorAll('img')) {
      const src = (img as HTMLImageElement).src || img.getAttribute('data-src') || img.getAttribute('data-ks-lazyload');
      if (src) out.push(src.startsWith('//') ? `https:${src}` : src);
    }
    return out;
  }, detailSel.split(',')[0]!.trim());

  return dedupeUrls(imgs);
}

export function resolvePriceFromPayload(payload: TaobaoPagePayload): {
  price?: number;
  priceText: string;
  priceRange: string;
} {
  const price = parsePriceText(payload.priceText);
  return {
    price,
    priceText: payload.priceText,
    priceRange: payload.priceRange || payload.priceText,
  };
}
