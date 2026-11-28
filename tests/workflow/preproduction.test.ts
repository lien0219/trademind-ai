import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { parse } from 'yaml';
import {
  parseEnvText,
  validateCanonicalTemplate,
  validatePreproductionCompose,
  validatePreproductionContract,
} from '../../scripts/preproduction-contract.mjs';

const root = path.resolve(__dirname, '..', '..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const reference = (digit: string) => `ghcr.io/lien0219/trademind@sha256:${digit.repeat(64)}`;

function stagingEnv() {
  return {
    ...parseEnvText(read('.env.example')),
    ...parseEnvText(read('tests/workflow/fixtures/preproduction.env')),
  };
}

const previousImages = {
  P10_PREVIOUS_API_IMAGE: reference('d'),
  P10_PREVIOUS_ADMIN_IMAGE: reference('e'),
  P10_PREVIOUS_COLLECTOR_IMAGE: reference('f'),
  P10_ROLLBACK_MIGRATION_COMPATIBLE: 'true',
};

describe('independent preproduction deployment', () => {
  it('accepts a first deployment and requires all three compatible images only for rollback', () => {
    expect(validateCanonicalTemplate(parseEnvText(read('.env.example'))).failed).toEqual([]);
    const env = stagingEnv();
    expect(validatePreproductionContract(env, 'startup').failed).toEqual([]);
    expect(validatePreproductionContract(env, 'rollback').failed).toContain('rollbackFoundation');
    expect(validatePreproductionContract({ ...env, ...previousImages }, 'rollback').failed).toEqual([]);
    expect(validatePreproductionContract({ ...env, ...previousImages, P10_PREVIOUS_COLLECTOR_IMAGE: '' }, 'rollback').failed).toContain('rollbackFoundation');
  });

  it('rejects missing, mutable or reused application images', () => {
    const env = stagingEnv();
    for (const image of ['', 'ghcr.io/lien0219/trademind:collector-main', env.P10_ADMIN_IMAGE]) {
      expect(validatePreproductionContract({ ...env, P10_COLLECTOR_IMAGE: image }, 'startup').failed).toContain('applicationImages');
    }
    expect(validatePreproductionContract({ ...env, ...previousImages, P10_PREVIOUS_COLLECTOR_IMAGE: previousImages.P10_PREVIOUS_ADMIN_IMAGE }, 'rollback').failed).toContain('rollbackFoundation');
  });

  it('rejects platform promotion and credentials reused from another deployment', () => {
    const env = stagingEnv();
    for (const unsafe of [
      { P10_CURRENT_ALLOWED_LEVEL: 'L1' },
      { P10_REAL_PLATFORM_NETWORK_ENABLED: 'true' },
      { P10_REAL_PRODUCT_DRAFT_WRITE_ENABLED: 'true' },
      { COLLECT_QUEUE_ENABLED: 'true' },
    ]) {
      expect(validatePreproductionContract({ ...env, ...unsafe }, 'startup').failed).toContain('capabilityDefaults');
    }
    expect(validatePreproductionContract({ ...env, COLLECTOR_SERVICE_TOKEN: 'a'.repeat(32) }, 'startup').failed).toContain('secretExternalization');
    expect(validatePreproductionContract({ ...env, P10_COLLECTOR_SERVICE_TOKEN_REF: '' }, 'startup').failed).toContain('secretExternalization');
  });

  it('checks the service environment Compose will inject after interpolation', () => {
    const env = stagingEnv();
    const token = 'preproduction-test-token-'.repeat(2);
    const compose = { services: {
      backend: { image: env.P10_API_IMAGE, environment: { ...env, COLLECTOR_BASE_URL: 'http://collector:3001', COLLECTOR_SERVICE_TOKEN: token } },
      admin: { image: env.P10_ADMIN_IMAGE },
      collector: { image: env.P10_COLLECTOR_IMAGE, environment: { COLLECTOR_SERVICE_TOKEN: token } },
    } };
    expect(validatePreproductionCompose(compose, env).failed).toEqual([]);
    compose.services.backend.environment.P10_CURRENT_ALLOWED_LEVEL = 'L3';
    expect(validatePreproductionCompose(compose, env).failed).toContain('resolvedRuntimeProfile');
    compose.services.backend.environment.P10_CURRENT_ALLOWED_LEVEL = 'L0';
    compose.services.backend.environment.COLLECTOR_SERVICE_TOKEN = 'mismatch';
    expect(validatePreproductionCompose(compose, env).failed).toContain('resolvedCollectorConnection');
    compose.services.collector.image = env.P10_ADMIN_IMAGE;
    expect(validatePreproductionCompose(compose, env).failed).toContain('resolvedApplicationImages');
  });

  it('gives Collector an isolated service and matched backend credentials', () => {
    const { services, networks, volumes } = parse(read('deploy/preproduction/compose.yml'));
    expect(Object.keys(services).sort()).toEqual(['admin', 'backend', 'collector', 'postgres', 'redis']);
    expect(networks.preproduction.internal).toBe(true);
    expect(services.collector.image).toMatch(/^\$\{P10_COLLECTOR_IMAGE:\?/u);
    expect(services.collector.networks).toEqual(['preproduction']);
    expect(services.collector).not.toHaveProperty('ports');
    expect(services.collector).not.toHaveProperty('env_file');
    expect(services.collector.environment.COLLECTOR_SERVICE_TOKEN).toMatch(/^\$\{PREPRODUCTION_COLLECTOR_SERVICE_TOKEN:\?/u);
    expect(services.collector.environment.COLLECTOR_SERVICE_TOKEN).toBe(services.backend.environment.COLLECTOR_SERVICE_TOKEN);
    expect(services.backend.environment.COLLECTOR_BASE_URL).toBe('http://collector:3001');
    expect(services.backend.depends_on.collector.condition).toBe('service_healthy');
    expect(services.admin.healthcheck.test).toContain('http://127.0.0.1/');
    expect(volumes.preproduction_collector_profiles.name).toBe('trademind-preproduction-collector-profiles');
    expect(volumes.preproduction_collector_storage_states.name).toBe('trademind-preproduction-collector-storage-states');
  });

  it('wires migrations to the same database identity as the isolated PostgreSQL service', () => {
    const { services } = parse(read('deploy/preproduction/compose.yml'));
    expect(services.backend.environment.DB_DRIVER).toBe('postgres');
    expect(services.backend.environment.DB_NAME).toBe(services.postgres.environment.POSTGRES_DB);
    expect(services.backend.environment.DB_USER).toBe(services.postgres.environment.POSTGRES_USER);
  });

  it('publishes the Admin proxy on host loopback while API and Collector stay on the internal bridge', () => {
    const { services, networks } = parse(read('deploy/preproduction/compose.yml'));
    expect(networks.admin_ingress.name).toBe('trademind-preproduction-ingress');
    expect(services.admin.networks).toEqual(['preproduction', 'admin_ingress']);
    expect(services.admin.ports).toEqual(['127.0.0.1:${P10_ADMIN_BIND_PORT:-18000}:80']);
    for (const service of [services.backend, services.postgres, services.redis, services.collector]) {
      expect(service.networks).toEqual(['preproduction']);
      expect(service).not.toHaveProperty('ports');
    }
  });

  it('deploys and rolls back the three image set with container-network probes', () => {
    const deploy = read('deploy/scripts/deploy-preproduction.sh');
    const rollback = read('deploy/scripts/rollback-preproduction.sh');
    const readiness = read('deploy/scripts/check-preproduction-readiness.sh');
    expect(deploy).toContain('--dry-run');
    expect(deploy).toContain('pull backend admin collector');
    expect(deploy).toContain('up -d --wait');
    expect(rollback).toContain('P10_COLLECTOR_IMAGE="$P10_PREVIOUS_COLLECTOR_IMAGE"');
    expect(rollback).toContain('bash "$ROOT/deploy/scripts/deploy-preproduction.sh"');
    expect(readiness).toContain('exec -T admin');
    expect(readiness).toContain('exec -T backend');
    expect(readiness).toContain('/api/v1/health');
    expect(readiness).toContain('/v1/providers');
  });
});
