import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { fail, ok } from '../mocks/envelope';
import {
  E2E_OPERATION_TASK_ID,
  e2eOperationAttempt,
  e2eOperationDraft,
  e2eOperationTask,
  e2eOperationTaskDetail,
  e2eProductionStatus,
} from '../mocks/operation-tasks';
import { e2eProduct, E2E_PRODUCT_ID, E2E_SHOP_ID } from '../mocks/product.fixture';
import {
  expectHeaderContentAligned,
  expectModalWithinViewport,
  expectNoRootOverflow,
} from '../utils/assertions';

const listPath = '/ops/task-center/operation-tasks';
const detailPath = `${listPath}/${E2E_OPERATION_TASK_ID}`;

function idempotencyKey(headers: Record<string, string>) {
  return headers['idempotency-key'];
}

async function expectOperationTaskListLayout(page: Page) {
  const title = page.locator('.operation-tasks-page-shell .ant-page-header-heading-title');
  const subtitle = page.locator('.operation-tasks-page-shell .ant-page-header-heading-sub-title');
  const headerActions = page.locator('.operation-tasks-page__header-actions');
  const tableScroller = page.locator('.operation-tasks-page__table .ant-table-content');

  await expect(title).toHaveText('运营任务中心');
  await expect(subtitle).toContainText('查看运营任务、草稿版本、人工审核');
  await expect(page.getByText('任务列表', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '应用筛选' })).toBeVisible();
  await expect(page.getByRole('button', { name: '清除筛选' })).toBeVisible();

  const metrics = await page.evaluate(() => {
    const titleElement = document.querySelector<HTMLElement>('.operation-tasks-page-shell .ant-page-header-heading-title');
    const subtitleElement = document.querySelector<HTMLElement>('.operation-tasks-page-shell .ant-page-header-heading-sub-title');
    const actionsElement = document.querySelector<HTMLElement>('.operation-tasks-page__header-actions');
    const tableElement = document.querySelector<HTMLElement>('.operation-tasks-page__table');
    const tableScrollerElement = document.querySelector<HTMLElement>('.operation-tasks-page__table .ant-table-content');
    if (!titleElement || !subtitleElement || !actionsElement || !tableElement || !tableScrollerElement) return null;
    const actions = actionsElement.getBoundingClientRect();
    const table = tableElement.getBoundingClientRect();
    return {
      titleClientWidth: titleElement.clientWidth,
      titleScrollWidth: titleElement.scrollWidth,
      subtitleClientWidth: subtitleElement.clientWidth,
      subtitleScrollWidth: subtitleElement.scrollWidth,
      actionsLeft: actions.left,
      actionsRight: actions.right,
      tableLeft: table.left,
      tableRight: table.right,
      tableClientWidth: tableScrollerElement.clientWidth,
      tableScrollWidth: tableScrollerElement.scrollWidth,
      viewportWidth: window.innerWidth,
    };
  });

  expect(metrics, 'operation task list layout metrics').not.toBeNull();
  if (!metrics) return;
  expect(metrics.titleScrollWidth, `page title clipping ${JSON.stringify(metrics)}`).toBeLessThanOrEqual(metrics.titleClientWidth + 1);
  expect(metrics.subtitleScrollWidth, `page subtitle clipping ${JSON.stringify(metrics)}`).toBeLessThanOrEqual(metrics.subtitleClientWidth + 1);
  expect(metrics.actionsLeft, `header actions left ${JSON.stringify(metrics)}`).toBeGreaterThanOrEqual(0);
  expect(metrics.actionsRight, `header actions right ${JSON.stringify(metrics)}`).toBeLessThanOrEqual(metrics.viewportWidth + 1);
  expect(metrics.tableLeft, `table left ${JSON.stringify(metrics)}`).toBeGreaterThanOrEqual(0);
  expect(metrics.tableRight, `table right ${JSON.stringify(metrics)}`).toBeLessThanOrEqual(metrics.viewportWidth + 1);
  expect(metrics.tableScrollWidth, `table internal scroll ${JSON.stringify(metrics)}`).toBeGreaterThan(metrics.tableClientWidth);
  await expect(tableScroller).toBeVisible();
}

