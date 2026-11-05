import { tr } from '@/i18n';
import { groupPermissions, permissionLabel, ROLE_OPTIONS } from '@/constants/permissionLabels';

// Role names supplied by administrators are data. Only built-in role labels
// and the application's permission catalog are translated.
export function localizedRoleLabel(code: string, name?: string): string {
  const builtin = ROLE_OPTIONS.find((role) => role.value === (code || '').toLowerCase());
  return builtin ? tr(builtin.label) : name || tr('自定义角色');
}

export function localizedPermissionLabel(key: string): string {
  return tr(permissionLabel(key));
}

export function localizedPermissionGroups(keys: readonly string[]) {
  return groupPermissions(keys).map((group) => ({ ...group, label: tr(group.label) }));
}
