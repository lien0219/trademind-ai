import { describe, expect, it } from 'vitest';
import { isPinduoduoCollectSupportedUrl } from '../pinduoduoUrl';

describe('Pinduoduo collect URL support', () => {
  it('allows only the currently implemented wholesale detail URL shape', () => {
    expect(isPinduoduoCollectSupportedUrl('https://pifa.pinduoduo.com/goods/detail/?gid=123')).toBe(true);
    expect(isPinduoduoCollectSupportedUrl('https://mobile.yangkeduo.com/goods.html?goods_id=123')).toBe(false);
    expect(isPinduoduoCollectSupportedUrl('https://pifa.pinduoduo.com/')).toBe(false);
  });
});
