import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const requiredChecks = ['PLATFORM', 'AMD64', 'ARM64', 'SECRET_AMD64', 'SECRET_ARM64', 'SANITIZE'];

export function failedImageScanChecks(outcomes) {
  return requiredChecks.filter((name) => outcomes?.[name] !== 'success');
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const outcomes = Object.fromEntries(requiredChecks.map((name) => [name, process.env[`${name}_OUTCOME`] ?? '']));
  const failures = failedImageScanChecks(outcomes);
  for (const name of failures) {
    process.stderr.write(`::error::${name} image security verification did not succeed\n`);
  }
  if (failures.length > 0) process.exit(1);
}
