import dayjs from 'dayjs';

/** 后台统一日期时间展示格式 */
export const DATETIME_FORMAT = 'YYYY-MM-DD HH:mm:ss';

export type DateTimeInput = string | number | Date | null | undefined;

/** 将 ISO / 时间戳格式化为本地可读时间；无效或空值返回 fallback（默认 —） */
export function formatDateTime(value: DateTimeInput, fallback = '—'): string {
  if (value === null || value === undefined || value === '') return fallback;
  const d = dayjs(value);
  return d.isValid() ? d.format(DATETIME_FORMAT) : String(value);
}

/** @deprecated 请使用 formatDateTime */
export const formatTs = formatDateTime;
