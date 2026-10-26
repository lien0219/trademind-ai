// Keep utility types structural to avoid a forbidden utils -> services dependency.
type TenantWorkspaceCandidate = { id: number; status: string };

export const PLATFORM_WORKSPACE_PATH = '/platform/tenants' as const;
export const ENTERPRISE_WORKSPACE_PATH = '/dashboard/product-operations' as const;

// The backend decides whether an identity can enter the requested tenant.
// Client-side lists only prevent misleading navigation and never grant access.
export function canEnterTenantWorkspace(
  tenant: TenantWorkspaceCandidate,
  authorized: readonly TenantWorkspaceCandidate[],
): boolean {
  return tenant.id > 0 && tenant.status === 'active' &&
    authorized.some((member) => member.id === tenant.id && member.status === 'active');
}

export function workspaceLandingPath(currentTenantId: number, targetTenantId: number) {
  if (targetTenantId === 0) return PLATFORM_WORKSPACE_PATH;
  if (currentTenantId === 0 && targetTenantId > 0) return ENTERPRISE_WORKSPACE_PATH;
  return undefined;
}

// Root has one protected business workspace. Platform management remains a
// navigation capability, not an additional tenant-zero switcher option.
// Keep the legacy system option for non-Root platform operators.
export function visibleWorkspaceOptions<T extends { id: number }>(
  items: readonly T[],
  isRoot: boolean,
): T[] {
  return isRoot ? items.filter((item) => item.id !== 0) : [...items];
}

export function workspaceDisplayName(workspace: { code: string; name: string }): string {
  return workspace.code === 'root-workspace' ? 'Root 工作台' : workspace.name;
}
