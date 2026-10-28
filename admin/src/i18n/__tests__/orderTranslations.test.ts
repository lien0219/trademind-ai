import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import ts from 'typescript';
import { orderTranslations } from '../orderTranslations';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';

describe('order translations', () => {
  it('provides five usable translations for every order message', () => {
    for (const [source, values] of Object.entries(orderTranslations)) {
      expect(values, source).toHaveLength(5);
      for (const value of values) {
        expect(value.trim(), source).not.toBe('');
        expect(value, source).not.toBe('Order management message');
        expect(value, source).not.toMatch(/\\u[0-9a-f]{4}/i);
        expect([...value.matchAll(/\{\w+\}/g)].map(match => match[0]).sort()).toEqual(
          [...source.matchAll(/\{\w+\}/g)].map(match => match[0]).sort(),
        );
      }
    }
  });

  it('returns localized text through the runtime dictionary for all target locales', () => {
    const source = '订单详情';
    for (const language of supportedLanguages) {
      if (language === 'zh-CN') continue;
      expect(translateMessage(language, source)).not.toBe(source);
    }
  });

  it('covers every literal translation call in the Orders pages', () => {
    const uncovered: string[] = [];
    const root = join(process.cwd(), 'src/pages/Orders');

    function walk(directory: string) {
      for (const entry of readdirSync(directory, { withFileTypes: true })) {
        const path = join(directory, entry.name);
        if (entry.isDirectory()) {
          walk(path);
          continue;
        }
        if (!/\.[jt]sx?$/.test(path) || /\.(?:test|spec)\./.test(path)) continue;

        const source = ts.createSourceFile(
          path,
          readFileSync(path, 'utf8'),
          ts.ScriptTarget.Latest,
          true,
        );
        function visit(node: ts.Node) {
          if (
            ts.isCallExpression(node) &&
            ['tr', 'trFormat'].includes(node.expression.getText(source))
          ) {
            const key = node.arguments[0];
            if (key && ts.isStringLiteralLike(key)) {
              for (const language of supportedLanguages) {
                if (!hasTranslation(language, key.text)) {
                  uncovered.push(`${language}: ${key.text}`);
                }
              }
            }
          }
          ts.forEachChild(node, visit);
        }
        visit(source);
      }
    }

    walk(root);
    expect([...new Set(uncovered)]).toEqual([]);
  });
});
