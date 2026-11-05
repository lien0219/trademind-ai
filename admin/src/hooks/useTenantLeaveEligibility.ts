import { useEffect, useState } from 'react';
import { getCurrentTenant } from '@/services/tenants';
import { mayLeaveTenant } from '@/utils/tenantLeave';

// The service endpoint is authoritative. The UI fails closed on stale or
// missing tenant details and rechecks whenever identity/scope changes.
export function useTenantLeaveEligibility(user?: API.CurrentUser): boolean {
  const [verified, setVerified] = useState<{ userId: string; tenantId: number; allowed: boolean } | null>(null);
  const userId = user?.id;
  const tenantId = user?.tenantId;
  const role = user?.role;
  const isRoot = user?.isRoot;
  const permissions = user?.permissions;
  const hasManage = permissions?.includes('user.manage') === true;

  useEffect(() => {
    let active = true;
    setVerified(null);
    const roleName = role?.trim().toLowerCase();
    if (!userId || tenantId == null || tenantId <= 0 || isRoot || !roleName ||
        roleName === 'admin' || roleName === 'owner' || hasManage) {
      return () => { active = false; };
    }
    void getCurrentTenant().then(({ tenant }) => {
      if (active) setVerified({ userId, tenantId, allowed: tenant.id === tenantId &&
        tenant.status === 'active' && tenant.ownerUserId !== userId });
    }).catch(() => { if (active) setVerified(null); });
    return () => { active = false; };
  }, [userId, tenantId, role, isRoot, hasManage]);

  return Boolean(userId && tenantId && verified?.userId === userId &&
    verified.tenantId === tenantId && verified.allowed && mayLeaveTenant(user, {
      id: tenantId, status: 'active',
    }));
}
