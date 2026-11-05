import { describe, expect, it, vi } from 'vitest';
import { financialActionRequestKey } from '../financialActionKey';

describe('financial action idempotency key', () => {
  it('reuses the same key for an identical command retry', () => {
    const generate = vi.fn().mockReturnValue('key-1');
    const original = financialActionRequestKey(undefined, 'adjust', ['charge-1', 250, 'carrier correction'], generate);
    const retry = financialActionRequestKey(original, 'adjust', ['charge-1', 250, 'carrier correction'], generate);

    expect(retry).toBe(original);
    expect(generate).toHaveBeenCalledTimes(1);
  });

  it('generates new keys when amount or reason changes after a failed attempt', () => {
    let count = 0;
    const generate = () => `key-${++count}`;
    const original = financialActionRequestKey(undefined, 'adjust', ['charge-1', 250, 'correction'], generate);
    const changedAmount = financialActionRequestKey(original, 'adjust', ['charge-1', -250, 'correction'], generate);
    const changedReason = financialActionRequestKey(changedAmount, 'adjust', ['charge-1', -250, 'updated reason'], generate);

    expect([original.key, changedAmount.key, changedReason.key]).toEqual(['key-1', 'key-2', 'key-3']);
  });

  it('separates distinct target facts, finance records and action types', () => {
    let count = 0;
    const generate = () => `key-${++count}`;
    const first = financialActionRequestKey(undefined, 'reverse', ['charge-1', 'fact-a', null, 'same reason'], generate);
    const otherFact = financialActionRequestKey(first, 'reverse', ['charge-1', 'fact-b', null, 'same reason'], generate);
    const otherRecord = financialActionRequestKey(otherFact, 'reverse', ['charge-2', 'fact-b', null, 'same reason'], generate);
    const otherAction = financialActionRequestKey(otherRecord, 'adjust', ['charge-2', 'fact-b', null, 'same reason'], generate);

    expect(new Set([first.key, otherFact.key, otherRecord.key, otherAction.key]).size).toBe(4);
  });

  it('uses unambiguous signatures when values contain delimiters', () => {
    const generate = vi.fn().mockReturnValueOnce('key-1').mockReturnValueOnce('key-2');
    const first = financialActionRequestKey(undefined, 'adjust', ['a:b', 1, 'c'], generate);
    const other = financialActionRequestKey(first, 'adjust', ['a', 1, 'b:c'], generate);
    expect(other.key).not.toBe(first.key);
  });
});
