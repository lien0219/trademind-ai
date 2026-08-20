import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const root = path.resolve(__dirname, '..', '..');
const workflow = readFileSync(path.join(root, '.github', 'workflows', 'container-images.yml'), 'utf8');
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

  it('automatically publishes only main branch builds and v-prefixed release tags', () => {
    expect(workflow).toContain('    branches:\n      - main\n    tags:');
    expect(workflow).not.toContain('      - dev');
    expect(workflow).not.toContain('      - "feat/**"');
    expect(workflow).not.toContain('      - "fix/**"');
    expect(workflow).not.toContain('      - "release/**"');
    expect(workflow).toContain('    tags:\n      - "v*"');
    expect(workflow).toContain('"$REF_TYPE" == "branch" && "$GITHUB_REF_NAME" != "main"');
  });

  it('publishes all service images under one GHCR package with isolated tag prefixes', () => {
    expect(workflow).toContain('IMAGE_NAME: trademind');
    expect(workflow).toContain(
      'images: ${{ env.REGISTRY }}/${{ needs.image_metadata.outputs.namespace }}/${{ env.IMAGE_NAME }}',
    );
    expect(workflow).not.toContain('image: trademind-backend');
    expect(workflow).not.toContain('image: trademind-admin');
    expect(workflow).not.toContain('image: trademind-collector');
    expect(workflow).toContain('type=raw,value=${{ matrix.service }}-sha-${{ github.sha }}');
  });

  it('requires the release tag to match IMAGE_VERSION', () => {
    expect(workflow).toContain('expected_tag="v${version}"');
    expect(workflow).toContain(
      'Release tag $GITHUB_REF_NAME must exactly match deploy/IMAGE_VERSION as $expected_tag',
    );
  });

  it('requires the tagged commit to be contained in main', () => {
    expect(workflow).toContain('git fetch --no-tags origin +refs/heads/main:refs/remotes/origin/main');
    expect(workflow).toContain('git merge-base --is-ancestor "$release_commit" refs/remotes/origin/main');
  });

  it('publishes stable version tags and latest only for releases', () => {
    expect(workflow).toContain(
      "type=raw,value=${{ matrix.service }}-v${{ needs.image_metadata.outputs.version }},enable=${{ needs.image_metadata.outputs.is_release == 'true' }}",
    );
    expect(workflow).toContain(
      "type=raw,value=${{ matrix.service }}-${{ needs.image_metadata.outputs.version }},enable=${{ needs.image_metadata.outputs.is_release == 'true' }}",
    );
    expect(workflow).toContain(
      "type=raw,value=${{ matrix.service }}-latest,enable=${{ needs.image_metadata.outputs.is_release == 'true' }}",
    );
    expect(workflow).not.toContain("needs.image_metadata.outputs.ref_slug == 'main'");
  });

  it('keeps branch and branch-version tags on non-release builds', () => {
    expect(workflow).toContain(
      "type=raw,value=${{ matrix.service }}-${{ needs.image_metadata.outputs.ref_slug }},enable=${{ needs.image_metadata.outputs.is_release != 'true' }}",
    );
    expect(workflow).toContain(
      "type=raw,value=${{ matrix.service }}-${{ needs.image_metadata.outputs.ref_slug }}-v${{ needs.image_metadata.outputs.version }},enable=${{ needs.image_metadata.outputs.is_release != 'true' }}",
    );
  });
});
