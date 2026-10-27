import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isUntranslatedUIString, parseAddedSourceLines } from './i18n-audit.mjs';

test('audits visible labels and messages without treating translations as raw Chinese', () => {
  assert.equal(isUntranslatedUIString('  label="保存设置"'), true);
  assert.equal(isUntranslatedUIString('  <Button>保存设置</Button>'), true);
  assert.equal(isUntranslatedUIString("  message.error('保存失败')"), true);
  assert.equal(isUntranslatedUIString('  label={tr("保存设置")}'), false);
  assert.equal(isUntranslatedUIString("  message.error(tr('保存失败'))"), false);
  assert.equal(isUntranslatedUIString('  // label="保存设置"'), false);
  assert.equal(isUntranslatedUIString('  // <Button>保存设置</Button>'), false);
  assert.equal(isUntranslatedUIString("  const apiCode = '错误码';"), false);
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
