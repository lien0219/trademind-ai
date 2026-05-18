import { execa } from 'execa';

/** Run a command; returns trimmed stdout or empty string. */
export async function runCapture(cmd: string, args: string[]): Promise<{ ok: boolean; out: string; err: string }> {
  try {
    const r = await execa(cmd, args, {
      reject: false,
      stripFinalNewline: true,
    });
    return {
      ok: r.exitCode === 0,
      out: typeof r.stdout === 'string' ? r.stdout.trim() : '',
      err: typeof r.stderr === 'string' ? r.stderr.trim() : '',
    };
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    return { ok: false, out: '', err: msg };
  }
}

export async function commandPrintableVersion(cmd: string, args: string[]): Promise<string | undefined> {
  const r = await runCapture(cmd, args);
  if (!r.ok) return undefined;
  const merged = [r.out, r.err].filter(Boolean).join('\n').trim();
  if (!merged) return undefined;
  const line = merged.split(/\r?\n/)[0]?.trim();
  return line || undefined;
}
