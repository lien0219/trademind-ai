/**
 * Bind a rate-card create/revision to the modal session that started it.
 * This is a presentation guard, not a substitute for backend authorization.
 */
export type WarehouseFeeRateMutationScope = {
  mode: 'create' | 'revise';
  rateId?: string;
  revision?: number;
  isCurrent: () => boolean;
};

export function canSubmitWarehouseFeeRateMutation(
  scope: WarehouseFeeRateMutationScope | undefined,
  input: {
    canManage: boolean;
    editing?: { id: string; revision: number };
  },
): boolean {
  if (!scope || !scope.isCurrent() || !input.canManage) return false;
  if (scope.mode === 'create') {
    return !input.editing && !scope.rateId && scope.revision === undefined;
  }
  return Boolean(
    input.editing &&
    scope.rateId &&
    scope.rateId === input.editing.id &&
    Number.isSafeInteger(scope.revision) &&
    scope.revision === input.editing.revision,
  );
}
