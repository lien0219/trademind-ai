import { describe, expect, it } from 'vitest';
import { preferredFXCurrency } from '../fxReport';

describe('preferredFXCurrency', () => {
  it('accepts valid enterprise currencies and extended report currencies', () => {
    expect(preferredFXCurrency('CNY')).toBe('CNY');
    expect(preferredFXCurrency('JPY')).toBe('JPY');
    expect(preferredFXCurrency('EUR')).toBe('EUR');
  });
  it('rejects missing and unsupported settings', () => {
    expect(preferredFXCurrency(undefined)).toBeNull();
    expect(preferredFXCurrency('usd')).toBeNull();
    expect(preferredFXCurrency('INVALID')).toBeNull();
    expect(preferredFXCurrency(1)).toBeNull();
  });
});
