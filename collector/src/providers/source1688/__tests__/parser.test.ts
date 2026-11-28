// Synthetic parser fixture; this is not a captured live platform snapshot.
import { describe, expect, it } from 'vitest';
import { assembleParsedProduct } from '../parser.js';
import type { BrowserExtractPayload } from '../types.js';

function makePayload(overrides: Partial<BrowserExtractPayload> = {}): BrowserExtractPayload {
  return {
    finalUrl: 'https://detail.1688.com/offer/123456.html',
    docTitle: '测试商品详情',
    meta: {},
    headingText: '多颜色不锈钢更衣柜商品标题',
    galleryUrls: [],
    detailUrls: [],
    domPriceTexts: [],
    paramPairs: [],
    domSkuDimensions: [],
    domSkuTableRows: [],
    scriptSnippets: [],
    ...overrides,
  };
}

describe('1688 parser regressions', () => {
  it('does not infer product price from weight fields', () => {
    const product = assembleParsedProduct(
      'https://detail.1688.com/offer/123456.html',
      makePayload({
        paramPairs: [{ key: 'unitWeight', value: '39' }],
        scriptSnippets: [JSON.stringify({ unitWeight: 39 })],
      }),
    );

    expect(product.raw.productPrice).toBeUndefined();
    expect(product.skus.some((sku) => sku.price === 39)).toBe(false);
  });

  it('keeps color and millimeter size dimensions separate and maps each row price', () => {
    const product = assembleParsedProduct(
      'https://detail.1688.com/offer/123456.html',
      makePayload({
        domSkuDimensions: [
          { name: '颜色', values: ['黑色', '白色'] },
          { name: '尺寸', values: ['1.2mm', '1.4mm'] },
        ],
        domSkuTableRows: [
          { label: '1.2mm', priceText: '¥790', stockText: '库存299件' },
          { label: '1.4mm', priceText: '¥860', stockText: '库存199件' },
        ],
      }),
    );

    expect(product.skus).toHaveLength(4);
    expect(product.skus.map((sku) => sku.properties)).toEqual([
      { 颜色: '黑色', 尺寸: '1.2mm' },
      { 颜色: '黑色', 尺寸: '1.4mm' },
      { 颜色: '白色', 尺寸: '1.2mm' },
      { 颜色: '白色', 尺寸: '1.4mm' },
    ]);
    expect(product.skus.map((sku) => sku.price)).toEqual([790, 860, 790, 860]);
    expect(product.skus.map((sku) => sku.stock)).toEqual([299, 199, 299, 199]);
    expect(product.raw.domSkuTableRowCount).toBe(2);
    expect(product.raw.collectStatus).toBe('partial_success');
    expect(product.raw.missingFields).toContain('mainImages');
    expect(product.skus.every((sku) => !Object.values(sku.properties ?? {}).some((value) => /¥|库存/.test(value)))).toBe(true);
  });
});
