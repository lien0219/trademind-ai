import { request } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  createShippingChannel,
  createShippingRateTemplate,
  listShippingChannels,
  listShippingRateTemplates,
  updateShippingChannel,
  updateShippingRateTemplate,
} from '../logistics';

const requestMock = vi.mocked(request);

describe('logistics service', () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: {} });
  });

  it('keeps local channel and rate contracts stable', async () => {
    await listShippingChannels();
    expect(requestMock).toHaveBeenLastCalledWith('/api/v1/logistics/channels', {
      method: 'GET',
    });
    const channel = { code: 'LOCAL', name: 'Local', carrier: 'Carrier' };
    await createShippingChannel(channel);
    expect(requestMock).toHaveBeenLastCalledWith('/api/v1/logistics/channels', {
      method: 'POST',
      data: channel,
    });
    const channelUpdate = {
      expectedRevision: 1,
      name: 'Local 2',
      carrier: 'Carrier',
      status: 'active' as const,
    };
    await updateShippingChannel('channel/1', channelUpdate);
    expect(requestMock).toHaveBeenLastCalledWith(
      '/api/v1/logistics/channels/channel%2F1',
      { method: 'PUT', data: channelUpdate },
    );

    await listShippingRateTemplates();
    expect(requestMock).toHaveBeenLastCalledWith(
      '/api/v1/logistics/rate-templates',
      { method: 'GET' },
    );
    const rate = {
      channelId: 'channel-1',
      code: 'CN-1',
      name: 'CN',
      countryCode: 'CN',
      minWeightGrams: 0,
      maxWeightGrams: 1000,
      baseFeeMinor: 800,
      perKilogramFeeMinor: 200,
      currency: 'CNY',
      priority: 1,
    };
    await createShippingRateTemplate(rate);
    expect(requestMock).toHaveBeenLastCalledWith(
      '/api/v1/logistics/rate-templates',
      { method: 'POST', data: rate },
    );
    const update = { ...rate, expectedRevision: 1, status: 'active' as const };
    await updateShippingRateTemplate('rate/1', update);
    expect(requestMock).toHaveBeenLastCalledWith(
      '/api/v1/logistics/rate-templates/rate%2F1',
      { method: 'PUT', data: update },
    );
  });
});
