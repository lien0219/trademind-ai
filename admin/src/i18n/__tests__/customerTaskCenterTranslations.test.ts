import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import ts from 'typescript';
import { hasTranslation, supportedLanguages } from '../dictionaries';
import { customerTaskCenterTranslations } from '../customerTaskCenterTranslations';

const targetRoots = [
  join(process.cwd(), 'src/pages/Customer'),
  join(process.cwd(), 'src/pages/TaskCenter/Alerts'),
  join(process.cwd(), 'src/pages/TaskCenter/Failures'),
  join(process.cwd(), 'src/pages/TaskCenter/OperationTasks'),
];

function collectTranslationSources() {
  const keys = new Set<string>();

  function walk(directory: string) {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        if (entry.name !== '__tests__') walk(path);
        continue;
      }
      if (!/\.[jt]sx?$/.test(path) || /\.(?:test|spec)\./.test(path) || /\.d\.ts$/.test(path)) continue;

      const source = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true);
      function visit(node: ts.Node) {
        if (ts.isCallExpression(node) && ['tr', 'trFormat'].includes(node.expression.getText(source))) {
          const argument = node.arguments[0];
          if (argument && ts.isStringLiteralLike(argument)) keys.add(argument.text);
        }
        ts.forEachChild(node, visit);
      }
      visit(source);
    }
  }

  for (const root of targetRoots) walk(root);
  return keys;
}

const customerTaskCenterMessages = [
  '自动回复策略加载失败',
  'AI 自动回复已生效',
  '策略已保存',
  '策略保存失败',
  '自动回复运行设置已生效',
  '运行设置已保存',
  '运行设置保存失败',
  '请先开启自动同步客服消息',
  '业务告警加载失败',
  '默认规格',
  '批准说明',
  '拒绝原因',
  'JSON 格式不正确',
  '平台草稿任务已创建，冻结快照等待人工审核。',
  '运营任务已创建，等待准备草稿。',
] as const;

describe('Customer and Task Center translations', () => {
  it('provides non-empty translations with matching placeholders', () => {
    const placeholders = (value: string) => [...value.matchAll(/\{\w+\}/g)].map(([placeholder]) => placeholder).sort();
    for (const [source, values] of Object.entries(customerTaskCenterTranslations)) {
      expect(values, source).toHaveLength(5);
      for (const value of values) {
        expect(value.trim(), source).not.toBe('');
        expect(placeholders(value), source).toEqual(placeholders(source));
      }
    }
  });

  it('registers every migrated message for all supported locales', () => {
    for (const source of customerTaskCenterMessages) {
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
      }
    }
  });

  it('covers every literal translation call in Customer and Task Center pages', () => {
    const uncovered: string[] = [];
    for (const source of collectTranslationSources()) {
      for (const language of supportedLanguages) {
        if (!hasTranslation(language, source)) uncovered.push(`${language}: ${source}`);
      }
    }
    expect([...new Set(uncovered.map((entry) => entry.slice(entry.indexOf(': ') + 2)))]).toEqual([]);
  });
});
