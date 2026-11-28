import fs from 'node:fs';

export const ENVIRONMENT_PROFILES = [
  'development', 'demo', 'performance', 'test', 'staging', 'production',
];
export const PREPRODUCTION_APP_ENV = 'staging';

export const DISABLED_CAPABILITY_KEYS = [
  'REAL_DOUYIN_ENABLED',
  'REAL_PLATFORM_READ',
  'REAL_PLATFORM_WRITE',
  'REAL_INVENTORY_READ',
  'REAL_INVENTORY_WRITE',
  'INVENTORY_MUTATION_ENABLED',
  'AUTO_INVENTORY_SYNC',
  'AUTO_RETRY',
  'INVENTORY_SYNC_AUTO_RETRY',
  'INVENTORY_SYNC_BACKGROUND_WORKER_ENABLED',
  'INVENTORY_SYNC_NETWORK_ACCESS',
  'COLLECT_QUEUE_ENABLED',
  'COLLECT_AUTO_RETRY_ENABLED',
  'COLLECT_BATCH_RETRY_ON_BLOCKED',
  'COLLECT_BATCH_RETRY_ON_TIMEOUT',
  'IMAGE_QUEUE_ENABLED',
  'IMAGE_AUTO_RETRY_ENABLED',
  'ORDER_SYNC_QUEUE_ENABLED',
  'PRODUCT_PUBLISH_QUEUE_ENABLED',
  'INVENTORY_SYNC_QUEUE_ENABLED',
  'WORKER_HEARTBEAT_ENABLED',
  'WORKER_REAPER_ENABLED',
  'TASK_ALERT_SCAN_ENABLED',
  'DOUYIN_WRITE_ENABLED',
  'AUTO_LISTING_ENABLED',
];

const L0_DISABLED_FLAGS = [
  ...DISABLED_CAPABILITY_KEYS,
  'P10_OFFLINE_OAUTH_ENABLED',
  'P10_REAL_PROVIDER_ENABLED',
  'P10_REAL_PLATFORM_NETWORK_ENABLED',
  'P10_REAL_CREDENTIALS_ENABLED',
  'P10_REAL_INVENTORY_READ_ENABLED',
  'P10_REAL_PRODUCT_DRAFT_WRITE_ENABLED',
  'P10_INVENTORY_MUTATION_ENABLED',
  'P10_BACKGROUND_WORKER_ENABLED',
  'P10_AUTOMATIC_RETRY_ENABLED',
];
const APPLICATION_IMAGES = [
  ['backend', 'P10_API_IMAGE', 'P10_PREVIOUS_API_IMAGE'],
  ['admin', 'P10_ADMIN_IMAGE', 'P10_PREVIOUS_ADMIN_IMAGE'],
  ['collector', 'P10_COLLECTOR_IMAGE', 'P10_PREVIOUS_COLLECTOR_IMAGE'],
];
const SECRET_REFERENCES = [
  'P10_DB_PASSWORD_REF', 'P10_REDIS_PASSWORD_REF', 'P10_APP_MASTER_KEY_REF',
  'P10_JWT_SECRET_REF', 'P10_COLLECTOR_SERVICE_TOKEN_REF',
];
const DIGEST_REFERENCE = /^ghcr\.io\/lien0219\/trademind@sha256:[0-9a-f]{64}$/u;

const REQUIRED_KEYS = [
  'APP_ENV', 'P10_ENVIRONMENT_PURPOSE', 'P10_DEPLOYMENT_ID', 'P10_CURRENT_ALLOWED_LEVEL',
  'ADMIN_PUBLIC_URL', 'API_PUBLIC_URL',
  'P10_PRODUCTION_ADMIN_PUBLIC_URL', 'P10_PRODUCTION_API_PUBLIC_URL',
  'DB_DRIVER', 'DB_NAME', 'DB_USER', 'P10_DATABASE_PURPOSE', 'P10_DATABASE_ID',
  'P10_PRODUCTION_DATABASE_ID', 'P10_TEST_DATABASE_ID',
  'P10_REDIS_PURPOSE', 'P10_REDIS_ID', 'P10_PRODUCTION_REDIS_ID', 'P10_TEST_REDIS_ID',
  'AUTH_SESSION_MODE', 'AUTH_SECURE_COOKIE', 'AUTH_COOKIE_DOMAIN',
  'P10_PRODUCTION_COOKIE_DOMAIN', 'P10_SESSION_NAMESPACE', 'P10_PRODUCTION_SESSION_NAMESPACE',
  'STORAGE_PROVIDER', 'CORS_ALLOWED_ORIGINS', 'CORS_ALLOW_CREDENTIALS',
  'P10_SECRET_SOURCE', ...SECRET_REFERENCES,
  'P10_MIGRATION_TARGET', 'MIGRATION_RUN_ON_STARTUP',
  'P10_BACKUP_TARGET', 'P10_RESTORE_TARGET',
  'P10_PREPRODUCTION_RESTORE_ENABLED', 'P10_PRODUCTION_RESTORE_ENABLED',
  'P10_COMPOSE_PROJECT_NAME', ...L0_DISABLED_FLAGS,
];

