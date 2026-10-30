/**
 * Tracks the exact payload associated with a client-generated idempotency key.
 * An uncertain network result can be retried with the same key; an edited
 * command must use a fresh key to avoid conflicting with the original write.
 *
 * Only pass the normalized request body WITHOUT its idempotencyKey.
 * Payloads must contain JSON-serializable values in a deterministic order.
 */
export type IdempotentCommandKey = {
  signature: string;
  key: string;
};

export function idempotentCommandKey(
  previous: IdempotentCommandKey | undefined,
  action: string,
  payload: unknown,
  generate: (action: string) => string,
  initialKey?: string,
): IdempotentCommandKey {
  const signature = JSON.stringify([action, payload]);
  if (previous?.signature === signature) return previous;
  return {
    signature,
    key: previous ? generate(action) : (initialKey || generate(action)),
  };
}
