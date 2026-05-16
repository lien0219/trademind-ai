import type { AppConfigSchemaDTO } from '@/services/shops';

/** Whether platform Partner / Open app settings satisfy required fields (masked secrets count as set). */
export function isDeployAppConfigComplete(
  schema: AppConfigSchemaDTO | undefined,
  values: Record<string, string> | undefined,
): boolean {
  const fields = schema?.fields;
  if (!fields?.length) return true;
  const vals = values || {};
  for (const f of fields) {
    if (!f.required) continue;
    const v = String(vals[f.name] ?? '').trim();
    if (f.sensitive) {
      if (v === '****') continue;
      if (!v) return false;
      continue;
    }
    if (!v) return false;
  }
  return true;
}
