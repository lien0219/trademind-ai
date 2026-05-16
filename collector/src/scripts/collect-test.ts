/**
 * 本地一键调试采集：
 *
 * ```
 * pnpm collect:test -- --url "https://detail.1688.com/offer/xxx.html"
 * pnpm collect:test -- --source aliexpress --url "https://www.aliexpress.com/item/100500xxxx.html"
 * ```
 *
 * 或通过环境变量 `COLLECT_TEST_URL`；可选 `COLLECT_TEST_SOURCE`（默认 1688）。
 */

import { BrowserManager } from '../browser/manager.js';
import { runCollectTask } from '../tasks/collect-task.js';

function trimArg(s: string): string {
  let v = s.trim();
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    v = v.slice(1, -1);
  }
  return v.trim();
}

function argvFlag(flag: string): string | undefined {
  const args = process.argv.slice(2);
  for (let i = 0; i < args.length - 1; i++) {
    if (args[i] === flag) return trimArg(args[i + 1] ?? '');
  }
  return undefined;
}

async function main(): Promise<void> {
  const url = argvFlag('--url') ?? trimArg(process.env.COLLECT_TEST_URL ?? '');
  /** 不传 `--source` / `COLLECT_TEST_SOURCE` → 仍为 1688（旧脚本体验） */
  const explicitRaw =
    argvFlag('--source') ?? argvFlag('-s') ?? trimArg(process.env.COLLECT_TEST_SOURCE ?? '');
  const source = explicitRaw.trim() ? explicitRaw.trim().toLowerCase() : '1688';

  if (!url) {
    console.error(
      'Usage: COLLECT_TEST_URL=... OR COLLECT_TEST_SOURCE=...\n',
      'pnpm collect:test -- --url "...1688..."\n',
      'pnpm collect:test -- --source aliexpress --url "https://www.aliexpress.com/item/100500xxxx.html"',
    );
    process.exitCode = 1;
    return;
  }

  const browser = new BrowserManager();
  try {
    const result = await runCollectTask({ source, url }, browser);
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
