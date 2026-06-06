import type { Page } from 'playwright';
import type { ProductSku } from '../../types/product.js';
import type { TaobaoPagePayload } from './page-extract.js';

export type TaobaoSkuGroupOption = {
  name: string;
  available: boolean;
  imageUrl: string;
};

export type TaobaoSkuGroup = {
  name: string;
  options: TaobaoSkuGroupOption[];
};

export type TaobaoSkuCollectOptions = {
  enabled: boolean;
  maxClicks: number;
};

function parsePriceFromText(text: string): number | undefined {
  const t = text.replace(/,/g, '').trim();
  const m = t.match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function buildSkuName(attrs: Record<string, string>): string {
  return Object.values(attrs).filter(Boolean).join(' / ');
}

function cartesianCombos(
  groups: TaobaoSkuGroup[],
  max: number,
): { attrs: Record<string, string>; available: boolean; imageUrl: string }[] {
  let combos: { attrs: Record<string, string>; available: boolean; imageUrl: string }[] = [
    { attrs: {}, available: true, imageUrl: '' },
  ];
  for (const g of groups) {
    const opts = g.options.filter((o) => o.name);
    if (!opts.length) continue;
    const next: typeof combos = [];
    for (const combo of combos) {
      for (const opt of opts) {
        next.push({
          attrs: { ...combo.attrs, [g.name]: opt.name },
          available: combo.available && opt.available,
          imageUrl: opt.imageUrl || combo.imageUrl,
        });
      }
    }
    combos = next.length ? next : combos;
    if (combos.length > max) break;
  }
  return combos.slice(0, max);
}

export function toTaobaoSkuGroups(payload: TaobaoPagePayload): TaobaoSkuGroup[] {
  return payload.skuGroups.map((g) => ({
    name: g.name,
    options: g.options.map((o) => ({
      name: o.label,
      available: !o.disabled,
      imageUrl: '',
    })),
  }));
}

export async function collectSkuPricesByClick(
  page: Page,
  groups: TaobaoSkuGroup[],
  options: TaobaoSkuCollectOptions,
): Promise<ProductSku[]> {
  if (!options.enabled || groups.length === 0) return [];

  const combos = cartesianCombos(groups, Math.min(options.maxClicks, 48));
  if (combos.length > options.maxClicks) {
    return combos.slice(0, options.maxClicks).map((c) => ({
      properties: c.attrs,
      price: undefined,
      stock: undefined,
      image: c.imageUrl || undefined,
      raw: { fromSkuClick: false, available: c.available },
    }));
  }

  const skus: ProductSku[] = [];
  const priceSel =
    '[class*="Price--priceText"], [class*="priceText"], .tm-price, #J_StrPrice, [class*="Price"]';

  for (const combo of combos.slice(0, options.maxClicks)) {
    try {
      for (const [groupName, optName] of Object.entries(combo.attrs)) {
        const group = groups.find((g) => g.name === groupName);
        if (!group) continue;
        const opt = group.options.find((o) => o.name === optName);
        if (!opt || !opt.available) continue;

        const locator = page
          .locator(
            `[class*="SkuContent"] [class*="skuItem"], #J_isku [class*="prop"], .tm-sale-prop`,
          )
          .filter({ hasText: groupName })
          .locator('li, [class*="valueItem"], .tm-img-prop span')
          .filter({ hasText: optName })
          .first();

        if ((await locator.count()) > 0) {
          await locator.click({ timeout: 1500 }).catch(() => undefined);
          await page.waitForTimeout(280);
        }
      }

      const priceText = await page
        .locator(priceSel)
        .first()
        .textContent()
        .catch(() => '');
      const price = parsePriceFromText(priceText ?? '');

      skus.push({
        properties: combo.attrs,
        price,
        stock: undefined,
        image: combo.imageUrl || undefined,
        raw: { fromSkuClick: true, available: combo.available, priceText: priceText?.trim() },
      });
    } catch {
      skus.push({
        properties: combo.attrs,
        price: undefined,
        stock: undefined,
        image: combo.imageUrl || undefined,
        raw: { fromSkuClick: true, available: combo.available, clickFailed: true },
      });
    }
  }

  return skus;
}

export function mergeSkuResults(
  base: ProductSku[],
  clicked: ProductSku[],
): { skus: ProductSku[]; skuGroups: TaobaoSkuGroup[] } {
  if (!clicked.length) {
    return { skus: base, skuGroups: [] };
  }
  const byKey = new Map<string, ProductSku>();
  for (const s of base) {
    const key = JSON.stringify(s.properties ?? {});
    byKey.set(key, s);
  }
  for (const s of clicked) {
    const key = JSON.stringify(s.properties ?? {});
    const prev = byKey.get(key);
    if (!prev || (s.price && s.price > 0 && (!prev.price || prev.price <= 0))) {
      byKey.set(key, { ...prev, ...s, properties: s.properties });
    }
  }
  return { skus: [...byKey.values()], skuGroups: [] };
}
