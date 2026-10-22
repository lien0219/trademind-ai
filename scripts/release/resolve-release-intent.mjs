import path from 'node:path';
import { fileURLToPath } from 'node:url';

const shaPattern = /^[0-9a-f]{40}$/u;

// Unreleased versions automatically publish after verification, even if the
// version change happened in a prior failed build. Published tags are immutable.
export function shouldPublishVersion({
  currentCommit,
  tagCommit = '',
  releaseExists = false,
  versionChanged = false,
  manualRequested = false,
}) {
  if (!shaPattern.test(currentCommit) || (tagCommit && !shaPattern.test(tagCommit))) {
    throw new Error('Invalid release source commit or existing tag target');
  }
  if (releaseExists && !tagCommit) {
    throw new Error('GitHub Release exists but its version Git tag is missing');
  }
  if (tagCommit && tagCommit !== currentCommit) {
    if (!releaseExists || versionChanged || manualRequested) {
      throw new Error(`Version tag already points to ${tagCommit}; refusing to rebind it to ${currentCommit}`);
    }
    return false;
  }
  return !releaseExists;
}

function parseBoolean(value) {
  if (value === 'true') return true;
  if (value === 'false') return false;
  throw new Error(`Invalid boolean release flag: ${value}`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const args = process.argv.slice(2);
    if (args.length !== 5) throw new Error('Expected commit, tag commit, release exists, version changed, manual requested');
    const [currentCommit, tagCommit, releaseExists, versionChanged, manualRequested] = args;
    console.log(shouldPublishVersion({
      currentCommit,
      tagCommit,
      releaseExists: parseBoolean(releaseExists),
      versionChanged: parseBoolean(versionChanged),
      manualRequested: parseBoolean(manualRequested),
    }));
  } catch (error) {
    console.error(`::error::${error instanceof Error ? error.message : String(error)}`);
    process.exitCode = 1;
  }
}