function value(env, key) { return String(env?.[key] ?? '').trim(); }
function hasKeys(env, keys) { return keys.every((key) => Object.hasOwn(env, key)); }
function isFalse(env, key) { return value(env, key).toLowerCase() === 'false'; }
function distinct(values) { return values.every((item, index) => item && values.indexOf(item) === index); }
function immutableApplicationImages(refs) {
  return distinct(refs) && refs.every((ref) => DIGEST_REFERENCE.test(ref));
}
function isL0(env) {
  return value(env, 'P10_CURRENT_ALLOWED_LEVEL') === 'L0'
    && L0_DISABLED_FLAGS.every((key) => isFalse(env, key))
    && value(env, 'P10_LOCAL_CREDENTIAL_KEY') === ''
    && value(env, 'EXTERNAL_PROVIDER_MODE') === 'mock'
    && value(env, 'INVENTORY_SYNC_PROVIDER_MODE') === 'fixture';
}
function isHTTPS(raw) {
  try { return new URL(raw).protocol === 'https:'; } catch { return false; }
}
function normalizedCookieDomain(raw) {
  return value({ raw }, 'raw').toLowerCase().replace(/^\./, '');
}
function cookieDomainsAreIsolated(left, right) {
  const first = normalizedCookieDomain(left);
  const second = normalizedCookieDomain(right);
  return first && second && first !== second
    && !first.endsWith(`.${second}`) && !second.endsWith(`.${first}`);
}
function isExternalSecretReference(raw) {
  return /^(?:external|docker-secret|managed):\/\/[a-z0-9][a-z0-9/_.-]*$/i.test(String(raw).trim());
}

export function parseEnvText(raw = '') {
  const env = {};
  for (const sourceLine of String(raw).split(/\r?\n/)) {
    const line = sourceLine.trim();
    if (!line || line.startsWith('#')) continue;
    const index = line.indexOf('=');
    if (index < 1) continue;
    const key = line.slice(0, index).trim();
    let item = line.slice(index + 1).trim();
    if ((item.startsWith('"') && item.endsWith('"')) || (item.startsWith("'") && item.endsWith("'"))) {
      item = item.slice(1, -1);
    }
    env[key] = item;
  }
  return env;
}

export function readEnvFile(filePath) {
  return parseEnvText(fs.readFileSync(filePath, 'utf8'));
}

