/**
 * Patch @ant-design/pro-field Select/Cascader/TreeSelect for antd 5.24+ deprecated APIs.
 * Replaces dropdownRender -> popupRender, onDropdownVisibleChange -> onOpenChange.
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function findProFieldRoots() {
  const pnpmDir = path.join(root, 'node_modules', '.pnpm');
  if (!fs.existsSync(pnpmDir)) return [];
  const out = [];
  for (const entry of fs.readdirSync(pnpmDir)) {
    if (!entry.startsWith('@ant-design+pro-field@3.1.0_')) continue;
    const pkg = path.join(pnpmDir, entry, 'node_modules', '@ant-design', 'pro-field');
    if (fs.existsSync(pkg)) out.push(pkg);
  }
  return out;
}

const REL_FILES = [
  'es/components/Select/LightSelect/index.js',
  'lib/components/Select/LightSelect/index.js',
  'es/components/Cascader/index.js',
  'lib/components/Cascader/index.js',
  'es/components/TreeSelect/index.js',
  'lib/components/TreeSelect/index.js',
];

function patchContent(source) {
  let next = source;
  next = next.replaceAll('dropdownRender: function dropdownRender', 'popupRender: function popupRender');
  next = next.replaceAll(
    'onDropdownVisibleChange: function onDropdownVisibleChange',
    'onOpenChange: function onOpenChange',
  );
  // Idempotent: collapse accidental double fallback chains from re-runs.
  next = next.replace(
    /([\w$]+)\.onOpenChange \|\| \1\.onOpenChange(?: \|\| \1\.onOpenChange)* \|\| \1\.onDropdownVisibleChange/g,
    '$1.onOpenChange || $1.onDropdownVisibleChange',
  );
  next = next.replace(/([\w$]+)\.onDropdownVisibleChange/g, '$1.onOpenChange || $1.onDropdownVisibleChange');
  // Final cleanup if still duplicated.
  next = next.replace(
    /([\w$]+)\.onOpenChange \|\| \1\.onOpenChange \|\| \1\.onDropdownVisibleChange/g,
    '$1.onOpenChange || $1.onDropdownVisibleChange',
  );
  return next;
}

let patchedFiles = 0;
for (const pkgRoot of findProFieldRoots()) {
  for (const rel of REL_FILES) {
    const file = path.join(pkgRoot, rel);
    if (!fs.existsSync(file)) continue;
    const before = fs.readFileSync(file, 'utf8');
    const after = patchContent(before);
    if (after !== before) {
      fs.writeFileSync(file, after, 'utf8');
      patchedFiles += 1;
    }
  }
}

if (patchedFiles > 0) {
  console.log(`[patch-pro-field] updated ${patchedFiles} file(s) for antd Select API compatibility`);
}
