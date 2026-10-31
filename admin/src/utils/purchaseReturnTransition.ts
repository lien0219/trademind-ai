/**
 * UI-level guard for purchase-return transitions. The backend remains the
 * authority for tenant permissions, approval separation and inventory writes.
 * Keep this dependency-free so it is safe to use in shared client code.
 */
export type PurchaseReturnTransition = 'submit' | 'approve' | 'complete' | 'cancel';

export type PurchaseReturnTransitionPermissions = {
  canManage: boolean;
  canApprove: boolean;
  canReturn: boolean;
};

export function canTransitionPurchaseReturn(
  action: PurchaseReturnTransition,
  status: string,
  permissions: PurchaseReturnTransitionPermissions,
): boolean {
  switch (action) {
    case 'submit':
      return permissions.canManage && status === 'draft';
    case 'approve':
      return permissions.canApprove && status === 'pending_approval';
    case 'complete':
      return permissions.canReturn && status === 'approved';
    case 'cancel':
      return permissions.canManage && ['draft', 'pending_approval', 'approved'].includes(status);
    default:
      return false;
  }
}
