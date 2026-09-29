#!/usr/bin/env node
import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execa } from 'execa';
import pc from 'picocolors';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const args = process.argv.slice(2);
const all = args.includes('--all');
const baseArg = args.find((arg) => arg.startsWith('--base='));
const headArg = args.find((arg) => arg.startsWith('--head='));
const base = baseArg?.slice('--base='.length) || process.env.QUALITY_BASE_SHA || process.env.TEST_AFFECTED_BASE;
const head = headArg?.slice('--head='.length) || process.env.QUALITY_HEAD_SHA || process.env.GITHUB_SHA || 'HEAD';

const checks = new Map([
  ['sensitive', { command: ['pnpm', ['quality:sensitive']], reason: '所有代码/配置变更都检查 changed diff 高置信敏感信息' }],
  ['admin', { command: ['pnpm', ['quality:admin']], reason: 'Admin TS/TSX、样式、UI、API service 或 package 变更' }],
  ['collector', { command: ['pnpm', ['quality:collector']], reason: 'Collector TypeScript、采集逻辑或 package 变更' }],
  ['backend', { command: ['pnpm', ['quality:backend']], reason: 'Go backend、数据库、Redis、队列、worker、adapter 或 Go 配置变更' }],
  ['naming', { command: ['pnpm', ['quality:naming']], reason: 'Go 标识符、数据库模型、迁移或原始 SQL 命名变更' }],
  ['contracts', { command: ['pnpm', ['quality:contracts']], reason: 'API route、DTO、service、contract、Admin mock 或 envelope 变更' }],
  ['affected-tests', { command: ['pnpm', ['test:affected', '--', '--skip=e2e-smoke']], reason: '与 project-testing 联动运行受影响测试选择，排除本门禁已执行的 E2E smoke' }],
  ['ui-copy', { command: ['pnpm', ['check:ui-copy', '--strict']], reason: 'Admin 文案、TSX、页面或 UI 规则变更' }],
  ['e2e-smoke', { command: ['pnpm', ['test:e2e:smoke']], reason: 'Admin 页面/样式/路由/交互变更需要轻量浏览器 smoke' }],
  ['db', { command: ['pnpm', ['test:db']], reason: 'migration/repository/database 变更需要测试数据库集成' }],
  ['redis', { command: ['pnpm', ['test:redis']], reason: 'Redis/queue/worker 变更需要 Redis 集成' }],
  ['architecture', { command: ['pnpm', ['architecture:affected', '--skip-quality']], reason: '新模块、跨模块、shared/common、adapter、worker、repository、migration 或架构配置变更需要模块边界检查' }],
]);

function add(selected, name, reason) {
  if (!selected.has(name)) selected.set(name, new Set());
  selected.get(name).add(reason || checks.get(name)?.reason || 'matched');
}

async function git(commandArgs, options = {}) {
  const result = await execa('git', commandArgs, { cwd: root, reject: false, ...options });
  if (result.exitCode !== 0) throw new Error(result.stderr || `git ${commandArgs.join(' ')} failed`);
  return result.stdout;
}

async function changedFiles() {
  if (all) return ['package.json'];

  if (base) {
    const commandArgs = head && head !== 'HEAD' ? ['diff', '--name-only', base, head, '--'] : ['diff', '--name-only', base, '--'];
    const stdout = await git(commandArgs);
    const files = stdout.split('\n').map((line) => line.trim()).filter(Boolean);
    return files.length ? files : ['package.json'];
  }

  const unstaged = await git(['diff', '--name-only', '--']);
  const staged = await git(['diff', '--cached', '--name-only', '--']);
  const untracked = await git(['ls-files', '--others', '--exclude-standard']);
  const files = new Set([...unstaged.split('\n'), ...staged.split('\n'), ...untracked.split('\n')].map((line) => line.trim()).filter(Boolean));
  return [...files];
}

function isTs(file) {
  return /\.(ts|tsx)$/.test(file);
}

function moduleKey(file) {
  const parts = file.split('/');
  if (file.startsWith('admin/src/')) return parts.slice(0, 3).join('/');
  if (file.startsWith('collector/src/')) return parts.slice(0, 3).join('/');
  if (file.startsWith('backend/internal/modules/')) return parts.slice(0, 4).join('/');
  if (file.startsWith('backend/internal/providers/')) return parts.slice(0, 4).join('/');
  return null;
}

