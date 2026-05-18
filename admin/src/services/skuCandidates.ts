import { getWithParams, postJSON } from '@/services/request';

export type SkuCandidateRow = {
  productId: string;
  productTitle?: string;
  productSkuId: string;
  skuCode?: string;
  skuName?: string;
  stock?: number;
  attrs?: Record<string, unknown>;
  confidence: number;
  reason: string;
  matchSignals?: string[];
  source: string;
  sourceBreakdown?: Record<string, number>;
};

export type OrderItemSkuCandidatesDTO = {
  orderItemId: string;
  list: SkuCandidateRow[];
};

export async function getOrderItemSkuCandidates(
  itemId: string,
  opts?: { limit?: number; includeLowConfidence?: boolean },
): Promise<OrderItemSkuCandidatesDTO> {
  return getWithParams<OrderItemSkuCandidatesDTO>(
    `/api/v1/order-items/${encodeURIComponent(itemId)}/sku-candidates`,
    {
      limit: opts?.limit,
      includeLowConfidence: opts?.includeLowConfidence === true ? 'true' : undefined,
    },
  );
}

export type OrderSkuCandidatesBatchDTO = {
  orderId: string;
  items: OrderItemSkuCandidatesDTO[];
};

export async function postOrderSkuCandidatesBatch(
  orderId: string,
  body: { orderItemIds: string[]; limit?: number; includeLowConfidence?: boolean },
): Promise<OrderSkuCandidatesBatchDTO> {
  return postJSON(`/api/v1/orders/${encodeURIComponent(orderId)}/sku-candidates/batch`, body);
}
