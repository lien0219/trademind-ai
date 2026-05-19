import net from 'node:net';

import { commandPrintableVersion, runCapture } from './command.js';
import { readEnvKey } from './env-file.js';

export type InfraMode = 'docker' | 'local';

export type InfraResolution = {
  mode: InfraMode;
  dockerAvailable: boolean;
  postgres: { host: string; port: number; reachable: boolean };
  redis: { host: string; port: number; reachable: boolean };
};

export function parseRedisAddr(addr: string | undefined): { host: string; port: number } {
  const raw = addr?.trim() || '127.0.0.1:6379';
  const lastColon = raw.lastIndexOf(':');
  if (lastColon <= 0) {
    return { host: raw, port: 6379 };
  }
  const host = raw.slice(0, lastColon);
  const port = Number.parseInt(raw.slice(lastColon + 1), 10);
  return { host: host || '127.0.0.1', port: Number.isFinite(port) ? port : 6379 };
}

export function readInfraTargets(envPath: string | undefined): {
  postgres: { host: string; port: number };
  redis: { host: string; port: number };
} {
  const host = envPath ? (readEnvKey(envPath, 'DB_HOST') ?? '127.0.0.1') : '127.0.0.1';
  const portRaw = envPath ? (readEnvKey(envPath, 'DB_PORT') ?? '5432') : '5432';
  const port = Number.parseInt(portRaw, 10);
  const redisAddr = envPath ? readEnvKey(envPath, 'REDIS_ADDR') : undefined;
  return {
    postgres: { host, port: Number.isFinite(port) ? port : 5432 },
    redis: parseRedisAddr(redisAddr),
  };
}

export function checkTcpReachable(host: string, port: number, timeoutMs = 2000): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = net.createConnection({ host, port });
    const done = (ok: boolean) => {
      socket.removeAllListeners();
      try {
        socket.destroy();
      } catch {
        /* ignore */
      }
      resolve(ok);
    };
    socket.setTimeout(timeoutMs);
    socket.once('connect', () => done(true));
    socket.once('timeout', () => done(false));
    socket.once('error', () => done(false));
  });
}

export async function isDockerCliAvailable(): Promise<boolean> {
  const v = await commandPrintableVersion('docker', ['version']);
  return Boolean(v);
}

export async function isDockerComposeAvailable(): Promise<boolean> {
  const r = await runCapture('docker', ['compose', 'version']);
  return r.ok;
}

export async function isDockerDaemonRunning(): Promise<boolean> {
  const r = await runCapture('docker', ['info']);
  return r.ok;
}

export async function isDockerReady(): Promise<boolean> {
  if (!(await isDockerCliAvailable())) return false;
  if (!(await isDockerComposeAvailable())) return false;
  return isDockerDaemonRunning();
}

export async function resolveInfra(envPath: string | undefined): Promise<InfraResolution> {
  const targets = readInfraTargets(envPath);
  const dockerAvailable = await isDockerReady();
  const [pgReachable, redisReachable] = await Promise.all([
    checkTcpReachable(targets.postgres.host, targets.postgres.port),
    checkTcpReachable(targets.redis.host, targets.redis.port),
  ]);

  return {
    mode: dockerAvailable ? 'docker' : 'local',
    dockerAvailable,
    postgres: { ...targets.postgres, reachable: pgReachable },
    redis: { ...targets.redis, reachable: redisReachable },
  };
}

export function formatHostPort(host: string, port: number): string {
  return `${host}:${port}`;
}

export function infraUsable(res: InfraResolution): boolean {
  if (res.dockerAvailable) return true;
  return res.postgres.reachable && res.redis.reachable;
}
