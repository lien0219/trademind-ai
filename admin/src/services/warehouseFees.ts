import { getJSON, getWithParams, postJSON, putJSON } from '@/services/request';

export type WarehouseFeeRateCard = {
  id: string;
  warehouseId: string;
  warehouseCode?: string;
  warehouseName?: string;
  code: string;
  name: string;
  currency: string;
  outboundBaseFeeMinor: number;
  pickingFeePerItemMinor: number;
  packingFeePerPackageMinor: number;
  status: 'active' | 'inactive';
  revision: number;
  createdAt: string;
  updatedAt: string;
};

export type WarehouseFeeRateRevision = Omit<
  WarehouseFeeRateCard,
  'warehouseId' | 'warehouseCode' | 'warehouseName' | 'code' | 'updatedAt'
> & {
  rateCardId: string;
};

export type WarehouseFeeRateCardDetail = WarehouseFeeRateCard & {
  revisions: WarehouseFeeRateRevision[];
};

export type WarehouseFeeCandidate = {
  orderId: string;
  orderNo: string;
  shopId?: string;
  shopName?: string;
  platform: string;
  currency: string;
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  waveId: string;
  waveNo: string;
  waveRevision: number;
  waveStatus: string;
  waveOrderId: string;
  waveOrderStatus: string;
  shipmentId?: string;
  packVerificationId?: string;
  packageCode: string;
  itemQuantity: number;
  packageQuantity: number;
  fulfilledAt?: string;
  verifiedAt?: string;
  snapshotId?: string;
  confirmed: boolean;
};

export type WarehouseFeePreview = {
  orderId: string;
  orderNo: string;
  shopId?: string;
  warehouseId: string;
  warehouseCode: string;
  warehouseName: string;
  waveId: string;
  waveNo: string;
  waveRevision: number;
  waveOrderId: string;
  packVerificationId: string;
  packageCode: string;
  itemQuantity: number;
  packageQuantity: number;
  rateCardId: string;
  rateCardCode: string;
  rateCardName: string;
  rateCardRevision: number;
  outboundBaseFeeMinor: number;
  pickingFeePerItemMinor: number;
  pickingFeeMinor: number;
  packingFeePerPackageMinor: number;
  packingFeeMinor: number;
  amountMinor: number;
  currency: string;
  calculationHash: string;
};

export type WarehouseFeeAdjustment = {
  id: string;
  snapshotId: string;
  orderId: string;
  factType: 'adjustment' | 'reversal';
  amountMinor: number;
  currency: string;
  reason: string;
  reversesAdjustmentId?: string;
  actorId?: string;
  createdAt: string;
};

export type WarehouseFeeSnapshot = WarehouseFeePreview & {
  id: string;
  rateCardCode: string;
  rateCardName: string;
  idempotencyKey: string;
  confirmedBy?: string;
  confirmedAt: string;
  adjustmentCount: number;
  adjustmentMinor: number;
  netAmountMinor: number;
};

export type WarehouseFeeSnapshotDetail = WarehouseFeeSnapshot & {
  adjustments: WarehouseFeeAdjustment[];
};

export type WarehouseFeePage<T> = {
  list: T[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

type RateCardPayload = {
  warehouseId: string;
  code: string;
  name: string;
  currency: string;
  outboundBaseFeeMinor: number;
  pickingFeePerItemMinor: number;
  packingFeePerPackageMinor: number;
};

export function listWarehouseFeeRateCards(params?: {
  warehouseId?: string;
  includeInactive?: boolean;
}) {
  return getWithParams<{ list: WarehouseFeeRateCard[] }>('/api/v1/warehouse-fee-rate-cards', {
    warehouseId: params?.warehouseId,
    includeInactive: params?.includeInactive ? 'true' : undefined,
  });
}

export function getWarehouseFeeRateCard(id: string) {
  return getJSON<WarehouseFeeRateCardDetail>(
    `/api/v1/warehouse-fee-rate-cards/${encodeURIComponent(id)}`,
  );
}

export function createWarehouseFeeRateCard(payload: RateCardPayload) {
  return postJSON<WarehouseFeeRateCard>('/api/v1/warehouse-fee-rate-cards', payload);
}

export function updateWarehouseFeeRateCard(
  id: string,
  payload: Omit<RateCardPayload, 'warehouseId' | 'code'> & {
    expectedRevision: number;
    status: 'active' | 'inactive';
  },
) {
  return putJSON<WarehouseFeeRateCard, typeof payload>(
    `/api/v1/warehouse-fee-rate-cards/${encodeURIComponent(id)}`,
    payload,
  );
}

export function queryWarehouseFeeCandidates(params: {
  page?: number;
  pageSize?: number;
  orderNo?: string;
  warehouseId?: string;
}) {
  return getWithParams<WarehouseFeePage<WarehouseFeeCandidate>>(
    '/api/v1/warehouse-operation-fees/candidates',
    params,
  );
}

export function previewWarehouseFee(payload: {
  orderId: string;
  rateCardId: string;
  rateCardRevision: number;
}) {
  return postJSON<WarehouseFeePreview>('/api/v1/warehouse-operation-fees/preview', payload);
}

export function confirmWarehouseFee(
  payload: Pick<
    WarehouseFeePreview,
    'orderId' | 'rateCardId' | 'rateCardRevision' | 'waveRevision' | 'calculationHash'
  > & { idempotencyKey: string },
) {
  return postJSON<{ snapshot: WarehouseFeeSnapshot; replayed: boolean }>(
    '/api/v1/warehouse-operation-fees',
    payload,
  );
}

export function queryWarehouseFeeSnapshots(params: {
  page?: number;
  pageSize?: number;
  orderNo?: string;
  warehouseId?: string;
  currency?: string;
}) {
  return getWithParams<WarehouseFeePage<WarehouseFeeSnapshot>>(
    '/api/v1/warehouse-operation-fees',
    params,
  );
}

export function getWarehouseFeeSnapshot(id: string) {
  return getJSON<WarehouseFeeSnapshotDetail>(
    `/api/v1/warehouse-operation-fees/${encodeURIComponent(id)}`,
  );
}

export function createWarehouseFeeAdjustment(
  snapshotId: string,
  payload: { amountMinor: number; reason: string; idempotencyKey: string },
) {
  return postJSON<{ adjustment: WarehouseFeeAdjustment; replayed: boolean }>(
    `/api/v1/warehouse-operation-fees/${encodeURIComponent(snapshotId)}/adjustments`,
    payload,
  );
}

export function reverseWarehouseFeeAdjustment(
  snapshotId: string,
  adjustmentId: string,
  payload: { reason: string; idempotencyKey: string },
) {
  return postJSON<{ adjustment: WarehouseFeeAdjustment; replayed: boolean }>(
    `/api/v1/warehouse-operation-fees/${encodeURIComponent(snapshotId)}/adjustments/${encodeURIComponent(adjustmentId)}/reverse`,
    payload,
  );
}

export function warehouseFeeIdempotencyKey(action: string) {
  const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `admin-warehouse-fee-${action}-${random}`.slice(0, 128);
}
