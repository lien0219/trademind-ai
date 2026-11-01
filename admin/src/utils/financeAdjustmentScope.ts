/**
 * A finance adjustment is valid only for the exact ledger entry and operation
 * selected when its form was opened. Backend authorization remains required.
 */
export type FinanceAdjustmentAction = 'adjust' | 'reverse';

export type FinanceAdjustmentScope = {
  recordId: string;
  action: FinanceAdjustmentAction;
  targetId?: string;
  isCurrent: () => boolean;
};

export function canSubmitFinanceAdjustment(
  scope: FinanceAdjustmentScope | undefined,
  input: {
    recordId?: string;
    action: FinanceAdjustmentAction;
    targetId?: string;
    canManage: boolean;
  },
): boolean {
  if (!scope || !scope.isCurrent() || !input.canManage || !input.recordId) return false;
  if (scope.recordId !== input.recordId || scope.action !== input.action) return false;
  if ((scope.targetId || '') !== (input.targetId || '')) return false;
  return input.action !== 'reverse' || Boolean(input.targetId);
}
