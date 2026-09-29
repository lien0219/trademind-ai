import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const channel = {
  id: 'e2e-channel-1',
  code: 'LOCAL',
  name: 'E2E 本地渠道',
  carrier: 'E2E 承运商',
  status: 'active',
  revision: 1,
  createdAt: '2026-08-27T00:00:00Z',
  updatedAt: '2026-08-27T00:00:00Z',
};
const rate = {
  id: 'e2e-rate-1',
  channelId: channel.id,
  code: 'CN-1KG',
  name: '国内 1kg',
  countryCode: 'CN',
  minWeightGrams: 0,
  maxWeightGrams: 1000,
  baseFeeMinor: 800,
  perKilogramFeeMinor: 200,
  currency: 'CNY',
  priority: 10,
  status: 'active',
  revision: 1,
  channelCode: channel.code,
  channelName: channel.name,
  carrier: channel.carrier,
  createdAt: '2026-08-27T00:00:00Z',
  updatedAt: '2026-08-27T00:00:00Z',
};

test.describe('logistics channel workspace', () => {
  test('renders local channels and explicitly creates one configured channel', async ({
    admin,
    page,
  }) => {
    await page.route('**/api/v1/logistics/**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      const path = new URL(route.request().url()).pathname;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          path.endsWith('/rate-templates')
            ? ok({ list: [rate] })
            : ok({ list: [channel] }),
        ),
      });
    });
    admin.writeGuard.allow({
      operation: 'create-logistics-channel',
      method: 'POST',
      path: /^\/api\/v1\/logistics\/channels$/,
      response: ok(channel),
    });

    await admin.goto('/orders/logistics-channels');
    await expect(
      page.getByText('物流渠道与运费模板', { exact: true }),
    ).toBeVisible();
    await expect(page.getByText('E2E 本地渠道').first()).toBeVisible();
    await expectNoRootOverflow(page);

    await page.getByRole('button', { name: '新建渠道' }).click();
    await page.getByLabel('渠道编码').fill('EXPRESS');
    await page.getByLabel('渠道名称').fill('E2E 快线');
    await page.getByLabel('承运商').fill('E2E 承运商 B');
    await page
      .locator('.ant-modal-content')
      .getByRole('button', { name: '确 定' })
      .click();
    await admin.writeGuard.expectRequestCount('create-logistics-channel', 1);
    expect(
      admin.writeGuard.calls('create-logistics-channel')[0]?.postDataJSON,
    ).toEqual({ code: 'EXPRESS', name: 'E2E 快线', carrier: 'E2E 承运商 B' });
  });
});
