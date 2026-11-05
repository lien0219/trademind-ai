import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { deleteAdminRole, deleteAdminUser } from '../adminUsers';

describe('user and role deletion contracts', () => {
  it.each([
    { remove: deleteAdminUser, value: 'e2e-user', path: '/api/v1/admin/users/e2e-user' },
    { remove: deleteAdminRole, value: 'r_e2e/unsafe', path: '/api/v1/admin/roles/r_e2e%2Funsafe' },
  ])('uses DELETE without a payload and unwraps the result', async ({ remove, value, path }) => {
    vi.mocked(request).mockResolvedValueOnce({ code: 0, message: 'ok', data: { deleted: true } });
    await expect(remove(value)).resolves.toEqual({ deleted: true });
    expect(request).toHaveBeenCalledWith(path, expect.objectContaining({ method: 'DELETE' }));
    expect(vi.mocked(request).mock.calls[0][1]).not.toHaveProperty('data');
  });

  it('preserves a deletion rejection for the page to display', async () => {
    vi.mocked(request).mockResolvedValueOnce({ code: 40001, message: '此角色仍有关联用户', data: null });
    await expect(deleteAdminRole('r_e2e')).rejects.toThrow('此角色仍有关联用户');
  });
});
