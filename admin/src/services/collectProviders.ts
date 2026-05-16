import { getJSON } from './request';

export type CollectProviderStatus = 'available' | 'beta' | 'planned' | 'disabled';

export type CollectProviderRow = {
  source: string;
  name: string;
  description: string;
  status: CollectProviderStatus;
  batchSupported: boolean;
  urlPatterns: string[];
  features: string[];
  notes: string;
};

export async function queryCollectProviders() {
  return getJSON<CollectProviderRow[]>('/api/v1/collect/providers');
}
