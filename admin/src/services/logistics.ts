import { getJSON, postJSON, putJSON } from '@/services/request';

export type LogisticsStatus = 'active' | 'inactive';

export type ShippingChannel = {
  id: string;
  code: string;
  name: string;
  carrier: string;
  status: LogisticsStatus;
  revision: number;
  createdAt: string;
  updatedAt: string;
};

export type ShippingRateTemplate = {
  id: string;
  channelId: string;
  warehouseId?: string;
  code: string;
  name: string;
  countryCode: string;
  region?: string;
  postalCodePrefix?: string;
  minWeightGrams: number;
  maxWeightGrams: number;
  baseFeeMinor: number;
  perKilogramFeeMinor: number;
  currency: string;
  priority: number;
  status: LogisticsStatus;
  revision: number;
  channelCode?: string;
  channelName?: string;
  carrier?: string;
  warehouseCode?: string;
  warehouseName?: string;
  createdAt: string;
  updatedAt: string;
};

export type RatePayload = Omit<
  ShippingRateTemplate,
  | 'id'
  | 'status'
  | 'revision'
  | 'channelCode'
  | 'channelName'
  | 'carrier'
  | 'warehouseCode'
  | 'warehouseName'
  | 'createdAt'
  | 'updatedAt'
>;

export const listShippingChannels = () =>
  getJSON<{ list: ShippingChannel[] }>('/api/v1/logistics/channels');
export const createShippingChannel = (
  payload: Pick<ShippingChannel, 'code' | 'name' | 'carrier'>,
) => postJSON<ShippingChannel>('/api/v1/logistics/channels', payload);
export const updateShippingChannel = (
  id: string,
  payload: Pick<ShippingChannel, 'name' | 'carrier' | 'status'> & {
    expectedRevision: number;
  },
) =>
  putJSON<ShippingChannel, typeof payload>(
    `/api/v1/logistics/channels/${encodeURIComponent(id)}`,
    payload,
  );
export const listShippingRateTemplates = () =>
  getJSON<{ list: ShippingRateTemplate[] }>('/api/v1/logistics/rate-templates');
export const createShippingRateTemplate = (payload: RatePayload) =>
  postJSON<ShippingRateTemplate>('/api/v1/logistics/rate-templates', payload);
export const updateShippingRateTemplate = (
  id: string,
  payload: RatePayload & { expectedRevision: number; status: LogisticsStatus },
) =>
  putJSON<ShippingRateTemplate, typeof payload>(
    `/api/v1/logistics/rate-templates/${encodeURIComponent(id)}`,
    payload,
  );
