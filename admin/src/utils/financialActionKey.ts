/**
 * Keep the same idempotency key when retrying an identical financial command.
 * A changed amount, reason, target or action requires a new key; otherwise the
 * backend correctly rejects it as a conflicting replay of the old payload.
 */
export type FinancialActionRequestKey = { signature: string; key: string };

export function financialActionRequestKey(
  current: FinancialActionRequestKey | undefined,
  action: string,
  payload: ReadonlyArray<string | number | null>,
  generate: (action: string) => string,
): FinancialActionRequestKey {
  const signature = JSON.stringify([action, ...payload]);
  if (current?.signature === signature) return current;
  return { signature, key: generate(action) };
}
