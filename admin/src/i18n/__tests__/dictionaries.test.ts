import { describe, expect, it } from 'vitest';
import routes from '../../../config/routes';
import { supportedLanguages, translateMessage, hasTranslation } from '../dictionaries';

function collectNames(items: any[]): string[] {
  const out: string[] = [];
  for (const item of items) {
    if (typeof item.name === 'string') out.push(item.name);
    out.push(...collectNames(item.routes || []));
  }
  return out;
}

describe('six-language navigation coverage', () => {
  const routeNames = collectNames(routes as any[]);
  it('translates every public route label in all non-default locales', () => {
    const missing: string[] = [];
    for (const language of supportedLanguages) {
      if (language === 'zh-CN') continue;
      for (const name of routeNames) {
        if (!hasTranslation(language, name)) missing.push(language + ': ' + name);
      }
    }
    expect(missing).toEqual([]);
  });
  it('preserves source key as a safe fallback', () => {
    expect(translateMessage('zh-CN', '未翻译的内部术语')).toBe('未翻译的内部术语');
  });
});
