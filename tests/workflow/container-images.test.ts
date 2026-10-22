import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { parse } from 'yaml';
import { describe, expect, it } from 'vitest';
import { sanitizeTrivyReport } from '../../scripts/release/sanitize-trivy-report.mjs';
import { failedImageScanChecks } from '../../scripts/release/enforce-image-scan-outcomes.mjs';
import {
  fetchVerifiedStatementBlobs,
  verifyPlatformAttestations,
} from '../../scripts/release/verify-image-attestations.mjs';

const root = path.resolve(__dirname, '..', '..');
const workflow = readFileSync(path.join(root, '.github', 'workflows', 'container-images.yml'), 'utf8').replace(/\r\n/gu, '\n');
const releaseWorkflow = readFileSync(path.join(root, '.github', 'workflows', 'release.yml'), 'utf8');
const productionCompose = readFileSync(path.join(root, 'deploy', 'production', 'compose.yml'), 'utf8');
const packageJson = JSON.parse(readFileSync(path.join(root, 'package.json'), 'utf8')) as {
  packageManager: string;
  pnpm: { onlyBuiltDependencies?: string[] };
  scripts: Record<string, string>;
};
const pnpmVersion = packageJson.packageManager.replace(/^pnpm@/u, '');
const nodeDockerfiles = ['admin/Dockerfile', 'collector/Dockerfile'];
const collectorDockerfile = readFileSync(path.join(root, 'collector', 'Dockerfile'), 'utf8');
const workflowConfig = parse(workflow) as { jobs: { scan: { steps: Array<{ id?: string; with?: Record<string, unknown>; env?: Record<string, string>; run?: string }> } } };
const postinstallScript = 'scripts/patch-pro-field-antd-select.mjs';
const dockerIgnorePatterns = readFileSync(path.join(root, '.dockerignore'), 'utf8')
  .split(/\r?\n/u)
  .filter(Boolean);

