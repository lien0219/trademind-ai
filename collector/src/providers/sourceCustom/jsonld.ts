/**
 * Extract JSON-LD Product hints (trimmed; no huge blobs).
 */
import { evaluateInPageVoid } from '../../browser/evaluate-in-page.js';

export type JsonLdHints = {
  title?: string;
  currency?: string;
  descriptionSnippet?: string;
  images: string[];
  priceAmount?: number;
  brand?: string;
};

function asRecord(v: unknown): Record<string, unknown> | null {
  return v !== null && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

function readImageUrls(val: unknown): string[] {
  if (!val) return [];
  if (typeof val === 'string') return [val];
  if (Array.isArray(val)) {
    const out: string[] = [];
    for (const x of val) {
      if (typeof x === 'string') out.push(x);
      else if (x && typeof x === 'object' && typeof (x as { url?: unknown }).url === 'string') {
        out.push(String((x as { url: string }).url));
      }
    }
    return out;
  }
  if (typeof val === 'object' && typeof (val as { url?: unknown }).url === 'string') {
    return [String((val as { url: string }).url)];
  }
  return [];
}

function parseOffers(offers: unknown): { price?: number; currency?: string } {
  const o = offers;
  const single = asRecord(o);
  const arr = Array.isArray(o) ? o : [];
  const pick = single ?? (arr[0] ? asRecord(arr[0]) : null);
  if (!pick) return {};
  let price: number | undefined;
  const pa = pick.price ?? pick.lowPrice ?? pick.highPrice;
  if (typeof pa === 'number') price = pa;
  else if (typeof pa === 'string') price = Number.parseFloat(pa);
  let currency: string | undefined;
  const pc = pick.priceCurrency;
  if (typeof pc === 'string') currency = pc.trim();
  return { price, currency };
}

function digestProductNode(node: Record<string, unknown>): Partial<JsonLdHints> {
  const types = node['@type'];
  const typeStr =
    typeof types === 'string'
      ? types
      : Array.isArray(types)
        ? types.filter((x): x is string => typeof x === 'string').join(',')
        : '';
  if (!typeStr.toLowerCase().includes('product')) return {};

  const name = node.name;
  const title = typeof name === 'string' ? name.trim() : undefined;

  const imgs = readImageUrls(node.image);

  let descriptionSnippet: string | undefined;
  const desc = node.description;
  if (typeof desc === 'string') {
    descriptionSnippet = desc.trim().slice(0, 800);
  }

  const offers = parseOffers(node.offers);

  let brand: string | undefined;
  const b = node.brand;
  const br = asRecord(b);
  if (typeof b === 'string') brand = b.trim();
  else if (br && typeof br.name === 'string') brand = String(br.name).trim();

  return {
    title,
    currency: offers.currency,
    images: imgs,
    priceAmount: offers.price,
    descriptionSnippet,
    brand,
  };
}

function walkJsonLdObjects(parsed: unknown, found: Partial<JsonLdHints>[]): void {
  const obj = asRecord(parsed);
  if (!obj) return;

  const graph = obj['@graph'];
  if (Array.isArray(graph)) {
    for (const g of graph) {
      walkJsonLdObjects(g, found);
    }
  }

  const merged = digestProductNode(obj);
  if (Object.keys(merged).length > 0) {
    found.push(merged);
  }

  for (const k of Object.keys(obj)) {
    const v = obj[k];
    if (Array.isArray(v)) {
      for (const item of v) walkJsonLdObjects(item, found);
    } else if (v && typeof v === 'object') {
      walkJsonLdObjects(v, found);
    }
  }
}

export async function extractJsonLdHints(page: import('playwright').Page): Promise<JsonLdHints | null> {
  const snippets = await evaluateInPageVoid(page, () => {
    const scripts = Array.from(document.querySelectorAll('script[type="application/ld+json"]'));
    const texts: string[] = [];
    for (const s of scripts) {
      const t = (s.textContent ?? '').trim();
      if (t && t.length < 800_000) texts.push(t.slice(0, 400_000));
    }
    return texts;
  });

  const merged: Partial<JsonLdHints>[] = [];
  for (const raw of snippets) {
    try {
      const parsed: unknown = JSON.parse(raw);
      walkJsonLdObjects(parsed, merged);
    } catch {
      /** ignore malformed blocks */
    }
  }

  if (merged.length === 0) return null;

  let title: string | undefined;
  let currency: string | undefined;
  let descriptionSnippet: string | undefined;
  let brand: string | undefined;
  let priceAmount: number | undefined;
  const images: string[] = [];

  for (const m of merged) {
    if (!title && m.title) title = m.title;
    if (!currency && m.currency) currency = m.currency;
    if (!descriptionSnippet && m.descriptionSnippet) descriptionSnippet = m.descriptionSnippet;
    if (!brand && m.brand) brand = m.brand;
    if (priceAmount === undefined && m.priceAmount !== undefined) priceAmount = m.priceAmount;
    if (m.images?.length) images.push(...m.images);
  }

  return {
    title,
    currency,
    descriptionSnippet,
    brand,
    priceAmount,
    images,
  };
}