export function validateCanonicalTemplate(env = {}) {
  const databaseKeys = [
    'DB_DRIVER', 'DB_HOST', 'DB_PORT', 'DB_USER', 'DB_PASSWORD', 'DB_NAME',
    'POSTGRES_DB', 'POSTGRES_USER', 'POSTGRES_PASSWORD',
    'P10_DATABASE_PURPOSE', 'P10_DATABASE_ID', 'P10_PRODUCTION_DATABASE_ID', 'P10_TEST_DATABASE_ID',
  ];
  const redisKeys = [
    'REDIS_ADDR', 'REDIS_PASSWORD', 'REDIS_DB',
    'P10_REDIS_PURPOSE', 'P10_REDIS_ID', 'P10_PRODUCTION_REDIS_ID', 'P10_TEST_REDIS_ID',
  ];
  const sessionKeys = [
    'AUTH_SESSION_MODE', 'AUTH_SECURE_COOKIE', 'AUTH_COOKIE_DOMAIN',
    'P10_PRODUCTION_COOKIE_DOMAIN', 'P10_SESSION_NAMESPACE', 'P10_PRODUCTION_SESSION_NAMESPACE',
  ];
  const checks = [
    ['requiredConfiguration', hasKeys(env, REQUIRED_KEYS)],
    ['environmentKnown', ENVIRONMENT_PROFILES.includes(value(env, 'APP_ENV').toLowerCase())],
    ['preproductionEnvironmentMapping', PREPRODUCTION_APP_ENV === 'staging'
      && hasKeys(env, ['P10_ENVIRONMENT_PURPOSE', 'P10_DEPLOYMENT_ID', 'P10_COMPOSE_PROJECT_NAME'])],
    ['deploymentIdentity', hasKeys(env, ['P10_DEPLOYMENT_ID', 'P10_COMPOSE_PROJECT_NAME'])],
    ['publicEndpointIsolation', hasKeys(env, [
      'ADMIN_PUBLIC_URL', 'API_PUBLIC_URL', 'P10_PRODUCTION_ADMIN_PUBLIC_URL', 'P10_PRODUCTION_API_PUBLIC_URL',
    ])],
    ['databaseIsolation', hasKeys(env, databaseKeys)],
    ['redisIsolation', hasKeys(env, redisKeys)],
    ['secretExternalization', hasKeys(env, ['P10_SECRET_SOURCE', ...SECRET_REFERENCES])
      && value(env, 'COLLECTOR_SERVICE_TOKEN') === ''
      && value(env, 'APP_MASTER_KEY') === ''
      && value(env, 'P10_LOCAL_CREDENTIAL_KEY') === ''
      && value(env, 'TEST_DATABASE_URL') === ''
      && value(env, 'TEST_REDIS_URL') === ''],
    ['sessionIsolation', hasKeys(env, sessionKeys)],
    ['stagingRuntimeSafety', hasKeys(env, [
      'STORAGE_PROVIDER', 'CORS_ALLOWED_ORIGINS', 'CORS_ALLOW_CREDENTIALS',
      'ENABLE_SWAGGER', 'ENABLE_DEV_ROUTES', 'ENABLE_DEMO_SEED', 'ENABLE_DEBUG_ENDPOINTS',
    ])],
    ['capabilityDefaults', isL0(env)],
    ['applicationImages', hasKeys(env, APPLICATION_IMAGES.map(([, current]) => current))],
    ['migrationSafety', hasKeys(env, ['P10_MIGRATION_TARGET', 'MIGRATION_RUN_ON_STARTUP'])],
    ['backupRestoreSafety', hasKeys(env, [
      'P10_BACKUP_TARGET', 'P10_RESTORE_TARGET', 'P10_PREPRODUCTION_RESTORE_ENABLED', 'P10_PRODUCTION_RESTORE_ENABLED',
    ]) && isFalse(env, 'P10_PRODUCTION_RESTORE_ENABLED')],
    ['rollbackFoundation', hasKeys(env, [
      ...APPLICATION_IMAGES.map(([, , previous]) => previous), 'P10_ROLLBACK_MIGRATION_COMPATIBLE',
    ])],
  ];
  const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
  return {
    status: failed.length === 0 ? 'passed' : 'failed',
    failed,
    failedCount: failed.length,
    environmentModel: {
      development: 'development', test: 'test', preproduction: 'staging', production: 'production',
    },
    currentAllowedLevel: 'L0',
    checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  };
}

