import { describe, expect, it } from 'vitest';
import { PERMISSIONS } from '@/utils/permission';
import {
  groupPermissions,
  permissionLabel,
  permissionModuleLabel,
  roleLabel,
} from '../permissionLabels';

describe('permission presentation', () => {
  it.each(Object.values(PERMISSIONS))('shows a Chinese label for %s', (key) => {
    const label = permissionLabel(key);
    expect(label).toMatch(/[\u4e00-\u9fff]/);
    expect(label).not.toMatch(/[a-z_]/i);
    expect(label).not.toBe('未识别权限');
    expect(permissionModuleLabel(key)).not.toBe('其他权限');
  });

  it('distinguishes permissions and retains the original keys when grouping', () => {
    const keys = Object.values(PERMISSIONS);
    expect(new Set(keys.map(permissionLabel)).size).toBe(keys.length);
    const original = [...keys];
    const groups = groupPermissions(keys);
    expect(groups.flatMap((group) => group.permissions).sort()).toEqual([...keys].sort());
    expect(groups.every((group) => /[\u4e00-\u9fff]/.test(group.label))).toBe(true);
    expect(keys).toEqual(original);
  });

  it('gives unknown keys a Chinese fallback without confusing them with known grants', () => {
    for (const key of ['future.experimental', 'constructor', 'toString', '']) {
      expect(permissionLabel(key)).toBe('未识别权限');
      expect(permissionModuleLabel(key)).toBe('其他权限');
    }
    expect(groupPermissions(['future.experimental'])[0].permissions).toEqual(['future.experimental']);
    expect(groupPermissions([])).toEqual([]);
  });

  it('maps system roles while preserving user-defined role names', () => {
    expect(roleLabel('admin', 'admin')).toBe('管理员');
    expect(roleLabel('operator', 'operator')).toBe('运营');
    expect(roleLabel('readonly', 'readonly')).toBe('只读');
    expect(roleLabel('reviewer', 'reviewer')).toBe('审核');
    expect(roleLabel('r_e2e', '财务审核')).toBe('财务审核');
    expect(roleLabel('unexpected')).toBe('自定义角色');
  });
});
