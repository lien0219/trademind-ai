import { test, expect } from '../fixtures/admin.fixture';
import { e2eUser } from '../mocks/auth';
import { ok } from '../mocks/envelope';
import {
  E2E_PRODUCT_SKU_ID,
  E2E_PURCHASE_ORDER_ID,
  E2E_PURCHASE_ORDER_ITEM_ID,
  E2E_SUPPLIER_ID,
  E2E_SUPPLIER_SKU_ID,
  E2E_WAREHOUSE_ID,
  e2eProductSkuHit,
  e2ePurchaseOrder,
  e2eSupplier,
} from '../mocks/procurement';
import {
  expectHeaderContentAligned,
  expectModalWithinViewport,
  expectNoRootOverflow,
} from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@smoke procurement workspace', () => {
  for (const viewport of viewports) {
    test(`renders procurement workspace at ${viewport.width}x${viewport.height} without writes`, async ({ admin, page }) => {
      await page.setViewportSize(viewport);
      for (const route of [
        { path: '/procurement/warehouses', text: 'E2E 华东主仓' },
        { path: '/procurement/suppliers', text: 'E2E 核心供应商' },
        { path: '/procurement/purchase-orders', text: 'PO-E2E-0001' },
      ]) {
        await admin.goto(route.path);
        await expect(page.getByText(route.text).first()).toBeVisible({ timeout: 30_000 });
        await expectNoRootOverflow(page);
        await expectHeaderContentAligned(page);
      }
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }

  test('creates a warehouse only after explicit confirmation', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-warehouse',
      method: 'POST',
      path: /^\/api\/v1\/warehouses$/,
      response: ok({ id: 'e2e-warehouse-west', code: 'WEST', name: 'E2E 西部仓', status: 'active', isDefault: false }),
    });

    await admin.goto('/procurement/warehouses');
    await page.getByRole('button', { name: '新建仓库' }).click();
    let dialog = page.getByRole('dialog', { name: '新建仓库' });
    await expectModalWithinViewport(page);
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('create-warehouse', 0);

    await page.getByRole('button', { name: '新建仓库' }).click();
    dialog = page.getByRole('dialog', { name: '新建仓库' });
    await dialog.getByLabel('仓库编码').fill('west');
    await dialog.getByLabel('仓库名称').fill('E2E 西部仓');
    await dialog.getByRole('button', { name: '创建仓库' }).click();

    await admin.writeGuard.expectRequestCount('create-warehouse', 1);
    expect(admin.writeGuard.calls('create-warehouse')[0]?.postDataJSON).toEqual({
      code: 'WEST',
      name: 'E2E 西部仓',
      isDefault: false,
    });
  });

  test('creates a purchase-order draft with an idempotency key and exact minor-unit amount', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-purchase-order',
      method: 'POST',
      path: /^\/api\/v1\/purchase-orders$/,
      response: ok({ ...e2ePurchaseOrder, id: 'e2e-purchase-order-new', status: 'draft', revision: 1 }),
    });
    await page.route('**/api/v1/purchase-orders/e2e-purchase-order-new', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2ePurchaseOrder, id: 'e2e-purchase-order-new', status: 'draft', revision: 1 })),
      });
    });
    await page.route('**/api/v1/product-skus/search**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [e2eProductSkuHit] })),
      });
    });

    await admin.goto('/procurement/purchase-orders');
    await page.getByRole('button', { name: '新建采购单' }).click();
    const dialog = page.getByRole('dialog', { name: '新建采购单' });
    await expectModalWithinViewport(page);
    await dialog.getByRole('combobox', { name: '商品规格 1' }).press('ArrowDown');
    await page.locator('.ant-select-dropdown:visible').getByText('E2E 蓝牙耳机 · 蓝色', { exact: true }).click();
    await dialog.getByLabel('采购单价').fill('25.99');
    await dialog.getByRole('button', { name: '创建采购单草稿' }).click();

    await admin.writeGuard.expectRequestCount('create-purchase-order', 1);
    const payload = admin.writeGuard.calls('create-purchase-order')[0]?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      supplierId: E2E_SUPPLIER_ID,
      warehouseId: E2E_WAREHOUSE_ID,
      currency: 'CNY',
      items: [{
        productSkuId: E2E_PRODUCT_SKU_ID,
        supplierSkuId: E2E_SUPPLIER_SKU_ID,
        quantity: 2,
        unitCostMinor: 2599,
      }],
    });
    expect(String(payload.idempotencyKey)).toMatch(/^admin-purchase-order-/);
  });

  test('preserves masked supplier contact fields when an operator updates other data', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({
          ...e2eUser,
          role: 'operator',
          permissions: ['supplier.view', 'supplier.manage'],
        })),
      });
    });
    admin.writeGuard.allow({
      operation: 'update-supplier',
      method: 'PUT',
      path: new RegExp(`^/api/v1/suppliers/${E2E_SUPPLIER_ID}$`),
      response: ok({ ...e2eSupplier, name: 'E2E 核心供应商（更新）' }),
    });

    await admin.goto('/procurement/suppliers');
    await page.getByRole('button', { name: '编辑' }).click();
    const dialog = page.getByRole('dialog', { name: /编辑供应商/ });
    await expect(dialog.getByLabel('联系电话')).toHaveValue('');
    await expect(dialog.getByLabel('联系邮箱')).toHaveValue('');
    await expect(dialog.getByText(/留空将保留原值/)).toHaveCount(2);
    await dialog.getByLabel('供应商名称').fill('E2E 核心供应商（更新）');
    await dialog.getByRole('button', { name: '保存供应商' }).click();

    await admin.writeGuard.expectRequestCount('update-supplier', 1);
    expect(admin.writeGuard.calls('update-supplier')[0]?.postDataJSON).toEqual({
      name: 'E2E 核心供应商（更新）',
      status: 'active',
      contactName: '测试联系人',
    });
  });

  test('cancels receipt without a write and submits one revision-checked receipt', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'receive-purchase-order',
      method: 'POST',
      path: new RegExp(`^/api/v1/purchase-orders/${E2E_PURCHASE_ORDER_ID}/receipts$`),
      response: ok({
        purchaseOrder: {
          ...e2ePurchaseOrder,
          status: 'partially_received',
          revision: 4,
          items: [{ ...e2ePurchaseOrder.items[0], receivedQuantity: 2 }],
        },
        receipt: { id: 'e2e-receipt-1', receiptNo: 'GR-E2E-0001', purchaseOrderId: E2E_PURCHASE_ORDER_ID },
      }),
    });

    await admin.goto(`/procurement/purchase-orders/${E2E_PURCHASE_ORDER_ID}`);
    await page.getByRole('button', { name: '确认收货' }).click();
    let dialog = page.getByRole('dialog', { name: '确认收货' });
    await dialog.getByRole('button', { name: /取\s*消/ }).click();
    await admin.writeGuard.expectRequestCount('receive-purchase-order', 0);

    await page.getByRole('button', { name: '确认收货' }).click();
    dialog = page.getByRole('dialog', { name: '确认收货' });
    await dialog.getByLabel('本次收货').fill('2');
    await dialog.getByRole('button', { name: '确认本次收货' }).click();

    await admin.writeGuard.expectRequestCount('receive-purchase-order', 1);
    const payload = admin.writeGuard.calls('receive-purchase-order')[0]?.postDataJSON as Record<string, unknown>;
    expect(payload).toMatchObject({
      expectedRevision: 3,
      items: [{ purchaseOrderItemId: E2E_PURCHASE_ORDER_ITEM_ID, quantity: 2 }],
    });
    expect(String(payload.idempotencyKey)).toMatch(/^admin-goods-receipt-/);
  });

  test('distinguishes empty, error, and readonly states', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: ['warehouse.view', 'supplier.view', 'procurement.view'] })) });
    });
    await page.route('**/api/v1/warehouses', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ list: [] })) });
    });
    await page.route('**/api/v1/suppliers', async (route) => {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 50000, message: 'supplier unavailable', data: null }) });
    });
    admin.consoleGuard.allowError(/Failed to load resource: the server responded with a status of 503/);

    await admin.goto('/procurement/warehouses');
    await expect(page.getByText('暂无仓库，请先新建仓库。')).toBeVisible();
    await expect(page.getByRole('button', { name: '新建仓库' })).toBeDisabled();
    await expect(page.getByText(/只读模式/)).toBeVisible();

    await admin.goto('/procurement/suppliers');
    await expect(page.getByText('供应商列表加载失败，请稍后重试。')).toBeVisible();
    await expect(page.getByRole('button', { name: '新建供应商' })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
