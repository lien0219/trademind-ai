#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { compareVersions, findHighestReleaseVersion } from './release-version.mjs';

export { compareVersions, findHighestReleaseVersion, parseVersion } from './release-version.mjs';

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
