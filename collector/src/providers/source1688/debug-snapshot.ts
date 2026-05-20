import fs from 'node:fs';
import path from 'node:path';
import type { Page } from 'playwright';

import { COLLECTOR_PACKAGE_ROOT } from '../../browser/browser-paths.js';

function snapshotDir(): string {
  const raw = process.env.COLLECTOR_SNAPSHOT_DIR?.trim();
  const base = raw
    ? path.isAbsolute(raw)
      ? raw
      : path.resolve(COLLECTOR_PACKAGE_ROOT, raw)
    : path.join(COLLECTOR_PACKAGE_ROOT, 'data', 'snapshots', '1688');
  fs.mkdirSync(base, { recursive: true });
  return base;
}

export type CollectDebugLog = {
  sourceUrl: string;
  finalUrl: string;
  pageTitle: string;
  loginOrVerifyHit: boolean;
  titleFound: boolean;
  priceFound: boolean;
  mainImagesCount: number;
  detailImagesCount: number;
  skuCount: number;
  extractors: string[];
  missingFields: string[];
  collectStatus: string;
  error?: string;
  snapshotHtml?: string;
  snapshotPng?: string;
};

export function log1688CollectDebug(entry: CollectDebugLog): void {
  console.info('[1688-collect]', JSON.stringify(entry));
}

export async function save1688FailureSnapshot(
  page: Page,
  tag: string,
): Promise<{ htmlPath?: string; screenshotPath?: string }> {
  try {
    const dir = snapshotDir();
    const stamp = `${Date.now()}_${tag.replace(/[^\w-]+/g, '_').slice(0, 40)}`;
    const htmlPath = path.join(dir, `${stamp}.html`);
    const screenshotPath = path.join(dir, `${stamp}.png`);
    const html = await page.content();
    fs.writeFileSync(htmlPath, html, 'utf8');
    await page.screenshot({ path: screenshotPath, fullPage: false }).catch(() => undefined);
    return { htmlPath, screenshotPath };
  } catch {
    return {};
  }
}
