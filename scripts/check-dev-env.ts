import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

import { commandPrintableVersion, runCapture } from './utils/command.js';
import { resolveEffectiveEnvPath } from './utils/env-file.js';
import {
  formatHostPort,
  infraUsable,
  isDockerCliAvailable,
  isDockerComposeAvailable,
  isDockerDaemonRunning,
  type InfraMode,
  type InfraResolution,
  resolveInfra,
} from './utils/infra.js';
import { banner, checkFail, checkOk, checkWarn } from './utils/log.js';
import { backendDir, repoRoot } from './utils/paths.js';

let hadFailure = false;

function fail(msg: string): void {
  hadFailure = true;
  checkFail(msg);
}

async function containerRunning(name: string): Promise<boolean> {
  const r = await runCapture('docker', ['inspect', '-f', '{{.State.Running}}', name]);
  return r.ok && r.out.trim() === 'true';
}

function checkInfraResolution(infra: InfraResolution): InfraMode | null {
  if (infra.dockerAvailable) {
    return 'docker';
  }

  checkWarn('未检测到可用的 Docker（CLI / Compose / 引擎），将尝试使用本机 PostgreSQL / Redis');

  const pgTarget = formatHostPort(infra.postgres.host, infra.postgres.port);
  const redisTarget = formatHostPort(infra.redis.host, infra.redis.port);

  if (infra.postgres.reachable) {
    checkOk(`本机 PostgreSQL 可连接（${pgTarget}）`);
  } else {
    fail(
      `本机 PostgreSQL 不可连接（${pgTarget}）。请启动 PostgreSQL，或安装并启动 Docker Desktop 后重试。`,
    );
  }

  if (infra.redis.reachable) {
    checkOk(`本机 Redis 可连接（${redisTarget}）`);
  } else {
    fail(`本机 Redis 不可连接（${redisTarget}）。请启动 Redis，或安装并启动 Docker Desktop 后重试。`);
  }

  if (hadFailure) {
    return null;
  }
  return 'local';
}

async function checkDockerDetails(infra: InfraResolution): Promise<void> {
  if (!infra.dockerAvailable) return;

  const dockerV = await commandPrintableVersion('docker', ['version']);
  if (dockerV) {
    checkOk(`Docker CLI OK (${dockerV.split(/\s+/).slice(0, 2).join(' ')})`);
  }

  if (await isDockerComposeAvailable()) {
    checkOk('Docker Compose OK');
  }

  if (await isDockerDaemonRunning()) {
    checkOk('Docker 引擎可访问');
  }
}

export type DevEnvCheckResult = {
  ok: boolean;
  infraMode: InfraMode | null;
};

export async function runDevEnvChecks(
  options: { quietBanner?: boolean; skipContainerStatus?: boolean } = {},
): Promise<DevEnvCheckResult> {
  hadFailure = false;
  if (!options.quietBanner) {
    banner('TradeMind Dev — 环境检查');
  }

  checkOk(`Node OK (${process.version})`);

  const pnpmV = await commandPrintableVersion('pnpm', ['--version']);
  if (!pnpmV) {
    fail('未检测到 pnpm。请先安装 Node.js，再执行：`npm install -g pnpm@9`，然后重新打开终端。');
  } else {
    checkOk(`pnpm OK (${pnpmV})`);
  }

  const goV = await commandPrintableVersion('go', ['version']);
  if (!goV) {
    fail('未检测到 Go。请从 https://go.dev/dl/ 安装，并确保 `go version` 在终端可用。');
  } else {
    checkOk(`Go OK (${goV})`);
  }

  const effective = resolveEffectiveEnvPath(repoRoot);
  const infra = await resolveInfra(effective);

  if (!(await isDockerCliAvailable()) && !infra.dockerAvailable) {
    checkWarn('未检测到 Docker CLI');
  }

  await checkDockerDetails(infra);
  const infraMode = checkInfraResolution(infra);

  const rootEnv = path.join(repoRoot, '.env');
  const beEnv = path.join(backendDir, '.env');

  if (fs.existsSync(rootEnv)) {
    checkOk('根目录 .env 已存在（不会打印内容）');
  } else if (fs.existsSync(beEnv)) {
    checkWarn('根目录无 .env，但 backend/.env 存在；后端从 backend 目录启动时会优先加载 backend/.env，也会尝试加载上级 .env。');
  } else {
    fail('未找到环境文件：请复制根目录 `.env.example` 为 `.env`，或创建 backend/.env。执行 `pnpm dev` 时若根目录无 .env 会自动从 .env.example 复制一份。');
  }

  if (!hadFailure && effective) {
    checkOk(`有效配置文件：${path.relative(repoRoot, effective) || '.env'}`);
  }

  if (!hadFailure && !options.skipContainerStatus && infraMode) {
    if (infraMode === 'docker') {
      const pg = await containerRunning('trademind-postgres');
      const rd = await containerRunning('trademind-redis');
      if (pg && rd) {
        checkOk('PostgreSQL / Redis 容器已在运行（trademind-postgres、trademind-redis）');
      } else {
        checkWarn(
          `基础容器未全部运行（PostgreSQL: ${pg ? '运行中' : '未运行'}，Redis: ${rd ? '运行中' : '未运行'}）。可执行 \`pnpm dev:infra\` 或 \`pnpm dev\` 拉起。`,
        );
      }
    } else if (infraUsable(infra)) {
      checkOk('本机 PostgreSQL / Redis 已在运行');
    }
  }

  if (hadFailure) {
    checkFail('环境检查未通过，请先处理上述问题后再运行 `pnpm dev`。');
    return { ok: false, infraMode: null };
  }
  checkOk('环境检查完成');
  return { ok: true, infraMode };
}

function isDirectRun(): boolean {
  const entry = process.argv[1];
  if (!entry) return false;
  return path.resolve(fileURLToPath(import.meta.url)) === path.resolve(entry);
}

async function main(): Promise<void> {
  const result = await runDevEnvChecks();
  process.exit(result.ok ? 0 : 1);
}

if (isDirectRun()) {
  main().catch((e: unknown) => {
    console.error(e);
    process.exit(1);
  });
}
