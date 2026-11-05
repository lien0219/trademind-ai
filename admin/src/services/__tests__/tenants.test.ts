import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { acceptTenantInvitation } from '../tenants';

describe('tenant invitation service', () => {
  it('posts the invitation token and returns the accepted tenant id', async () => {
    vi.mocked(request).mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { tenantId: 101 },
    });

    const result: { tenantId: number } = await acceptTenantInvitation('e2e-invitation-token');
    expect(result).toEqual({ tenantId: 101 });
    expect(request).toHaveBeenCalledWith('/api/v1/tenant/invitations/accept', {
      method: 'POST',
      data: { token: 'e2e-invitation-token' },
    });
  });
});