describe('container image release workflow', () => {
  it('installs the repository pnpm version without relying on Corepack', () => {
    for (const dockerfile of nodeDockerfiles) {
      const source = readFileSync(path.join(root, dockerfile), 'utf8');
      expect(source, dockerfile).toContain(`npm install --global pnpm@${pnpmVersion}`);
      expect(source, dockerfile).not.toContain('corepack');
    }
  });

  it('includes the root postinstall script in Node image dependency installation', () => {
    expect(packageJson.scripts.postinstall).toBe(`node ${postinstallScript}`);
    expect(dockerIgnorePatterns).not.toContain('scripts');
    expect(dockerIgnorePatterns).toContain('scripts/*');
    expect(dockerIgnorePatterns).toContain(`!${postinstallScript}`);
    expect(workflow).toContain(`      - "${postinstallScript}"`);

    for (const dockerfile of nodeDockerfiles) {
      const source = readFileSync(path.join(root, dockerfile), 'utf8');
      const copyIndex = source.indexOf(`COPY ${postinstallScript} ./scripts/`);
      const installIndex = source.indexOf('RUN pnpm install --frozen-lockfile');

      expect(copyIndex, `${dockerfile} must copy the root postinstall script`).toBeGreaterThanOrEqual(0);
      expect(installIndex, `${dockerfile} must install dependencies after copying lifecycle scripts`).toBeGreaterThan(
        copyIndex,
      );
    }
  });

  it('approves only the esbuild install script required by the workspace toolchain', () => {
    expect(packageJson.pnpm.onlyBuiltDependencies).toEqual(['esbuild']);
  });

  it('uses a clean Node runtime and installs the lockstep Chromium browser', () => {
    expect(collectorDockerfile).toContain('FROM node:24-bookworm-slim AS runtime');
    expect(collectorDockerfile).not.toContain('mcr.microsoft.com/playwright');
    expect(collectorDockerfile).toContain('PLAYWRIGHT_BROWSERS_PATH=/ms-playwright');
    expect(collectorDockerfile).toContain('./node_modules/.bin/playwright install --with-deps chromium');
    expect(collectorDockerfile).toContain('USER node');
    expect(collectorDockerfile).toContain('["node", "dist/index.js"]');
    expect(collectorDockerfile.slice(collectorDockerfile.indexOf('AS runtime'))).not.toContain('pnpm install --frozen-lockfile');
  });

  it('deploys Collector with pnpm 10 support for the existing non-injected workspace', () => {
    expect(collectorDockerfile).toContain('deploy --prod --legacy /out/collector');
  });

  it('publishes only from main and rejects branch/tag publishing outside that source', () => {
    expect(workflow).toContain('    branches:\n      - main');
    expect(workflow).not.toContain('      - dev');
    expect(workflow).not.toContain('      - "v*"');
    expect(workflow).toContain('"${GITHUB_REF_TYPE}" != "branch" || "$GITHUB_REF_NAME" != "main"');
    expect(workflow).toContain('cancel-in-progress: false');
  });

  it('uses one GHCR package and commit-scoped candidate tags', () => {
    expect(workflow).toContain('IMAGE_NAME: trademind');
    expect(workflow).toContain(
      'images: ${{ env.REGISTRY }}/${{ needs.image_metadata.outputs.namespace }}/${{ env.IMAGE_NAME }}',
    );
    expect(workflow).not.toContain('image: trademind-backend');
    expect(workflow).not.toContain('image: trademind-admin');
    expect(workflow).not.toContain('image: trademind-collector');
    expect(workflow).toContain('type=raw,value=${{ matrix.service }}-sha-${{ needs.image_metadata.outputs.commit }}');
    expect(workflow).toContain('org.opencontainers.image.revision=${{ needs.image_metadata.outputs.commit }}');
    expect(workflow).toContain('provenance: mode=max,version=v1');
    expect(workflow).toContain('sbom: true');
    expect(workflow).toContain('outputs: type=image,oci-mediatypes=true,oci-artifact=true');
    expect(workflow).toContain(
      'context: https://github.com/${{ github.repository }}.git#${{ needs.image_metadata.outputs.commit }}${{ matrix.git_context_suffix }}',
    );
    expect(workflow).toContain('git_context_suffix: ":backend"');
    expect(workflow).toContain('github-token: ${{ secrets.GITHUB_TOKEN }}');
    expect(workflow).toContain('EXPECTED_REPOSITORY: ${{ github.repository }}');
    expect(workflow).toContain('EXPECTED_SOURCE_SUBDIR: ${{ matrix.source_subdir }}');
    expect(workflow).toContain('EXPECTED_SOURCE_PATH: ${{ matrix.source_path }}');
    expect(workflow).toContain('aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25');
    expect(workflow).toContain('          version: v0.76.1');
  });

  it('scans each immutable platform manifest with blocking vulnerability and secret checks', () => {
    expect(workflow).toContain('  scan:');
    expect(workflow).toContain('needs: [image_metadata, publish]');
    expect(workflow).toContain('    packages: read');
    expect(workflow).toContain('id: platforms');
    expect(workflow).toContain('steps.platforms.outputs.amd64_digest');
    expect(workflow).toContain('steps.platforms.outputs.arm64_digest');
    expect(
      workflow.match(/uses: aquasecurity\/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25/gu),
    ).toHaveLength(4);
    expect(workflow.match(/severity: CRITICAL,HIGH/gu)).toHaveLength(2);
    expect(workflow.match(/ignore-unfixed: true/gu)).toHaveLength(2);
    expect(workflow.match(/exit-code: "1"/gu)).toHaveLength(4);
    expect(workflow.match(/scanners: vuln\b/gu)).toHaveLength(2);
    expect(workflow.match(/scanners: secret\b/gu)).toHaveLength(2);
    expect(workflow).not.toContain('continue-on-error');
    expect(workflow).toContain('"scripts/release/enforce-image-scan-outcomes.mjs"');
    expect(workflow).toContain('if: ${{ always() && steps.platforms.outcome == \'success\' }}');
    for (const platform of ['amd64', 'arm64']) {
      const vulnerabilityScan = workflowConfig.jobs.scan.steps.find((step) => step.id === `scan_${platform}`);
      const secretScan = workflowConfig.jobs.scan.steps.find((step) => step.id === `secret_${platform}`);
      expect(vulnerabilityScan?.with).toMatchObject({ scanners: 'vuln', severity: 'CRITICAL,HIGH', 'ignore-unfixed': true, 'exit-code': '1' });
      expect(secretScan?.with).toMatchObject({ scanners: 'secret', 'exit-code': '1' });
      expect(secretScan?.with).not.toHaveProperty('severity');
      expect(secretScan?.with).not.toHaveProperty('ignore-unfixed');
      expect(workflow).toContain(`image-metadata/trivy-${platform}-vuln.json`);
      expect(workflow).toContain(`image-metadata/trivy-${platform}-secret.json`);
    }
  });

  it('provides the attestation source context and Dockerfile on each scan matrix row', () => {
    const scanMatrix = workflowConfig.jobs.scan.steps;
    const parsed = parse(workflow) as {
      jobs: {
        scan: { strategy: { matrix: { include: Array<{ service: string; source_subdir: string; source_path: string }> } } };
        publish: { strategy: { matrix: { include: Array<Record<string, unknown>> } } };
      };
    };
    expect(parsed.jobs.scan.strategy.matrix.include).toEqual([
      { service: 'backend', source_subdir: 'backend', source_path: 'Dockerfile' },
      { service: 'admin', source_subdir: '.', source_path: 'admin/Dockerfile' },
      { service: 'collector', source_subdir: '.', source_path: 'collector/Dockerfile' },
    ]);
    expect(parsed.jobs.publish.strategy.matrix.include.every((row) => !('source_subdir' in row) && !('source_path' in row))).toBe(true);
    expect(scanMatrix.find((step) => step.id === 'platforms')?.env).toMatchObject({
      EXPECTED_SOURCE_SUBDIR: '${{ matrix.source_subdir }}',
      EXPECTED_SOURCE_PATH: '${{ matrix.source_path }}',
    });
  });

  it('requires attestation verification and every security scan outcome before promotion', () => {
    expect(workflow).toContain('node scripts/release/verify-image-attestations.mjs "$image" "$EXPECTED_REVISION" "$amd64_digest" "$arm64_digest" "$EXPECTED_REPOSITORY" "$EXPECTED_SOURCE_SUBDIR" "$EXPECTED_SOURCE_PATH"');
    const enforce = workflowConfig.jobs.scan.steps.find((step) => step.run?.includes('enforce-image-scan-outcomes.mjs'));
    expect(enforce?.env).toMatchObject({
      SECRET_AMD64_OUTCOME: '${{ steps.secret_amd64.outcome }}',
      SECRET_ARM64_OUTCOME: '${{ steps.secret_arm64.outcome }}',
    });
    expect(enforce?.run).toContain('node scripts/release/enforce-image-scan-outcomes.mjs');
    expect(failedImageScanChecks({
      PLATFORM: 'success',
      AMD64: 'success',
      ARM64: 'success',
      SECRET_AMD64: 'success',
      SECRET_ARM64: 'success',
      SANITIZE: 'success',
    })).toEqual([]);
    expect(failedImageScanChecks({
      PLATFORM: 'success',
      AMD64: 'success',
      ARM64: 'failure',
      SECRET_AMD64: 'success',
      SECRET_ARM64: 'success',
      SANITIZE: 'success',
    })).toEqual(['ARM64']);
  });

  it('binds each platform SBOM and provenance to its manifest and source revision', async () => {
    const expectedRevision = 'a'.repeat(40);
    const expectedRepository = 'lien0219/trademind-ai';
    const platformDigests = {
      'linux/amd64': `sha256:${'1'.repeat(64)}`,
      'linux/arm64': `sha256:${'2'.repeat(64)}`,
    };
    const statements: Record<string, Record<string, unknown>> = {};
    const attestations = Object.fromEntries(Object.entries(platformDigests).map(([platform, digest]) => {
      const attestationDigest = `sha256:${platform.endsWith('amd64') ? '3' : '4'}${'0'.repeat(63)}`;
      const layers = ['sbom', 'provenance'].map((kind) => {
        const predicateType = kind === 'sbom' ? 'https://spdx.dev/Document' : 'https://slsa.dev/provenance/v1';
        const statement = {
          _type: 'https://in-toto.io/Statement/v1',
          subject: [{ name: `ghcr.io/lien0219/trademind:${platform}`, digest: { sha256: digest.slice('sha256:'.length) } }],
          predicateType,
          predicate: kind === 'sbom'
            ? { spdxVersion: 'SPDX-2.3', SPDXID: 'SPDXRef-DOCUMENT', dataLicense: 'CC0-1.0' }
            : {
              buildDefinition: {
                externalParameters: {
                  configSource: {
                    uri: `https://github.com/${expectedRepository}.git#${expectedRevision}`,
                    path: 'admin/Dockerfile',
                    digest: { sha1: expectedRevision },
                  },
                },
              },
            },
        };
        const body = JSON.stringify(statement);
        const layerDigest = `sha256:${createHash('sha256').update(body).digest('hex')}`;
        statements[layerDigest] = statement;
        return {
          mediaType: 'application/vnd.in-toto+json',
          digest: layerDigest,
          size: Buffer.byteLength(body),
          annotations: { 'in-toto.io/predicate-type': predicateType },
        };
      });
      return [attestationDigest, {
        artifactType: 'application/vnd.docker.attestation.manifest.v1+json',
        subject: { digest },
        layers,
      }];
    }));
    const index = {
      manifests: [
        ...Object.entries(platformDigests).map(([platform, digest]) => {
          const [os, architecture] = platform.split('/');
          return { digest, platform: { os, architecture } };
        }),
        ...Object.entries(platformDigests).map(([platform, digest]) => ({
          digest: `sha256:${platform.endsWith('amd64') ? '3' : '4'}${'0'.repeat(63)}`,
          annotations: { 'vnd.docker.reference.type': 'attestation-manifest', 'vnd.docker.reference.digest': digest },
        })),
      ],
    };
    const input = {
      index,
      attestations,
      statements,
      expectedRepository,
      expectedSourceSubdir: '.',
      expectedSourcePath: 'admin/Dockerfile',
      expectedRevision,
      platformDigests,
    };

    expect(verifyPlatformAttestations(input)).toBe(true);
    expect(() => verifyPlatformAttestations({
      ...input,
      attestations: Object.fromEntries(Object.entries(attestations).map(([digest, manifest]) => [
        digest,
        { ...manifest, subject: { digest: platformDigests['linux/arm64'] } },
      ])),
    })).toThrow(/subject digest/u);
    expect(() => verifyPlatformAttestations({
      ...input,
      statements: Object.fromEntries(Object.entries(statements).map(([digest, statement]) => [
        digest,
        digest === Object.values(attestations)[0].layers[1].digest
          ? {
            ...statement,
            predicate: {
              buildDefinition: {
                externalParameters: {
                  configSource: { uri: `https://github.com/${expectedRepository}.git`, digest: { sha1: 'b'.repeat(40) } },
                },
              },
            },
          }
          : statement,
      ])),
    })).toThrow(/source revision/u);
    expect(() => verifyPlatformAttestations({ ...input, expectedRepository: 'other/project' })).toThrow(/source repository/u);
    expect(() => verifyPlatformAttestations({ ...input, expectedSourceSubdir: 'backend' })).toThrow(/context path/u);
    expect(() => verifyPlatformAttestations({ ...input, expectedSourcePath: 'collector/Dockerfile' })).toThrow(/source path/u);
    expect(() => verifyPlatformAttestations({ ...input, statements: {} })).toThrow(/statement/u);
    expect(() => verifyPlatformAttestations({
      ...input,
      statements: Object.fromEntries(Object.entries(statements).map(([digest, statement]) => [
        digest,
        digest === Object.values(attestations)[0].layers[0].digest
          ? { ...statement, subject: [{ digest: { sha256: 'f'.repeat(64) } }] }
          : statement,
      ])),
    })).toThrow(/subject digest/u);
    expect(() => verifyPlatformAttestations({
      ...input,
      statements: Object.fromEntries(Object.entries(statements).map(([digest, statement]) => [
        digest,
        digest === Object.values(attestations)[0].layers[0].digest
          ? { ...statement, predicateType: 'https://example.invalid/other' }
          : statement,
      ])),
    })).toThrow(/predicate type/u);
  });

  it('blocks promotion for failure, cancellation, skipped scans, or missing outcomes', () => {
    const allSuccessful = {
      PLATFORM: 'success',
      AMD64: 'success',
      ARM64: 'success',
      SECRET_AMD64: 'success',
      SECRET_ARM64: 'success',
      SANITIZE: 'success',
    };
    expect(failedImageScanChecks(allSuccessful)).toEqual([]);
    for (const outcome of ['failure', 'cancelled', 'skipped', '']) {
      expect(failedImageScanChecks({ ...allSuccessful, SECRET_ARM64: outcome })).toEqual(['SECRET_ARM64']);
    }
    expect(failedImageScanChecks({ ...allSuccessful, SECRET_ARM64: undefined })).toEqual(['SECRET_ARM64']);
  });

  it('fetches only bounded GHCR statement blobs and verifies descriptor size and digest', async () => {
    const body = Buffer.from(JSON.stringify({ safe: true }));
    const digest = `sha256:${createHash('sha256').update(body).digest('hex')}`;
    const calls: Array<{ url: string; authorization?: string }> = [];
    const fetchMock = async (input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      const headers = new Headers(init?.headers);
      calls.push({ url, authorization: headers.get('authorization') ?? undefined });
      if (url.startsWith('https://ghcr.io/token?')) {
        return new Response(JSON.stringify({ token: 'mock-registry-token' }), { status: 200 });
      }
      if (url.startsWith('https://ghcr.io/v2/lien0219/trademind/blobs/')) {
        return new Response(body, { status: 200 });
      }
      throw new Error('unexpected registry request');
    };

    const loaded = await fetchVerifiedStatementBlobs(
      'ghcr.io/lien0219/trademind:admin-sha-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
      [{ digest, size: body.length }],
      'github-actions[bot]',
      'mock-github-token',
      fetchMock,
    );

    expect(loaded[digest]).toEqual({ safe: true });
    expect(calls).toHaveLength(2);
    expect(calls[0].authorization).toMatch(/^Basic /u);
    expect(calls[1].authorization).toBe('Bearer mock-registry-token');
    await expect(fetchVerifiedStatementBlobs(
      'ghcr.io/lien0219/trademind:admin-sha-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
      [{ digest: `sha256:${'f'.repeat(64)}`, size: body.length }],
      'github-actions[bot]',
      'mock-github-token',
      fetchMock,
    )).rejects.toThrow(/digest mismatch/u);

    await expect(fetchVerifiedStatementBlobs(
      'ghcr.io/lien0219/trademind:backend-main-v0.3.2@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
      [{ digest, size: body.length }],
      'github-actions[bot]',
      'mock-github-token',
      fetchMock,
    )).rejects.toThrow(/digest-pinned TradeMind GHCR image/u);
  });

  it('sanitizes reports without retaining secret matches or source code', () => {
    const secretCanary = 'NEVER-STORE-THIS-SECRET-CONTENT';
    const sanitized = sanitizeTrivyReport({
      Results: [
        {
          Class: 'os-pkgs',
          Type: 'alpine',
          Vulnerabilities: [
            {
              VulnerabilityID: 'CVE-2026-12345',
              PkgName: 'libexpat',
              InstalledVersion: '2.8.5-r0',
              FixedVersion: '2.9.0-r0',
              Severity: 'HIGH',
              Title: secretCanary,
              PrimaryURL: secretCanary,
            },
          ],
          Secrets: [{ Title: secretCanary, Match: secretCanary, Code: { Lines: [{ Content: secretCanary }] } }],
        },
      ],
      Errors: [{ Message: secretCanary }],
      Metadata: { ImageConfig: { env: [secretCanary] } },
    });

    const serialized = JSON.stringify(sanitized);
    expect(serialized).not.toContain(secretCanary);
    expect(serialized).toContain('CVE-2026-12345');
    expect(sanitized.secretFindings).toEqual({ total: 1 });
    expect(sanitized.vulnerabilities).toMatchObject({
      total: 1,
      severities: { HIGH: 1 },
      items: [
        {
          id: 'CVE-2026-12345',
          package: 'libexpat',
          installedVersion: '2.8.5-r0',
          fixedVersion: '2.9.0-r0',
          severity: 'HIGH',
        },
      ],
    });
  });

  it('accepts a valid successful container image report with omitted Results as a clean scan', () => {
    const cleanImageReport = {
      SchemaVersion: 2,
      ArtifactName: 'registry/image@sha256:abc',
      ArtifactType: 'container_image',
    };
    expect(sanitizeTrivyReport(cleanImageReport, 'success')).toMatchObject({
      status: 'complete',
      scannerOutcome: 'success',
      vulnerabilities: { total: 0, items: [] },
      secretFindings: { total: 0 },
    });
  });

  it('fails closed for omitted Results on failed scans or invalid Trivy report identities', () => {
    const cleanImageReport = {
      SchemaVersion: 2,
      ArtifactName: 'registry/image@sha256:abc',
      ArtifactType: 'container_image',
    };
    expect(sanitizeTrivyReport(cleanImageReport, 'failure').status).toBe('unavailable');
    expect(sanitizeTrivyReport({ ...cleanImageReport, SchemaVersion: 1 }, 'success').status).toBe('unavailable');
    expect(sanitizeTrivyReport({ ...cleanImageReport, ArtifactType: 'filesystem' }, 'success').status).toBe('unavailable');
    expect(sanitizeTrivyReport({ ...cleanImageReport, ArtifactName: '' }, 'success').status).toBe('unavailable');
    expect(sanitizeTrivyReport({ ...cleanImageReport, Results: null }, 'success').status).toBe('unavailable');
  });

  it('requires all three verified digests before moving main or starting a release', () => {
    expect(workflow).toContain('  quality_gate:');
    expect(workflow).toContain('run: pnpm workflow:verify');
    expect(workflow).toContain('go test ./...');
    expect(workflow).toContain('needs: [image_metadata, publish, scan, quality_gate]');
    expect(workflow).toContain('      - name: Set up Docker Buildx\n        uses: docker/setup-buildx-action@v3\n\n      - name: Login to GHCR');
    expect(workflow).toContain('group: trademind-production-publish');
    expect(workflow).toContain('Refusing to move main tags for stale commit');
    expect(workflow).toContain('docker buildx imagetools inspect "$image@$digest"');
    expect(workflow).toContain('needs: [image_metadata, verify]');
    expect(workflow).toContain('if: ${{ needs.image_metadata.outputs.publish_release != \'true\' }}');
    expect(workflow).toContain('if: ${{ needs.image_metadata.outputs.publish_release == \'true\' }}');
  });

  it('keeps formal tags immutable and creates Release only from validated inputs', () => {
    expect(releaseWorkflow).toContain('workflow_call:');
    expect(releaseWorkflow).not.toContain('workflow_dispatch:');
    expect(releaseWorkflow).toContain('contents: write');
    expect(releaseWorkflow).toContain('group: trademind-production-publish');
    expect(releaseWorkflow).toContain('git merge-base --is-ancestor "$release_commit" origin/main');
    expect(releaseWorkflow).toContain('node scripts/release/validate-release-version.mjs "$RELEASE_VERSION"');
    expect(releaseWorkflow).toContain('tag="v$RELEASE_VERSION"');
    expect(releaseWorkflow).toContain('formal="$image:$service-main-v$RELEASE_VERSION"');
    expect(releaseWorkflow).toContain('already points to $existing, expected $digest');
    expect(releaseWorkflow).toContain('Could not verify whether immutable image $formal exists');
    expect(releaseWorkflow).toContain('keeping movable main tags unchanged instead of regressing them');
    expect(releaseWorkflow).toContain('Create or update GitHub Release');
    expect(releaseWorkflow).not.toContain('service-latest');
    expect(releaseWorkflow).not.toContain('service-v$RELEASE_VERSION');
  });

  it('keeps production Compose pull-only and digest-pinned', () => {
    expect(productionCompose).not.toContain('build:');
    expect(productionCompose).toContain('TRADEMIND_BACKEND_IMAGE:?');
    expect(productionCompose).toContain('TRADEMIND_ADMIN_IMAGE:?');
    expect(productionCompose).toContain('TRADEMIND_COLLECTOR_IMAGE:?');
    expect(productionCompose).toContain('condition: service_healthy');
    expect(productionCompose).toContain('trademind_production_pg_data');
    expect(productionCompose).toContain('trademind_production_collector_profiles');
  });
});
