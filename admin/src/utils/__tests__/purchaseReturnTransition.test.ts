import { describe, expect, it } from 'vitest';
import { canTransitionPurchaseReturn, type PurchaseReturnTransition } from '../purchaseReturnTransition';

describe('purchase return transition permissions and status', () => {
  const all = { canManage: true, canApprove: true, canReturn: true };
  const actions: PurchaseReturnTransition[] = ['submit', 'approve', 'complete', 'cancel'];

  it('permits only actions valid for the current return status', () => {
    const statusActions: Record<string, PurchaseReturnTransition[]> = {
      draft: ['submit', 'cancel'],
      pending_approval: ['approve', 'cancel'],
      approved: ['complete', 'cancel'],
      completed: [],
      cancelled: [],
      unknown: [],
    };
    for (const [status, allowed] of Object.entries(statusActions)) {
      for (const action of actions) {
        expect(canTransitionPurchaseReturn(action, status, all)).toBe(allowed.includes(action));
      }
    }
  });

  it('does not let a manager approve or execute a return without the corresponding permission', () => {
    const manager = { canManage: true, canApprove: false, canReturn: false };
    expect(canTransitionPurchaseReturn('submit', 'draft', manager)).toBe(true);
    expect(canTransitionPurchaseReturn('cancel', 'approved', manager)).toBe(true);
    expect(canTransitionPurchaseReturn('approve', 'pending_approval', manager)).toBe(false);
    expect(canTransitionPurchaseReturn('complete', 'approved', manager)).toBe(false);
  });

  it('does not let an approver execute, cancel or submit a return without manage/return rights', () => {
    const approver = { canManage: false, canApprove: true, canReturn: false };
    expect(canTransitionPurchaseReturn('approve', 'pending_approval', approver)).toBe(true);
    expect(canTransitionPurchaseReturn('complete', 'approved', approver)).toBe(false);
    expect(canTransitionPurchaseReturn('cancel', 'draft', approver)).toBe(false);
    expect(canTransitionPurchaseReturn('submit', 'draft', approver)).toBe(false);
  });

  it('only permits execution for a return operator and only when approved', () => {
    const operator = { canManage: false, canApprove: false, canReturn: true };
    for (const status of ['draft', 'pending_approval', 'completed', 'cancelled']) {
      expect(canTransitionPurchaseReturn('complete', status, operator)).toBe(false);
    }
    expect(canTransitionPurchaseReturn('complete', 'approved', operator)).toBe(true);
    expect(canTransitionPurchaseReturn('approve', 'pending_approval', operator)).toBe(false);
  });

  it('blocks all transitions for readonly users, even when the return appears actionable', () => {
    const readonly = { canManage: false, canApprove: false, canReturn: false };
    for (const status of ['draft', 'pending_approval', 'approved']) {
      for (const action of actions) {
        expect(canTransitionPurchaseReturn(action, status, readonly)).toBe(false);
      }
    }
  });
});