export function validatePreproductionContract(env = {}, mode = 'startup') {
  const appEnv = value(env, 'APP_ENV').toLowerCase();
  const adminURL = value(env, 'ADMIN_PUBLIC_URL');
  const apiURL = value(env, 'API_PUBLIC_URL');
  const productionAdminURL = value(env, 'P10_PRODUCTION_ADMIN_PUBLIC_URL');
  const productionAPIURL = value(env, 'P10_PRODUCTION_API_PUBLIC_URL');
  const checks = [
    ['requiredConfiguration', REQUIRED_KEYS.every((key) => value(env, key).length > 0)],
    ['environmentKnown', ENVIRONMENT_PROFILES.includes(appEnv)],
    ['preproductionEnvironmentMapping', appEnv === PREPRODUCTION_APP_ENV
      && value(env, 'P10_ENVIRONMENT_PURPOSE') === 'preproduction'],
    ['deploymentIdentity', value(env, 'P10_DEPLOYMENT_ID').startsWith('trademind-preproduction-')
      && value(env, 'P10_COMPOSE_PROJECT_NAME') === 'trademind-preproduction'],
    ['publicEndpointIsolation', isHTTPS(adminURL) && isHTTPS(apiURL)
      && isHTTPS(productionAdminURL) && isHTTPS(productionAPIURL)
      && adminURL !== productionAdminURL && apiURL !== productionAPIURL],
    ['databaseIsolation', value(env, 'DB_DRIVER') === 'postgres'
      && value(env, 'P10_DATABASE_PURPOSE') === 'preproduction'
      && /(?:preproduction|staging)/i.test(value(env, 'DB_NAME'))
      && distinct([value(env, 'P10_DATABASE_ID'), value(env, 'P10_PRODUCTION_DATABASE_ID'), value(env, 'P10_TEST_DATABASE_ID')])],
    ['redisIsolation', value(env, 'P10_REDIS_PURPOSE') === 'preproduction'
      && distinct([value(env, 'P10_REDIS_ID'), value(env, 'P10_PRODUCTION_REDIS_ID'), value(env, 'P10_TEST_REDIS_ID')])],
    ['secretExternalization', ['external', 'docker_secret', 'managed'].includes(value(env, 'P10_SECRET_SOURCE'))
      && SECRET_REFERENCES.every((key) => isExternalSecretReference(value(env, key)))
      && ['APP_MASTER_KEY', 'JWT_SECRET', 'DB_PASSWORD', 'REDIS_PASSWORD', 'COLLECTOR_SERVICE_TOKEN']
        .every((key) => value(env, key).length === 0)],
    ['sessionIsolation', value(env, 'AUTH_SESSION_MODE') === 'secure_session'
      && value(env, 'AUTH_SECURE_COOKIE').toLowerCase() === 'true'
      && cookieDomainsAreIsolated(value(env, 'AUTH_COOKIE_DOMAIN'), value(env, 'P10_PRODUCTION_COOKIE_DOMAIN'))
      && value(env, 'P10_SESSION_NAMESPACE') !== value(env, 'P10_PRODUCTION_SESSION_NAMESPACE')],
    ['stagingRuntimeSafety', ['cos', 'oss', 's3', 'r2', 'minio'].includes(value(env, 'STORAGE_PROVIDER').toLowerCase())
      && value(env, 'CORS_ALLOWED_ORIGINS') === adminURL
      && value(env, 'CORS_ALLOW_CREDENTIALS').toLowerCase() === 'true'
      && ['ENABLE_SWAGGER', 'ENABLE_DEV_ROUTES', 'ENABLE_DEMO_SEED', 'ENABLE_DEBUG_ENDPOINTS']
        .every((key) => isFalse(env, key))],
    ['capabilityDefaults', isL0(env)],
    ['applicationImages', mode === 'rollback' || immutableApplicationImages(APPLICATION_IMAGES.map(([, current]) => value(env, current)))],
    ['migrationSafety', value(env, 'P10_MIGRATION_TARGET') === 'preproduction'
      && value(env, 'MIGRATION_RUN_ON_STARTUP').toLowerCase() === 'true'],
    ['backupRestoreSafety', value(env, 'P10_BACKUP_TARGET') === 'preproduction'
      && value(env, 'P10_RESTORE_TARGET') === 'preproduction_restore'
      && value(env, 'P10_PREPRODUCTION_RESTORE_ENABLED').toLowerCase() === 'true'
      && isFalse(env, 'P10_PRODUCTION_RESTORE_ENABLED')],
    ['rollbackFoundation', mode !== 'rollback' || (
      immutableApplicationImages(APPLICATION_IMAGES.map(([, , previous]) => value(env, previous)))
      && value(env, 'P10_ROLLBACK_MIGRATION_COMPATIBLE').toLowerCase() === 'true'
    )],
  ];
  const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
  return {
    status: failed.length === 0 ? 'passed' : 'failed',
    failed,
    failedCount: failed.length,
    environmentModel: {
      development: 'development', test: 'test', preproduction: 'staging', production: 'production',
    },
    currentAllowedLevel: 'L0',
    checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  };
}

// Check the values Docker Compose will actually pass to the processes. Exported
// shell variables can differ from the env_file included in a service.
export function validatePreproductionCompose(compose, env) {
  const services = compose?.services ?? {};
  const backend = services.backend?.environment ?? {};
  const collector = services.collector?.environment ?? {};
  const images = APPLICATION_IMAGES.map(([service]) => value(services[service], 'image'));
  const expected = APPLICATION_IMAGES.map(([, current]) => value(env, current));
  const token = collector.COLLECTOR_SERVICE_TOKEN;
  const checks = [
    ['resolvedApplicationImages', immutableApplicationImages(images) && images.every((ref, index) => ref === expected[index])],
    ['resolvedRuntimeProfile', value(backend, 'APP_ENV') === PREPRODUCTION_APP_ENV && isL0(backend)],
    ['resolvedCollectorConnection', value(backend, 'COLLECTOR_BASE_URL') === 'http://collector:3001'
      && typeof token === 'string' && /^[\x21-\x7e]{32,}$/u.test(token) && token === backend.COLLECTOR_SERVICE_TOKEN],
  ];
  return { images, failed: checks.filter(([, passed]) => !passed).map(([id]) => id) };
}

export function validateExternalInfrastructureEvidence(evidence = {}) {
  const checks = [
    ['status', evidence.externalInfrastructureStatus === 'provisioned'],
    ['host', evidence.preproductionHostAvailable === true],
    ['database', evidence.preproductionDatabaseProvisioned === true && evidence.databaseIsolationProven === true],
    ['redis', evidence.preproductionRedisProvisioned === true && evidence.redisIsolationProven === true],
    ['domain', evidence.preproductionDomainProvisioned === true],
    ['deploymentCredential', evidence.deploymentCredentialAvailable === true],
    ['deploymentRehearsal', evidence.deploymentRehearsed === true],
    ['teardownRehearsal', evidence.teardownRehearsed === true],
  ];
  const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
  return { status: failed.length === 0 ? 'passed' : 'failed', failed, failedCount: failed.length };
}
