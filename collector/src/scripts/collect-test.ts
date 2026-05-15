/**
 * 本地一键调试采集：`pnpm collect:test -- --url "https://detail.1688.com/offer/xxx.html"`
 * 或使用环境变量 `COLLECT_TEST_URL`。
 */

import { BrowserManager } from '../browser/manager.js';
import { runCollectTask } from '../tasks/collect-task.js';

function argvUrl(): string {
  const args = process.argv.slice(2);
  for (let i = 0; i < args.length - 1; i++) {
    if (args[i] === '--url') return trimArg(args[i + 1] ?? '');
  }
  return trimArg(process.env.COLLECT_TEST_URL ?? '');
}

function trimArg(s: string): string {
  let v = s.trim();
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    v = v.slice(1, -1);
  }
  return v.trim();
}

async function main(): Promise<void> {
  const url = argvUrl();
  if (!url) {
    console.error('Usage: COLLECT_TEST_URL=... OR pnpm collect:test -- --url "https://detail.1688.com/offer/..."');
    process.exitCode = 1;
    return;
  }

  const browser = new BrowserManager();
  try {
    const result = await runCollectTask({ source: '1688', url }, browser);
    console.log(JSON.stringify(result, null, 2));
    if (result.status === 'failed') process.exitCode = 1;
  } catch (e) {
    console.error(e);
    process.exitCode = 1;
  } finally {
    await browser.close().catch(() => undefined);
  }
}

void main();
