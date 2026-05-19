import fs from 'node:fs';
import path from 'node:path';
import readline from 'node:readline';
import process from 'node:process';

import { execa } from 'execa';
import pc from 'picocolors';

import { runDevEnvChecks } from './check-dev-env.js';
import type { InfraMode } from './utils/infra.js';
import { addrToHttpUrl, readEnvKey, resolveEffectiveEnvPath } from './utils/env-file.js';
import { banner, tagLine } from './utils/log.js';
import { freeDevPorts, resolveDevServicePorts, sleep } from './utils/port-cleanup.js';
import { repoRoot } from './utils/paths.js';

type ManagedProc = {
  tag: string;
  subprocess: ReturnType<typeof execa>;
};

function ensureRootEnvFromExample(): void {
  const envPath = path.join(repoRoot, '.env');
  const example = path.join(repoRoot, '.env.example');
  if (fs.existsSync(envPath)) {
    tagLine('env', '.env exists', pc.green);
    return;
  }
  if (!fs.existsSync(example)) {
    console.error(pc.red('[env]'), '根目录缺少 .env，且找不到 .env.example，无法自动生成。请手动创建 .env。');
    process.exit(1);
  }
  fs.copyFileSync(example, envPath);
  console.log(
    pc.yellow('[env]'),
    '已从 .env.example 复制生成根目录 .env。首次启动可使用示例中的本地默认配置；生产密钥请勿写入仓库。',
  );
}

function attachPrefixedLines(stream: NodeJS.ReadableStream | null, tag: string, kind: 'out' | 'err'): void {
  if (!stream) return;
  const rl = readline.createInterface({ input: stream, crlfDelay: Infinity });
  const color = kind === 'err' ? pc.red : pc.cyan;
  const prefix = color(`[${tag}]`);
  rl.on('line', (line) => {
    console.log(prefix, line);
  });
}

async function startInfra(mode: InfraMode): Promise<void> {
  if (mode === 'local') {
    tagLine('infra', 'Using local PostgreSQL / Redis (skipping Docker Compose)', pc.green);
    return;
  }

  tagLine('infra', 'Starting PostgreSQL + Redis via Docker Compose...', pc.magenta);
  try {
    await execa('docker', ['compose', 'up', '-d', '--wait', 'postgres', 'redis'], {
      cwd: repoRoot,
      stdio: 'inherit',
    });
  } catch {
    await execa('docker', ['compose', 'up', '-d', 'postgres', 'redis'], {
      cwd: repoRoot,
      stdio: 'inherit',
    });
    console.log(
      pc.yellow('[infra]'),
      '提示：当前 Docker Compose 可能不支持 `--wait`，容器已启动；若后端暂时连不上数据库，请等待数秒后重试。',
    );
  }
  tagLine('infra', 'PostgreSQL / Redis started', pc.green);
}

function printServiceHints(): void {
  const envPath = path.join(repoRoot, '.env');
  const backendAddr = readEnvKey(envPath, 'APP_HTTP_ADDR') ?? ':8080';
  const collectorAddr = readEnvKey(envPath, 'COLLECTOR_HTTP_ADDR') ?? ':3100';
  const backendUrl = addrToHttpUrl(backendAddr);
  const collectorUrl = addrToHttpUrl(collectorAddr);

  if (backendUrl) console.log(pc.bold(pc.green(`[backend] ${backendUrl}`)));
  if (collectorUrl) console.log(pc.bold(pc.green(`[collector] ${collectorUrl}`)));
  console.log(
    pc.bold(pc.green('[admin]')),
    pc.green('http://127.0.0.1:8000'),
    pc.dim('（Umi / Ant Design Pro 默认端口，以终端 “Local:” 为准）'),
  );
}

