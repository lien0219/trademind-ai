import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { E2E_SHOP_ID } from '../mocks/product.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

const policy = {
  shopId: E2E_SHOP_ID,
  shopName: 'E2E 抖店测试店铺',
  platform: 'douyin_shop',
  globalEnabled: false,
  workerAvailable: true,
  enabled: false,
  effectiveEnabled: false,
  tone: 'professional',
  shopPolicy: '',
  maxReplyRunes: 600,
  maxRepliesPerHour: 20,
  requireOrderContext: true,
  lowRiskOnly: true,
  updatedAt: '2026-08-12T00:00:00Z',
};

const setting = {
  messageSyncEnabled: false,
  autoReplyEnabled: false,
  pollIntervalSeconds: 60,
  workerAvailable: true,
  effectiveEnabled: false,
  updatedAt: '2026-08-12T00:00:00Z',
};

async function mockAutoReplySetting(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/customer/auto-reply-setting', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(setting)) });
  });
}

test.describe('@smoke customer AI auto reply', () => {
  test('renders default-off policy and recent runs without writes', async ({ admin, page }) => {
    await mockAutoReplySetting(page);
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-policy`, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(policy)) });
    });
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-runs`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok([])) });
    });

    await admin.goto('/customer/auto-reply-settings');

    await expect(page.getByText('运行设置', { exact: true })).toBeVisible();
    await expect(page.getByText('AI 自动回复后台任务未启用')).toBeVisible();
    await expect(page.getByText('当前不会自动向买家发送消息')).toBeVisible();
    await expect(page.getByText('暂无自动回复记录')).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('only submits after explicit enable confirmation', async ({ admin, page }) => {
    await mockAutoReplySetting(page);
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-policy`, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(policy)) });
    });
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-runs`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok([])) });
    });
    admin.writeGuard.allow({
      operation: 'update-auto-reply-policy',
      method: 'PUT',
      path: new RegExp(`/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-policy$`),
      response: ok({ ...policy, enabled: true }),
    });

    await admin.goto('/customer/auto-reply-settings');
    await page.getByRole('switch', { name: '店铺自动回复' }).click();
    await page.getByRole('button', { name: '保存策略' }).dblclick();

    await expect(page.locator('.ant-modal-confirm-title').filter({ hasText: '启用 AI 自动回复' })).toBeVisible();
    await expect(page.locator('.ant-modal-confirm-title').filter({ hasText: '启用 AI 自动回复' })).toHaveCount(1);
    await admin.writeGuard.expectRequestCount('update-auto-reply-policy', 0);
    await page.getByRole('button', { name: '确认执行' }).click();
    await admin.writeGuard.expectRequestCount('update-auto-reply-policy', 1);
    expect(admin.writeGuard.calls('update-auto-reply-policy')[0]?.postDataJSON).toMatchObject({
      enabled: true,
      lowRiskOnly: true,
      requireOrderContext: true,
    });
  });

  test('confirms before enabling the tenant runtime switch', async ({ admin, page }) => {
    await mockAutoReplySetting(page);
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-policy`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(policy)) });
    });
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-runs`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok([])) });
    });
    admin.writeGuard.allow({
      operation: 'update-auto-reply-setting',
      method: 'PUT',
      path: /\/api\/v1\/customer\/auto-reply-setting$/,
      response: ok({ ...setting, messageSyncEnabled: true, autoReplyEnabled: true, effectiveEnabled: true }),
    });

    await admin.goto('/customer/auto-reply-settings');
    await page.getByRole('switch', { name: '自动同步客服消息' }).click();
    await page.getByRole('switch', { name: 'AI 自动回复总开关' }).click();
    await page.getByRole('button', { name: '保存运行设置' }).click();

    await expect(page.locator('.ant-modal-confirm-title').filter({ hasText: '开启自动回复总开关' })).toBeVisible();
    await admin.writeGuard.expectRequestCount('update-auto-reply-setting', 0);
    await page.getByRole('button', { name: '确认执行' }).click();
    await admin.writeGuard.expectRequestCount('update-auto-reply-setting', 1);
  });

  test('blocks runtime setting writes after load failure and recovers on retry', async ({ admin, page }) => {
    let settingRequests = 0;
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503 \(Service Unavailable\)/);
    await page.route('**/api/v1/customer/auto-reply-setting', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      settingRequests += 1;
      if (settingRequests === 1) {
        await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 50000, message: 'setting unavailable', data: null }) });
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(setting)) });
    });
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-policy`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(policy)) });
    });
    await page.route(`**/api/v1/customer/shops/${E2E_SHOP_ID}/auto-reply-runs`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok([])) });
    });

    await admin.goto('/customer/auto-reply-settings');
    await expect(page.getByText('运行设置加载失败')).toBeVisible();
    await expect(page.getByRole('button', { name: '保存运行设置' })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);

    await page.getByRole('button', { name: '重新加载' }).click();
    await expect(page.getByRole('button', { name: '保存运行设置' })).toBeEnabled();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
