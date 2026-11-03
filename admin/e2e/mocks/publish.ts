import { ok } from './envelope';
import type { PlatformAppSettingsResp } from '../../src/services/platformOpen';
import { E2E_PRODUCT_ID, E2E_PUBLICATION_OLD, E2E_PUBLICATION_NEW, E2E_SHOP_ID, publication } from './product.fixture';

export const E2E_SHOPEE_SHOP_ID = 'e2e-shop-shopee';
export const E2E_PUBLISH_BATCH_ID = 'e2e-batch';

export const e2eShopifySettings: PlatformAppSettingsResp = {
  platform: 'shopify',
  groupKey: 'shopify',
  schema: { groupKey: 'shopify', title: 'Shopify', fields: [] },
  values: {},
};

export const e2ePlatformProviders = [
  {
    platform: 'douyin_shop',
    label: '抖店',
    status: 'available',
    capabilityStatus: { product_publish: 'available', inventory_sync: 'available' },
  },
  {
    platform: 'shopee',
    label: 'Shopee',
    status: 'available',
    capabilityStatus: { product_publish: 'available', inventory_sync: 'available' },
  },
  {
    platform: 'shopify',
    name: 'Shopify',
    label: 'Shopify',
    status: 'planned',
    authType: 'oauth',
    capabilities: [],
    authSchema: [],
    appConfigSchema: e2eShopifySettings.schema,
    settingsGroupKey: e2eShopifySettings.groupKey,
  },
];

export const e2eShops = [
  {
    id: E2E_SHOP_ID,
    platform: 'douyin_shop',
    shopName: 'E2E 抖店测试店铺',
    status: 'active',
    authStatus: 'authorized',
    capabilities: { product_publish: 'available', inventory_sync: 'available' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: E2E_SHOPEE_SHOP_ID,
    platform: 'shopee',
    shopName: 'E2E Shopee 测试店铺',
    status: 'active',
    authStatus: 'authorized',
    capabilities: { product_publish: 'available', inventory_sync: 'available' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const e2ePublishTargets = {
  productId: E2E_PRODUCT_ID,
  platforms: [
    {
      platform: 'douyin_shop',
      platformLabel: '抖店',
      capability: 'real_draft_create',
      capabilityLabel: '可创建平台草稿',
      settingsPath: '/settings/platforms',
      shops: [
        { shopId: E2E_SHOP_ID, shopName: 'E2E 抖店测试店铺', authStatus: 'authorized', authStatusLabel: '已授权', enabled: true },
      ],
    },
    {
      platform: 'shopee',
      platformLabel: 'Shopee',
      capability: 'local_draft_only',
      capabilityLabel: '本地刊登草稿',
      settingsPath: '/settings/platforms',
      shops: [
        { shopId: E2E_SHOPEE_SHOP_ID, shopName: 'E2E Shopee 测试店铺', authStatus: 'authorized', authStatusLabel: '已授权', enabled: true },
      ],
    },
  ],
};

export const e2eDouyinConfig = {
  platform: 'douyin_shop',
  shopId: E2E_SHOP_ID,
  categoryId: 'e2e-douyin-category',
  categoryPath: ['服饰内衣', '女装', 'T恤'],
  attrs: { brand: 'E2E' },
  updatedAt: '2026-01-01T00:00:00Z',
};

export const e2eDouyinMapping = {
  productId: E2E_PRODUCT_ID,
  platform: 'douyin_shop',
  title: 'E2E 商品草稿长标题用于回归测试响应式和换行能力',
  categoryId: 'e2e-douyin-category',
  attrs: { brand: 'E2E' },
  mainImages: [{ imageKey: 'main-1', url: 'https://example.test/e2e-main.jpg', uploaded: true, uploadStatus: 'uploaded', platformImageId: 'e2e-platform-image' }],
  detailImages: [{ imageKey: 'detail-1', url: 'https://example.test/e2e-detail.jpg', uploaded: true, uploadStatus: 'uploaded', platformImageId: 'e2e-platform-detail' }],
  skus: [{ skuId: 'e2e-sku-1', skuCode: 'E2E-SKU-1', platformSkuId: 'e2e-platform-sku-1' }],
  errors: [],
  warnings: [],
};

export function publishResponse(path: string) {
  if (path === '/api/v1/platform/providers') return ok({ list: e2ePlatformProviders });
  if (path === '/api/v1/platform/settings/shopify') return ok(e2eShopifySettings);
  if (path === '/api/v1/shops') return ok({ list: e2eShops, pagination: { page: 1, pageSize: 500, total: e2eShops.length, totalPages: 1 } });
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publish-targets`) return ok(e2ePublishTargets);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publications`) return ok({ list: [publication(E2E_PUBLICATION_OLD)] });
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop`) return ok(e2eDouyinConfig);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop/mapping`) return ok(e2eDouyinMapping);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/platform-configs/douyin_shop/publish-tasks`) return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === `/api/v1/product-publish/batches/${E2E_PUBLISH_BATCH_ID}`) {
    return ok({
      id: E2E_PUBLISH_BATCH_ID,
      batchType: 'multi_product',
      name: 'E2E 刊登批次',
      status: 'partial_success',
      statusLabel: '部分成功',
      productCount: 1,
      targetCount: 2,
      taskCount: 2,
      successCount: 1,
      failedCount: 1,
      skippedCount: 0,
      createdAt: '2026-01-03T00:00:00Z',
      finishedAt: '2026-01-03T00:05:00Z',
      items: [
        {
          productId: E2E_PRODUCT_ID,
          productTitle: 'E2E 商品草稿长标题用于回归测试响应式和换行能力',
          targetKey: `douyin_shop:${E2E_SHOP_ID}`,
          platform: 'douyin_shop',
          platformLabel: '抖店',
          shopId: E2E_SHOP_ID,
          shopName: 'E2E 抖店测试店铺',
          taskId: 'e2e-publish-task-1',
          status: 'success',
          statusLabel: '成功',
          capability: 'real_draft_create',
          localDraftOnly: true,
        },
        {
          productId: E2E_PRODUCT_ID,
          productTitle: 'E2E 商品草稿长标题用于回归测试响应式和换行能力',
          targetKey: `shopee:${E2E_SHOPEE_SHOP_ID}`,
          platform: 'shopee',
          platformLabel: 'Shopee',
          shopId: E2E_SHOPEE_SHOP_ID,
          shopName: 'E2E Shopee 测试店铺',
          taskId: 'e2e-publish-task-2',
          status: 'failed',
          statusLabel: '失败',
          capability: 'local_draft_only',
          errorCode: 'e2e_failed',
          errorMessage: 'E2E synthetic publish failure',
        },
      ],
    });
  }
  return null;
}

export function latestPublicationsResponse() {
  return ok({ list: [publication(E2E_PUBLICATION_NEW, 'e2e-platform-product-new')] });
}

export function skuBindingsResponse(publicationId = E2E_PUBLICATION_OLD) {
  return ok({
    publicationId,
    externalProductId: publicationId === E2E_PUBLICATION_NEW ? 'e2e-platform-product-new' : 'e2e-platform-product-old',
    total: 1,
    bound: 1,
    skipped: 0,
    unmatched: 0,
    ambiguous: 0,
    failed: 0,
    rows: [
      {
        publicationSkuId: `${publicationId}-sku`,
        productSkuId: 'e2e-sku-1',
        skuCode: 'E2E-SKU-1',
        specName: '蓝色 / M',
        status: 'bound',
        platformSkuId: 'e2e-platform-sku-1',
        price: 129.9,
        stock: 88,
      },
    ],
    platformSkus: [],
    inventorySyncReady: true,
  });
}
