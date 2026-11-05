import { AUTH_TOKEN_KEY } from '@/constants/auth';

const SWITCH_EVENT_KEY = 'trademind_tenant_switch_event';
const TENANT_CACHE_PREFIX = 'trademind_tenant_cache:';
const TENANT_DRAFT_PREFIXES = [TENANT_CACHE_PREFIX, 'tenant:', 'publish-batch-wizard:'];
const listeners = new Set<(switching: boolean) => void>();
let switching = false;
let contextVersion = 0;
let switchChannel: BroadcastChannel | null = null;

function notify(next: boolean) {
  switching = next;
  if (typeof document !== 'undefined') {
    document.documentElement.classList.toggle('tm-tenant-switching', next);
  }
  listeners.forEach((listener) => listener(next));
}

export function isTenantSwitching() {
  return switching;
}

export type TenantContextSnapshot = {
  token: string;
  version: number;
};

export function captureTenantContext(): TenantContextSnapshot {
  return {
    token: typeof localStorage === 'undefined' ? '' : localStorage.getItem(AUTH_TOKEN_KEY) || '',
    version: contextVersion,
  };
}

export function isTenantContextCurrent(snapshot: TenantContextSnapshot) {
  const currentToken = typeof localStorage === 'undefined'
    ? ''
    : localStorage.getItem(AUTH_TOKEN_KEY) || '';
  return snapshot.version === contextVersion && snapshot.token === currentToken;
}

export function markTenantContextChanged() {
  contextVersion += 1;
}

export function isTenantSwitchRequest(url: string) {
  try {
    const parsed = new URL(url, typeof window === 'undefined' ? 'http://localhost' : window.location.origin);
    return parsed.pathname === '/api/v1/auth/tenants/switch';
  } catch {
    return url.split('?')[0] === '/api/v1/auth/tenants/switch';
  }
}
export function subscribeTenantSwitch(listener: (switching: boolean) => void) {
  listeners.add(listener);
  listener(switching);
  return () => listeners.delete(listener);
}

export function clearTenantScopedStorage() {
  if (typeof localStorage === 'undefined') return;
  const keysToClear: string[] = [];
  for (let i = 0; i < localStorage.length; i += 1) {
    const key = localStorage.key(i);
    if (key && TENANT_DRAFT_PREFIXES.some((prefix) => key.startsWith(prefix))) {
      keysToClear.push(key);
    }
  }
  keysToClear.forEach((key) => localStorage.removeItem(key));
}

type TenantSwitchSignal = {
  id: string;
  phase: 'started' | 'failed' | 'switched';
  tenantId: number;
  at: number;
};

function publishSignal(event: TenantSwitchSignal) {
  try {
    if (typeof BroadcastChannel !== 'undefined') {
      switchChannel ??= new BroadcastChannel('trademind-tenant-context');
      switchChannel.postMessage(event);
    }
  } catch {
    // BroadcastChannel may be unavailable in older embedded browsers.
  }
  try {
    localStorage.setItem(SWITCH_EVENT_KEY, JSON.stringify(event));
  } catch {
    // A storage quota failure must not prevent the current tab from reloading.
  }
}

export async function switchTenantAndReload(
  tenantId: number,
  switchTenant: (tenantId: number) => Promise<{ token: string }>,
  destination?: '/platform/tenants' | '/dashboard/product-operations',
) {
  if (switching) return;
  const id = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  markTenantContextChanged();
  notify(true);
  publishSignal({ id, tenantId, phase: 'started', at: Date.now() });
  try {
    const result = await switchTenant(tenantId);
    if (!result.token) throw new Error('租户切换凭据为空，请重新登录');
    localStorage.setItem(AUTH_TOKEN_KEY, result.token);
    markTenantContextChanged();
    clearTenantScopedStorage();
    publishSignal({ id, tenantId, phase: 'switched', at: Date.now() });
    // Crossing platform/enterprise boundaries must load a valid page for
    // the new authorization context, not reload a now-forbidden old route.
    if (destination) {
      window.location.assign(destination);
    } else {
      window.location.reload();
    }
  } catch (error) {
    markTenantContextChanged();
    notify(false);
    publishSignal({ id, tenantId, phase: 'failed', at: Date.now() });
    throw error;
  }
}

export function installTenantSwitchListener() {
  if (typeof window === 'undefined') return () => undefined;
  const seen = new Set<string>();
  const onMessage = (data: TenantSwitchSignal) => {
    const key = `${data.id}:${data.phase}`;
    if (!data.id || seen.has(key)) return;
    seen.add(key);
    if (data.phase === 'started') {
      markTenantContextChanged();
      notify(true);
    } else if (data.phase === 'failed') {
      markTenantContextChanged();
      notify(false);
    } else {
      markTenantContextChanged();
      notify(true);
      window.location.reload();
    }
  };
  const onStorage = (event: StorageEvent) => {
    if (event.key === SWITCH_EVENT_KEY && event.newValue) {
      try {
        onMessage(JSON.parse(event.newValue) as TenantSwitchSignal);
      } catch {
        // Ignore malformed cross-tab notifications.
      }
    }
  };
  const onChannelMessage = (event: MessageEvent<TenantSwitchSignal>) => onMessage(event.data);
  window.addEventListener('storage', onStorage);
  try {
    if (typeof BroadcastChannel !== 'undefined') {
      switchChannel ??= new BroadcastChannel('trademind-tenant-context');
      switchChannel.addEventListener('message', onChannelMessage);
    }
  } catch {
    // Ignore unsupported channel implementations.
  }
  return () => {
    window.removeEventListener('storage', onStorage);
    try {
      switchChannel?.removeEventListener('message', onChannelMessage);
    } catch {
      // Ignore teardown failures from browser shims.
    }
  };
}

export function resetTenantSwitchState() {
  markTenantContextChanged();
  notify(false);
  if (typeof document !== 'undefined') {
    document.documentElement.classList.remove('tm-tenant-switching');
  }
}

export function tenantScopedCacheKey(key: string) {
  return `${TENANT_CACHE_PREFIX}${key}`;
}
