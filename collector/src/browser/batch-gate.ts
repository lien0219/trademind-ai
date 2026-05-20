import type { CollectTaskErrorCode } from '../types/task.js';

/** Serialize 1688 batch-mode collects inside one collector process (defense in depth). */
let batch1688Active = 0;
const batch1688Waiters: Array<() => void> = [];
const batch1688Max = Math.max(1, Number(process.env.COLLECTOR_BATCH_1688_CONCURRENCY ?? '1') || 1);

async function acquire1688BatchSlot(): Promise<() => void> {
  if (batch1688Active < batch1688Max) {
    batch1688Active++;
    return () => {
      batch1688Active = Math.max(0, batch1688Active - 1);
      const next = batch1688Waiters.shift();
      if (next) next();
    };
  }
  await new Promise<void>((resolve) => batch1688Waiters.push(resolve));
  batch1688Active++;
  return () => {
    batch1688Active = Math.max(0, batch1688Active - 1);
    const next = batch1688Waiters.shift();
    if (next) next();
  };
}

export async function with1688BatchGate<T>(
  batchMode: boolean | undefined,
  fn: () => Promise<T>,
): Promise<T> {
  if (!batchMode) {
    return fn();
  }
  const release = await acquire1688BatchSlot();
  try {
    return await fn();
  } finally {
    release();
  }
}

export type { CollectTaskErrorCode };
