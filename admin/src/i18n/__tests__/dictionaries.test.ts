import { describe, expect, it } from 'vitest';
import routes from '../../../config/routes';
import { supportedLanguages, translateMessage, hasTranslation, tenantBusinessTranslations } from '../dictionaries';
import { interfaceTranslations } from '../interfaceTranslations';
import { loginTranslations } from '../loginTranslations';
import ts from 'typescript';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

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
  it.each(['constructor', 'toString', '__proto__'])('does not render inherited object members for unknown key %s', source => {
    expect(translateMessage('en-US', source)).toBe(source);
    expect(hasTranslation('en-US', source)).toBe(false);
  });
});

describe('organization business settings dictionary', () => {
  it('defines each translated message for every supported target locale', () => {
    for (const [key, translations] of Object.entries({...tenantBusinessTranslations, ...interfaceTranslations, ...loginTranslations})) {
      expect(translations).toHaveLength(5);
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        expect(hasTranslation(language, key)).toBe(true);
        expect(translateMessage(language, key)).toBeTruthy();
        expect([...translateMessage(language, key).matchAll(/\{\w+\}/g)].map(match=>match[0]).sort()).toEqual([...key.matchAll(/\{\w+\}/g)].map(match=>match[0]).sort());
      }
    }
  });
});

it('has all five translations for every literal tr call, including shared utilities', () => {
  const missing: string[] = [];
  function walk(directory: string) {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) { if (!['__tests__', '__mocks__', 'test'].includes(entry.name)) walk(path); continue; }
      if (!/\.[jt]sx?$/.test(path) || /\.(?:test|spec)\.|\.d\.ts$/.test(path)) continue;
      const source = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true);
      function visit(node: ts.Node) {
        if (ts.isCallExpression(node) && node.expression.getText(source) === 'tr') {
          const key = node.arguments[0];
          if (key && ts.isStringLiteralLike(key)) {
            for (const language of supportedLanguages) {
              if (!hasTranslation(language, key.text)) missing.push(`${language}: ${key.text}`);
            }
          }
        }
        ts.forEachChild(node, visit);
      }
      visit(source);
    }
  }
  const root = existsSync(join(process.cwd(), 'admin', 'src')) ? join(process.cwd(), 'admin', 'src') : join(process.cwd(), 'src');
  walk(root);
  expect([...new Set(missing)]).toEqual([]);
});
