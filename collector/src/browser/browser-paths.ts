import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { sanitizeProfileKey } from './profile-key.js';

/** collector 包根目录（与运行时 cwd 无关）。 */
export const COLLECTOR_PACKAGE_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../..',
);

/**
 * 1688 等 Provider 持久化 Profile 根目录。
 * 优先 COLLECTOR_BROWSER_PROFILE_DIR，其次 BROWSER_PROFILE_ROOT，默认 collector/data/browser-profiles。
 */
export function getBrowserProfileRoot(): string {
  const raw =
    process.env.COLLECTOR_BROWSER_PROFILE_DIR?.trim() ||
    process.env.COLLECTOR_PROFILE_DIR?.trim() ||
    process.env.BROWSER_PROFILE_ROOT?.trim() ||
    '';
  if (raw) {
    return path.isAbsolute(raw) ? raw : path.resolve(COLLECTOR_PACKAGE_ROOT, raw);
  }
  return path.join(COLLECTOR_PACKAGE_ROOT, 'data', 'browser-profiles');
}

/** 1688 userDataDir：BROWSER_PROFILE_ROOT/1688 */
export function get1688UserDataDir(): string {
  return path.join(getBrowserProfileRoot(), '1688');
}

/** 拼多多专用 Profile（与 1688 / custom 隔离）：BROWSER_PROFILE_ROOT/pinduoduo */
export function getPinduoduoUserDataDir(): string {
  return path.join(getBrowserProfileRoot(), 'pinduoduo');
}

/** Custom collect browser profile: BROWSER_PROFILE_ROOT/custom/{profileKey} */
export function getCustomProfileUserDataDir(profileKey: string): string {
  const safe = sanitizeProfileKey(profileKey);
  return path.join(getBrowserProfileRoot(), 'custom', safe);
}

export function getStorageStateRoot(): string {
  const raw = process.env.COLLECTOR_STORAGE_STATE_DIR?.trim() || '';
  if (raw) {
    return path.isAbsolute(raw) ? raw : path.resolve(COLLECTOR_PACKAGE_ROOT, raw);
  }
  return path.join(COLLECTOR_PACKAGE_ROOT, 'data', 'storage-states');
}

export function ensureBrowserDataDirs(): void {
  for (const dir of [
    getBrowserProfileRoot(),
    get1688UserDataDir(),
    getPinduoduoUserDataDir(),
    getStorageStateRoot(),
  ]) {
    fs.mkdirSync(dir, { recursive: true });
  }
}
