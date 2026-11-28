// Synthetic parser fixture; this is not a captured live platform snapshot.
import { describe, expect, it } from 'vitest';
import type { ProductSku } from '../../../types/product.js';
import { mergeTaobaoPayload } from '../json-extract.js';
import { validateTaobaoCollectQuality } from '../quality.js';
import { resolvePriceFromPayload, type TaobaoPagePayload } from '../page-extract.js';

function makePayload(overrides: Partial<TaobaoPagePayload> = {}): TaobaoPagePayload {
  return {
    title: '测试商品标题',
    originalTitle: '测试商品标题',
    priceText: '¥1,299.50',
    priceRange: '¥1,299.50 - ¥1,499.50',
    shopName: '测试店铺',
    mainImages: ['https://img.example.com/main.jpg'],
    descriptionImages: ['https://img.example.com/detail.jpg'],
    attributes: { 材质: '棉' },
    skuGroups: [{ name: '颜色', options: [{ label: '黑色', selected: true, disabled: false }] }],
    skus: [{ properties: { 颜色: '黑色' }, price: 1299.5 }],
    debug: {},
    ...overrides,
  };
}

describe('Taobao/Tmall parser regressions', () => {
  it('parses displayed product price while preserving an explicit range', () => {
    expect(resolvePriceFromPayload(makePayload())).toEqual({
      price: 1299.5,
      priceText: '¥1,299.50',
      priceRange: '¥1,299.50 - ¥1,499.50',
    });
  });

  it('merges structured images, attributes, and SKU data without duplicate images', () => {
    const dom = makePayload();
    const jsonSku: ProductSku = { properties: { 颜色: '白色' }, price: 1399.5, stock: 12 };
    const merged = mergeTaobaoPayload(dom, {
      mainImages: ['https://img.example.com/main.jpg', 'https://img.example.com/alternate.jpg'],
      descriptionImages: ['https://img.example.com/detail.jpg', 'https://img.example.com/detail-2.jpg'],
      attributes: { 产地: '中国' },
      skuGroups: [
        { name: '颜色', options: [{ label: '黑色', selected: false, disabled: false }, { label: '白色', selected: true, disabled: false }] },
      ],
      skus: [jsonSku],
      debug: { jsonSkuCount: 1 },
    });

    expect(merged.mainImages).toEqual(['https://img.example.com/main.jpg', 'https://img.example.com/alternate.jpg']);
    expect(merged.descriptionImages).toEqual(['https://img.example.com/detail.jpg', 'https://img.example.com/detail-2.jpg']);
    expect(merged.attributes).toEqual({ 材质: '棉', 产地: '中国' });
    expect(merged.skuGroups[0]?.options).toHaveLength(2);
    expect(merged.skus).toEqual([jsonSku]);
  });

  it('rejects missing main images instead of reporting a complete result', () => {
    const result = validateTaobaoCollectQuality({
      title: '测试商品标题',
      originalTitle: '测试商品标题',
      price: 10,
      priceText: '10',
      priceRange: '10',
      priceSource: 'page_display',
      currency: 'CNY',
      shopName: '',
      shopUrl: '',
      sellerName: '',
      mainImages: [],
      descriptionImages: [],
      attributes: {},
      skuGroups: [],
      skus: [],
      warnings: [],
      raw: {},
    });

    expect(result).toMatchObject({ ok: false, partial: false, error: 'MAIN_IMAGES_EMPTY:no_main_images' });
  });
});
