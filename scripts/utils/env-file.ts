import fs from 'node:fs';
import path from 'node:path';

/**
 * Read a single key from a dotenv-style file.
 * Does not expand variables; trims unquoted values only.
 */
export function readEnvKey(filePath: string, key: string): string | undefined {
  if (!fs.existsSync(filePath)) return undefined;
  const text = fs.readFileSync(filePath, 'utf8');
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const eq = line.indexOf('=');
    if (eq <= 0) continue;
    const k = line.slice(0, eq).trim();
    if (k !== key) continue;
    let v = line.slice(eq + 1).trim();
    if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
      v = v.slice(1, -1);
    }
    return v;
  }
  return undefined;
}

/** Prefer root .env; backend/.env as fallback (backend also loads ../.env when cwd is backend). */
export function resolveEffectiveEnvPath(repoRoot: string): string | undefined {
  const rootEnv = path.join(repoRoot, '.env');
  const backendEnv = path.join(repoRoot, 'backend', '.env');
  if (fs.existsSync(rootEnv)) return rootEnv;
  if (fs.existsSync(backendEnv)) return backendEnv;
  return undefined;
}

/** Turn ":8080" or "127.0.0.1:8080" into a browser-friendly http URL (defaults host localhost). */
export function addrToHttpUrl(addr: string | undefined, defaultHost = '127.0.0.1'): string | undefined {
  if (!addr?.trim()) return undefined;
  const a = addr.trim();
  if (a.startsWith('http://') || a.startsWith('https://')) return a;
  if (a.startsWith(':')) return `http://${defaultHost}${a}`;
  if (/^\d+$/.test(a)) return `http://${defaultHost}:${a}`;
  return `http://${a}`;
}
