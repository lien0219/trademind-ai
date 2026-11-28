// Synthetic parser fixture; this is not a captured live platform snapshot.
import { describe, expect, it } from 'vitest';
import {
  assemblePifaWholesaleProduct,
  validateWholesaleCollectQuality,
} from '../wholesale-detail.js';
import type { PifaWholesaleDomPayload } from '../wholesale-detail-extract.js';

function makePayload(overrides: Partial<PifaWholesaleDomPayload> = {}): PifaWholesaleDomPayload {
  return {
    pageUrl: 'https://pifa.pinduoduo.com/goods/detail/?gid=123',
    docTitle: '商品详情',
    titleCandidates: ['休闲纯棉短袖女装T恤春夏新款'],
    priceRangeText: '¥19.90 - ¥29.90',
    priceTexts: [],
    mainImageCandidates: [
      { url: 'https://img.pddpic.com/mms-material-img/main-1.jpg', source: 'main_gallery', order: 0, width: 900, height: 900 },
      { url: 'https://img.pddpic.com/mms-material-img/main-2.jpg', source: 'thumbnail_gallery', order: 1, width: 800, height: 800 },
      { url: 'https://img.pddpic.com/mms-material-img/main-3.jpg', source: 'thumbnail_gallery', order: 2, width: 800, height: 800 },
    ],
    detailImageCandidates: [
      { url: 'https://img.pddpic.com/mms-material-img/detail-1.jpg', source: 'detail_section', order: 0, width: 1000, height: 1600 },
    ],
    unknownImageCandidates: [],
    introTexts: ['采用柔软纯棉面料，适合春夏日常穿着和多种搭配场景。'],
    skuRows: [{ name: '黑色 / M', priceText: '¥21.90', stock: 18 }],
    attributes: { 材质: '纯棉' },
    specButtonCount: 1,
    introFound: true,
    ...overrides,
  };
}

describe('Pinduoduo Pifa parser regressions', () => {
  it('extracts product title, price range, SKU price/stock, attributes, and image regions', () => {
    const product = assemblePifaWholesaleProduct(makePayload());
    const quality = validateWholesaleCollectQuality(product);

    expect(product.title).toBe('休闲纯棉短袖女装T恤春夏新款');
    expect(product.price).toBe(19.9);
    expect(product.raw.priceMax).toBe(29.9);
    expect(product.skus).toHaveLength(1);
    expect(product.skus[0]).toMatchObject({
      properties: { 规格: '黑色 / M' },
      price: 21.9,
      stock: 18,
    });
    expect(product.attributes).toEqual({ 材质: '纯棉' });
    expect(product.mainImages).toHaveLength(3);
    expect(product.descriptionImages).toContain('https://img.pddpic.com/mms-material-img/detail-1.jpg');
    expect(quality.ok).toBe(true);
  });

  it('keeps a product with a public price but no images explicitly partial', () => {
    const product = assemblePifaWholesaleProduct(makePayload({
      mainImageCandidates: [],
      detailImageCandidates: [],
      unknownImageCandidates: [],
      skuRows: [],
      attributes: {},
      introTexts: [],
      introFound: false,
    }));

    expect(product.price).toBe(19.9);
    expect(product.mainImages).toEqual([]);
    expect(product.raw.warnings).toContain('no_main_images');
    expect(validateWholesaleCollectQuality(product)).toMatchObject({ ok: true, partial: true });
  });
});
