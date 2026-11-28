import { describe, expect, it } from 'vitest';
import {
  unsupportedPinduoduoUrlMessage,
  wholesaleLoginSuggestion,
  wholesaleUrlHint,
} from '../url-type.js';

describe('Pinduoduo access guidance', () => {
  it('marks ordinary mobile consumer detail pages as unsupported', () => {
    expect(unsupportedPinduoduoUrlMessage('goods_detail')).toContain('当前未支持');
  });

  it('keeps login recovery within the implemented wholesale detail scope', () => {
    expect(wholesaleLoginSuggestion()).toContain('批发商品详情页');
    expect(wholesaleLoginSuggestion()).not.toContain('普通商品');
    expect(wholesaleUrlHint()).not.toContain('普通商品详情页');
  });
});
