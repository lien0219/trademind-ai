import type { Page } from 'playwright';

export type OgHints = {
  title?: string;
  description?: string;
  currency?: string;
  priceAmount?: number;
  images: string[];
};

export async function extractOpenGraphHints(page: Page): Promise<OgHints> {
  return page.evaluate(() => {
    const metaContent = (sel: string): string | undefined => {
      const el = document.querySelector(sel);
      const c = el?.getAttribute('content');
      return typeof c === 'string' ? c.trim() : undefined;
    };

    const title =
      metaContent('meta[property="og:title"]') ?? metaContent('meta[name="twitter:title"]');
    const description =
      metaContent('meta[property="og:description"]') ?? metaContent('meta[name="description"]');
    const currency =
      metaContent('meta[property="product:price:currency"]') ??
      metaContent('meta[itemprop="priceCurrency"]');

    let priceAmount: number | undefined;
    const pa =
      metaContent('meta[property="product:price:amount"]') ??
      metaContent('meta[property="og:price:amount"]');
    if (pa) {
      const n = Number.parseFloat(pa.replace(/,/g, ''));
      if (!Number.isNaN(n)) priceAmount = n;
    }

    const imgs: string[] = [];
    const ogImg = metaContent('meta[property="og:image"]');
    if (ogImg) imgs.push(ogImg);
    const ogImgSecure = metaContent('meta[property="og:image:secure_url"]');
    if (ogImgSecure) imgs.push(ogImgSecure);

    return {
      title: title || undefined,
      description: description || undefined,
      currency: currency || undefined,
      priceAmount,
      images: imgs,
    };
  });
}
