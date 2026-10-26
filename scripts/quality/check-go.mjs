#!/usr/bin/env node
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { execa } from 'execa';
import pc from 'picocolors';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const backend = path.join(root, 'backend');
const args = process.argv.slice(2);
const checkFmt = args.includes('--fmt') || args.length === 0;
const checkVet = args.includes('--vet') || args.length === 0;
const checkTest = args.includes('--test') || args.length === 0;

async function run(label, bin, commandArgs, options = {}) {
  console.log(pc.bold(`\n> ${bin} ${commandArgs.join(' ')}`));
  const result = await execa(bin, commandArgs, { cwd: backend, all: true, reject: false, ...options });
  if (result.all?.trim()) console.log(result.all.trim());
  if (result.exitCode !== 0) throw new Error(`${label} failed with exit code ${result.exitCode}`);
  return result;
}

try {
  if (checkFmt) {
    const result = await run('gofmt', 'gofmt', ['-l', '.']);
    const unformatted = result.stdout.split('\n').map((line) => line.trim()).filter(Boolean);
    console.log(`gofmt unformatted files: ${unformatted.length}`);
    if (unformatted.length) process.exit(1);
  }
  if (checkVet) await run('go vet', 'go', ['vet', './...']);
  if (checkTest) await run('go test', 'go', ['test', './...']);
  console.log(pc.green('\nGo quality checks passed.'));
} catch (error) {
  console.error(pc.red(error.message));
  process.exit(1);
}
