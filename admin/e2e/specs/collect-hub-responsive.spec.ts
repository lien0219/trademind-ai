import type { Page } from '@playwright/test';
import { layoutTokens } from '../../src/constants/layoutTokens';
import { expect, test } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

const longToken = 'long-unbroken-value-'.repeat(32);
const longSourceUrl = `https://detail.1688.com/offer/${longToken}.html?trace=${longToken}`;
const longProviderName = `超长采集来源-${longToken}`;
const longProfileDomain = `${longToken}.example.test`;
const collectTaskId = 'e2e-collect-task-long-url';

async function routeLongCollectContent(page: Page) {
  await page.route('**/api/v1/collect/**', async (route) => {
    const url = new URL(route.request().url());
    const pagination = { page: 1, pageSize: 20, total: 1, totalPages: 1 };

    if (url.pathname === '/api/v1/collect/providers') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok([
            {
              source: 'custom',
              name: longProviderName,
              description: '自定义采集来源',
              status: 'available',
              batchSupported: false,
              urlPatterns: [longSourceUrl],
              features: ['title', 'price', 'mainImages'],
              notes: '',
            },
          ]),
        ),
      });
      return;
    }

    if (url.pathname === '/api/v1/collect/browser-profiles') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: [
              {
                id: 'e2e-browser-profile-long-content',
                name: longProviderName,
                domain: longProfileDomain,
                profileKey: 'e2e-browser-profile',
                provider: 'custom',
                status: 'active',
                lastCheckStatus: 'login_required',
                createdAt: '2026-08-11T08:00:00Z',
                updatedAt: '2026-08-11T08:00:00Z',
              },
            ],
            pagination,
          }),
        ),
      });
      return;
    }

    if (url.pathname === '/api/v1/collect/tasks') {
      const failedOnly = url.searchParams.get('status') === 'failed';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: failedOnly
              ? []
              : [
                  {
                    id: collectTaskId,
                    source: '1688',
                    sourceUrl: longSourceUrl,
                    status: 'pending',
                    createdAt: '2026-08-11T08:00:00Z',
                    updatedAt: '2026-08-11T08:00:00Z',
                  },
                ],
            pagination: failedOnly ? { ...pagination, total: 0 } : pagination,
          }),
        ),
      });
      return;
    }

    if (url.pathname === `/api/v1/collect/tasks/${collectTaskId}/events`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: [
              {
                id: 'e2e-collect-task-event-created',
                taskId: collectTaskId,
                eventType: 'task.created',
                fromStatus: null,
                toStatus: 'pending',
                message: 'collect task persisted',
                retryCount: 0,
                maxRetries: 3,
                createdAt: '2026-08-11T08:00:00Z',
              },
            ],
            pagination,
          }),
        ),
      });
      return;
    }

    if (url.pathname === `/api/v1/collect/tasks/${collectTaskId}`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            id: collectTaskId,
            source: '1688',
            sourceUrl: longSourceUrl,
            status: 'pending',
            retryCount: 0,
            maxRetries: 3,
            createdAt: '2026-08-11T08:00:00Z',
            updatedAt: '2026-08-11T08:00:00Z',
          }),
        ),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [], pagination: { ...pagination, total: 0 } })),
    });
  });
}

test.describe('@smoke @collect-hub-responsive 采集中心长文本', () => {
  for (const viewport of viewports) {
    test(`keeps long dynamic content contained at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await routeLongCollectContent(page);
      await admin.goto('/collect/hub');

      const recentUrl = page.locator('.tm-collect-hub-task-list__url');
      await expect(recentUrl).toHaveText(longSourceUrl);
      await expect(page.getByText(longProfileDomain, { exact: true })).toBeVisible();
      const sourceCard = page.locator('.tm-section-card').filter({
        has: page.getByText('采集来源', { exact: true }),
      });
      const relatedEntriesCard = page.locator('.tm-section-card').filter({
        has: page.locator('.tm-collect-hub-links'),
      });
      await expect(sourceCard.locator(':scope > .ant-pro-card-body')).toHaveCSS(
        'padding',
        `${layoutTokens.cardPadding}px`,
      );
      await expect(relatedEntriesCard.locator(':scope > .ant-pro-card-body')).toHaveCSS(
        'padding',
        `${layoutTokens.cardPaddingCompact}px`,
      );
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});

test.describe('@smoke @collect-drawer-responsive 采集任务抽屉', () => {
  for (const viewport of viewports) {
    test(`uses the responsive project width at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(viewport);
      await routeLongCollectContent(page);
      await admin.goto(`/collect/tasks?drawer=events&id=${collectTaskId}`);

      const drawer = page.getByRole('dialog', { name: /任务事件/ });
      await expect(drawer).toBeVisible();
      const box = await drawer.boundingBox();
      const expectedWidth = viewport.width <= 768 ? viewport.width - 24 : viewport.width / 2;

      expect(box, 'task event drawer bounding box').not.toBeNull();
      expect(
        Math.abs((box?.width ?? 0) - expectedWidth),
        `drawer width at ${viewport.width}x${viewport.height}`,
      ).toBeLessThanOrEqual(1);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});
