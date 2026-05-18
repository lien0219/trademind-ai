import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

import { commandPrintableVersion, runCapture } from './utils/command.js';
import { resolveEffectiveEnvPath } from './utils/env-file.js';
import { banner, checkFail, checkOk, checkWarn } from './utils/log.js';
import { backendDir, repoRoot } from './utils/paths.js';

let hadFailure = false;

function fail(msg: string): void {
  hadFailure = true;
  checkFail(msg);
}

async function verifyDockerDaemon(): Promise<boolean> {
  const r = await runCapture('docker', ['info']);
  if (r.ok) return true;
  fail(
    'Docker 引擎未运行或未就绪：请先启动 Docker Desktop（Windows/macOS）或本机 docker 服务（Linux），并在终端执行 `docker ps` 确认无报错。',
  );
  return false;
}

async function verifyCompose(): Promise<boolean> {
  const r = await runCapture('docker', ['compose', 'version']);
  if (r.ok) return true;
  fail('未检测到 `docker compose`（Compose V2）。请升级 Docker Desktop / Docker Engine，或安装 Compose 插件。');
  return false;
}

async function containerRunning(name: string): Promise<boolean> {
  const r = await runCapture('docker', ['inspect', '-f', '{{.State.Running}}', name]);
  return r.ok && r.out.trim() === 'true';
}

export async function runDevEnvChecks(
  options: { quietBanner?: boolean; skipContainerStatus?: boolean } = {},
): Promise<boolean> {
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

  const dockerV = await commandPrintableVersion('docker', ['version']);
  if (!dockerV) {
    fail('未检测到 Docker CLI。请安装 Docker Desktop 或 Docker Engine，并确保 `docker version` 可用。');
  } else {
    checkOk(`Docker CLI OK (${dockerV.split(/\s+/).slice(0, 2).join(' ')})`);
  }

  await verifyCompose();

  if (!hadFailure) {
    const daemonOk = await verifyDockerDaemon();
    if (!daemonOk) {
      // verifyDockerDaemon already printed failure
    } else {
      checkOk('Docker 引擎可访问');
    }
  }

  const rootEnv = path.join(repoRoot, '.env');
  const beEnv = path.join(backendDir, '.env');
  const effective = resolveEffectiveEnvPath(repoRoot);

  if (fs.existsSync(rootEnv)) {
    checkOk('根目录 .env 已存在（不会打印内容）');
  } else if (fs.existsSync(beEnv)) {
    checkWarn('根目录无 .env，但 backend/.env 存在；后端从 backend 目录启动时会优先加载 backend/.env，也会尝试加载上级 .env。');
  } else {
    fail('未找到环境文件：请复制根目录 `.env.example` 为 `.env`，或创建 backend/.env。执行 `pnpm dev` 时若根目录无 .env 会自动从 .env.example 复制一份。');
  }

  if (!hadFailure && effective) {
    // touch-only check; never print values
    checkOk(`有效配置文件：${path.relative(repoRoot, effective) || '.env'}`);
  }

  if (!hadFailure && !options.skipContainerStatus) {
    const pg = await containerRunning('trademind-postgres');
    const rd = await containerRunning('trademind-redis');
    if (pg && rd) {
      checkOk('PostgreSQL / Redis 容器已在运行（trademind-postgres、trademind-redis）');
    } else {
      checkWarn(
        `基础容器未全部运行（PostgreSQL: ${pg ? '运行中' : '未运行'}，Redis: ${rd ? '运行中' : '未运行'}）。可执行 \`pnpm dev:infra\` 或 \`pnpm dev\` 拉起。`,
      );
    }
  }

  if (hadFailure) {
    checkFail('环境检查未通过，请先处理上述问题后再运行 `pnpm dev`。');
    return false;
  }
  checkOk('环境检查完成');
  return true;
}

async function main(): Promise<void> {
  const ok = await runDevEnvChecks();
  process.exit(ok ? 0 : 1);
}

main().catch((e: unknown) => {
  console.error(e);
  process.exit(1);
});