function classify(file, selected) {
  add(selected, 'sensitive');

  if (file.startsWith('admin/src/') || file.startsWith('admin/test/') || file === 'admin/vitest.config.ts' || file === 'admin/package.json' || file === 'admin/tsconfig.json') {
    add(selected, 'admin');
    add(selected, 'affected-tests');
  }
  if (file.startsWith('admin/src/') && /\.(tsx|jsx|less|css|scss)$/.test(file)) {
    add(selected, 'ui-copy');
    add(selected, 'e2e-smoke');
  }
  if (file.startsWith('admin/e2e/') || file === 'playwright.config.ts') {
    add(selected, 'admin');
    add(selected, 'e2e-smoke');
    add(selected, 'affected-tests');
  }
  if (file.startsWith('collector/src/') || file === 'collector/package.json' || file === 'collector/tsconfig.json' || file === 'collector/vitest.config.ts') {
    add(selected, 'collector');
    add(selected, 'affected-tests');
  }
  if (file.startsWith('backend/') || file === 'go.work') {
    add(selected, 'backend');
    add(selected, 'affected-tests');
    if (file.endsWith('.go')) add(selected, 'naming');
  }
  if (file.startsWith('backend/internal/database/') || file.startsWith('backend/migrations/') || file.toLowerCase().includes('migration') || file.toLowerCase().includes('repository')) {
    add(selected, 'db');
    add(selected, 'naming');
    add(selected, 'contracts');
    add(selected, 'architecture');
  }
  if (file.toLowerCase().includes('redis') || file.toLowerCase().includes('queue') || file.toLowerCase().includes('worker') || file.toLowerCase().includes('scheduler')) {
    add(selected, 'redis');
    add(selected, 'backend');
    add(selected, 'architecture');
  }
  if (file.startsWith('tests/contracts/') || file.includes('/router.go') || file.includes('/handler.go') || file.includes('/dto.go') || file.includes('/services/') || file.includes('api-contract') || file.includes('envelope')) {
    add(selected, 'contracts');
    add(selected, 'admin');
    add(selected, 'affected-tests');
  }
  if (file === 'package.json' || file === 'pnpm-lock.yaml' || file === 'pnpm-workspace.yaml' || file.startsWith('.github/workflows/')) {
    add(selected, 'admin');
    add(selected, 'collector');
    add(selected, 'backend');
    add(selected, 'naming');
    add(selected, 'contracts');
    add(selected, 'affected-tests');
  }
  if (file.startsWith('.agents/skills/') || file.startsWith('.cursor/rules/') || file.startsWith('scripts/quality/') || file.startsWith('scripts/testing/')) {
    add(selected, 'sensitive');
    add(selected, 'affected-tests');
    if (file.includes('naming')) add(selected, 'naming');
  }
  if (file.startsWith('scripts/architecture/') || file.startsWith('tests/architecture/') || file === 'vitest.architecture.config.ts' || file === '.agents/skills/modular-architecture/SKILL.md') {
    add(selected, 'architecture');
  }
  if (file.includes('/shared/') || file.includes('/common/') || file.includes('/types/') || file.includes('/constants/') || file.includes('/providers/') || file.toLowerCase().includes('adapter') || file.toLowerCase().includes('dto') || file.toLowerCase().includes('enum')) {
    add(selected, 'architecture');
  }
  if (isTs(file) && file.startsWith('scripts/')) add(selected, 'contracts');
}

async function runCheck(name) {
  const check = checks.get(name);
  if (!check) return;
  const [bin, commandArgs] = check.command;
  console.log(pc.bold(`\n> ${bin} ${commandArgs.join(' ')}`));
  await execa(bin, commandArgs, { cwd: root, stdio: 'inherit' });
}

const files = await changedFiles();
const selected = new Map();
const changedModules = new Set(files.map(moduleKey).filter(Boolean));
for (const file of files) classify(file, selected);
if (changedModules.size >= 3) add(selected, 'architecture', '跨三个以上业务模块变更需要模块边界检查');

if (!files.length) {
  add(selected, 'sensitive', '无本地变更时仍执行敏感 diff 空扫描');
  add(selected, 'contracts', '无变更安全默认集合');
}
if (!selected.size) {
  add(selected, 'sensitive', '无法确定范围时运行安全默认集合');
  add(selected, 'admin', '无法确定范围时运行安全默认集合');
  add(selected, 'collector', '无法确定范围时运行安全默认集合');
  add(selected, 'backend', '无法确定范围时运行安全默认集合');
  add(selected, 'contracts', '无法确定范围时运行安全默认集合');
}

console.log(pc.cyan('Changed files:'));
if (files.length) for (const file of files) console.log(`- ${file}`);
else console.log('- <none>');

console.log(pc.cyan('\nSelected quality checks:'));
for (const [name, reasons] of selected) {
  console.log(`- ${name}: ${[...reasons].join('; ')}`);
}

const order = ['sensitive', 'architecture', 'admin', 'collector', 'backend', 'naming', 'contracts', 'ui-copy', 'db', 'redis', 'e2e-smoke', 'affected-tests'];
const failures = [];
for (const name of order.filter((item) => selected.has(item))) {
  try {
    await runCheck(name);
  } catch (error) {
    failures.push({ name, error });
    console.error(pc.red(`Quality check failed: ${name}`));
    break;
  }
}

console.log(pc.cyan('\nQuality report'));
console.log(`Changed files: ${files.length}`);
console.log(`Triggered rules: ${[...selected.keys()].join(', ')}`);
console.log(`Failures: ${failures.length}`);
console.log('Critical: 0 scripted findings');
console.log(failures.length ? 'High: 1 blocking check failure' : 'High: 0 scripted findings');
console.log('Medium: 0 scripted findings');
console.log('Advisory: AI review required for contextual quality risks');

if (failures.length) process.exit(1);
console.log(pc.green('Affected quality checks passed.'));
