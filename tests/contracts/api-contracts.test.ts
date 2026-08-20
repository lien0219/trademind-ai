import { describe, expect, it } from 'vitest';
import contracts from './api-contracts.json';

const routeKey = (endpoint: { method: string; path: string }) => `${endpoint.method} ${endpoint.path}`;

describe('TradeMind API contract registry', () => {
  it('keeps the backend envelope explicit for frontend and E2E mocks', () => {
    expect(contracts.envelope.success).toEqual(['code', 'message', 'data']);
    expect(contracts.envelope.optional).toContain('traceId');
    expect(contracts.envelope.errorCodeRule).toContain('non-zero');
  });

  it('covers core Admin production endpoints', () => {
    const routes = new Set(contracts.endpoints.map(routeKey));

    expect(routes).toEqual(
      new Set([
        'GET /api/v1/auth/profile',
        'GET /api/v1/image/providers',
        'GET /api/v1/warehouses',
        'POST /api/v1/warehouses',
        'PUT /api/v1/warehouses/:id',
        'POST /api/v1/products/:id/skus',
        'PUT /api/v1/products/:id/skus/:skuId',
        'PUT /api/v1/products/:id/skus/:skuId/stock-settings',
        'DELETE /api/v1/products/:id/skus/:skuId',
        'GET /api/v1/products/:id/skus/:skuId/warehouse-balances',
        'POST /api/v1/products/:id/skus/:skuId/adjust-stock',
        'GET /api/v1/inventory/warehouse-ledger/reconciliation',
        'POST /api/v1/inventory/warehouse-ledger/migrate-legacy',
        'GET /api/v1/suppliers',
        'POST /api/v1/suppliers',
        'PUT /api/v1/suppliers/:id',
        'GET /api/v1/suppliers/:id/skus',
        'POST /api/v1/suppliers/:id/skus',
        'GET /api/v1/purchase-orders',
        'POST /api/v1/purchase-orders',
        'GET /api/v1/purchase-orders/:id',
        'POST /api/v1/purchase-orders/:id/submit',
        'POST /api/v1/purchase-orders/:id/approve',
        'POST /api/v1/purchase-orders/:id/cancel',
        'POST /api/v1/purchase-orders/:id/close',
        'POST /api/v1/purchase-orders/:id/receipts',
        'GET /api/v1/p10/status',
        'POST /api/v1/operation-tasks',
        'GET /api/v1/operation-tasks/:id',
        'POST /api/v1/operation-tasks/:id/approve',
        'POST /api/v1/operation-tasks/:id/execute',
        'GET /api/v1/observability/overview',
        'GET /api/v1/observability/alerts',
        'POST /api/v1/observability/alerts/:id/ack',
        'POST /api/v1/observability/alerts/:id/silence',
        'GET /api/v1/products/:id',
        'GET /api/v1/products/:id/readiness',
        'GET /api/v1/products/:id/publications',
        'GET /api/v1/product-publications/:id/douyin/sku-bindings',
        'GET /api/v1/products/:id/publish-targets',
        'POST /api/v1/products/:id/publish-targets/create-drafts',
        'POST /api/v1/product-publish/batch-targets/create-drafts',
        'POST /api/v1/product-publish/tasks/:id/retry',
        'POST /api/v1/product-publish/tasks/:id/recover-douyin-draft',
        'POST /api/v1/product-publish/batches/:id/retry-failed',
        'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft',
        'POST /api/v1/products/:id/publish',
        'GET /api/v1/customer/dashboard',
        'GET /api/v1/customer/auto-reply-setting',
        'PUT /api/v1/customer/auto-reply-setting',
        'GET /api/v1/customer/shops/:shopId/auto-reply-policy',
        'PUT /api/v1/customer/shops/:shopId/auto-reply-policy',
        'GET /api/v1/customer/shops/:shopId/auto-reply-runs',
        'POST /api/v1/customer/conversations/:id/send-platform-message',
      ]),
    );
  });

  it('defines ERP master data, state transition, and idempotent receipt contracts', () => {
    const endpoint = (key: string) => contracts.endpoints.find((item) => routeKey(item) === key);

    expect(endpoint('POST /api/v1/warehouses')?.requestBody).toEqual(['code', 'name', 'isDefault']);
    expect(endpoint('PUT /api/v1/warehouses/:id')?.requestBody).toEqual(['name', 'status', 'isDefault']);
    expect(endpoint('GET /api/v1/suppliers/:id/skus')?.requiredPermission).toBe('supplier.view');
    expect(endpoint('POST /api/v1/suppliers/:id/skus')?.requestBody).toEqual([
      'productSkuId',
      'supplierSkuCode',
      'unitCostMinor',
      'currency',
      'minOrderQty',
      'leadTimeDays',
    ]);
    expect(endpoint('POST /api/v1/purchase-orders')?.requestBody).toEqual([
      'idempotencyKey',
      'supplierId',
      'warehouseId',
      'currency',
      'remark',
      'items',
    ]);
    expect(endpoint('POST /api/v1/purchase-orders/:id/approve')?.requiredPermission).toBe('procurement.approve');
    expect(endpoint('POST /api/v1/purchase-orders/:id/receipts')?.requestBody).toEqual(['expectedRevision', 'idempotencyKey', 'items']);
    expect(endpoint('POST /api/v1/purchase-orders/:id/receipts')?.requiredPermission).toBe('procurement.receive');
  });

  it('defines warehouse-ledger adjustment, migration, and reconciliation contracts', () => {
    const endpoint = (key: string) => contracts.endpoints.find((item) => routeKey(item) === key);

    expect(endpoint('POST /api/v1/products/:id/skus/:skuId/adjust-stock')?.requestBody).toEqual([
      'warehouseId',
      'stock',
      'idempotencyKey',
      'reason',
      'remark',
    ]);
    expect(endpoint('POST /api/v1/products/:id/skus/:skuId/adjust-stock')?.requiredPermission).toBe('inventory.operate');
    expect(endpoint('GET /api/v1/inventory/warehouse-ledger/reconciliation')?.query).toEqual(['page', 'pageSize', 'status']);
    expect(endpoint('POST /api/v1/inventory/warehouse-ledger/migrate-legacy')?.requiredPermission).toBe('inventory.operate');
  });

  it('keeps SKU metadata writes tenant-scoped and separate from warehouse inventory', () => {
    const endpoint = (key: string) => contracts.endpoints.find((item) => routeKey(item) === key) as {
      requestBody?: string[];
      forbiddenRequestBody?: string[];
      requiredPermission?: string;
      tenantScope?: string;
    } | undefined;
    const create = endpoint('POST /api/v1/products/:id/skus');
    const update = endpoint('PUT /api/v1/products/:id/skus/:skuId');
    const stockSettings = endpoint('PUT /api/v1/products/:id/skus/:skuId/stock-settings');
    const remove = endpoint('DELETE /api/v1/products/:id/skus/:skuId');

    expect(create?.requestBody).not.toContain('stock');
    expect(update?.requestBody).not.toContain('stock');
    expect(create?.forbiddenRequestBody).toEqual(['stock']);
    expect(update?.forbiddenRequestBody).toEqual(['stock']);
    expect(stockSettings?.requestBody).toEqual(['warningStock', 'safetyStock']);
    for (const item of [create, update, stockSettings, remove]) {
      expect(item?.requiredPermission).toBe('product.write');
      expect(item?.tenantScope).toBe('current_tenant_product_or_not_found');
    }
  });

  it('defines payload/query contracts for state-changing publish APIs', () => {
    const createDraft = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft');
    const publish = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/publish');
    const readiness = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/products/:id/readiness');

    expect(createDraft?.requestBody).toEqual(['shopId', 'publishMode', 'force']);
    expect(publish?.requestBody).toEqual(['shopId', 'options', 'force']);
    expect(readiness?.query).toEqual(['platform', 'shopId', 'mode']);
  });

  it('keeps Douyin writes exclusive to approved operation tasks', () => {
    const endpoint = (key: string) => contracts.endpoints.find((item) => routeKey(item) === key) as {
      fixedError?: { httpStatus: number; dataErrorCode: string };
      douyinPolicy?: string;
    } | undefined;
    expect(endpoint('POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft')?.fixedError).toEqual({
      httpStatus: 409,
      dataErrorCode: 'DOUYIN_OPERATION_TASK_REQUIRED',
    });
    expect(endpoint('POST /api/v1/products/:id/publish')?.douyinPolicy).toBe('reject_before_task_write');
    expect(endpoint('POST /api/v1/products/:id/publish-targets/create-drafts')?.douyinPolicy).toBe('reject_entire_request_before_write');
    expect(endpoint('POST /api/v1/product-publish/batch-targets/create-drafts')?.douyinPolicy).toBe('reject_entire_request_before_idempotency_or_batch_write');
    expect(endpoint('POST /api/v1/product-publish/tasks/:id/retry')?.douyinPolicy).toBe('reject_without_task_state_change');
    expect(endpoint('POST /api/v1/product-publish/batches/:id/retry-failed')?.douyinPolicy).toBe('reject_entire_batch_without_task_state_change');
  });

  it('requires fail-closed customer auto-reply and idempotent send fields', () => {
    const setting = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/customer/auto-reply-setting');
    const policy = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/customer/shops/:shopId/auto-reply-policy');
    const send = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/customer/conversations/:id/send-platform-message');

    expect(setting?.requestBody).toEqual(['messageSyncEnabled', 'autoReplyEnabled', 'pollIntervalSeconds']);
    expect(policy?.requestBody).toEqual([
      'enabled',
      'tone',
      'shopPolicy',
      'maxReplyRunes',
      'maxRepliesPerHour',
      'requireOrderContext',
      'lowRiskOnly',
    ]);
    expect(send?.requestBody).toEqual(['reply', 'clientMessageId', 'suggestionId']);
  });

  it('defines the reviewed production draft operation task contract', () => {
    const runtimeStatus = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/p10/status');
    const create = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/operation-tasks');
    const approve = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/operation-tasks/:id/approve');
    const execute = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/operation-tasks/:id/execute');

    expect(runtimeStatus?.requiredResponseFields).toEqual(['providerWriteReady', 'productionReady']);
    expect(create?.requestBody).toEqual(['sourceType', 'sourceReference', 'taskType', 'platform', 'title', 'summary', 'payload', 'priority']);
    expect(approve?.requestBody).toEqual(['draftVersion', 'draftPayloadHash', 'reason', 'comment', 'expectedTaskRevision']);
    expect(execute?.requestBody).toEqual(['expectedTaskRevision', 'adapterMode']);
  });

  it('limits manual Douyin reconciliation to unknown results', () => {
    const recover = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/product-publish/tasks/:id/recover-douyin-draft',
    ) as {
      requestBody?: string[];
      requiredPermission?: string;
      douyinPolicy?: string;
      fixedStateError?: { httpStatus: number; dataErrorCode: string };
    } | undefined;

    expect(recover?.requestBody).toEqual([]);
    expect(recover?.requiredPermission).toBe('operationtask.execute');
    expect(recover?.douyinPolicy).toBe('read_only_reconcile_result_unknown_only');
    expect(recover?.fixedStateError).toEqual({
      httpStatus: 409,
      dataErrorCode: 'DOUYIN_RECOVERY_NOT_ALLOWED',
    });
  });

  it('defines filtered system alert queries and audited silence fields', () => {
    const overview = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/observability/overview',
    );
    const list = contracts.endpoints.find(
      (item) => routeKey(item) === 'GET /api/v1/observability/alerts',
    );
    const acknowledge = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/observability/alerts/:id/ack',
    );
    const silence = contracts.endpoints.find(
      (item) => routeKey(item) === 'POST /api/v1/observability/alerts/:id/silence',
    );

    expect(overview?.requiredResponseFields).toEqual([
      'overallStatus',
      'metrics',
      'alerts',
      'evaluation',
      'slo',
      'telemetry',
      'environment',
      'timestamp',
    ]);

    expect(list?.query).toEqual(['page', 'pageSize', 'status', 'severity', 'module']);
    expect(acknowledge?.requestBody).toEqual([]);
    expect(silence?.requestBody).toEqual(['reason', 'durationHours']);
  });

  it('marks every protected Admin endpoint as authenticated', () => {
    expect(contracts.endpoints).toHaveLength(54);
    expect(contracts.endpoints.every((endpoint) => endpoint.auth === true)).toBe(true);
  });
});
