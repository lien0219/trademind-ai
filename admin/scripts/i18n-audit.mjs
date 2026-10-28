#!/usr/bin/env node
// Internationalization migration debt inventory and changed-line regression gate.
// This scanner is intentionally a heuristic, not proof of complete localization.
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import ts from 'typescript';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const repoRoot = resolve(root, '..', '..');
const exclude = /(?:^|\/)(?:__tests__|__mocks__)(?:\/|$)|\.test\.|\.spec\.|\.d\.ts$|\.generated\.|\/i18n\/(?:[^/]+Translations|dictionaries)\.ts$/;
const sourceFile = /\.[jt]sx?$/;
const chinese = /[\u3400-\u9fff]/;
const userFacing = /\b(?:label|title|subTitle|placeholder|message|description|text|tooltip|aria-label|okText|cancelText|name)\s*[:=]\s*['"`]|message\.(?:success|error|warning|info)\s*\(|<[A-Za-z][^>]*>\s*[^<{]*[\u3400-\u9fff]/;
const userFacingProperties = new Set([
  'label', 'title', 'subTitle', 'placeholder', 'message', 'description', 'text', 'tooltip',
  'aria-label', 'okText', 'cancelText', 'content', 'extra', 'help', 'tip', 'hint',
  'render', 'renderText',
  'emptyText', 'enterButton', 'suffix', 'prefix', 'addonBefore', 'addonAfter',
  'checkedChildren', 'unCheckedChildren', 'headerTitle', 'actionHint', 'detail', 'action',
]);

export function isUntranslatedUIString(line) {
  if (!chinese.test(line) || /^\s*\/\//.test(line)) return false;
  // A translated string key is not itself a missing translation. Do not hide
  // other literals elsewhere on the same line.
  const withoutTranslatedKeys = line.replace(/\b(?:tr|trFormat)\(\s*(['"`])(?:\\.|(?!\1).)*?\1/g, '');
  const withoutStableBrand = withoutTranslatedKeys.replace(/<span className="tm-app-brand-header__name-primary">贸灵<\/span>/g, '');
  return chinese.test(withoutStableBrand) && userFacing.test(withoutStableBrand);
}

function trackedSource(path) {
  return path.startsWith('admin/src/') && sourceFile.test(path) && !exclude.test(path);
}

export function parseAddedSourceLines(diff) {
  const added = [];
  let path = '';
  for (const line of diff.split(/\r?\n/)) {
    if (line.startsWith('+++ b/')) {
      path = line.slice(6);
    } else if (line.startsWith('+++ /dev/null')) {
      path = '';
    } else if (path && trackedSource(path) && line.startsWith('+') && !line.startsWith('+++')) {
      added.push({ file: path.slice('admin/src/'.length), line: line.slice(1) });
    }
  }
  return added;
}

function isTranslationArgument(node) {
  const call = node.parent;
  return ts.isCallExpression(call)
    && ts.isIdentifier(call.expression)
    && (call.expression.text === 'tr' || call.expression.text === 'trFormat')
    && call.arguments[0] === node;
}

function isSettingsMessageSource(node) {
  for (let parent = node.parent; parent; parent = parent.parent) {
    if (ts.isJsxAttribute(parent)) {
      if (parent.name.getText() !== 'source' || !parent.initializer
        || parent.initializer.getStart() > node.getStart() || parent.initializer.getEnd() < node.getEnd()) return false;
      const element = parent.parent?.parent;
      return (ts.isJsxOpeningElement(element) || ts.isJsxSelfClosingElement(element))
        && element.tagName.getText() === 'SettingsMessage';
    }
    if (ts.isJsxElement(parent) || ts.isJsxOpeningElement(parent) || ts.isJsxSelfClosingElement(parent)) return false;
  }
  return false;
}

function isMessageCall(node) {
  const call = node.parent;
  return ts.isCallExpression(call)
    && ts.isPropertyAccessExpression(call.expression)
    && ts.isIdentifier(call.expression.expression)
    && call.expression.expression.text === 'message'
    && ['success', 'error', 'warning', 'info'].includes(call.expression.name.text)
    && call.arguments[0] === node;
}

function isMessageArgumentDescendant(node) {
  for (let parent = node.parent; parent; parent = parent.parent) {
    if (ts.isCallExpression(parent)) {
      return ts.isPropertyAccessExpression(parent.expression)
        && ts.isIdentifier(parent.expression.expression)
        && parent.expression.expression.text === 'message'
        && ['success', 'error', 'warning', 'info'].includes(parent.expression.name.text)
        && parent.arguments[0] !== undefined
        && parent.arguments[0].getStart() <= node.getStart()
        && parent.arguments[0].getEnd() >= node.getEnd();
    }
    if (ts.isJsxExpression(parent)) return false;
  }
  return false;
}

function isDirectJsxExpressionLiteral(node) {
  let current = node;
  for (let parent = node.parent; parent; parent = parent.parent) {
    if (ts.isJsxExpression(parent)) return true;
    if (ts.isCallExpression(parent) || ts.isNewExpression(parent)
      || ts.isArrayLiteralExpression(parent) || ts.isObjectLiteralExpression(parent)
      || ts.isPropertyAssignment(parent) || ts.isElementAccessExpression(parent)
      || ts.isPropertyAccessExpression(parent)) return false;
    if (ts.isConditionalExpression(parent) || ts.isBinaryExpression(parent)
      || ts.isPrefixUnaryExpression(parent) || ts.isParenthesizedExpression(parent)
      || ts.isNonNullExpression(parent) || ts.isAsExpression(parent)
      || ts.isTypeAssertionExpression(parent) || ts.isTemplateExpression(parent)) {
      current = parent;
      continue;
    }
    return false;
  }
  return current !== node;
}

function isUserFacingLiteral(node) {
  if (isSettingsMessageSource(node)) return false;
  if (ts.isJsxAttribute(node.parent) && node.parent.initializer === node) {
    const attributeName = ts.isIdentifier(node.parent.name) ? node.parent.name.text : '';
    return userFacingProperties.has(attributeName);
  }
  if (isDirectJsxExpressionLiteral(node)) return true;
  if (isMessageArgumentDescendant(node)) return true;
  for (let parent = node.parent; parent; parent = parent.parent) {
    if (ts.isPropertyAssignment(parent)) {
      const name = ts.isIdentifier(parent.name) || ts.isStringLiteral(parent.name) ? parent.name.text : '';
      if (userFacingProperties.has(name) && parent.initializer.getStart() <= node.getStart()
        && parent.initializer.getEnd() >= node.getEnd()) return true;
    }
    if (ts.isCallExpression(parent)) return isMessageCall(node);
  }
  return false;
}

function astCandidates(path, source) {
  const sourceFile = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true);
  const gaps = [];
  function visit(node) {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)
      && ['tr', 'trFormat'].includes(node.expression.text)
      && node.arguments.length === (node.expression.text === 'tr' ? 1 : 2)) {
      // Translations must be read during rendering or through a getter. A
      // module-level value would keep its first language after a hot switch.
      let parent = node.parent;
      while (parent && !ts.isFunctionLike(parent) && !ts.isSourceFile(parent)) parent = parent.parent;
      if (parent && ts.isSourceFile(parent)) {
        gaps.push({ file: relative(root, join(repoRoot, path)).replaceAll('\\', '/'), line: sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1, sample: node.getText(sourceFile) });
      }
    } else if (ts.isJsxText(node)) {
      const sample = node.getText(sourceFile).trim();
      const withoutBrand = sample.replace(/^贸灵$/u, '');
      if (withoutBrand && chinese.test(withoutBrand)) {
        gaps.push({ file: relative(root, join(repoRoot, path)).replaceAll('\\', '/'), line: sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1, sample });
      }
    } else if (ts.isTemplateExpression(node)
      && [node.head.text, ...node.templateSpans.map(span => span.literal.text)].some(text => chinese.test(text))
      && isUserFacingLiteral(node)) {
      gaps.push({ file: relative(root, join(repoRoot, path)).replaceAll('\\', '/'), line: sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1, sample: node.getText(sourceFile) });
    } else if ((ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) && chinese.test(node.text)) {
      const sample = node.text.replace(/^贸灵$/u, '');
      if (sample && !isTranslationArgument(node) && isUserFacingLiteral(node)) {
        gaps.push({ file: relative(root, join(repoRoot, path)).replaceAll('\\', '/'), line: sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1, sample });
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return gaps;
}

export function sourcePaths() {
  // Include tracked files and new source files. Respect the repository's
  // existing generated-output rules so running `max setup` cannot turn .umi
  // copies into application-source debt. No source path exemptions are added.
  return execFileSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'admin/src'], {
    cwd: repoRoot, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024,
  }).split('\0').filter(Boolean);
}

export function inventory(paths = sourcePaths(), details = false) {
  let files = 0;
  const gaps = [];
  for (const path of [...new Set(paths)].sort()) {
    if (!trackedSource(path)) continue;
    const relativePath = relative(root, join(repoRoot, path)).replaceAll('\\', '/');
    files++;
    const source = readFileSync(join(repoRoot, path), 'utf8');
    const lineGaps = [];
    source.split(/\r?\n/).forEach((line, i) => {
      if (isUntranslatedUIString(line)) lineGaps.push({
        file: relativePath, line: i + 1, sample: line.trim(),
      });
    });
    const syntaxGaps = astCandidates(path, source);
    gaps.push(...lineGaps.filter((gap) => !syntaxGaps.some((candidate) => candidate.file === gap.file && candidate.line === gap.line)), ...syntaxGaps);
  }
  const unique = new Map(gaps.map((gap) => [`${gap.file}:${gap.line}:${gap.sample}`, gap]));
  const deduped = [...unique.values()];
  const byModule = {};
  for (const gap of deduped) {
    const module = gap.file.split('/').slice(0, 2).join('/');
    byModule[module] = (byModule[module] || 0) + 1;
  }
  return {
    sourceFiles: files, unmigratedTextSites: deduped.length, affectedFiles: new Set(deduped.map(x => x.file)).size,
    byModule,
    samples: deduped.slice(0, 35).map(gap => ({ ...gap, sample: gap.sample.slice(0, 130) })),
    ...(details ? { candidates: deduped } : {}),
  };
}

function diffGaps(base) {
  if (!/^(?:[a-f0-9]{7,40}|HEAD(?:~[1-9][0-9]*)?)$/.test(base)) {
    throw new Error('Invalid git baseline for i18n regression check');
  }
  const patch = execFileSync('git',[
    'diff','--no-ext-diff','--unified=0','--diff-filter=ACMR',base,'HEAD','--','admin/src',
  ],{cwd:repoRoot,encoding:'utf8',maxBuffer:32*1024*1024});
  return parseAddedSourceLines(patch).filter(x=>isUntranslatedUIString(x.line));
}

function main() {
  const inventoryResult = inventory(sourcePaths(), process.argv.includes('--details'));
  console.log(JSON.stringify(inventoryResult,null,2));
  if (process.argv.includes('--strict') && inventoryResult.unmigratedTextSites > 0) {
    process.exitCode = 1;
  }
  const baseArg = process.argv.find(v=>v.startsWith('--diff-base='));
  if (baseArg) {
    const gaps = diffGaps(baseArg.slice('--diff-base='.length));
    console.log(JSON.stringify({
      newlyIntroducedUntranslatedTextSites:gaps.length,
      samples:gaps.slice(0,35),
    },null,2));
    if (gaps.length) {
      console.error('New unlocalized UI text found. Translate new text before merging.');
      process.exitCode = 1;
    }
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
