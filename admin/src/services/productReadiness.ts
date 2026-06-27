import { getWithParams, postJSON } from '@/services/request';

export type ReadinessCheckItem = {
  group: string;
  code: string;
  title?: string;
  level: 'warning' | 'error' | string;
  message: string;
  suggestion?: string;
  relatedResourceType?: string;
  relatedResourceId?: string;
  technicalDetails?: Record<string, unknown>;
};

export type ProductReadinessResult = {
  productId: string;
  platform?: string;
  shopId?: string;
  mode?: string;
  status: string;
  statusLabel?: string;
  result?: 'passed' | 'warning' | 'failed' | string;
  resultLabel?: string;
  score: number;
  canPublish: boolean;
  errorCount: number;
  warningCount: number;
  checks: ReadinessCheckItem[];
};

export async function getProductReadiness(
  productId: string,
  params: { platform?: string; shopId?: string; mode?: string },
): Promise<ProductReadinessResult> {
  return getWithParams<ProductReadinessResult>(`/api/v1/products/${encodeURIComponent(productId)}/readiness`, {
    platform: params.platform,
    shopId: params.shopId,
    mode: params.mode ?? 'draft',
  });
}

export async function batchCheckProductReadiness(payload: {
  productIds: string[];
  platform: string;
  shopId: string;
}): Promise<{ list: ProductReadinessResult[] }> {
  return postJSON<{ list: ProductReadinessResult[] }>('/api/v1/products/readiness/batch', payload);
}
