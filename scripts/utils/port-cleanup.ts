import process from 'node:process';

import { runCapture } from './command.js';
import { readEnvKey } from './env-file.js';

export function parsePortFromAddr(addr: string | undefined, defaultPort: number): number {
  if (!addr?.trim()) return defaultPort;
  const a = addr.trim();
  if (a.startsWith(':')) {
    const p = Number.parseInt(a.slice(1), 10);
    return Number.isFinite(p) ? p : defaultPort;
  }
  const lastColon = a.lastIndexOf(':');
  if (lastColon > 0) {
    const p = Number.parseInt(a.slice(lastColon + 1), 10);
    if (Number.isFinite(p)) return p;
  }
  if (/^\d+$/.test(a)) return Number.parseInt(a, 10);
  return defaultPort;
}

function lineHasListeningPort(line: string, port: number): boolean {
  if (!line.includes('LISTENING')) return false;
  return new RegExp(`:${port}(\\s|$)`).test(line);
}

async function listListeningPids(port: number): Promise<number[]> {
  if (process.platform === 'win32') {
    const r = await runCapture('netstat', ['-ano']);
    if (!r.ok) return [];
    const pids = new Set<number>();
    for (const line of r.out.split(/\r?\n/)) {
      if (!lineHasListeningPort(line, port)) continue;
      const parts = line.trim().split(/\s+/);
      const pid = Number.parseInt(parts[parts.length - 1] ?? '', 10);
      if (Number.isFinite(pid) && pid > 4) pids.add(pid);
    }
    return [...pids];
  }

  const r = await runCapture('lsof', ['-nP', `-iTCP:${port}`, '-sTCP:LISTEN', '-t']);
  if (!r.ok || !r.out) return [];
  return r.out
    .split(/\r?\n/)
    .map((s) => Number.parseInt(s.trim(), 10))
    .filter((n) => Number.isFinite(n) && n > 0 && n !== process.pid);
}

async function killPid(pid: number): Promise<boolean> {
  if (pid === process.pid || pid <= 4) return false;
  if (process.platform === 'win32') {
    const r = await runCapture('taskkill', ['/PID', String(pid), '/F', '/T']);
    return r.ok;
  }
  const r = await runCapture('kill', ['-TERM', String(pid)]);
  return r.ok;
}

export type FreedPort = {
  port: number;
  killed: number[];
};

/** Stop processes listening on dev service ports before a fresh launch. */
export async function freeDevPorts(ports: number[]): Promise<FreedPort[]> {
  const unique = [...new Set(ports.filter((p) => Number.isFinite(p) && p > 0 && p < 65536))];
  const freed: FreedPort[] = [];
  for (const port of unique) {
    const pids = await listListeningPids(port);
    const killed: number[] = [];
    for (const pid of pids) {
      if (await killPid(pid)) killed.push(pid);
    }
    if (killed.length > 0) {
      freed.push({ port, killed });
    }
  }
  return freed;
}

/** Backend / collector ports from .env plus common Umi admin dev ports. */
export function resolveDevServicePorts(envPath: string | undefined): number[] {
  const backend = parsePortFromAddr(envPath ? readEnvKey(envPath, 'APP_HTTP_ADDR') : undefined, 8080);
  const collector = parsePortFromAddr(
    envPath ? readEnvKey(envPath, 'COLLECTOR_HTTP_ADDR') : undefined,
    3100,
  );
  const adminPorts = Array.from({ length: 11 }, (_, i) => 8000 + i);
  return [backend, collector, ...adminPorts];
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
