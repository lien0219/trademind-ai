#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

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

async function fetchRegistryTags() {
  const namespace = (process.env.GHCR_NAMESPACE || 'lien0219').toLowerCase();
  const imageName = process.env.GHCR_IMAGE_NAME || 'trademind';
  const tokenUrl = new URL('https://ghcr.io/token');
  tokenUrl.searchParams.set('scope', `repository:${namespace}/${imageName}:pull`);

  const tokenHeaders = {};
  if (process.env.GHCR_TOKEN) {
    const username = process.env.GHCR_USERNAME || 'github-actions[bot]';
    tokenHeaders.authorization = `Basic ${Buffer.from(`${username}:${process.env.GHCR_TOKEN}`).toString('base64')}`;
  }

  const tokenResponse = await fetch(tokenUrl, { headers: tokenHeaders });
  if (!tokenResponse.ok) throw new Error(`GHCR token request failed with HTTP ${tokenResponse.status}`);
  const { token } = await tokenResponse.json();
  if (typeof token !== 'string' || token.length === 0) throw new Error('GHCR did not return a pull token');

  const tags = [];
  let nextUrl = new URL(`https://ghcr.io/v2/${namespace}/${imageName}/tags/list?n=1000`);
  while (nextUrl) {
    const response = await fetch(nextUrl, { headers: { authorization: `Bearer ${token}` } });
    if (!response.ok) throw new Error(`GHCR tag request failed with HTTP ${response.status}`);
    const body = await response.json();
    if (Array.isArray(body.tags)) tags.push(...body.tags.filter((tag) => typeof tag === 'string'));

    const nextLink = /<([^>]+)>\s*;\s*rel="next"/iu.exec(response.headers.get('link') || '');
    nextUrl = nextLink ? new URL(nextLink[1]) : null;
  }

  return tags;
}

async function main() {
  const candidate = process.argv[2];
  if (!candidate) throw new Error('A release version is required');

  const gitTags = execFileSync('git', ['tag', '--list', 'v*'], { encoding: 'utf8' }).split(/\r?\n/u);
  const imageTags = await fetchRegistryTags();
  const highest = findHighestReleaseVersion(gitTags, imageTags);
  if (highest && compareVersions(candidate, highest.version) < 0) {
    throw new Error(`Release version ${candidate} is lower than existing highest version ${highest.version}`);
  }

  console.log(highest ? `Release version ${candidate} is at or above ${highest.version}` : `Release version ${candidate} is valid`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(`::error::${error instanceof Error ? error.message : String(error)}`);
    process.exitCode = 1;
  });
}
