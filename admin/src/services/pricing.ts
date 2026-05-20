import { postJSON } from './request';

export type PricingRule = {
  markupType?: 'percent' | 'fixed' | 'none';
  markupPercent?: number;
  markupAmount?: number;
  minPublishPrice?: number;
  minMarginPercent?: number;
  roundingMode?: 'none' | 'integer' | '.9' | '.99' | '.95';
  exchangeRate?: number;
};

export type PricingCalculateResult = {
  basePrice: number;
  costPrice?: number;
  currentPrice?: number;
  calculatedPrice: number;
  currency: string;
};

export type PricingPreviewLine = {
  productSkuId: string;
  productId: string;
  skuCode: string;
  skuName: string;
  costPrice?: number;
  currentPrice?: number;
  calculatedPrice: number;
  delta: number;
};

export type PricingApplySummary = {
  productCount: number;
  skuCount: number;
  updated?: number;
  skipped?: number;
  preview?: PricingPreviewLine[];
};

export async function calculatePublishPrice(body: {
  productSkuId?: string;
  basePrice?: number;
  costPrice?: number;
  platform?: string;
  currency?: string;
  rule?: PricingRule;
}) {
  return postJSON<PricingCalculateResult>('/api/v1/pricing/calculate', body);
}

export async function applyProductPricing(
  productId: string,
  body: {
    platform?: string;
    rule?: PricingRule;
    skuIds?: string[];
    confirm: boolean;
  },
) {
  return postJSON<PricingApplySummary>(`/api/v1/products/${productId}/pricing/apply`, body);
}

export async function batchApplyProductPricing(body: {
  productIds?: string[];
  filters?: { status?: string; source?: string; keyword?: string };
  platform?: string;
  rule?: PricingRule;
  confirm: boolean;
  confirmAll?: boolean;
}) {
  return postJSON<PricingApplySummary>('/api/v1/products/pricing/batch-apply', body);
}
