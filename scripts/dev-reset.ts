import process, { stdin as input, stdout as output } from 'node:process';
import readline from 'node:readline/promises';

import { execa } from 'execa';
import pc from 'picocolors';

import { repoRoot } from './utils/paths.js';

async function main(): Promise<void> {
  const rl = readline.createInterface({ input, output });
  const ans = await rl.question(
    pc.red(
      '即将执行 `docker compose down -v`（会删除 Compose 声明的数据卷，PostgreSQL 数据将清空）。输入 RESET 确认继续：',
    ),
  );
  rl.close();
  if (ans.trim() !== 'RESET') {
    console.log(pc.yellow('已取消，未做任何更改。'));
    return;
  }

  console.log(pc.yellow('[dev:reset]'), '执行 docker compose down -v …');
  await execa('docker', ['compose', 'down', '-v'], {
    cwd: repoRoot,
    stdio: 'inherit',
  });
  console.log(
    pc.green('[dev:reset]'),
    '已完成。请执行 `pnpm dev:infra` 或 `pnpm dev` 重新创建容器；后端迁移会在启动时按项目逻辑执行。',
  );
}

main().catch((e: unknown) => {
  console.error(pc.red('[dev:reset]'), e);
  process.exit(1);
});
