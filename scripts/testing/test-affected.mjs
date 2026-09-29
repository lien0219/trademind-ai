import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execa } from 'execa';
import pc from 'picocolors';

const commands = new Map([
  ['frontend', ['pnpm', ['test:frontend']]],
  ['collector', ['pnpm', ['test:collector']]],
  ['dev-scripts', ['pnpm', ['test:dev-scripts']]],
  ['backend', ['pnpm', ['test:backend']]],
  ['contracts', ['pnpm', ['test:contracts']]],
  ['architecture', ['pnpm', ['architecture:test']]],
  ['e2e-smoke', ['pnpm', ['test:e2e:smoke']]],
]);

function classify(path) {
  const selected = new Set();
  if (path.startsWith('admin/src/') || path === 'admin/vitest.config.ts') selected.add('frontend');
  if (path.startsWith('collector/src/') || path === 'collector/vitest.config.ts') selected.add('collector');
  if (path === 'scripts/dev-all.ts' || path.startsWith('scripts/utils/collector-dev-env')) selected.add('dev-scripts');
  if (path.startsWith('backend/') && path.endsWith('.go')) selected.add('backend');
  if (path.startsWith('backend/internal/testing/integration/') || path.includes('migrate')) selected.add('backend');
  if (path.startsWith('backend/internal/testing/redis/') || path.toLowerCase().includes('redis') || path.toLowerCase().includes('queue')) selected.add('backend');
  if (path.startsWith('tests/contracts/') || path.includes('/services/') || path.includes('router.go') || path.includes('handler.go')) selected.add('contracts');
  if (path.startsWith('admin/e2e/') || path === 'playwright.config.ts') selected.add('e2e-smoke');
  if (path === 'package.json' || path === 'pnpm-lock.yaml' || path.startsWith('.github/workflows/') || path.startsWith('scripts/testing/')) {
    selected.add('frontend');
    selected.add('collector');
    selected.add('backend');
    selected.add('contracts');
  }
  if (path.startsWith('scripts/architecture/') || path.startsWith('tests/architecture/') || path === 'vitest.architecture.config.ts' || path === '.agents/skills/modular-architecture/SKILL.md') {
    selected.add('architecture');
  }
  return selected;
}

export function selectAffectedSuites(files, { skip = [] } = {}) {
  const selected = new Set();
  for (const file of files) {
    for (const name of classify(file)) selected.add(name);
  }
  if (!selected.size) selected.add('contracts');

  for (const name of skip) {
    if (!commands.has(name)) throw new Error(`Unknown affected test suite: ${name}`);
    selected.delete(name);
  }
  return selected;
}

async function changedFiles({ all, base }) {
  if (all) return ['package.json'];
  const { stdout } = await execa('git', ['diff', '--name-only', base, '--']);
  return stdout.split('\n').map((line) => line.trim()).filter(Boolean);
}

async function main() {
  const args = process.argv.slice(2);
  const all = args.includes('--all');
  const baseArg = args.find((arg) => arg.startsWith('--base='));
  const base = baseArg?.slice('--base='.length) || process.env.TEST_AFFECTED_BASE || 'HEAD~1';
  const skip = args
    .filter((arg) => arg.startsWith('--skip='))
    .flatMap((arg) => arg.slice('--skip='.length).split(','))
    .filter(Boolean);

  const files = await changedFiles({ all, base });
  const selected = selectAffectedSuites(files, { skip });

  console.log(pc.cyan('Affected files:'));
  for (const file of files) console.log(`- ${file}`);
  console.log(pc.cyan('Selected test suites:'), [...selected].join(', ') || '<none>');

  for (const name of selected) {
    const [bin, commandArgs] = commands.get(name);
    console.log(pc.bold(`\n> ${bin} ${commandArgs.join(' ')}`));
    await execa(bin, commandArgs, { stdio: 'inherit' });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main();
}