async function cleanupPreviousDevProcesses(): Promise<void> {
  const envPath = resolveEffectiveEnvPath(repoRoot);
  const ports = resolveDevServicePorts(envPath);
  tagLine('dev', 'Checking for previous local dev processes…', pc.yellow);
  const freed = await freeDevPorts(ports);
  if (freed.length === 0) {
    tagLine('dev', 'No previous dev listeners found on backend / admin / collector ports', pc.green);
    return;
  }
  for (const item of freed) {
    tagLine(
      'dev',
      `Freed port ${item.port} (stopped PID ${item.killed.join(', ')})`,
      pc.yellow,
    );
  }
  await sleep(process.platform === 'win32' ? 800 : 300);
}

async function main(): Promise<void> {
  banner('TradeMind Dev Launcher');

  ensureRootEnvFromExample();

  const check = await runDevEnvChecks({ quietBanner: true, skipContainerStatus: true });
  if (!check.ok || !check.infraMode) {
    process.exit(1);
  }

  await startInfra(check.infraMode);

  await cleanupPreviousDevProcesses();

  tagLine('backend', 'starting...', pc.blue);
  tagLine('admin', 'starting...', pc.blue);
  tagLine('collector', 'starting...', pc.blue);

  printServiceHints();

  const managed: ManagedProc[] = [
    {
      tag: 'backend',
      subprocess: execa('pnpm', ['run', 'dev:backend'], {
        cwd: repoRoot,
        reject: false,
        stdout: 'pipe',
        stderr: 'pipe',
        env: { ...process.env },
      }),
    },
    {
      tag: 'admin',
      subprocess: execa('pnpm', ['run', 'dev:admin'], {
        cwd: repoRoot,
        reject: false,
        stdout: 'pipe',
        stderr: 'pipe',
        env: { ...process.env },
      }),
    },
    {
      tag: 'collector',
      subprocess: execa('pnpm', ['run', 'dev:collector'], {
        cwd: repoRoot,
        reject: false,
        stdout: 'pipe',
        stderr: 'pipe',
        env: { ...process.env },
      }),
    },
  ];

  for (const m of managed) {
    attachPrefixedLines(m.subprocess.stdout, m.tag, 'out');
    attachPrefixedLines(m.subprocess.stderr, m.tag, 'err');
  }

  let shuttingDown = false;
  let fatal: { tag: string; code: number } | undefined;

  async function stopChildren(reason: string): Promise<void> {
    console.log(pc.yellow('\n[dev]'), reason);
    await Promise.all(
      managed.map(async (m) => {
        try {
          m.subprocess.kill('SIGTERM');
          await m.subprocess.catch(() => undefined);
        } catch {
          /* ignore */
        }
      }),
    );
  }

  function considerChildExit(tag: string, code: number | null, signal: NodeJS.Signals | null): void {
    if (shuttingDown) return;
    const sig = signal ?? undefined;
    const benign =
      sig === 'SIGINT' ||
      sig === 'SIGTERM' ||
      sig === 'SIGKILL' ||
      (code === null && (sig === 'SIGTERM' || sig === 'SIGINT'));
    if (benign) return;
    if (code !== 0 && code !== null) {
      if (fatal) return;
      fatal = { tag, code };
      shuttingDown = true;
      console.error(pc.red(`\n[dev] 子进程「${tag}」异常退出（code=${code}）。正在结束其余进程…`));
      void stopChildren('正在停止其余子进程…');
    }
  }

  for (const m of managed) {
    m.subprocess.on('exit', (code, signal) => {
      considerChildExit(m.tag, code, signal);
    });
  }

  const onSignal = (sig: NodeJS.Signals) => {
    if (shuttingDown) return;
    shuttingDown = true;
    void stopChildren(`收到 ${sig}，正在停止子进程…`).then(() => process.exit(0));
  };

  process.on('SIGINT', () => onSignal('SIGINT'));
  process.on('SIGTERM', () => onSignal('SIGTERM'));

  const outcomes = await Promise.all(managed.map((m) => m.subprocess));

  if (fatal) {
    process.exit(fatal.code);
  }

  const worst = outcomes.reduce((acc, r) => {
    const c = r.exitCode ?? 1;
    return c > acc ? c : acc;
  }, 0);
  if (worst !== 0) {
    process.exit(worst);
  }
}

main().catch((e: unknown) => {
  console.error(pc.red('[dev]'), e);
  process.exit(1);
});
