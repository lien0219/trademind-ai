import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { basename, join } from 'node:path';
import { isUntranslatedUIString, parseAddedSourceLines, sourcePaths, inventory } from './i18n-audit.mjs';

test('audits visible labels and messages without treating translations as raw Chinese', () => {
  assert.equal(isUntranslatedUIString('  label="保存设置"'), true);
  assert.equal(isUntranslatedUIString('  <Button>保存设置</Button>'), true);
  assert.equal(isUntranslatedUIString("  message.error('保存失败')"), true);
  assert.equal(isUntranslatedUIString('  label={tr("保存设置")}'), false);
  assert.equal(isUntranslatedUIString("  message.error(tr('保存失败'))"), false);
  assert.equal(isUntranslatedUIString('  // label="保存设置"'), false);
  assert.equal(isUntranslatedUIString('  // <Button>保存设置</Button>'), false);
  assert.equal(isUntranslatedUIString("  const apiCode = '错误码';"), false);
  assert.equal(isUntranslatedUIString('title={trFormat("删除企业：{name}", { name })}'), false);
  assert.equal(isUntranslatedUIString('title={tr("标题", language)}'), false);
  assert.equal(isUntranslatedUIString('title={trFormat("标题", { description: "未翻译说明" })}'), true);
  assert.equal(isUntranslatedUIString('<Button>{tr("保存")}</Button><Button>取消</Button>'), true);
  assert.equal(isUntranslatedUIString("warnings.some((w) => w.includes('原文字'));"), false);
  assert.equal(isUntranslatedUIString("if (value.length > 72) return '密码长度须为 12–72 位';"), false);
  assert.equal(isUntranslatedUIString('<span className="tm-app-brand-header__name-primary">贸灵</span>'), false);
});

test('inventory includes repository source and new files, but not ignored generated copies', () => {
  const paths = sourcePaths();
  assert.ok(paths.includes('admin/src/app.tsx'));
  assert.equal(paths.some(path => path.startsWith('admin/src/.umi/')), false);
  const full = inventory(paths, true);
  assert.equal(full.candidates.length, full.unmigratedTextSites);
  assert.equal(Object.values(full.byModule).reduce((sum, count) => sum + count, 0), full.unmigratedTextSites);
});

test('does not count Chinese source keys inside dedicated locale resources as untranslated UI', () => {
  assert.equal(inventory(['admin/src/i18n/siteTranslations.ts'], true).unmigratedTextSites, 0);
  assert.equal(inventory(['admin/src/pages/Product/Drafts/index.tsx'], true).unmigratedTextSites, 0);
});

test('new untranslated application files remain visible before they are staged', () => {
  const directory = mkdtempSync(fileURLToPath(new URL('../src/audit-candidate-', import.meta.url)));
  const path = join(directory, 'Candidate.tsx');
  try {
    writeFileSync(path, '<Button title="尚未迁移的真实文案" />;');
    const paths = sourcePaths();
    const candidate = paths.find(item => item.endsWith(`${basename(directory)}/Candidate.tsx`));
    assert.ok(candidate);
    assert.equal(inventory([candidate], true).unmigratedTextSites, 1);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('strict inventory detects raw Chinese text inside JSX expressions', () => {
  const directory = mkdtempSync(fileURLToPath(new URL('../src/audit-jsx-', import.meta.url)));
  const path = join(directory, 'Dynamic.tsx');
  try {
    writeFileSync(path, 'function Candidate() { return <span>{ready ? "未本地化文案" : tr("已翻译文案")}</span>; }');
    const candidate = sourcePaths().find(item => item.endsWith(`${basename(directory)}/Dynamic.tsx`));
    assert.ok(candidate);
    const result = inventory([candidate], true);
    assert.equal(result.unmigratedTextSites, 1);
    assert.equal(result.candidates[0].sample, '未本地化文案');
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('strict inventory recognizes the language-subscribed SettingsMessage source', () => {
  const directory = mkdtempSync(fileURLToPath(new URL('../src/audit-settings-message-', import.meta.url)));
  const path = join(directory, 'Candidate.tsx');
  try {
    writeFileSync(path, '<SettingsMessage source={"已翻译的确认标题"} />;');
    const candidate = sourcePaths().find(item => item.endsWith(`${basename(directory)}/Candidate.tsx`));
    assert.ok(candidate);
    assert.equal(inventory([candidate], true).unmigratedTextSites, 0);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('strict inventory detects interpolated JSX labels and table render copy', () => {
  const directory = mkdtempSync(fileURLToPath(new URL('../src/audit-template-', import.meta.url)));
  const path = join(directory, 'Candidate.tsx');
  try {
    writeFileSync(path, [
      'function Candidate() { return <Modal title={ready ? `编辑供应商 · ${supplier.code}` : tr("新建供应商")} />; }',
      'function Count() { return <span>{`${count} 项`}</span>; }',
      'const columns = [{ render: (_, row) => `${row.days} 天` }];',
      'const prompt = `为商品 ${product.title} 生成标题`;',
    ].join('\n'));
    const candidate = sourcePaths().find(item => item.endsWith(`${basename(directory)}/Candidate.tsx`));
    assert.ok(candidate);
    assert.equal(inventory([candidate], true).unmigratedTextSites, 3);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('strict inventory detects error guidance and translations frozen during module initialization', () => {
  const directory = mkdtempSync(fileURLToPath(new URL('../src/audit-eager-', import.meta.url)));
  const path = join(directory, 'Candidate.tsx');
  try {
    writeFileSync(path, [
      'const error = { detail: "请重新授权店铺", action: "前往店铺管理" };',
      'const frozen = { label: tr("库存") };',
      'const live = { get label() { return tr("库存"); } };',
    ].join('\n'));
    const candidate = sourcePaths().find(item => item.endsWith(`${basename(directory)}/Candidate.tsx`));
    assert.ok(candidate);
    assert.equal(inventory([candidate], true).unmigratedTextSites, 3);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('strict whole-app inventory contains no untranslated user-facing Chinese', () => {
  const result = inventory(sourcePaths());
  assert.equal(result.unmigratedTextSites, 0, JSON.stringify(result.byModule));
});

test('diff audit considers only new or modified source and ignores deletions', () => {
  const patch = [
    'diff --git a/admin/src/pages/X.tsx b/admin/src/pages/X.tsx',
    '--- a/admin/src/pages/X.tsx',
    '+++ b/admin/src/pages/X.tsx',
    '@@ -1 +1,2 @@',
    '-  label="已删除的标题"',
    '+  label="新增标题"',
    '+  label={tr("翻译后的标题")}',
    'diff --git a/admin/src/__tests__/X.test.tsx b/admin/src/__tests__/X.test.tsx',
    '+++ b/admin/src/__tests__/X.test.tsx',
    '+  label="测试说明"',
    'diff --git a/admin/src/pages/Y.tsx b/admin/src/pages/Y.tsx',
    '+++ b/admin/src/pages/Y.tsx',
    '+  <Button>新增操作</Button>',
  ].join('\n');
  assert.deepEqual(parseAddedSourceLines(patch).map(x=>x.line), [
    '  label="新增标题"',
    '  label={tr("翻译后的标题")}',
    '  <Button>新增操作</Button>',
  ]);
});
