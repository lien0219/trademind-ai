import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const root = path.resolve(__dirname, '..', '..');
const workflow = readFileSync(path.join(root, '.github', 'workflows', 'container-images.yml'), 'utf8').replace(/\r\n/gu, '\n');
const releaseWorkflow = readFileSync(path.join(root, '.github', 'workflows', 'release.yml'), 'utf8');
const productionCompose = readFileSync(path.join(root, 'deploy', 'production', 'compose.yml'), 'utf8');
const packageJson = JSON.parse(readFileSync(path.join(root, 'package.json'), 'utf8')) as {
  packageManager: string;
  scripts: Record<string, string>;
};
const pnpmVersion = packageJson.packageManager.replace(/^pnpm@/u, '');
const nodeDockerfiles = ['admin/Dockerfile', 'collector/Dockerfile'];
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
    expect(workflow).toContain('provenance: mode=max');
    expect(workflow).toContain('sbom: true');
    expect(workflow).toContain('aquasecurity/trivy-action@v0.28.0');
  });

  it('requires all three verified digests before moving main or starting a release', () => {
    expect(workflow).toContain('  quality_gate:');
    expect(workflow).toContain('run: pnpm workflow:verify');
    expect(workflow).toContain('go test ./...');
    expect(workflow).toContain('needs: [image_metadata, publish, quality_gate]');
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
