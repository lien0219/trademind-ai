import { describe, expect, it } from 'vitest';
import routes from '../../../config/routes';
import { supportedLanguages, translateMessage, hasTranslation, tenantBusinessTranslations } from '../dictionaries';
import { interfaceTranslations } from '../interfaceTranslations';
import { loginTranslations } from '../loginTranslations';
import { tenantMemberTranslations } from '../tenantMemberTranslations';
import { tenantInvitationTranslations } from '../tenantInvitationTranslations';
import { tenantSettingsTranslations } from '../tenantSettingsTranslations';
import { tenantPreferenceTranslations } from '../tenantPreferenceTranslations';
import { platformTenantTranslations } from '../platformTenantTranslations';
import { commonControlTranslations } from '../commonControlTranslations';
import { workspaceTranslations } from '../workspaceTranslations';
import { aiTaskTranslations } from '../aiTaskTranslations';
import { aiPromptTranslations } from '../aiPromptTranslations';
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
    for (const [key, translations] of Object.entries({...tenantBusinessTranslations, ...interfaceTranslations, ...loginTranslations, ...tenantMemberTranslations, ...tenantInvitationTranslations, ...tenantSettingsTranslations, ...tenantPreferenceTranslations, ...platformTenantTranslations, ...commonControlTranslations, ...workspaceTranslations, ...aiTaskTranslations, ...aiPromptTranslations})) {
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

it('registers remaining inventory, operations, returns, task, worker, and publish copy', () => {
  const sources = [
    '调整库存', '预警线', '计算中…', '盘点详情', '详情', '未命名商品',
    '每次最多迁移 100 个尚未建立仓库余额的规格。优先使用启用的默认仓；没有默认仓时写入“待分配仓”。该操作不调用平台接口，也不创建后台任务进程。',
    '编辑 SKU 绑定', '新增 SKU 绑定', '调拨详情',
    '正在加载绑定历史', '暂无校准说明', '确认人工绑定', '拒绝人工绑定', '确认绑定', '确认拒绝',
    '正在加载库存同步运行', '未知', '内部端点已保护', '端点未限制内部访问', '未配置路径', '未配置协议',
    '店铺授权异常', '等待状态数据', '尚未完成健康检查', '已开启', '已关闭',
    '售后详情', '售后操作', '确认', '平台售后事实详情', '已应用', '已忽略', '已按平台事实确认退款结果',
    '退款执行单已取消', '退款执行详情', '平台事实', '人工登记', '需要人工核对平台事实', '按平台事实确认',
    '取消退款执行单', '确认平台结果', '确认取消',
    '系统将校验平台事实与售后单的关联、金额、币种和终态后更新本地结果，不会调用平台写接口。',
    '取消后不能再登记结果；如外部退款已经发生，请不要取消。',
    '忽略此失败任务（列表默认隐藏）', '标记为已线下处理（列表默认隐藏）',
    '将通过 {value0} 确认平台侧是否已创建草稿，不会盲目重复提交 {value1}。',
    '后台任务监控', '查看采集、图片处理、订单同步、商品刊登等后台任务是否在正常运行',
    '展示各类型后台任务进程状态与正在执行中的任务（每 {value0} 秒刷新；页面隐藏时暂停）。',
    '店铺未授权', '（本地草稿）',
  ];
  const placeholders = (value: string) => [...value.matchAll(/\{\w+\}/g)].map(([name]) => name).sort();
  for (const source of sources) {
    for (const language of supportedLanguages) {
      if (language === 'zh-CN') continue;
      const translated = translateMessage(language, source);
      expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
      expect(translated.trim(), `${language}: ${source}`).not.toBe('');
      // Japanese and Traditional Chinese can legitimately share source glyphs.
      if (['en-US', 'ko-KR', 'th-TH'].includes(language)) {
        expect(translated, `${language}: ${source}`).not.toBe(source);
      }
      expect(placeholders(translated), `${language}: ${source}`).toEqual(placeholders(source));
    }
  }
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
        if (ts.isCallExpression(node) && ['tr', 'trFormat'].includes(node.expression.getText(source))) {
          const key = node.arguments[0];
          if (key && ts.isStringLiteralLike(key)) {
            for (const language of supportedLanguages) {
              if (!hasTranslation(language, key.text)) missing.push(`${language}: ${key.text}`);
            }
          }
        }
        if (ts.isJsxAttribute(node) && node.name.getText(source) === 'source'
          && (ts.isJsxOpeningElement(node.parent.parent) || ts.isJsxSelfClosingElement(node.parent.parent))
          && node.parent.parent.tagName.getText(source) === 'SettingsMessage'
          && node.initializer) {
          const key = ts.isStringLiteralLike(node.initializer) ? node.initializer
            : ts.isJsxExpression(node.initializer) && node.initializer.expression
              && ts.isStringLiteralLike(node.initializer.expression) ? node.initializer.expression : undefined;
          if (!key) return;
          for (const language of supportedLanguages) {
            if (!hasTranslation(language, key.text)) {
              missing.push(`${language}: ${key.text}`);
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

it('keeps translation source keys unique across resources and preserves all interpolation values', () => {
  const root = existsSync(join(process.cwd(), 'admin', 'src')) ? join(process.cwd(), 'admin', 'src') : join(process.cwd(), 'src');
  const directory = join(root, 'i18n');
  const issues: string[] = [];
  const owners = new Map<string, string>();
  const placeholders = (value: string) => [...value.matchAll(/\{\w+\}/g)].map(([name]) => name).sort().join(',');
  for (const filename of readdirSync(directory).filter(name => /Translations\.ts$/.test(name))) {
    const source = ts.createSourceFile(filename, readFileSync(join(directory, filename), 'utf8'), ts.ScriptTarget.Latest, true);
    const keys = new Set<string>();
    function visit(node: ts.Node) {
      if (ts.isPropertyAssignment(node) && ts.isStringLiteralLike(node.name) && ts.isArrayLiteralExpression(node.initializer)) {
        const key = node.name.text;
        if (keys.has(key)) issues.push(`${filename}: duplicate ${key}`);
        keys.add(key);
        if (owners.has(key)) issues.push(`${filename}: ${key} already belongs to ${owners.get(key)}`);
        owners.set(key, filename);
        if (node.initializer.elements.length !== 5) issues.push(`${filename}: incomplete ${key}`);
        for (const value of node.initializer.elements) {
          if (!ts.isStringLiteralLike(value) || !value.text.trim() || placeholders(value.text) !== placeholders(key)) {
            issues.push(`${filename}: invalid translation for ${key}`);
          }
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(source);
  }
  expect(issues).toEqual([]);
});

it('localizes settings comparison labels in Japanese', () => {
  expect(translateMessage('ja-JP', '多出：{value0}')).toBe('余分：{value0}');
  expect(translateMessage('ja-JP', '缺少：{value0}')).toBe('不足：{value0}');
  expect(translateMessage('zh-TW', '已忽略')).toBe('已忽略');
});
