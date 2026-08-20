import { describe, expect, it } from 'vitest';
import { canReceivePurchaseOrder, formatMinorAmount, parseMajorAmountToMinor, remainingQuantity } from './helpers';

describe('procurement helpers', () => {
  it('converts decimal major amounts without floating point arithmetic', () => {
    expect(parseMajorAmountToMinor('12.34')).toBe(1234);
    expect(parseMajorAmountToMinor('12.3')).toBe(1230);
    expect(parseMajorAmountToMinor(12)).toBe(1200);
    expect(() => parseMajorAmountToMinor('12.345')).toThrow('金额最多保留两位小数');
  });

  it('formats minor amounts and protects remaining quantity', () => {
    expect(formatMinorAmount(1234, 'cny')).toBe('CNY 12.34');
    expect(formatMinorAmount(-5, 'usd')).toBe('USD -0.05');
    expect(remainingQuantity(5, 2)).toBe(3);
    expect(remainingQuantity(2, 5)).toBe(0);
  });

  it('allows receipts only for approved states', () => {
    expect(canReceivePurchaseOrder('approved')).toBe(true);
    expect(canReceivePurchaseOrder('partially_received')).toBe(true);
    expect(canReceivePurchaseOrder('received')).toBe(false);
  });
});
