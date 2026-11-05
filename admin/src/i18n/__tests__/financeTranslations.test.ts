import { describe, expect, it } from 'vitest';
import { financeTranslations } from '../financeTranslations';

describe('finance translations', () => {
  it('provides five localized values for every finance message', () => {
    for (const [source, values] of Object.entries(financeTranslations)) {
      expect(values, source).toHaveLength(5);
      expect(values.every((value) => value.trim() !== ''), source).toBe(true);
      expect(values.some((value) => value !== source), source).toBe(true);
      expect([...values[0].matchAll(/\{\w+\}/g)].map((match) => match[0]).sort()).toEqual(
        [...source.matchAll(/\{\w+\}/g)].map((match) => match[0]).sort(),
      );
    }
  });
});
