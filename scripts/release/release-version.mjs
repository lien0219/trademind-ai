const semverPattern = /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/u;

export function parseVersion(version) {
  const match = semverPattern.exec(version);
  if (!match || [match[1], match[2], match[3]].some((part) => part.length > 1 && part.startsWith('0'))) {
    throw new Error(`Invalid release version: ${version}`);
  }

  const prerelease = match[4]?.split('.') ?? [];
  if (prerelease.some((identifier) => /^\d+$/u.test(identifier) && identifier.length > 1 && identifier.startsWith('0'))) {
    throw new Error(`Invalid release version: ${version}`);
  }

  return {
    major: BigInt(match[1]),
    minor: BigInt(match[2]),
    patch: BigInt(match[3]),
    prerelease,
  };
}

function comparePrerelease(left, right) {
  if (left.length === 0 && right.length === 0) return 0;
  if (left.length === 0) return 1;
  if (right.length === 0) return -1;

  for (let index = 0; index < Math.min(left.length, right.length); index += 1) {
    const leftIdentifier = left[index];
    const rightIdentifier = right[index];
    if (leftIdentifier === rightIdentifier) continue;

    const leftNumeric = /^\d+$/u.test(leftIdentifier);
    const rightNumeric = /^\d+$/u.test(rightIdentifier);
    if (leftNumeric && rightNumeric) return BigInt(leftIdentifier) < BigInt(rightIdentifier) ? -1 : 1;
    if (leftNumeric !== rightNumeric) return leftNumeric ? -1 : 1;
    return leftIdentifier < rightIdentifier ? -1 : 1;
  }

  return left.length < right.length ? -1 : left.length === right.length ? 0 : 1;
}

function compareParsedVersions(left, right) {
  for (const component of ['major', 'minor', 'patch']) {
    if (left[component] !== right[component]) return left[component] < right[component] ? -1 : 1;
  }
  return comparePrerelease(left.prerelease, right.prerelease);
}

export function compareVersions(left, right) {
  return compareParsedVersions(parseVersion(left), parseVersion(right));
}

export function findHighestReleaseVersion(gitTags, imageTags) {
  let highest = null;

  const versions = [
    ...gitTags.filter((tag) => tag.startsWith('v')).map((tag) => tag.slice(1)),
    ...imageTags.flatMap((tag) => {
      const match = /^(?:backend|admin|collector)-main-v(.+)$/u.exec(tag);
      return match ? [match[1]] : [];
    }),
  ];

  for (const version of versions) {
    let parsed;
    try {
      parsed = parseVersion(version);
    } catch {
      continue;
    }
    if (!highest || compareParsedVersions(parsed, highest.parsed) > 0) highest = { parsed, version };
  }

  return highest;
}
