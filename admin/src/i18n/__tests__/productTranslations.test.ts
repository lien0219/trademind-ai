import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import ts from 'typescript';
import { productTranslations } from '../productTranslations';
import { productGapTranslations } from '../productGapTranslations';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';

const productPagesRoot = join(process.cwd(), 'src/pages/Product');
const translationsPath = join(process.cwd(), 'src/i18n/productTranslations.ts');
const placeholderNames = (value: string) => [...value.matchAll(/\{\w+\}/g)].map(([name]) => name).sort();

function collectProductCallSources() {
  const keys = new Set<string>();

  function walk(directory: string) {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        walk(path);
        continue;
      }
      if (!/\.[jt]sx?$/.test(path) || /\.(?:test|spec)\./.test(path) || /\.d\.ts$/.test(path)) continue;

      const source = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true);
      function visit(node: ts.Node) {
        if (
          ts.isCallExpression(node) &&
          ['tr', 'trFormat'].includes(node.expression.getText(source))
        ) {
          const argument = node.arguments[0];
          if (argument && ts.isStringLiteralLike(argument)) keys.add(argument.text);
        }
        ts.forEachChild(node, visit);
      }
      visit(source);
    }
  }

  walk(productPagesRoot);
  return keys;
}

function readResourcePropertyNames() {
  const file = ts.createSourceFile(
    translationsPath,
    readFileSync(translationsPath, 'utf8'),
    ts.ScriptTarget.Latest,
    true,
  );
  let resource: ts.ObjectLiteralExpression | undefined;

  function visit(node: ts.Node) {
    if (
      ts.isVariableDeclaration(node) &&
      ts.isIdentifier(node.name) &&
      node.name.text === 'productTranslations' &&
      node.initializer &&
      ts.isObjectLiteralExpression(node.initializer)
    ) {
      resource = node.initializer;
    }
    ts.forEachChild(node, visit);
  }

  visit(file);
  expect(resource, 'productTranslations object literal').toBeDefined();
  const names = (resource as ts.ObjectLiteralExpression).properties.flatMap((property) => {
    if (!ts.isPropertyAssignment(property) || !ts.isStringLiteralLike(property.name)) return [];
    return [property.name.text];
  });
  return names;
}

describe('product translations', () => {
  it('keeps supplemental Product translations complete and unique', () => {
    const sources = Object.keys(productGapTranslations);
    expect(new Set(sources).size).toBe(sources.length);
    expect(sources.filter((source) => Object.hasOwn(productTranslations, source))).toEqual([]);

    for (const [source, values] of Object.entries(productGapTranslations)) {
      expect(values, source).toHaveLength(5);
      for (const value of values) {
        expect(value.trim(), source).not.toBe('');
        expect(placeholderNames(value), source).toEqual(placeholderNames(source));
      }
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        const translationIndex = supportedLanguages.indexOf(language) - 1;
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
        expect(translateMessage(language, source)).toBe(values[translationIndex]);
      }
    }
  });

  it('provides five non-empty locale values with matching placeholders', () => {
    for (const [source, values] of Object.entries(productTranslations)) {
      expect(values, source).toHaveLength(5);
      for (const value of values) {
        expect(value.trim(), source).not.toBe('');
        expect(placeholderNames(value), source).toEqual(placeholderNames(source));
      }
    }
  });

  it('registers each Product message in the runtime dictionary', () => {
    for (const [source, values] of Object.entries(productTranslations)) {
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        const translationIndex = supportedLanguages.indexOf(language) - 1;
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
        expect(translateMessage(language, source)).toBe(values[translationIndex]);
      }
    }
  });

  it('contains unique Product resource keys and resolves every Product translation call', () => {
    const keys = readResourcePropertyNames();
    const productSources = collectProductCallSources();
    const duplicates = keys.filter((key, index) => keys.indexOf(key) !== index);
    const missingRuntimeEntries = [...productSources].flatMap((source) =>
      supportedLanguages
        .filter((language) => !hasTranslation(language, source))
        .map((language) => `${language}: ${source}`),
    );

    expect(duplicates).toEqual([]);
    expect(productSources.size).toBeGreaterThan(0);
    expect(missingRuntimeEntries).toEqual([]);
  });
});
