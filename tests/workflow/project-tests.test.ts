import { readFileSync } from 'node:fs';
import path from 'node:path';
import { parse } from 'yaml';
import { describe, expect, it } from 'vitest';

type Environment = Record<string, string>;

interface Service {
  env?: Environment;
  ports?: Array<string | number>;
  options?: string;
}

interface Job {
  env?: Environment;
  services?: Record<string, Service>;
  steps: Array<{ name?: string; env?: Environment; run?: string }>;
}

interface Workflow {
  env?: Environment;
  jobs: Record<string, Job>;
}

const root = path.resolve(__dirname, '..', '..');
const workflow = parse(
  readFileSync(path.join(root, '.github', 'workflows', 'project-tests.yml'), 'utf8'),
) as Workflow;

const infrastructureSteps = Object.entries(workflow.jobs).flatMap(([name, job]) =>
  job.steps
    .filter((step) => /\bpnpm\s+(?:quality:(?:ci|affected|all)|test:(?:db|redis|backend:integration))\b/u.test(step.run || ''))
    .map((step) => ({
      label: `${name}: ${step.name}`,
      job,
      env: { ...workflow.env, ...job.env, ...step.env },
      needsDatabase: /\bpnpm\s+(?:quality:(?:ci|affected|all)|test:(?:db|backend:integration))\b/u.test(step.run || ''),
    })),
);

describe('backend integration workflow isolation', () => {
  it('provides test services to every database, queue and affected-quality step', () => {
    expect(infrastructureSteps.length).toBeGreaterThan(0);
    for (const { label, job, env, needsDatabase } of infrastructureSteps) {
      const redis = new URL(env.TEST_REDIS_URL);
      expect(['redis:', 'rediss:'], label).toContain(redis.protocol);
      expect(['127.0.0.1', 'localhost'], label).toContain(redis.hostname);
      expect(redis.pathname, label).toMatch(/^\/[1-9]\d*$/u);
      expect(job.services?.redis?.ports?.map(String), label).toContain(`${redis.port || '6379'}:6379`);

      if (!needsDatabase) continue;
      expect(env.APP_ENV, label).toBe('test');
      const database = new URL(env.TEST_DATABASE_URL);
      expect(['postgres:', 'postgresql:'], label).toContain(database.protocol);
      expect(['127.0.0.1', 'localhost'], label).toContain(database.hostname);
      expect(database.pathname.slice(1), label).toMatch(/(?:^|[_-])(?:test|tests|e2e|ci)(?:$|[_-])/u);
      expect(database.pathname.slice(1), label).toBe(job.services?.postgres?.env?.POSTGRES_DB);
      expect(database.username, label).toBe(job.services?.postgres?.env?.POSTGRES_USER);
      expect(job.services?.postgres?.ports?.map(String), label).toContain(`${database.port || '5432'}:5432`);
    }
  });

  it('checks PostgreSQL health with the configured service identity', () => {
    const databaseJobs = new Set(infrastructureSteps.filter((step) => step.needsDatabase).map((step) => step.job));
    for (const job of databaseJobs) {
      const postgres = job.services?.postgres;
      expect(postgres?.options).toContain(
        `pg_isready -U ${postgres?.env?.POSTGRES_USER} -d ${postgres?.env?.POSTGRES_DB}`,
      );
    }
  });
});

describe('internationalization workflow', () => {
  const audit = parse(readFileSync(path.join(root, '.github', 'workflows', 'i18n-coverage-audit.yml'), 'utf8')) as Workflow;
  const steps = audit.jobs.coverage.steps;

  it('installs the locked parser dependencies before running the audit', () => {
    const install = steps.findIndex(step => step.run === 'pnpm install --frozen-lockfile');
    const parser = steps.findIndex(step => step.run === 'node --test admin/scripts/i18n-audit.test.mjs');
    expect(install).toBeGreaterThanOrEqual(0);
    expect(parser).toBeGreaterThan(install);
  });

  it('blocks existing untranslated UI debt as well as changed-line regressions', () => {
    expect(steps.some(step => /node scripts\/i18n-audit\.mjs --strict\b/.test(step.run || ''))).toBe(true);
    expect(steps.some(step => step.run?.includes('--diff-base='))).toBe(true);
  });
});
