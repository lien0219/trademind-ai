import type { Page } from 'playwright';
import type { ProductSku } from '../../types/product.js';
import { dedupeUrls, normalizeImageUrl } from './image-utils.js';
import type { TaobaoPagePayload } from './page-extract.js';

export type TaobaoJsonPatch = Pick<
  TaobaoPagePayload,
  'mainImages' | 'descriptionImages' | 'attributes' | 'skuGroups' | 'skus'
> & {
  debug?: Record<string, unknown>;
};

type SkuGroup = TaobaoPagePayload['skuGroups'][number];

type RawJsonScan = {
  imageUrls: string[];
  attrPairs: Record<string, string>;
  skuBases: Record<string, unknown>[];
  rootCount: number;
  skuBaseCount: number;
};

function parsePrice(raw: unknown): number | undefined {
  if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0) return raw;
  const text = String(raw ?? '').replace(/,/g, '');
  const m = text.match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function cartesianFromGroups(groups: SkuGroup[], max = 200): ProductSku[] {
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
  return combos.slice(0, max).map((properties) => ({
    properties,
    raw: { fromDomGroups: true },
  }));
}

function buildFromSkuBase(base: Record<string, unknown>): {
  skuGroups: SkuGroup[];
  skus: ProductSku[];
} {
  const propsRaw = (base.props ?? base.skuProps) as unknown;
  if (!Array.isArray(propsRaw) || propsRaw.length === 0) {
    return { skuGroups: [], skus: [] };
  }

  const propMeta = new Map<
    string,
    { name: string; values: Map<string, { name: string; image?: string }> }
  >();

  for (const p of propsRaw) {
    if (!p || typeof p !== 'object') continue;
    const po = p as Record<string, unknown>;
    const pid = String(po.pid ?? po.propertyId ?? po.propId ?? po.id ?? '').trim();
    const name = String(po.name ?? po.propName ?? po.propertyName ?? '规格').trim() || '规格';
    const valuesRaw = po.values ?? po.value;
    if (!Array.isArray(valuesRaw)) continue;
    const values = new Map<string, { name: string; image?: string }>();
    for (const v of valuesRaw) {
      if (!v || typeof v !== 'object') continue;
      const vo = v as Record<string, unknown>;
      const vid = String(vo.vid ?? vo.valueId ?? vo.id ?? vo.name ?? '').trim();
      const label = String(vo.name ?? vo.valueName ?? vo.text ?? '').trim();
      if (!vid || !label) continue;
      const imgRaw = String(vo.image ?? vo.img ?? vo.pic ?? '').trim();
      values.set(vid, { name: label, image: imgRaw || undefined });
    }
    if (values.size) propMeta.set(pid || name, { name, values });
  }

  const skuGroups: SkuGroup[] = [];
  for (const [, meta] of propMeta) {
    skuGroups.push({
      name: meta.name,
      options: [...meta.values.values()].map((v) => ({
        label: v.name,
        selected: false,
        disabled: false,
      })),
    });
  }

  const skusRaw = (base.skus ?? base.skuList ?? base.skuInfos) as unknown;
  const skus: ProductSku[] = [];
  if (Array.isArray(skusRaw)) {
    for (const s of skusRaw) {
      if (!s || typeof s !== 'object') continue;
      const so = s as Record<string, unknown>;
      const propPath = String(so.propPath ?? so.propPathStr ?? so.specId ?? '').trim();
      const properties: Record<string, string> = {};
      let image = '';
      if (propPath) {
        for (const seg of propPath.split(/[;；]/)) {
          const [pid, vid] = seg.split(':');
          if (!pid || !vid) continue;
          const meta = propMeta.get(pid.trim());
          const val = meta?.values.get(vid.trim());
          if (meta && val) {
            properties[meta.name] = val.name;
            if (val.image) image = val.image;
          }
        }
      }
      const priceObj = so.price;
      const price =
        parsePrice(so.price) ??
        (priceObj && typeof priceObj === 'object'
          ? parsePrice((priceObj as Record<string, unknown>).priceText) ??
            parsePrice((priceObj as Record<string, unknown>).priceMoney)
          : undefined);
      const stock = parsePrice(so.quantity ?? so.stock ?? so.amount);
      if (Object.keys(properties).length === 0 && skuGroups.length === 1 && skuGroups[0]!.options.length === 1) {
        properties[skuGroups[0]!.name] = skuGroups[0]!.options[0]!.label;
      }
      if (Object.keys(properties).length === 0) continue;
      skus.push({
        properties,
        price,
        stock: stock !== undefined ? Math.floor(stock) : undefined,
        skuCode: String(so.skuId ?? so.skuid ?? so.id ?? '').trim() || undefined,
        image: image ? normalizeImageUrl(image) : undefined,
        raw: { source: 'skuBase', propPath },
      });
    }
  }

  if (!skus.length && skuGroups.length) {
    return { skuGroups, skus: cartesianFromGroups(skuGroups) };
  }
  return { skuGroups, skus };
}

