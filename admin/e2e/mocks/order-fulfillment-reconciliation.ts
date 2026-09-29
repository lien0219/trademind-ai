import { ok } from './envelope';

export const E2E_RECONCILIATION_ORDER_ID = 'e2e-order-reconciliation-1';

export const e2eFulfillmentReconciliation = {
  orderId: E2E_RECONCILIATION_ORDER_ID,
  orderNo: 'SO-E2E-RECON-0001',
  status: 'shipped',
  paymentStatus: 'paid',
  fulfillmentStatus: 'fulfilled',
  warehouseId: 'e2e-warehouse-main',
  reserve: { expected: 2, actual: 2 },
  deduct: { expected: 2, actual: 2 },
  release: { expected: 0, actual: 0 },
  restore: { expected: 0, actual: 0 },
  shipmentCount: 1,
  effectCount: 2,
  lastInventoryActionAt: '2026-08-24T03:00:00Z',
  reconciliationStatus: 'matched',
  issues: [],
  timeline: [
    { id: 'e2e-reconciliation-effect-reserve', type: 'effect', action: 'reserve', status: 'success', quantity: 2, createdAt: '2026-08-24T02:00:00Z' },
    { id: 'e2e-reconciliation-effect-deduct', type: 'effect', action: 'deduct', status: 'success', quantity: 2, createdAt: '2026-08-24T03:00:00Z' },
  ],
};

export function fulfillmentReconciliationResponse(path: string) {
  if (path === '/api/v1/orders/fulfillment-reconciliation') {
    return ok({ list: [e2eFulfillmentReconciliation], pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 } });
  }
  if (path === `/api/v1/orders/${E2E_RECONCILIATION_ORDER_ID}/fulfillment-reconciliation`) return ok(e2eFulfillmentReconciliation);
  return null;
}