test.describe('@smoke @operation-task operation task center', () => {
  test('restores list filters from URL and returns from detail with the same context', async ({ admin, page }) => {
    const queries: Record<string, string>[] = [];
    await page.route('**/api/v1/operation-tasks?**', async (route) => {
      const url = new URL(route.request().url());
      queries.push(Object.fromEntries(url.searchParams.entries()));
      await route.fallback();
    });

    await admin.goto(`${listPath}?status=execution_failed&platform=local&taskType=product_content`);
    await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();
    await expect(page.getByText('执行失败').first()).toBeVisible();
    await expect(page.getByText('本地').first()).toBeVisible();
    await expect(page.getByText('商品内容').first()).toBeVisible();
    expect(queries.at(-1)).toMatchObject({
      status: 'execution_failed',
      platform: 'local',
      taskType: 'product_content',
      limit: '20',
    });

    await page.getByRole('button', { name: '查看详情' }).click();
    expect(new URL(page.url()).pathname).toBe(detailPath);
    expect(new URL(page.url()).searchParams.get('from')).toContain('status=execution_failed');
    await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();
    await page.getByRole('button', { name: '返回列表' }).click();
    await expect(page).toHaveURL(/status=execution_failed.*platform=local.*taskType=product_content/);
  });

  test('supports detail tab deep links and falls back from an invalid tab', async ({ admin, page }) => {
    await admin.goto(`${detailPath}?tab=events`);
    await expect(page.getByRole('tab', { name: '审计时间线' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByText('测试环境中的安全失败记录')).toBeVisible();

    await admin.goto(`${detailPath}?tab=invalid`);
    await expect(page.getByRole('tab', { name: '草稿版本' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByText('任务载荷预览')).toBeVisible();
  });

  test('distinguishes empty and business error states', async ({ admin, page }) => {
    await page.route('**/api/v1/operation-tasks?**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ items: [], hasMore: false, limit: 20 })) });
    });
    await admin.goto(listPath);
    await expect(page.getByText('暂无运营任务')).toBeVisible();
    await expect(page.getByText('系统暂时无法完成操作，请稍后重试。')).toHaveCount(0);

    await page.route('**/api/v1/operation-tasks?**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fail('operation task unavailable', 50001)) });
    });
    await page.reload();
    await expect(page.getByText('operation task unavailable')).toBeVisible();
    await expect(page.getByText('排查编号：e2e-trace')).toBeVisible();
    await expect(page.getByText('暂无运营任务')).toHaveCount(0);
  });

  test('does not show records from the previous query when a new filter fails', async ({ admin, page }) => {
    await page.route('**/api/v1/operation-tasks?**', async (route) => {
      const url = new URL(route.request().url());
      if (url.searchParams.get('status') === 'rejected') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fail('筛选结果加载失败', 50001)) });
        return;
      }
      await route.fallback();
    });
    await admin.goto(listPath);
    await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();

    await page.getByLabel('任务状态').click();
    await page.locator('.ant-select-dropdown:visible').getByText('已拒绝', { exact: true }).click();
    await page.getByRole('button', { name: '应用筛选' }).click();

    await expect(page.getByText('筛选结果加载失败')).toBeVisible();
    await expect(page.getByText('E2E 商品内容复核')).toHaveCount(0);
  });

  test('keeps and labels the last successful batch when a manual refresh fails', async ({ admin, page }) => {
    let failRefresh = false;
    await page.route('**/api/v1/operation-tasks?**', async (route) => {
      if (failRefresh) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fail('刷新失败', 50001)) });
        return;
      }
      await route.fallback();
    });
    await admin.goto(listPath);
    await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();

    failRefresh = true;
    await page.getByRole('button', { name: '刷新' }).click();

    await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();
    await expect(page.getByText('数据未更新')).toBeVisible();
    await expect(page.getByText('当前显示上次成功加载的结果')).toBeVisible();
  });

  test('restores cursor history after reload so the previous batch remains available', async ({ admin, page }) => {
    const secondTask = { ...e2eOperationTask, id: 'e2e-operation-task-2', title: 'E2E 第二批运营任务' };
    await page.route('**/api/v1/operation-tasks?**', async (route) => {
      const cursor = new URL(route.request().url()).searchParams.get('cursor');
      const data = cursor === 'cursor-1'
        ? { items: [secondTask], hasMore: false, limit: 20 }
        : { items: [e2eOperationTask], nextCursor: 'cursor-1', hasMore: true, limit: 20 };
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(data)) });
    });
    await admin.goto(listPath);
    await page.getByRole('button', { name: '下一批' }).click();
    await expect(page.getByText('E2E 第二批运营任务').first()).toBeVisible();
    expect(new URL(page.url()).searchParams.get('cursorHistory')).toBe('[""]');

    await page.reload();
    await expect(page.getByText('E2E 第二批运营任务').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '上一批' })).toBeEnabled();
    await page.getByRole('button', { name: '上一批' }).click();
    await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();
  });

  test('validates creation and reuses the same idempotency key after a lost response', async ({ admin, page }) => {
    let releaseFirstResponse: (() => void) | undefined;
    let createAttempt = 0;
    admin.writeGuard.allow({
      operation: 'create-operation-task',
      method: 'POST',
      path: /^\/api\/v1\/operation-tasks$/,
      response: async () => {
        createAttempt += 1;
        if (createAttempt === 1) {
          await new Promise<void>((resolve) => {
            releaseFirstResponse = resolve;
          });
          return fail('创建响应丢失，请重试', 50001);
        }
        return ok({ ...e2eOperationTaskDetail, id: E2E_OPERATION_TASK_ID });
      },
    });
    await admin.goto(listPath);
    await page.getByRole('button', { name: '创建运营任务' }).click();
    const dialog = page.getByRole('dialog', { name: '创建运营任务' });
    await dialog.getByText('通用本地任务', { exact: true }).click();
    await dialog.getByRole('button', { name: '创建任务' }).click();
    await expect(dialog.getByText('请填写任务标题')).toBeVisible();
    await admin.writeGuard.expectRequestCount('create-operation-task', 0);

    await dialog.getByLabel('任务标题').fill('E2E 创建运营任务');
    await dialog.getByRole('combobox').nth(1).click();
    await page.locator('.ant-select-dropdown:visible').getByText('商品内容', { exact: true }).click();
    await dialog.getByRole('button', { name: '创建任务' }).dblclick();
    await admin.writeGuard.expectRequestCount('create-operation-task', 1);
    releaseFirstResponse?.();
    await expect(
      page.locator('#root .ant-alert-message').filter({ hasText: '创建响应丢失，请重试' }),
    ).toBeVisible();
    await expect(dialog).toBeVisible();

    await dialog.getByRole('button', { name: '创建任务' }).click();
    await admin.writeGuard.expectRequestCount('create-operation-task', 2);
    const calls = admin.writeGuard.calls('create-operation-task');
    expect(idempotencyKey(calls[0].headers)).toBeTruthy();
    expect(idempotencyKey(calls[1].headers)).toBe(idempotencyKey(calls[0].headers));
    expect(calls[1].postDataJSON).toMatchObject({
      sourceType: 'manual',
      taskType: 'product_content',
      platform: 'local',
      title: 'E2E 创建运营任务',
      payload: {},
      priority: 'normal',
    });
    await expect(page).toHaveURL(detailPath);
  });

  test('prefills a production deep link without submitting and clears it on close', async ({ admin, page }) => {
    const readyStatus = {
      ...e2eProductionStatus,
      currentAllowedLevel: 'L3',
      allowlist: { tenantId: 1, shopId: E2E_SHOP_ID, enabled: true, revision: 1 },
    };
    await page.route('**/api/v1/p10/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(readyStatus)) });
    });

    await admin.goto(`${listPath}?create=production&productId=${E2E_PRODUCT_ID}&shopId=${E2E_SHOP_ID}`);
    const dialog = page.getByRole('dialog', { name: '创建运营任务' });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(e2eProduct.title);
    await expect(dialog).toContainText('E2E 抖店测试店铺');
    expect(admin.writeGuard.allCalls()).toHaveLength(0);

    await dialog.getByRole('button', { name: /^取\s*消$/ }).click();
    await expect(dialog).toHaveCount(0);
    const url = new URL(page.url());
    expect(url.searchParams.has('create')).toBe(false);
    expect(url.searchParams.has('productId')).toBe(false);
    expect(url.searchParams.has('shopId')).toBe(false);
    expect(admin.writeGuard.allCalls()).toHaveLength(0);
  });

  test('creates a production draft task with a fixed reviewed payload', async ({ admin, page }) => {
    const readyStatus = {
      ...e2eProductionStatus,
      currentAllowedLevel: 'L3',
      realProviderEnabled: true,
      realPlatformNetworkEnabled: true,
      realCredentialsEnabled: true,
      realProductDraftWriteEnabled: true,
      backgroundWorkerEnabled: true,
      productPublishQueueEnabled: true,
      providerWriteReady: true,
      control: {
        ...e2eProductionStatus.control,
        providerKillActive: false,
        tenantKillActive: false,
        shopKillActive: false,
        writeKillActive: false,
      },
      allowlist: { tenantId: 1, shopId: E2E_SHOP_ID, enabled: true, revision: 1 },
      gray: { tenantId: 1, shopId: E2E_SHOP_ID, maxSku: 100, status: 'active', ownerApproved: true, technicalLeadApproved: true, revision: 4 },
      productionReady: true,
      productionAcceptancePassed: true,
    };
    await page.route('**/api/v1/p10/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(readyStatus)) });
    });
    admin.writeGuard.allow({
      operation: 'create-production-operation-task',
      method: 'POST',
      path: /^\/api\/v1\/operation-tasks$/,
      response: ok({ ...e2eOperationTaskDetail, id: E2E_OPERATION_TASK_ID }),
    });
    await admin.goto(listPath);
    await page.getByRole('button', { name: '创建运营任务' }).click();
    const dialog = page.getByRole('dialog', { name: '创建运营任务' });
    await expect(dialog.getByText('平台草稿写入已受控启用')).toBeVisible();
    await dialog.getByLabel('商品').click();
    await page.locator('.ant-select-dropdown:visible').getByText(/E2E 商品草稿长标题/).click();
    await dialog.getByLabel('已授权白名单抖店').click();
    await page.locator('.ant-select-dropdown:visible').getByText('E2E 抖店测试店铺', { exact: true }).click();
    await dialog.getByRole('button', { name: '创建任务' }).click();

    await admin.writeGuard.expectRequestCount('create-production-operation-task', 1);
    expect(admin.writeGuard.calls('create-production-operation-task')[0].postDataJSON).toMatchObject({
      sourceType: 'manual',
      sourceReference: E2E_PRODUCT_ID,
      taskType: 'product_publish',
      platform: 'douyin',
      payload: {
        schemaVersion: 'douyin_draft_v1',
        productId: E2E_PRODUCT_ID,
        shopId: E2E_SHOP_ID,
        publishMode: 'save_as_platform_draft',
      },
      priority: 'normal',
    });
    expect(JSON.stringify(admin.writeGuard.calls('create-production-operation-task')[0].postDataJSON)).not.toContain('publish_online');
  });

  test('blocks production execution until runtime controls are ready', async ({ admin, page }) => {
    const productionDraft = {
      ...e2eOperationDraft,
      adapterMode: 'production_draft',
      payload: {
        schemaVersion: 'douyin_draft_v1',
        productId: E2E_PRODUCT_ID,
        shopId: E2E_SHOP_ID,
        publishMode: 'save_as_platform_draft',
        skuCount: 1,
        mappingHash: 'e2e-production-mapping-hash',
        mappingSnapshot: {
          title: 'E2E 冻结抖店标题',
          description: 'E2E 冻结描述',
          categoryPath: '服饰内衣 / 女装 / T恤',
          mainImages: [{ platformImageUrl: 'https://example.test/e2e-main.jpg' }],
          skus: [{ localSkuId: 'e2e-sku-1', name: '蓝色 / M', attrs: { 颜色: '蓝色', 尺码: 'M' }, price: 129.9, stock: 88 }],
          price: { currency: 'CNY', min: 129.9, max: 129.9 },
          stock: { total: 88 },
        },
      },
    };
    const productionDetail = {
      ...e2eOperationTaskDetail,
      taskType: 'product_publish',
      platform: 'douyin',
      status: 'approved',
      latestDraft: productionDraft,
      allowedActions: { ...e2eOperationTaskDetail.allowedActions, canExecute: true, canRetry: false },
    };
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(productionDetail)) });
    });
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/drafts?**`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ items: [productionDraft], limit: 50 })) });
    });
    await admin.goto(detailPath);

    await expect(page.getByText('审核冻结快照')).toBeVisible();
    await expect(page.getByText('E2E 冻结抖店标题')).toBeVisible();
    await expect(page.getByRole('button', { name: '创建平台草稿' })).toBeDisabled();
    await expect(page.getByRole('button', { name: '人工重试' })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('submits only the frozen platform draft mode when runtime is ready', async ({ admin, page }) => {
    const productionDraft = {
      ...e2eOperationDraft,
      adapterMode: 'production_draft',
      payload: {
        schemaVersion: 'douyin_draft_v1', productId: E2E_PRODUCT_ID, shopId: E2E_SHOP_ID,
        publishMode: 'save_as_platform_draft', skuCount: 1, mappingHash: 'hash',
        mappingSnapshot: { title: 'E2E 冻结抖店标题', skus: [{ name: '默认规格', price: 99, stock: 5 }] },
      },
    };
    const productionDetail = {
      ...e2eOperationTaskDetail,
      taskType: 'product_publish', platform: 'douyin', status: 'approved', latestDraft: productionDraft,
      allowedActions: { ...e2eOperationTaskDetail.allowedActions, canExecute: true, canRetry: false },
    };
    const readyStatus = {
      ...e2eProductionStatus,
      currentAllowedLevel: 'L3', realProviderEnabled: true, realPlatformNetworkEnabled: true,
      realCredentialsEnabled: true, realProductDraftWriteEnabled: true, backgroundWorkerEnabled: true,
      productPublishQueueEnabled: true,
      providerWriteReady: true,
      control: { ...e2eProductionStatus.control, providerKillActive: false, tenantKillActive: false, shopKillActive: false, writeKillActive: false },
      allowlist: { tenantId: 1, shopId: E2E_SHOP_ID, enabled: true, revision: 1 },
      gray: { tenantId: 1, shopId: E2E_SHOP_ID, maxSku: 100, status: 'active', ownerApproved: true, technicalLeadApproved: true, revision: 4 },
      productionReady: true,
      productionAcceptancePassed: true,
    };
    await page.route('**/api/v1/p10/status', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(readyStatus)) });
    });
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(productionDetail)) });
    });
    admin.writeGuard.allow({
      operation: 'execute-production-operation-task',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/execute$`),
      response: ok({ status: 'in_progress', attempt: { ...e2eOperationAttempt, adapterMode: 'production_draft', status: 'queued' } }),
    });
    await admin.goto(detailPath);
    await page.getByRole('button', { name: '创建平台草稿' }).click();
    const dialog = page.getByRole('dialog', { name: '创建平台草稿' });
    await expect(dialog.getByText('仅保存为平台草稿')).toBeVisible();
    await dialog.getByRole('button', { name: '确认创建平台草稿' }).click();

    await admin.writeGuard.expectRequestCount('execute-production-operation-task', 1);
    expect(admin.writeGuard.calls('execute-production-operation-task')[0].postDataJSON).toEqual({
      expectedTaskRevision: 7,
      adapterMode: 'production_draft',
    });
  });

  test('renders readonly allowed actions without issuing writes', async ({ admin, page }) => {
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          ...e2eOperationTaskDetail,
          allowedActions: {
            canEditDraft: false,
            canApprove: false,
            canReject: false,
            canExecute: false,
            canRetry: false,
            canCancel: false,
          },
        })),
      });
    });
    await admin.goto(detailPath);

    const actions = page.locator('.operation-task-detail__actions').getByRole('button');
    await expect(actions).toHaveCount(6);
    for (const action of await actions.all()) {
      await expect(action).toBeDisabled();
    }
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('cancel is zero writes and a double confirmation remains one request', async ({ admin, page }) => {
    let releaseCancelResponse: (() => void) | undefined;
    admin.writeGuard.allow({
      operation: 'cancel-operation-task',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/cancel$`),
      response: async () => {
        await new Promise<void>((resolve) => {
          releaseCancelResponse = resolve;
        });
        return ok({ ...e2eOperationTaskDetail, status: 'cancelled', revision: 8 });
      },
    });
    await admin.goto(detailPath);

    await page.locator('.operation-task-detail__actions').getByRole('button', { name: '取消任务', exact: true }).click();
    let dialog = page.getByRole('dialog', { name: '取消运营任务' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: /^取\s*消$/ }).click();
    await admin.writeGuard.expectRequestCount('cancel-operation-task', 0);

    await page.locator('.operation-task-detail__actions').getByRole('button', { name: '取消任务', exact: true }).click();
    dialog = page.getByRole('dialog', { name: '取消运营任务' });
    await dialog.getByLabel('取消原因').fill('E2E 人工取消原因');
    await dialog.getByRole('button', { name: '取消任务', exact: true }).dblclick();
    await admin.writeGuard.expectRequestCount('cancel-operation-task', 1);
    expect(admin.writeGuard.calls('cancel-operation-task')[0]).toMatchObject({
      method: 'POST',
      path: `/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/cancel`,
      postDataJSON: { reason: 'E2E 人工取消原因', expectedTaskRevision: 7 },
    });
    releaseCancelResponse?.();
    await expect(page.getByRole('dialog', { name: '取消运营任务' })).toHaveCount(0);
  });

  test('creates an initial draft only after confirmation with the expected revision', async ({ admin, page }) => {
    const suggestedDetail = {
      ...e2eOperationTaskDetail,
      status: 'suggested',
      revision: 1,
      latestDraftVersion: undefined,
      latestDraft: undefined,
      latestAttempt: undefined,
      allowedActions: {
        canEditDraft: true,
        canApprove: false,
        canReject: false,
        canExecute: false,
        canRetry: false,
        canCancel: true,
      },
    };
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(suggestedDetail)) });
    });
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/drafts?**`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ items: [], limit: 50 })) });
    });
    admin.writeGuard.allow({
      operation: 'create-operation-draft',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/drafts$`),
      response: ok(e2eOperationDraft),
    });
    await admin.goto(detailPath);

    await page.getByRole('button', { name: '创建首版草稿' }).click();
    await page.getByRole('dialog', { name: '创建首版草稿' }).getByRole('button', { name: /^取\s*消$/ }).click();
    await admin.writeGuard.expectRequestCount('create-operation-draft', 0);
    await page.getByRole('button', { name: '创建首版草稿' }).click();
    await page.getByLabel('变更原因').fill('E2E 创建首版草稿');
    await page.getByRole('button', { name: '创建草稿' }).click();

    await admin.writeGuard.expectRequestCount('create-operation-draft', 1);
    const call = admin.writeGuard.calls('create-operation-draft')[0];
    expect(idempotencyKey(call.headers)).toBeTruthy();
    expect(call.postDataJSON).toMatchObject({
      payload: e2eOperationTaskDetail.payload,
      changeReason: 'E2E 创建首版草稿',
      expectedTaskRevision: 1,
    });
    await expect(page.getByRole('dialog', { name: '创建首版草稿' })).toHaveCount(0);
  });

  test('submits review and execution writes once with bound draft data', async ({ admin, page }) => {
    const actionableDetail = {
      ...e2eOperationTaskDetail,
      status: 'pending_review',
      allowedActions: {
        canEditDraft: false,
        canApprove: true,
        canReject: true,
        canExecute: true,
        canRetry: false,
        canCancel: true,
      },
    };
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(actionableDetail)) });
    });
    admin.writeGuard.allow({
      operation: 'approve-operation-task',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/approve$`),
      response: ok(actionableDetail),
    });
    admin.writeGuard.allow({
      operation: 'reject-operation-task',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/reject$`),
      response: ok(actionableDetail),
    });
    admin.writeGuard.allow({
      operation: 'execute-operation-task',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/execute$`),
      response: ok({ status: 'succeeded', attempt: e2eOperationAttempt, resultType: 'local_draft' }),
    });
    await admin.goto(detailPath);

    await page.getByRole('button', { name: '确认批准' }).click();
    await page.getByRole('dialog', { name: '确认批准' }).getByRole('button', { name: /^取\s*消$/ }).click();
    await admin.writeGuard.expectRequestCount('approve-operation-task', 0);
    await page.getByRole('button', { name: '确认批准' }).click();
    await page.getByLabel('批准说明').fill('E2E 批准说明');
    await page.getByRole('button', { name: '批准草稿' }).click();
    await admin.writeGuard.expectRequestCount('approve-operation-task', 1);

    await page.getByRole('button', { name: /^拒\s*绝$/ }).click();
    await page.getByLabel('拒绝原因').fill('E2E 拒绝原因');
    await page.getByRole('button', { name: '拒绝草稿' }).click();
    await admin.writeGuard.expectRequestCount('reject-operation-task', 1);

    await page.getByRole('button', { name: '执行草稿生成' }).click();
    await page.getByRole('button', { name: '提交草稿生成' }).click();
    await admin.writeGuard.expectRequestCount('execute-operation-task', 1);

    const approveCall = admin.writeGuard.calls('approve-operation-task')[0];
    const rejectCall = admin.writeGuard.calls('reject-operation-task')[0];
    expect(approveCall.postDataJSON).toMatchObject({ draftVersion: 2, draftPayloadHash: e2eOperationDraft.payloadHash, expectedTaskRevision: 7 });
    expect(rejectCall.postDataJSON).toMatchObject({ draftVersion: 2, draftPayloadHash: e2eOperationDraft.payloadHash, expectedTaskRevision: 7 });
    expect(admin.writeGuard.calls('execute-operation-task')[0].postDataJSON).toEqual({ expectedTaskRevision: 7, adapterMode: 'local_draft_only' });
    for (const call of [approveCall, rejectCall, admin.writeGuard.calls('execute-operation-task')[0]]) {
      expect(idempotencyKey(call.headers)).toBeTruthy();
    }
  });

  test('refreshes persisted history after retry business failure and reuses its idempotency key', async ({ admin, page }) => {
    let detailReads = 0;
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}`, async (route) => {
      detailReads += 1;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(e2eOperationTaskDetail)) });
    });
    admin.writeGuard.allow({
      operation: 'retry-operation-task',
      method: 'POST',
      path: new RegExp(`/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/retry$`),
      response: ok({
        status: 'failed_retryable',
        attempt: e2eOperationAttempt,
        failure: { category: 'adapter', code: 'E2E_RETRY_FAILED', safeMessage: 'E2E 人工重试失败', retryable: true },
      }),
    });
    await admin.goto(detailPath);

    await page.getByRole('button', { name: '人工重试' }).click();
    await page.getByLabel('重试原因').fill('E2E 人工重试原因');
    await page.getByRole('button', { name: '发起人工重试' }).click();
    await admin.writeGuard.expectRequestCount('retry-operation-task', 1);
    await expect.poll(() => detailReads).toBeGreaterThan(1);
    await expect(page.getByRole('alert').getByText('E2E 人工重试失败').first()).toBeVisible();
    await expect(page.getByRole('dialog', { name: '人工重试' })).toBeVisible();

    await page.getByRole('button', { name: '发起人工重试' }).click();
    await admin.writeGuard.expectRequestCount('retry-operation-task', 2);
    const calls = admin.writeGuard.calls('retry-operation-task');
    expect(idempotencyKey(calls[1].headers)).toBe(idempotencyKey(calls[0].headers));
    expect(calls[0].postDataJSON).toEqual({
      failedAttemptId: e2eOperationAttempt.attemptId,
      reason: 'E2E 人工重试原因',
      expectedTaskRevision: 7,
    });
  });

  test('keeps previously loaded execution history when its refresh fails', async ({ admin, page }) => {
    let failAttemptRefresh = false;
    await page.route(`**/api/v1/operation-tasks/${E2E_OPERATION_TASK_ID}/attempts?**`, async (route) => {
      if (failAttemptRefresh) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(fail('执行历史刷新失败', 50001)) });
        return;
      }
      await route.fallback();
    });
    await admin.goto(`${detailPath}?tab=attempts`);
    await expect(page.getByRole('cell', { name: /e2e-operation-/ }).first()).toBeVisible();

    failAttemptRefresh = true;
    await page.getByRole('button', { name: '刷新' }).click();
    await expect(page.getByRole('cell', { name: /e2e-operation-/ }).first()).toBeVisible();
    await expect(page.getByText('当前显示上次成功加载的记录')).toBeVisible();
  });
});

test.describe('@operation-task operation task responsive coverage', () => {
  for (const viewport of [
    { width: 1440, height: 900 },
    { width: 1280, height: 800 },
    { width: 1024, height: 768 },
    { width: 768, height: 900 },
    { width: 375, height: 812 },
  ]) {
    test(`${viewport.width}x${viewport.height} list and detail stay within the viewport`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      await admin.goto(listPath);
      await expect(page.getByText('E2E 商品内容复核').first()).toBeVisible();
      await expectOperationTaskListLayout(page);
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);

      await admin.goto(detailPath);
      await expect(page.getByText('可用操作')).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);

      await page.locator('.operation-task-detail__actions').getByRole('button', { name: '取消任务', exact: true }).click();
      await expectModalWithinViewport(page);
      await page.getByRole('dialog', { name: '取消运营任务' }).getByRole('button', { name: /^取\s*消$/ }).click();
    });
  }
});
