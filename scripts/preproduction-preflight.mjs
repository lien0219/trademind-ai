import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readEnvFile, validatePreproductionCompose, validatePreproductionContract } from './preproduction-contract.mjs';

function argument(name, fallback = '') {
  const index = process.argv.indexOf(name);
  return index >= 0 && process.argv[index + 1] ? process.argv[index + 1] : fallback;
}

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const envFile = path.resolve(repoRoot, '.env');
const mode = argument('--mode', 'startup');
const allowedModes = ['config', 'startup', 'migration', 'backup', 'restore', 'rollback', 'teardown'];
const values = { ...readEnvFile(envFile), ...process.env };
const report = validatePreproductionContract(values, mode);
const resolveCompose = process.argv.includes('--resolve-compose');
let images = [];
if (!allowedModes.includes(mode)) report.failed.push('operationMode');
if (resolveCompose) {
  try {
    const result = validatePreproductionCompose(JSON.parse(readFileSync(0, 'utf8')), values);
    images = result.images;
    report.failed.push(...result.failed);
  } catch {
    report.failed.push('resolvedComposeConfiguration');
  }
}
report.mode = mode;
report.envFile = path.relative(repoRoot, envFile).replaceAll('\\', '/');
report.failedCount = report.failed.length;
report.status = report.failedCount === 0 ? 'passed' : 'failed';
if (report.failedCount > 0) {
  console.error(JSON.stringify(report, null, 2));
  process.exitCode = 1;
} else if (resolveCompose) {
  console.log(images.join('\n'));
} else {
  console.log(JSON.stringify(report, null, 2));
}
