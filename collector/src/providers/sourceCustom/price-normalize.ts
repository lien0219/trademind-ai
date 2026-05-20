export type NormalizedPrice = {
  price?: number;
  currency?: string;
  priceText?: string;
};

const CNY_MARKERS = /[¥￥]|元/;
const USD_MARKERS = /\$/;
const EUR_MARKERS = /€/;

/** Parse price text like ¥59, ￥59.00, 59元, $12.99 into numeric price + ISO currency. */
export function normalizePriceText(raw: string): NormalizedPrice {
  const text = raw.trim();
  if (!text) return {};

  let currency: string | undefined;
  if (CNY_MARKERS.test(text)) currency = 'CNY';
  else if (USD_MARKERS.test(text)) currency = 'USD';
  else if (EUR_MARKERS.test(text)) currency = 'EUR';

  const cleaned = text.replace(/,/g, '').replace(/[^\d.]/g, ' ');
  const m = cleaned.match(/(\d+(?:\.\d+)?)/);
  if (!m) return { priceText: text, currency };

  const price = Number.parseFloat(m[1]);
  if (!Number.isFinite(price) || price <= 0) return { priceText: text, currency };

  return { price, currency, priceText: text };
}

/** If currency field looks like a price string, split into price + currency. */
export function fixMisplacedPriceInCurrency(currencyRaw: string, existingPrice?: number): {
  currency: string;
  price?: number;
  priceText?: string;
} {
  const s = currencyRaw.trim();
  if (!s) return { currency: '' };

  const looksLikePrice =
    CNY_MARKERS.test(s) || USD_MARKERS.test(s) || EUR_MARKERS.test(s) || /^\d+(?:\.\d+)?$/.test(s);

  if (!looksLikePrice) {
    const upper = s.toUpperCase();
    if (/^[A-Z]{3}$/.test(upper)) return { currency: upper };
    return { currency: s };
  }

  const norm = normalizePriceText(s);
  return {
    currency: norm.currency ?? '',
    price: existingPrice ?? norm.price,
    priceText: norm.priceText,
  };
}
