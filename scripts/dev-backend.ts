import process from 'node:process';

import { execa } from 'execa';

import { addrToHttpUrl, readEnvKey, resolveEffectiveEnvPath } from './utils/env-file.js';
import { freeDevPorts, parsePortFromAddr, sleep } from './utils/port-cleanup.js';
import { backendDir, repoRoot } from './utils/paths.js';

async function main(): Promise<void> {
  const envFile = resolveEffectiveEnvPath(repoRoot);
  const httpAddr = envFile ? (readEnvKey(envFile, 'APP_HTTP_ADDR') ?? ':8080') : ':8080';
  const url = addrToHttpUrl(httpAddr);
  if (url) {
    console.log(`[backend] ${url}`);
  }

  const port = parsePortFromAddr(httpAddr, 8080);
  const freed = await freeDevPorts([port]);
  if (freed.length > 0) {
    console.log(`[backend] Freed port ${port} (stopped PID ${freed[0]?.killed.join(', ')})`);
    await sleep(process.platform === 'win32' ? 800 : 300);
  }

  const r = await execa('go', ['run', './cmd/server'], {
    cwd: backendDir,
    stdio: 'inherit',
    reject: false,
    env: {
      ...process.env,
      TRADEMIND_REPO_ROOT: repoRoot,
    },
  });

  if (r.exitCode !== 0) {
    console.error('\n[backend] 启动失败。请依次排查：');
    console.error('  1) Go 是否已安装且在 PATH 中（go version）');
    console.error('  2) 根目录 .env 或 backend/.env 是否存在，数据库与 Redis 配置是否正确（勿提交密钥）');
    console.error('  3) PostgreSQL / Redis 是否已启动（pnpm dev:infra 或 Docker）');
    console.error('  4) 端口是否被占用（见 APP_HTTP_ADDR）\n');
    process.exit(r.exitCode ?? 1);
  }
}

main().catch((e: unknown) => {
  console.error('[backend] 未预期的错误:', e);
  process.exit(1);
});