export async function extractTaobaoJsonPatch(page: Page): Promise<TaobaoJsonPatch> {
  const raw = (await page.evaluate(() => {
    const imageUrls: string[] = [];
    const attrPairs: Record<string, string> = {};
    const skuBases: Record<string, unknown>[] = [];

    const walkForSkuBase = (node: unknown, depth: number): void => {
      if (depth > 14 || !node || typeof node !== 'object') return;
      if (Array.isArray(node)) {
        for (const item of node.slice(0, 40)) walkForSkuBase(item, depth + 1);
        return;
      }
      const o = node as Record<string, unknown>;
      const props = o.props ?? o.skuProps;
      const skus = o.skus ?? o.skuList ?? o.skuInfos;
      if (Array.isArray(props) && (Array.isArray(skus) || o.skuMap)) {
        skuBases.push(o);
      }
      if (o.skuBase && typeof o.skuBase === 'object') {
        skuBases.push(o.skuBase as Record<string, unknown>);
      }
      if (o.skuCore && typeof o.skuCore === 'object') {
        const core = o.skuCore as Record<string, unknown>;
        if (core.sku2info && core.props) {
          const sku2info = core.sku2info as Record<string, unknown>;
          skuBases.push({
            props: core.props,
            skus: Object.keys(sku2info).map((id) => ({ skuId: id, ...(sku2info[id] as object) })),
          });
        }
      }
      for (const v of Object.values(o).slice(0, 50)) {
        walkForSkuBase(v, depth + 1);
      }
    };

    const walkForImageUrls = (node: unknown, depth: number): void => {
      if (depth > 12 || node == null) return;
      if (typeof node === 'string') {
        const s = node.trim();
        if (/alicdn\.com|tbcdn\.cn|taobaocdn\.com/i.test(s) && /\.(?:jpg|jpeg|png|webp)/i.test(s)) {
          imageUrls.push(s);
        }
        return;
      }
      if (Array.isArray(node)) {
        for (const item of node.slice(0, 80)) walkForImageUrls(item, depth + 1);
        return;
      }
      if (typeof node !== 'object') return;
      const o = node as Record<string, unknown>;
      for (const [k, v] of Object.entries(o).slice(0, 60)) {
        if (/desc|detail|content|gallery|image|pic|photo|thumb/i.test(k)) {
          walkForImageUrls(v, depth + 1);
        }
      }
    };

    const roots: unknown[] = [];
    const win = window as unknown as Record<string, unknown>;
    for (const key of [
      '__INITIAL_STATE__',
      '__ICE_APP_CONTEXT__',
      '__INIT_DATA',
      'g_config',
      'Hub',
      'TShop',
      'detailData',
      'pageData',
      'loaderData',
    ]) {
      try {
        const v = win[key];
        if (v && typeof v === 'object') roots.push(v);
      } catch {
        /* ignore */
      }
    }

    for (const script of Array.from(document.scripts)) {
      const text = script.textContent ?? '';
      if (text.length < 80 || text.length > 800000) continue;
      if (!/skuBase|skuCore|skuProps|propPath|picGallery|itemImages|auctionImages/i.test(text)) continue;
      const trimmed = text.trim();
      if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
        try {
          roots.push(JSON.parse(trimmed));
          continue;
        } catch {
          /* fall through */
        }
      }
      const m = text.match(/\{[\s\S]{120,}\}/);
      if (m) {
        try {
          roots.push(JSON.parse(m[0]!));
        } catch {
          /* ignore */
        }
      }
    }

    for (const root of roots) {
      walkForSkuBase(root, 0);
      walkForImageUrls(root, 0);
    }

    for (const root of roots) {
      if (!root || typeof root !== 'object') continue;
      const item = (root as Record<string, unknown>).item ?? (root as Record<string, unknown>).itemDO;
      if (item && typeof item === 'object') {
        const io = item as Record<string, unknown>;
        for (const key of ['images', 'itemImages', 'picList', 'mainPicList', 'auctionImages']) {
          const arr = io[key];
          if (Array.isArray(arr)) {
            for (const u of arr) imageUrls.push(String(u));
          }
        }
        const propsList = io.propsName ?? io.properties;
        if (Array.isArray(propsList)) {
          for (const row of propsList) {
            if (!row || typeof row !== 'object') continue;
            const ro = row as Record<string, unknown>;
            const k = String(ro.name ?? ro.key ?? '').trim();
            const v = String(ro.value ?? ro.val ?? '').trim();
            if (k && v) attrPairs[k] = v;
          }
        }
      }
    }

    return {
      imageUrls,
      attrPairs,
      skuBases,
      rootCount: roots.length,
      skuBaseCount: skuBases.length,
    };
  })) as RawJsonScan;

  let skuGroups: SkuGroup[] = [];
  let skus: ProductSku[] = [];
  for (const base of raw.skuBases ?? []) {
    const built = buildFromSkuBase(base);
    if (built.skus.length > skus.length) {
      skuGroups = built.skuGroups;
      skus = built.skus;
    } else if (built.skuGroups.length > skuGroups.length && !skus.length) {
      skuGroups = built.skuGroups;
    }
  }

  const mainImages = dedupeUrls((raw.imageUrls ?? []).slice(0, 30));
  return {
    mainImages,
    descriptionImages: [],
    attributes: raw.attrPairs ?? {},
    skuGroups,
    skus,
    debug: {
      jsonRootCount: raw.rootCount,
      skuBaseCount: raw.skuBaseCount,
      jsonMainImageCount: mainImages.length,
      jsonSkuCount: skus.length,
    },
  };
}

export function mergeTaobaoPayload(dom: TaobaoPagePayload, json: TaobaoJsonPatch): TaobaoPagePayload {
  const skuGroups = json.skuGroups.length >= dom.skuGroups.length ? json.skuGroups : dom.skuGroups;
  let skus = json.skus.length ? json.skus : dom.skus;
  if (!skus.length && skuGroups.length) {
    skus = cartesianFromGroups(skuGroups);
  }
  return {
    ...dom,
    mainImages: dedupeUrls([...dom.mainImages, ...json.mainImages]),
    descriptionImages: dedupeUrls([...dom.descriptionImages, ...json.descriptionImages]),
    attributes: { ...dom.attributes, ...json.attributes },
    skuGroups,
    skus,
    debug: {
      ...dom.debug,
      ...json.debug,
    },
  };
}
