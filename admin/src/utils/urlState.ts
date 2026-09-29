import { history } from '@umijs/max';
import dayjs, { type Dayjs } from 'dayjs';

export type UrlStateValue = string | number | boolean | null | undefined;
export type UrlState = Record<string, UrlStateValue>;

const ALLOWED_QUERY_KEYS = new Set([
  'page',
  'pageSize',
  'keyword',
  'status',
  'type',
  'taskType',
  'priority',
  'platform',
  'shopId',
  'warehouseId',
  'productId',
  'create',
  'tab',
  'id',
  'drawer',
  'source',
  'start',
  'end',
  'detailTaskType',
  'failureCategory',
  'severity',
  'recoveryStatus',
  'normalizedStatus',
  'includeResolved',
  'includeMarked',
  'timeRange',
  // H1.2 — orders / products / inventory / customer
  'payStatus',
  'skuStatus',
  'inventoryStatus',
  'fulfillmentStatus',
  'dateFrom',
  'dateTo',
  'createdFrom',
  'createdTo',
  'updatedFrom',
  'updatedTo',
  'exceptionType',
  'publishStatus',
  'aiStatus',
  'stockStatus',
  'syncStatus',
  'skuBindStatus',
  'productSkuId',
  'batchId',
  'alertType',
  'replyStatus',
  'aiSuggestionStatus',
  'sendStatus',
  'conversationId',
  'suggestionId',
  'productSource',
  // legacy deep links (read + write when explicitly set)
  'jumpOrder',
  'orderId',
  'itemId',
  'jumpId',
  'skuId',
  'missingAiTitle',
  'missingAiDescription',
  'readiness',
  'publishable',
  'pendingReply',
  'hasAiSuggestion',
  'sendFailed',
  'hasOrder',
  // H1.5 — secondary task lists
  'warningCode',
  'resultStatus',
  'retryable',
  'failedPagesOnly',
  'publishMode',
  'taskId',
  'cursor',
  'cursorHistory',
  'afterSequence',
  'from',
  'sourcePlatform',
  'targetShopId',
]);

export const URL_SOURCE_VALUES = new Set([
  'dashboard',
  'taskcenter',
  'order_detail',
  'inventory',
  'customer',
  'collect',
  'manual',
  'ai_workbench',
  'config_status',
  'publish_batch',
  'order_sync',
  'customer_sync',
]);

export function parsePositiveInt(value?: string, fallback = 1) {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}

const SAFE_RESOURCE_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/;

export function parseProductionCreatePrefill(state: { create?: string; productId?: string; shopId?: string }) {
  if (state.create !== 'production') return undefined;
  const productId = state.productId?.trim();
  const shopId = state.shopId?.trim();
  if (!productId || !SAFE_RESOURCE_ID_PATTERN.test(productId)) return undefined;
  if (shopId && !SAFE_RESOURCE_ID_PATTERN.test(shopId)) return undefined;
  return { productId, shopId };
}

/** Parse ISO date range from URL `start`/`end` (or legacy `dateFrom`/`dateTo`) for ProTable dateTimeRange fields. */
export function queryTimeRange(
  start?: string,
  end?: string,
  dateFrom?: string,
  dateTo?: string,
): [Dayjs, Dayjs] | undefined {
  const sRaw = start || dateFrom;
  const eRaw = end || dateTo;
  if (!sRaw || !eRaw) return undefined;
  const s = dayjs(sRaw);
  const e = dayjs(eRaw);
  if (!s.isValid() || !e.isValid()) return undefined;
  return [s, e];
}

export function normalizeSource(value?: string) {
  const v = (value || '').trim();
  if (!v || !URL_SOURCE_VALUES.has(v)) return undefined;
  return v;
}

export function isNavSourceValue(value?: string) {
  const v = (value || '').trim();
  return !!v && URL_SOURCE_VALUES.has(v);
}

/** Product-origin filter; ignores navigation `source` values like dashboard / taskcenter. */
export function resolveProductSourceFromQuery(productSource?: string, legacySource?: string) {
  if (productSource?.trim()) return productSource.trim();
  const legacy = legacySource?.trim();
  if (legacy && !isNavSourceValue(legacy)) return legacy;
  return undefined;
}

/** Collect-provider filter; ignores navigation `source` values. Prefers `sourcePlatform`. */
export function resolveCollectPlatformFromQuery(sourcePlatform?: string, legacySource?: string) {
  if (sourcePlatform?.trim()) return sourcePlatform.trim();
  const legacy = legacySource?.trim();
  if (legacy && !isNavSourceValue(legacy)) return legacy;
  return undefined;
}

function normalizeValue(value: UrlStateValue): string | undefined {
  if (value === undefined || value === null || value === '') return undefined;
  if (typeof value === 'boolean') return value ? 'true' : undefined;
  return String(value);
}

function sameValue(a: string | null, b: string | undefined) {
  return (a || undefined) === b;
}

export function readQueryState<T extends Record<string, string | undefined>>(
  search: string,
  keys: readonly (keyof T & string)[],
): T {
  const sp = new URLSearchParams(search || '');
  return keys.reduce<Record<string, string | undefined>>((acc, key) => {
    acc[key] = sp.get(key) || undefined;
    return acc;
  }, {}) as T;
}

export function writeQueryState(next: UrlState, options?: { replace?: boolean; pathname?: string }) {
  mergeQueryState(next, { replace: options?.replace, pathname: options?.pathname, resetKeys: [] });
}

export function mergeQueryState(
  next: UrlState,
  options?: { replace?: boolean; pathname?: string; resetKeys?: string[] },
) {
  const pathname = options?.pathname || history.location.pathname;
  const sp = new URLSearchParams(history.location.search || '');

  options?.resetKeys?.forEach((key) => sp.delete(key));

  let changed = false;
  Object.entries(next).forEach(([key, raw]) => {
    if (!ALLOWED_QUERY_KEYS.has(key)) return;
    const value = normalizeValue(raw);
    if (sameValue(sp.get(key), value)) return;
    changed = true;
    if (value === undefined) {
      sp.delete(key);
    } else {
      sp.set(key, value);
    }
  });

  if (!changed) return;
  const qs = sp.toString();
  const url = qs ? `${pathname}?${qs}` : pathname;
  if (options?.replace) {
    history.replace(url);
  } else {
    history.push(url);
  }
}

export function clearQueryState(keys: readonly string[], options?: { replace?: boolean; pathname?: string }) {
  const pathname = options?.pathname || history.location.pathname;
  const sp = new URLSearchParams(history.location.search || '');
  let changed = false;
  keys.forEach((key) => {
    if (sp.has(key)) {
      sp.delete(key);
      changed = true;
    }
  });
  if (!changed) return;
  const qs = sp.toString();
  const url = qs ? `${pathname}?${qs}` : pathname;
  if (options?.replace) {
    history.replace(url);
  } else {
    history.push(url);
  }
}

export function appendSourceToUrl(url: string, source = 'dashboard') {
  const normalized = normalizeSource(source) || 'dashboard';
  const [path, query = ''] = url.split('?');
  const sp = new URLSearchParams(query);
  if (!sp.has('source')) sp.set('source', normalized);
  const qs = sp.toString();
  return qs ? `${path}?${qs}` : path;
}
