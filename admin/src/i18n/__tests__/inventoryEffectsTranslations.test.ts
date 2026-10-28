import { describe, expect, it } from 'vitest';
import { inventoryEffectsTranslations } from '../inventoryEffectsTranslations';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';

const inventoryEffectsPageMessages = [
  '时间范围',
  '创建时间',
  '订单 ID',
  '订单号',
  '影响类型',
  '状态',
  '数量',
  '商品规格 ID',
  '错误',
  '订单库存影响',
  '查看订单库存的预占、出库、释放与回补记录。',
] as const;

describe('inventory effects translations', () => {
  it('provides five usable translations for every inventory effects message', () => {
    for (const [source, values] of Object.entries(inventoryEffectsTranslations)) {
      expect(values, source).toHaveLength(5);
      expect(values.every((value) => value.trim() !== ''), source).toBe(true);
      expect(values[0], source).not.toBe(source);
    }
  });

  it('returns localized text through the runtime dictionary for all target locales', () => {
    for (const source of Object.keys(inventoryEffectsTranslations)) {
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        const translationIndex = supportedLanguages.indexOf(language) - 1;
        expect(translateMessage(language, source)).toBe(inventoryEffectsTranslations[source][translationIndex]);
      }
    }
  });

  it('registers every visible page message for all supported locales', () => {
    for (const language of supportedLanguages) {
      for (const source of inventoryEffectsPageMessages) {
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
      }
    }
  });
});
