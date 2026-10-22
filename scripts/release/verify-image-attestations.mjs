import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const digestPattern = /^sha256:[0-9a-f]{64}$/u;
const revisionPattern = /^[0-9a-f]{40}$/u;
const repositoryPattern = /^[A-Za-z0-9_.-]{1,100}\/[A-Za-z0-9_.-]{1,100}$/u;
const platforms = ['linux/amd64', 'linux/arm64'];
const attestationType = 'application/vnd.docker.attestation.manifest.v1+json';
const statementType = 'https://in-toto.io/Statement/v1';
const registryUrl = 'https://ghcr.io';
const maxStatementBytes = 80 * 1024 * 1024;
const maxTotalStatementBytes = 192 * 1024 * 1024;
const predicateTypes = {
  sbom: 'https://spdx.dev/Document',
  provenance: 'https://slsa.dev/provenance/v1',
};

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function isDigest(value) {
  return typeof value === 'string' && digestPattern.test(value);
}

function parseJson(value, label) {
  try {
    return JSON.parse(value);
  } catch {
    throw new Error(`${label} is invalid JSON`);
  }
}

function parseImageRepository(imageReference) {
  const match = /^ghcr\.io\/([a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?)\/trademind(?::(?:backend|admin|collector)-sha-[0-9a-f]{40})?@sha256:[0-9a-f]{64}$/u.exec(imageReference);
  assert(match, 'candidate image must be a digest-pinned TradeMind GHCR image');
  return `${match[1]}/trademind`;
}

function validateCredentials(username, password) {
  assert(
    typeof username === 'string'
      && username.length > 0
      && username.length <= 128
      && !/[\r\n:]/u.test(username)
      && typeof password === 'string'
      && password.length > 0
      && password.length <= 8192
      && !/[\r\n]/u.test(password),
    'GHCR read credentials are missing or invalid',
  );
}

async function readLimitedBody(response, maximumBytes) {
  assert(response.body, 'GHCR returned an empty response body');
  const chunks = [];
  let totalBytes = 0;

  try {
    for await (const chunk of response.body) {
      totalBytes += chunk.byteLength;
      assert(totalBytes <= maximumBytes, 'GHCR response exceeds its declared size limit');
      chunks.push(Buffer.from(chunk));
    }
  } catch (error) {
    if (error instanceof Error && error.message === 'GHCR response exceeds its declared size limit') throw error;
    throw new Error('GHCR response could not be read');
  }

  return Buffer.concat(chunks, totalBytes);
}

async function requestRegistryToken(repository, username, password, fetchImpl) {
  const tokenUrl = new URL('/token', registryUrl);
  tokenUrl.searchParams.set('service', 'ghcr.io');
  tokenUrl.searchParams.set('scope', `repository:${repository}:pull`);

  let response;
  try {
    response = await fetchImpl(tokenUrl, {
      headers: {
        authorization: `Basic ${Buffer.from(`${username}:${password}`, 'utf8').toString('base64')}`,
        accept: 'application/json',
      },
      signal: AbortSignal.timeout(20_000),
    });
  } catch {
    throw new Error('GHCR read-token request failed');
  }

  assert(response.ok, 'GHCR denied the read-token request');
  const payload = parseJson((await readLimitedBody(response, 16 * 1024)).toString('utf8'), 'GHCR token response');
  const token = typeof payload?.token === 'string' ? payload.token : payload?.access_token;
  assert(typeof token === 'string' && token.length > 0 && token.length <= 8192, 'GHCR returned an invalid read token');
  return token;
}

/** Fetch bounded GHCR in-toto layers and verify their descriptor size and digest before parsing. */
export async function fetchVerifiedStatementBlobs(
  imageReference,
  descriptors,
  username,
  password,
  fetchImpl = globalThis.fetch,
) {
  validateCredentials(username, password);
  assert(typeof fetchImpl === 'function', 'a fetch implementation is required');
  const repository = parseImageRepository(imageReference);
  assert(Array.isArray(descriptors) && descriptors.length > 0 && descriptors.length <= 4, 'attestation statement descriptors are invalid');

  const uniqueDescriptors = new Map();
  let totalDeclaredBytes = 0;
  for (const descriptor of descriptors) {
    assert(
      descriptor && isDigest(descriptor.digest)
        && Number.isSafeInteger(descriptor.size)
        && descriptor.size > 0
        && descriptor.size <= maxStatementBytes,
      'attestation statement descriptor has an invalid size or digest',
    );
    const existing = uniqueDescriptors.get(descriptor.digest);
    assert(!existing || existing.size === descriptor.size, 'attestation statement descriptor sizes conflict');
    if (existing) continue;
    totalDeclaredBytes += descriptor.size;
    assert(totalDeclaredBytes <= maxTotalStatementBytes, 'attestation statements exceed the total size limit');
    uniqueDescriptors.set(descriptor.digest, descriptor);
  }

  const bearerToken = await requestRegistryToken(repository, username, password, fetchImpl);
  const statements = Object.create(null);

  for (const descriptor of uniqueDescriptors.values()) {
    let response;
    try {
      response = await fetchImpl(`${registryUrl}/v2/${repository}/blobs/${descriptor.digest}`, {
        headers: {
          authorization: `Bearer ${bearerToken}`,
          accept: 'application/vnd.in-toto+json, application/octet-stream',
        },
        signal: AbortSignal.timeout(60_000),
      });
    } catch {
      throw new Error('GHCR attestation statement request failed');
    }

    assert(response.ok, 'GHCR denied an attestation statement request');
    const body = await readLimitedBody(response, descriptor.size);
    assert(body.byteLength === descriptor.size, 'GHCR attestation statement size does not match its descriptor');
    const actualDigest = `sha256:${createHash('sha256').update(body).digest('hex')}`;
    assert(actualDigest === descriptor.digest, 'GHCR attestation statement digest mismatch');

    const statement = parseJson(body.toString('utf8'), 'GHCR attestation statement');
    assert(statement && typeof statement === 'object' && !Array.isArray(statement), 'GHCR attestation statement is invalid');
    statements[descriptor.digest] = statement;
  }

  return statements;
}

function findAttestationLayer(attestation, platform, kind, predicateType) {
  const layers = Array.isArray(attestation.layers)
    ? attestation.layers.filter((layer) => layer.annotations?.['in-toto.io/predicate-type'] === predicateType)
    : [];
  assert(
    layers.length === 1
      && layers[0].mediaType === 'application/vnd.in-toto+json'
      && isDigest(layers[0].digest)
      && Number.isSafeInteger(layers[0].size)
      && layers[0].size > 0
      && layers[0].size <= maxStatementBytes,
    `${platform} ${kind} attestation statement is missing or invalid`,
  );
  return layers[0];
}

function verifySourceConfigSource(uri, expectedRepository, expectedRevision, expectedSourceSubdir) {
  let source;
  try {
    source = new URL(uri);
  } catch {
    throw new Error('provenance source repository URI is invalid');
  }

  const sourcePath = source.pathname.replace(/\.git$/u, '').replace(/^\/|\/$/gu, '').toLowerCase();
  const normalizedRepository = expectedRepository.toLowerCase();
  const sourceSubdir = expectedSourceSubdir === '.' ? '' : expectedSourceSubdir;
  const expectedFragment = `${expectedRevision}${sourceSubdir ? `:${sourceSubdir}` : ''}`;
  assert(
    source.protocol === 'https:'
      && source.hostname.toLowerCase() === 'github.com'
      && source.port === ''
      && source.username === ''
      && source.password === ''
      && source.search === ''
      && sourcePath === normalizedRepository
      && source.hash === `#${expectedFragment}`,
    'provenance source repository or context path does not match the candidate',
  );
}

/** Verify platform subjects, in-toto statement bodies, and their repository/revision claims. */
export function verifyPlatformAttestations({
  index,
  attestations,
  statements,
  expectedRevision,
  expectedRepository,
  expectedSourceSubdir = '.',
  expectedSourcePath,
  platformDigests,
}) {
  assert(revisionPattern.test(expectedRevision), 'expected source revision must be a full Git SHA');
  assert(repositoryPattern.test(expectedRepository), 'expected source repository is invalid');
  assert(expectedSourceSubdir === '.' || /^[A-Za-z0-9._/-]{1,128}$/u.test(expectedSourceSubdir), 'expected source context path is invalid');
  assert(
    typeof expectedSourcePath === 'string'
      && /^(?:[A-Za-z0-9_.-]+\/)*[A-Za-z0-9_.-]+$/u.test(expectedSourcePath)
      && !expectedSourcePath.split('/').some((part) => part === '.' || part === '..'),
    'expected Dockerfile source path is invalid',
  );
  assert(index && Array.isArray(index.manifests), 'image index is missing its manifest list');

  for (const platform of platforms) {
    const digest = platformDigests?.[platform];
    assert(isDigest(digest), `${platform} image digest is invalid`);

    const [os, architecture] = platform.split('/');
    const images = index.manifests.filter((entry) => entry.platform?.os === os && entry.platform?.architecture === architecture);
    assert(images.length === 1 && images[0].digest === digest, `${platform} image digest does not match the index`);

    const descriptors = index.manifests.filter((entry) =>
      entry.annotations?.['vnd.docker.reference.type'] === 'attestation-manifest'
      && entry.annotations?.['vnd.docker.reference.digest'] === digest,
    );
    assert(descriptors.length === 1 && isDigest(descriptors[0].digest), `${platform} must have one attached attestation manifest`);

    const attestation = attestations?.[descriptors[0].digest];
    assert(attestation?.artifactType === attestationType, `${platform} attestation manifest is unavailable or invalid`);
    assert(attestation.subject?.digest === digest, `${platform} attestation subject digest does not match its image`);

    for (const [kind, predicateType] of Object.entries(predicateTypes)) {
      const layer = findAttestationLayer(attestation, platform, kind, predicateType);
      const statement = statements?.[layer.digest];
      assert(statement && typeof statement === 'object', `${platform} ${kind} attestation statement is unavailable`);
      assert(statement._type === statementType, `${platform} ${kind} attestation statement type is invalid`);
      assert(statement.predicateType === predicateType, `${platform} ${kind} attestation predicate type does not match its descriptor`);
      assert(
        Array.isArray(statement.subject)
          && statement.subject.some((subject) => subject?.digest?.sha256 === digest.slice('sha256:'.length)),
        `${platform} ${kind} statement subject digest does not match its image`,
      );
      assert(statement.predicate && typeof statement.predicate === 'object' && !Array.isArray(statement.predicate), `${platform} ${kind} attestation predicate is invalid`);

      if (kind === 'sbom') {
        assert(
          typeof statement.predicate.spdxVersion === 'string'
            && typeof statement.predicate.SPDXID === 'string',
          `${platform} SBOM predicate is missing its SPDX document`,
        );
        continue;
      }

      const configSource = statement.predicate.buildDefinition?.externalParameters?.configSource;
      assert(configSource && typeof configSource === 'object', `${platform} provenance source claim is missing`);
      assert(configSource.digest?.sha1 === expectedRevision, `${platform} provenance source revision does not match the candidate`);
      verifySourceConfigSource(configSource.uri, expectedRepository, expectedRevision, expectedSourceSubdir);
      assert(configSource.path === expectedSourcePath, `${platform} provenance source path does not match the candidate Dockerfile`);
    }
  }

  return true;
}

function inspect(reference, args = []) {
  return execFileSync('docker', ['buildx', 'imagetools', 'inspect', ...args, reference], {
    encoding: 'utf8',
    maxBuffer: 16 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

async function verifyImage(
  imageReference,
  expectedRevision,
  amd64Digest,
  arm64Digest,
  expectedRepository,
  expectedSourceSubdir,
  expectedSourcePath,
) {
  const imageRepository = `ghcr.io/${parseImageRepository(imageReference)}`;
  const index = parseJson(inspect(imageReference, ['--raw']), 'candidate image index');
  const platformDigests = {
    'linux/amd64': amd64Digest,
    'linux/arm64': arm64Digest,
  };
  const attestations = {};

  for (const platform of platforms) {
    const digest = platformDigests[platform];
    const [os, architecture] = platform.split('/');
    const image = index.manifests?.filter((entry) => entry.platform?.os === os && entry.platform?.architecture === architecture);
    assert(image?.length === 1 && image[0].digest === digest, `${platform} image digest does not match the index`);
    const descriptor = index.manifests.filter((entry) =>
      entry.annotations?.['vnd.docker.reference.type'] === 'attestation-manifest'
      && entry.annotations?.['vnd.docker.reference.digest'] === digest,
    );
    assert(descriptor.length === 1 && isDigest(descriptor[0].digest), `${platform} must have one attached attestation manifest`);
    attestations[descriptor[0].digest] = parseJson(
      inspect(`${imageRepository}@${descriptor[0].digest}`, ['--raw']),
      `${platform} attestation manifest`,
    );
  }

  const statementLayers = Object.values(attestations).flatMap((attestation) =>
    Array.isArray(attestation?.layers)
      ? attestation.layers.filter((layer) => Object.values(predicateTypes).includes(layer.annotations?.['in-toto.io/predicate-type']))
      : [],
  );
  const statements = await fetchVerifiedStatementBlobs(
    imageReference,
    statementLayers,
    process.env.GHCR_USERNAME,
    process.env.GHCR_TOKEN,
  );

  return verifyPlatformAttestations({
    index,
    attestations,
    statements,
    expectedRevision,
    expectedRepository,
    expectedSourceSubdir,
    expectedSourcePath,
    platformDigests,
  });
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const [, , imageReference, expectedRevision, amd64Digest, arm64Digest, expectedRepository, expectedSourceSubdir, expectedSourcePath] = process.argv;
  if (!imageReference || !expectedRevision || !amd64Digest || !arm64Digest || !expectedRepository || !expectedSourceSubdir || !expectedSourcePath) process.exit(2);

  try {
    await verifyImage(imageReference, expectedRevision, amd64Digest, arm64Digest, expectedRepository, expectedSourceSubdir, expectedSourcePath);
    process.stdout.write('Verified platform SBOM and provenance statement bindings for linux/amd64 and linux/arm64.\n');
  } catch (error) {
    const message = error instanceof Error ? error.message : 'unknown attestation verification error';
    process.stderr.write(`::error::Image attestation verification failed: ${message}\n`);
    process.exit(1);
  }
}
