#!/usr/bin/env node
// Internationalization migration debt inventory and changed-line regression gate.
// This scanner is intentionally a heuristic, not proof of complete localization.
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const repoRoot = resolve(root, '..', '..');
const exclude = /(?:^|\/)(?:__tests__|__mocks__)(?:\/|$)|\.test\.|\.spec\.|\.d\.ts$|\.generated\.|\/i18n\/dictionaries\.ts$/;
const sourceFile = /\.[jt]sx?$/;
const chinese = /[\u3400-\u9fff]/;
const userFacing = /\b(?:label|title|subTitle|placeholder|message|description|text|tooltip|aria-label|okText|cancelText|name)\s*[:=]\s*['"`]|message\.(?:success|error|warning|info)\s*\(|>\s*[^<{]*[\u3400-\u9fff]/;

export function isUntranslatedUIString(line) {
  if (!chinese.test(line) || /^\s*\/\//.test(line)) return false;
  // A translated string key is not itself a missing translation. Do not hide
  // other literals elsewhere on the same line.
  const withoutTranslatedKeys = line.replace(/\b(?:tr|trFormat)\(\s*(['"`])(?:\\.|(?!\1).)*?\1/g, '');
  return chinese.test(withoutTranslatedKeys) && userFacing.test(withoutTranslatedKeys);
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
    readFileSync(join(repoRoot, path), 'utf8').split(/\r?\n/).forEach((line, i) => {
      if (isUntranslatedUIString(line)) gaps.push({
        file: relativePath, line: i + 1, sample: line.trim(),
      });
    });
  }
  const byModule = {};
  for (const gap of gaps) {
    const module = gap.file.split('/').slice(0, 2).join('/');
    byModule[module] = (byModule[module] || 0) + 1;
  }
  return {
    sourceFiles: files, unmigratedTextSites: gaps.length, affectedFiles: new Set(gaps.map(x => x.file)).size,
    byModule,
    samples: gaps.slice(0, 35).map(gap => ({ ...gap, sample: gap.sample.slice(0, 130) })),
    ...(details ? { candidates: gaps } : {}),
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
