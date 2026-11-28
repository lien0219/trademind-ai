// Synthetic parser fixture; this is not a captured live platform snapshot.
import { describe, expect, it } from 'vitest';
import { assembleAeProduct } from '../parser.js';
import type { AeBrowserPayload } from '../types.js';

function makePayload(overrides: Partial<AeBrowserPayload> = {}): AeBrowserPayload {
  return {
    finalUrl: 'https://www.aliexpress.com/item/100500123.html',
    docTitle: 'Product detail',
    meta: { priceCurrency: 'USD' },
    headingText: '',
    galleryUrls: [],
    detailUrls: [],
    paramPairs: [],
    scriptSnippets: [],
    ...overrides,
  };
}

const publicProductJson = JSON.stringify({
  productTitle: 'Premium Cotton Bedding Set For Bedroom',
  productProperties: [{ attrName: 'Material', attrValue: 'Cotton' }],
  skuModule: {
    skuPropertyList: [{
      skuPropertyName: 'Color',
      skuPropertyValues: [
        { propertyValueDisplayName: 'Black', propertyValueIdLong: '14:100001' },
        { propertyValueDisplayName: 'White', propertyValueIdLong: '14:100002' },
      ],
    }],
  },
  skuMap: {
    '14:100001#7643024': { skuAmount: { value: '19.99' }, availQuantity: 10 },
    '14:100002#7643025': { skuAmount: { value: '21.99' }, availQuantity: 8 },
  },
  imagePathList: [
    'https://ae01.alicdn.com/kf/main-1.jpg',
    'https://ae01.alicdn.com/kf/main-2.jpg',
  ],
  description: '<img src="https://ae01.alicdn.com/kf/detail-1.jpg">',
});

describe('AliExpress parser regressions', () => {
  it('extracts structured title, images, attributes, and mapped SKU variants', () => {
    const mainImage = 'https://ae01.alicdn.com/kf/main-1.jpg';
    const product = assembleAeProduct(
      'https://www.aliexpress.com/item/100500123.html',
      makePayload({
        galleryUrls: [mainImage],
        detailUrls: ['https://ae01.alicdn.com/kf/detail-1.jpg'],
        scriptSnippets: [publicProductJson],
      }),
    );

    expect(product.title).toBe('Premium Cotton Bedding Set For Bedroom');
    expect(product.currency).toBe('USD');
    expect(product.mainImages).toContain(mainImage);
    expect(new Set(product.mainImages.map((url) => url.split('?')[0])).size).toBe(product.mainImages.length);
    expect(product.descriptionImages).not.toContain(mainImage);
    expect(product.attributes).toMatchObject({ Material: 'Cotton' });
    expect(product.skus).toHaveLength(2);
    expect(product.skus.map((sku) => sku.properties)).toEqual([{ Color: 'Black' }, { Color: 'White' }]);
    expect(product.skus.map((sku) => sku.price)).toEqual([19.99, 21.99]);
    expect(product.skus.map((sku) => sku.stock)).toEqual([10, 8]);
  });

  it('preserves the blocked signal when a challenge page has no product title', () => {
    const product = assembleAeProduct(
      'https://www.aliexpress.com/item/100500123.html',
      { ...makePayload({ docTitle: 'Verification' }), __blocked__: 1 },
    );

    expect(product.blocked).toBe(true);
    expect(product.title).toBe('Verification');
  });
});
