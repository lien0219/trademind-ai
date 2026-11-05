import { captureTenantContext, isTenantContextCurrent } from './tenantContext';

/**
 * Fences asynchronous read responses by both tenant identity and request order.
 * Starting a new request invalidates older responses; changing tenant, logging
 * out or invalidating on unmount prevents a late response from repainting the UI.
 *
 * This guards presentation only. Backend tenant authorization remains mandatory.
 */
export function createTenantReadGuard() {
  let generation = 0;
  return {
    begin(): () => boolean {
      const currentGeneration = ++generation;
      const context = captureTenantContext();
      return () => generation === currentGeneration && isTenantContextCurrent(context);
    },
    invalidate(): void {
      ++generation;
    },
  };
}
