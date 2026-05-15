import type { SettingPutItem, SettingRow } from '@/services/settings';

export function pickGroup(items: SettingRow[] | undefined, groupKey: string): Record<string, string> {
  const out: Record<string, string> = {};
  if (!items?.length) return out;
  for (const it of items) {
    if (it.groupKey === groupKey) {
      out[it.itemKey] = it.itemValue ?? '';
    }
  }
  return out;
}

export type FieldSpec = { encrypted?: boolean };

/** Build PUT items from form values; `specs` maps itemKey -> { encrypted }. */
export function toPutItems(
  groupKey: string,
  specs: Record<string, FieldSpec>,
  values: Record<string, unknown>,
  tenantId = 0,
): SettingPutItem[] {
  return Object.keys(specs).map((itemKey) => ({
    tenantId,
    groupKey,
    itemKey,
    itemValue: values[itemKey] == null ? '' : String(values[itemKey]),
    valueType: 'string',
    isEncrypted: !!specs[itemKey]?.encrypted,
    remark: '',
  }));
}
