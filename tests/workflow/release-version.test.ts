import { describe, expect, it } from 'vitest';
import { compareVersions, findHighestReleaseVersion } from '../../scripts/release/release-version.mjs';

describe('release version ordering', () => {
  it('orders stable releases above prereleases', () => {
    expect(compareVersions('1.0.0', '1.0.0-rc.1')).toBe(1);
  });

  it('orders numeric prerelease identifiers numerically', () => {
    expect(compareVersions('1.0.0-rc.2', '1.0.0-rc.10')).toBe(-1);
  });

  it('orders numeric prerelease identifiers below nonnumeric identifiers', () => {
    expect(compareVersions('1.0.0-1', '1.0.0-alpha')).toBe(-1);
  });

  it('includes existing formal image tags when finding the highest version', () => {
    expect(findHighestReleaseVersion(['v0.2.1'], ['backend-main-v0.3.1', 'admin-sha-abc'])).toMatchObject({
      version: '0.3.1',
    });
  });
});
